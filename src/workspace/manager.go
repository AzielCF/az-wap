package workspace

import (
	"context"
	"os"
	"strings"
	"time"

	botengine "github.com/AzielCF/az-wap/botengine"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/workspace/application"
	channelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	messageDomain "github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/AzielCF/az-wap/workspace/infrastructure"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/sirupsen/logrus"
)

type AdapterFactory = application.AdapterFactory

// ClientResolver es una interfaz para resolver el contexto del cliente
type ClientResolver interface {
	Resolve(ctx context.Context, platformID, secondaryID string, platformType string, channelID string) (*botengineDomain.ClientContext, string, error)
}

type Manager struct {
	repo           workspaceDomain.IWorkspaceRepository
	botEngine      *botengine.Engine
	channels       *application.ChannelService
	sessions       *application.SessionOrchestrator
	processor      *application.MessageProcessor
	presence       *application.PresenceManager
	typingStore    channelDomain.TypingStore
	clientResolver ClientResolver
	monitor        monitoring.MonitoringStore
	serverID       string
	startTime      time.Time
}

func NewManager(
	repo workspaceDomain.IWorkspaceRepository,
	botEngine *botengine.Engine,
	clientResolver ClientResolver,
	typingStore channelDomain.TypingStore,
	monitor monitoring.MonitoringStore,
) *Manager {
	// Generate a unique ID for this server instance
	serverID := "azwap-" + time.Now().Format("05.000")

	m := &Manager{
		repo:           repo,
		botEngine:      botEngine,
		clientResolver: clientResolver,
		typingStore:    typingStore,
		monitor:        monitor,
		serverID:       serverID,
		startTime:      time.Now(),
	}

	// 1. Initialize Presence Manager
	// Inyectar MemoryPresenceStore en el constructor de PresenceManager
	m.presence = application.NewPresenceManager(repository.NewMemoryPresenceStore())

	// 2. Initialize Session Orchestrator
	m.sessions = application.NewSessionOrchestrator(botEngine)

	// 3. Initialize Message Processor
	m.processor = application.NewMessageProcessor(repo, m.sessions)

	// 4. Initialize Channel Service
	m.channels = application.NewChannelService(repo, botEngine, func(adapter channelDomain.ChannelAdapter) botengineDomain.Transport {
		return &infrastructure.BotTransportAdapter{Adapter: adapter}
	})

	// 5. Wire up Callbacks
	m.sessions.OnProcessFinal = m.processFinalBridge
	m.sessions.OnInactivityWarn = m.sendInactivityWarning
	m.sessions.OnCleanupFiles = m.cleanupSessionFiles
	m.sessions.OnChannelIdle = func(channelID string) {
		m.presence.CheckChannelPresence(channelID)
		m.presence.CheckChannelSocket(channelID)
	}

	m.presence.IsChannelActive = func(channelID string) bool {
		return len(m.GetActiveChats(channelID)) > 0
	}

	m.channels.OnAdapterRegistered = func(adapter channelDomain.ChannelAdapter) {
		m.presence.RegisterAdapter(adapter.ID(), adapter)
	}

	m.sessions.OnWaitIdle = func(ctx context.Context, channelID, chatID string) {
		if adapter, ok := m.channels.GetAdapter(channelID); ok {
			waitIdle := time.Duration(globalConfig.AIWaitContactIdleMs) * time.Millisecond
			if waitIdle > 0 {
				_ = adapter.WaitIdle(ctx, chatID, waitIdle)
			}
		}
	}

	// 6. Start Internal Loops
	m.StartPresenceLoop(context.Background())

	// Initialize Monitoring Hooks for Global Pool
	m.setupMonitoringHooks(msgworker.GetGlobalPool(), "primary")

	// Start Heartbeat Loop
	go m.startHeartbeat()

	logrus.Infof("[WS_MANAGER] Initialized with ServerID: %s", serverID)

	return m
}

func (m *Manager) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial report
	_ = m.monitor.ReportHeartbeat(context.Background(), m.serverID, int64(time.Since(m.startTime).Seconds()), globalConfig.AppVersion)

	for range ticker.C {
		_ = m.monitor.ReportHeartbeat(context.Background(), m.serverID, int64(time.Since(m.startTime).Seconds()), globalConfig.AppVersion)
	}
}

func (m *Manager) setupMonitoringHooks(pool *msgworker.MessageWorkerPool, poolType string) {
	pool.OnWorkerStart = func(workerID int, chatKey string) {
		_ = m.monitor.UpdateWorkerActivity(context.Background(), monitoring.WorkerActivity{
			ServerID:     m.serverID,
			WorkerID:     workerID,
			PoolType:     poolType,
			IsProcessing: true,
			ChatID:       chatKey,
			StartedAt:    time.Now(),
		})
	}

	pool.OnWorkerEnd = func(workerID int, chatKey string) {
		_ = m.monitor.UpdateWorkerActivity(context.Background(), monitoring.WorkerActivity{
			ServerID:     m.serverID,
			WorkerID:     workerID,
			PoolType:     poolType,
			IsProcessing: false,
			ChatID:       "",          // Clear chat ID
			StartedAt:    time.Time{}, // reset time
		})
		// También incrementamos el contador global
		_ = m.monitor.IncrementStat(context.Background(), "processed")
	}
}

// RegisterExternalPool permite que pools creados fuera del Manager (ej: en rest) reporten actividad
func (m *Manager) RegisterExternalPool(pool *msgworker.MessageWorkerPool, poolType string) {
	m.setupMonitoringHooks(pool, poolType)
}

func (m *Manager) RegisterFactory(chType channelDomain.ChannelType, factory AdapterFactory) {
	m.channels.RegisterFactory(chType, factory)
}

func (m *Manager) RegisterAdapter(adapter channelDomain.ChannelAdapter) {
	m.channels.RegisterAdapter(adapter, m.handleIncomingMessage)
	m.presence.RegisterAdapter(adapter.ID(), adapter)
}

func (m *Manager) UnregisterAdapter(channelID string) {
	m.channels.UnregisterAdapter(channelID)
	m.presence.UnregisterAdapter(channelID)
}

// UnregisterAndCleanup stops the adapter and deletes all persistent data (DBs, files)
func (m *Manager) UnregisterAndCleanup(channelID string) {
	m.channels.UnregisterAndCleanup(channelID)
	m.presence.UnregisterAdapter(channelID)
}

func (m *Manager) GetAdapter(channelID string) (channelDomain.ChannelAdapter, bool) {
	return m.channels.GetAdapter(channelID)
}

func (m *Manager) StartChannel(ctx context.Context, channelID string) error {
	return m.channels.StartChannel(ctx, channelID, m.handleIncomingMessage)
}

func (m *Manager) UpdateChannelConfig(channelID string, config channelDomain.ChannelConfig) {
	m.channels.UpdateChannelConfig(channelID, config)
}

func (m *Manager) handleIncomingMessage(adapter channelDomain.ChannelAdapter, msg messageDomain.IncomingMessage) {
	if msg.IsStatus || strings.HasSuffix(msg.ChatID, "@newsletter") || strings.HasSuffix(msg.ChatID, "@broadcast") {
		logrus.Debugf("[WorkspaceManager] Skipping bot processing for system/newsletter/status: %s", msg.ChatID)
		return
	}

	// Notify presence of activity
	m.presence.HandleIncomingActivity(msg.ChannelID)

	logrus.Infof("[WorkspaceManager] Processing incoming message from channel %s (Sender: %s, Text: %s)", msg.ChannelID, msg.SenderID, msg.Text)

	ctx := context.Background()
	ch, err := m.repo.GetChannel(ctx, msg.ChannelID)
	if err != nil {
		logrus.WithError(err).WithField("channel_id", msg.ChannelID).Error("[WorkspaceManager] Failed to get channel for incoming message")
		return
	}

	botID := ch.Config.BotID
	var clientCtx *botengineDomain.ClientContext

	// Resolve Global Client Context
	if m.clientResolver != nil {
		pType := string(adapter.Type()) // Use adapter type (e.g., "whatsapp")

		// Use full SenderID for resolution to support LID
		platformID := msg.SenderID

		// Normalization for WhatsApp (JID and LID)
		// Remove @s.whatsapp.net if present
		platformID = strings.TrimSuffix(platformID, "@s.whatsapp.net")

		// Remove device index suffix if present (e.g. 12345:1@lid -> 12345@lid)
		if idx := strings.Index(platformID, ":"); idx != -1 {
			atIdx := strings.Index(platformID, "@")
			if atIdx != -1 && atIdx > idx {
				platformID = platformID[:idx] + platformID[atIdx:]
			} else if atIdx == -1 {
				platformID = platformID[:idx]
			}
		}

		// JID de respaldo (original de la plataforma o número de teléfono)
		secondaryID := ""
		if pn, ok := msg.Metadata["sender_pn"].(string); ok && pn != "" {
			secondaryID = pn
		} else if jid, ok := msg.Metadata["sender_jid"].(string); ok {
			secondaryID = jid
		}

		// Normalize secondaryID too
		secondaryID = strings.TrimSuffix(secondaryID, "@s.whatsapp.net")
		if idx := strings.Index(secondaryID, ":"); idx != -1 {
			atIdx := strings.Index(secondaryID, "@")
			if atIdx != -1 && atIdx > idx {
				secondaryID = secondaryID[:idx] + secondaryID[atIdx:]
			} else if atIdx == -1 {
				secondaryID = secondaryID[:idx]
			}
		}

		logrus.WithFields(logrus.Fields{
			"platform_id":   platformID,
			"secondary_id":  secondaryID,
			"platform_type": pType,
			"channel_id":    msg.ChannelID,
			"raw_sender":    msg.SenderID,
		}).Debugf("[WorkspaceManager] Attempting to resolve global client")

		resolvedCtx, overrideBotID, err := m.clientResolver.Resolve(ctx, platformID, secondaryID, pType, msg.ChannelID)
		if err == nil && resolvedCtx != nil {
			clientCtx = resolvedCtx
			logrus.WithFields(logrus.Fields{
				"client_id":        clientCtx.ClientID,
				"is_registered":    clientCtx.IsRegistered,
				"has_subscription": clientCtx.HasSubscription,
			}).Debugf("[WorkspaceManager] Resolved client context")

			if overrideBotID != "" {
				logrus.Infof("[WorkspaceManager] Overriding BotID from subscription: %s -> %s", botID, overrideBotID)
				botID = overrideBotID
			}

			// Operational check: Allowed Bots
			if len(clientCtx.AllowedBots) > 0 {
				allowed := false
				for _, bID := range clientCtx.AllowedBots {
					if bID == botID {
						allowed = true
						break
					}
				}
				if !allowed {
					logrus.Warnf("[WorkspaceManager] Client %s (%s) is NOT allowed to use bot %s. Operational restriction applied.", clientCtx.DisplayName, clientCtx.ClientID, botID)
					// If not allowed, we stop processing or return an error message
					// For now, let's just log and block the bot interaction by returning
					return
				}
			}
		} else if err != nil {
			logrus.WithError(err).Warn("[WorkspaceManager] Client resolution failed")
		} else {
			logrus.Debug("[WorkspaceManager] No global client found for this sender")
		}
	}

	// Resolve Language: Client Priority > Channel Default
	if clientCtx != nil && clientCtx.Language != "" {
		msg.Language = clientCtx.Language
	} else if ch.Config.DefaultLanguage != "" {
		msg.Language = ch.Config.DefaultLanguage
	}

	if msg.Language == "" {
		msg.Language = "en"
	}

	if botID == "" && (ch.Config.Chatwoot == nil || !ch.Config.Chatwoot.Enabled) {
		logrus.WithField("channel_id", msg.ChannelID).Warn("[WorkspaceManager] Message ignored: No BotID assigned to channel")
		return
	}

	// Attach Client Context to Message Metadata (consumed by Processor)
	if clientCtx != nil {
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]any)
		}
		msg.Metadata["client_context"] = clientCtx
	}

	// Access Control Check - Skip if client is registered (exclusive)
	if clientCtx == nil || !clientCtx.IsRegistered {
		// For access control, use original JID or Phone if available as it's more likely to match phone-based rules
		accessIdentity := msg.SenderID
		if pn, ok := msg.Metadata["sender_pn"].(string); ok && pn != "" {
			accessIdentity = pn
		} else if jid, ok := msg.Metadata["sender_jid"].(string); ok {
			accessIdentity = jid
		}

		if !m.processor.IsAccessAllowed(ctx, ch, accessIdentity) {
			logrus.WithFields(logrus.Fields{
				"channel_id":      msg.ChannelID,
				"sender_id":       msg.SenderID,
				"access_identity": accessIdentity,
				"mode":            ch.Config.AccessMode,
			}).Warn("[WorkspaceManager] Access denied for sender")
			return
		}
	} else {
		logrus.Debugf("[WorkspaceManager] Bypassing access control for registered client: %s", clientCtx.ClientID)
	}

	// Enqueue for debouncing
	m.sessions.EnqueueDebounced(ctx, ch, msg, botID, func(chatID string, ids []string) {
		_ = adapter.MarkRead(ctx, chatID, ids)
	})
}

func (m *Manager) processFinalBridge(ctx context.Context, ch channelDomain.Channel, msg messageDomain.IncomingMessage, botID string) (botengineDomain.BotOutput, error) {
	return m.processor.ProcessFinal(ctx, ch, msg, botID, m.botEngine.Process, func(ctx context.Context, chatID string, ids []string) {
		if adapter, ok := m.channels.GetAdapter(ch.ID); ok {
			_ = adapter.MarkRead(ctx, chatID, ids)
		}
	}, m.CloseSession)
}

func (m *Manager) GetActiveSessions() []application.ActiveSessionInfo {
	return m.sessions.GetActiveSessions()
}

func (m *Manager) ClearBotMemory(botID string) {
	for _, s := range m.sessions.GetActiveSessions() {
		if entry, ok := m.sessions.GetEntry(s.Key); ok {
			if entry.BotID == botID {
				entry.Memory.History = nil
			}
		}
	}
	logrus.Infof("[WS_MANAGER] Cleared memory for bot %s across all sessions", botID)
}

func (m *Manager) ClearWorkspaceBotMemory(workspaceID, botID string) {
	for _, s := range m.sessions.GetActiveSessions() {
		if entry, ok := m.sessions.GetEntry(s.Key); ok {
			if entry.Msg.WorkspaceID == workspaceID && entry.BotID == botID {
				entry.Memory.History = nil
			}
		}
	}
	logrus.Infof("[WS_MANAGER] Cleared memory for bot %s in workspace %s", botID, workspaceID)
}

func (m *Manager) GetActiveChats(channelID string) []string {
	var chats []string
	for _, s := range m.sessions.GetActiveSessions() {
		if s.ChannelID == channelID {
			chats = append(chats, s.ChatID)
		}
	}
	return chats
}

func (m *Manager) PrepareSessionFile(workspaceID, channelID, sessionKey string, fileName string, friendlyName string, mimeType string, fileHash string) (string, error) {
	return m.processor.PrepareSessionFile(workspaceID, channelID, sessionKey, fileName, friendlyName, mimeType, fileHash)
}

func (m *Manager) IsAccessAllowed(ctx context.Context, ch channelDomain.Channel, senderID string) bool {
	return m.processor.IsAccessAllowed(ctx, ch, senderID)
}

func (m *Manager) sendInactivityWarning(key string, ch channelDomain.Channel) {
	entry, ok := m.sessions.GetEntry(key)
	if !ok || entry.State != application.StateWaiting {
		return
	}

	templates := map[string]string{
		"en": "Are you still there? I'll be finishing the session in one minute if you don't need anything else.",
		"es": "¿Sigues ahí? Cerraré la sesión en un minuto si no necesitas nada más.",
		"fr": "Êtes-vous toujours là? Je fermerai la session dans une minute si vous n'avez besoin de rien d'autre.",
	}

	cfg := ch.Config.InactivityWarning
	lang := "en"
	enabled := true

	if cfg != nil {
		enabled = cfg.Enabled
		if entry.Language != "" {
			lang = entry.Language
		} else if cfg.DefaultLang != "" {
			lang = cfg.DefaultLang
		}
		for k, v := range cfg.Templates {
			if v != "" {
				templates[k] = v
			}
		}
	}

	if !enabled {
		return
	}

	text := templates[lang]
	if text == "" {
		text = templates["en"]
	}

	// Try Personalization
	clientName := ""
	if ctxRaw, ok := entry.Msg.Metadata["client_context"]; ok {
		if clientCtx, ok := ctxRaw.(*botengineDomain.ClientContext); ok && clientCtx.SocialName != "" {
			clientName = clientCtx.SocialName
		}
	}

	if clientName != "" {
		switch lang {
		case "es":
			text = strings.Replace(text, "¿Sigues ahí?", "¿Sigues ahí, "+clientName+"?", 1)
		case "en":
			text = strings.Replace(text, "Are you still there?", "Are you still there, "+clientName+"?", 1)
		case "fr":
			text = strings.Replace(text, "Êtes-vous toujours là?", "Êtes-vous toujours là, "+clientName+"?", 1)
		}
	}

	if adapter, ok := m.channels.GetAdapter(ch.ID); ok {
		logrus.Infof("[WS_MANAGER] Sending inactivity warning to %s", entry.Msg.ChatID)
		_, _ = adapter.SendMessage(context.Background(), entry.Msg.ChatID, text, "")
	}
}

func (m *Manager) cleanupSessionFiles(e *application.SessionEntry) {
	if e.SessionPath == "" {
		return
	}
	_ = os.RemoveAll(e.SessionPath)
	logrus.Infof("[WS_MANAGER] Cleaned up session files at %s", e.SessionPath)
	e.DownloadedFiles = nil
	e.SessionPath = ""
}

func (m *Manager) CloseSession(ctx context.Context, channelID, chatID string) error {
	if adapter, ok := m.channels.GetAdapter(channelID); ok {
		_ = adapter.CloseSession(ctx, chatID)
	}

	for _, s := range m.sessions.GetActiveSessions() {
		if s.ChannelID == channelID && s.ChatID == chatID {
			m.sessions.CloseSession(s.Key)
		}
	}
	return nil
}

func (m *Manager) StartPresenceLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				adapters := m.channels.GetAdapters()
				for _, adapter := range adapters {
					m.presence.CheckChannelPresence(adapter.ID())
					m.presence.CheckChannelSocket(adapter.ID())
				}
			}
		}
	}()
}

func (m *Manager) GetChannelPresence(ctx context.Context, channelID string) (*channelDomain.ChannelPresence, error) {
	return m.presence.GetStatus(ctx, channelID)
}

func (m *Manager) UpdateTyping(ctx context.Context, channelID, chatID string, isTyping bool, media channelDomain.TypingMedia) error {
	return m.typingStore.Update(ctx, channelID, chatID, isTyping, media)
}

func (m *Manager) IsTyping(ctx context.Context, channelID, chatID string) bool {
	s, _ := m.typingStore.Get(ctx, channelID, chatID)
	return s != nil
}

func (m *Manager) GetActiveTyping(ctx context.Context) ([]channelDomain.TypingState, error) {
	return m.typingStore.GetAll(ctx)
}

func (m *Manager) WaitIdle(ctx context.Context, channelID, chatID string, timeout time.Duration) bool {
	if timeout <= 0 {
		return true
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()

	for {
		if !m.IsTyping(ctx, channelID, chatID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-poll.C:
		}
	}
}

package workspace

import (
	"context"
	"os"
	"time"

	botengine "github.com/AzielCF/az-wap/botengine"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/workspace/application"
	channelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	messageDomain "github.com/AzielCF/az-wap/workspace/domain/message"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/AzielCF/az-wap/workspace/infrastructure"
	"github.com/sirupsen/logrus"
)

type AdapterFactory = application.AdapterFactory

type Manager struct {
	repo      workspaceDomain.IWorkspaceRepository
	botEngine *botengine.Engine
	channels  *application.ChannelService
	sessions  *application.SessionOrchestrator
	processor *application.MessageProcessor
	presence  *application.PresenceManager
}

func NewManager(repo workspaceDomain.IWorkspaceRepository, botEngine *botengine.Engine) *Manager {
	m := &Manager{
		repo:      repo,
		botEngine: botEngine,
	}

	// 1. Initialize Presence Manager
	m.presence = application.NewPresenceManager()

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

	return m
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
	if msg.IsStatus {
		logrus.Debug("[WS_MANAGER] Skipping bot processing for status update")
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
	if botID == "" && (ch.Config.Chatwoot == nil || !ch.Config.Chatwoot.Enabled) {
		logrus.WithField("channel_id", msg.ChannelID).Warn("[WorkspaceManager] Message ignored: No BotID assigned to channel")
		return
	}

	// Access Control Check
	if !m.processor.IsAccessAllowed(ctx, ch, msg.SenderID) {
		logrus.WithFields(logrus.Fields{
			"channel_id": msg.ChannelID,
			"sender_id":  msg.SenderID,
			"mode":       ch.Config.AccessMode,
		}).Warn("[WorkspaceManager] Access denied for sender")
		return
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
		if cfg.DefaultLang != "" {
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

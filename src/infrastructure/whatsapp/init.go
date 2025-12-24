package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	chatwoot "github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/chatpresence"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/ui/websocket"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// --- Types & Constants ---

type contextKey string

const instanceIDKey contextKey = "instanceID"

type ExtractedMedia struct {
	MediaPath string `json:"media_path"`
	MimeType  string `json:"mime_type"`
	Caption   string `json:"caption"`
}

type WebhookConfig struct {
	URLs               []string
	Secret             string
	InsecureSkipVerify bool
}

type InstanceClient struct {
	Client *whatsmeow.Client
	DB     *sqlstore.Container
}

// --- Global State ---

var (
	// Global singleton state
	globalStateMu sync.RWMutex
	cli           *whatsmeow.Client
	db            *sqlstore.Container
	keysDB        *sqlstore.Container
	log           waLog.Logger
	historySyncID int32
	startupTime   = time.Now().Unix()

	// Multi-instance state
	instanceWebhookMu   sync.RWMutex
	instanceWebhookByID = make(map[string]WebhookConfig)

	activeInstanceMu sync.RWMutex
	activeInstanceID string

	instanceClientsMu sync.RWMutex
	instanceClients   = make(map[string]*InstanceClient)

	// Message worker pool
	msgWorkerPool *msgworker.MessageWorkerPool
	poolInitOnce  sync.Once
	poolCtx       context.Context
	poolCancel    context.CancelFunc
)

// --- Context Helpers ---

func ContextWithInstanceID(ctx context.Context, instanceID string) context.Context {
	return context.WithValue(ctx, instanceIDKey, instanceID)
}

func GetInstanceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(instanceIDKey).(string); ok {
		return v
	}
	return ""
}

func resolveInstanceIDForAI(ctx context.Context) string {
	id := strings.TrimSpace(GetInstanceIDFromContext(ctx))
	if id != "" && id != "global" {
		return id
	}
	if active := strings.TrimSpace(GetActiveInstanceID()); active != "" {
		return active
	}
	instanceClientsMu.RLock()
	defer instanceClientsMu.RUnlock()
	if len(instanceClients) == 1 {
		for k, c := range instanceClients {
			if strings.TrimSpace(k) != "" && c != nil && c.Client != nil {
				return k
			}
		}
	}
	return ""
}

// --- Initialization & Setup ---

func InitWaDB(ctx context.Context, DBURI string) *sqlstore.Container {
	log = waLog.Stdout("Main", config.WhatsappLogLevel, true)
	container, err := initDatabase(ctx, waLog.Stdout("Database", config.WhatsappLogLevel, true), DBURI)
	if err != nil {
		panic(pkgError.InternalServerError(fmt.Sprintf("Database initialization error: %v", err)))
	}
	return container
}

func initDatabase(ctx context.Context, dbLog waLog.Logger, DBURI string) (*sqlstore.Container, error) {
	if strings.HasPrefix(DBURI, "postgres:") {
		return sqlstore.New(ctx, "postgres", DBURI, dbLog)
	}
	// Default to sqlite3 (file:)
	return sqlstore.New(ctx, "sqlite3", DBURI, dbLog)
}

func InitWaCLI(ctx context.Context, storeContainer, keysStoreContainer *sqlstore.Container, repo domainChatStorage.IChatStorageRepository) *whatsmeow.Client {
	device, err := storeContainer.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}
	if device == nil {
		panic("No device found")
	}

	configureDeviceProps()

	// Configure split keys database if needed
	if keysStoreContainer != nil && device.ID != nil {
		innerStore := sqlstore.NewSQLStore(keysStoreContainer, *device.ID)
		syncKeysDevice(ctx, storeContainer, keysStoreContainer)
		device.Identities = innerStore
		device.Sessions = innerStore
		device.PreKeys = innerStore
		device.SenderKeys = innerStore
		device.MsgSecrets = innerStore
		device.PrivacyTokens = innerStore
	}

	// Initialize message worker pool once
	initMessageWorkerPool()

	client := whatsmeow.NewClient(device, newFilteredLogger(waLog.Stdout("Client", config.WhatsappLogLevel, true)))
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true
	client.AddEventHandler(func(rawEvt interface{}) { handler(ctx, rawEvt, repo) })

	globalStateMu.Lock()
	cli = client
	db = storeContainer
	keysDB = keysStoreContainer
	globalStateMu.Unlock()

	return client
}

// initMessageWorkerPool initializes the message worker pool once
func initMessageWorkerPool() {
	poolInitOnce.Do(func() {
		poolCtx, poolCancel = context.WithCancel(context.Background())
		msgWorkerPool = msgworker.NewMessageWorkerPool(config.MessageWorkerPoolSize, config.MessageWorkerQueueSize)
		msgWorkerPool.Start(poolCtx)
		logrus.Infof("[MSG_WORKER_POOL] Initialized with %d workers, queue size: %d", config.MessageWorkerPoolSize, config.MessageWorkerQueueSize)
	})
}

// StopMessageWorkerPool stops the message worker pool gracefully
func StopMessageWorkerPool() {
	if poolCancel != nil {
		poolCancel()
	}
	if msgWorkerPool != nil {
		msgWorkerPool.Stop()
	}
}

// GetMessageWorkerPoolStats returns real-time statistics from the worker pool
func GetMessageWorkerPoolStats() *msgworker.PoolStats {
	if msgWorkerPool == nil {
		return nil
	}
	stats := msgWorkerPool.GetStats()
	return &stats
}

func GetOrInitInstanceClient(ctx context.Context, instanceID string, repo domainChatStorage.IChatStorageRepository) (*whatsmeow.Client, *sqlstore.Container, error) {
	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return nil, nil, pkgError.ValidationError("instanceID: cannot be blank.")
	}

	if client, db := GetInstanceClientAndDB(trimmed); client != nil {
		setActiveInstance(trimmed)
		return client, db, nil
	}

	// Initialize new instance
	dbURI := fmt.Sprintf("file:%s/whatsapp-%s.db?_foreign_keys=on", config.PathStorages, trimmed)
	logrus.Infof("[INSTANCE] Creating new WhatsApp client for instance %s", trimmed)

	instDB, err := initDatabase(ctx, waLog.Stdout(fmt.Sprintf("DB-%s", trimmed[:8]), config.WhatsappLogLevel, true), dbURI)
	if err != nil {
		return nil, nil, pkgError.InternalServerError(fmt.Sprintf("failed to init instance DB: %v", err))
	}

	device, err := instDB.GetFirstDevice(ctx)
	if err != nil {
		return nil, nil, pkgError.InternalServerError(fmt.Sprintf("failed to get device: %v", err))
	}

	if device == nil {
		logrus.Warnf("[INSTANCE] No device found for %s (needs login)", trimmed)
		SetInstanceClient(trimmed, nil, instDB)
		setActiveInstance(trimmed)
		return nil, instDB, nil
	}

	configureDeviceProps()
	instCli := whatsmeow.NewClient(device, waLog.Stdout(fmt.Sprintf("Client-%s", trimmed[:8]), config.WhatsappLogLevel, true))
	instCli.EnableAutoReconnect = true
	instCli.AutoTrustIdentity = true

	// Handler with context capturing
	capturedID := trimmed
	instCli.AddEventHandler(func(rawEvt interface{}) {
		handler(ContextWithInstanceID(context.Background(), capturedID), rawEvt, repo)
	})

	SetInstanceClient(trimmed, instCli, instDB)
	setActiveInstance(trimmed)
	logrus.Infof("[INSTANCE] Created WhatsApp client for instance %s", trimmed)

	return instCli, instDB, nil
}

// --- Client & State Management ---

func UpdateGlobalClient(newCli *whatsmeow.Client, newDB *sqlstore.Container) {
	globalStateMu.Lock()
	cli = newCli
	db = newDB
	globalStateMu.Unlock()
	log.Infof("Global WhatsApp client updated successfully")
}

func GetClient() *whatsmeow.Client {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return cli
}

func GetDB() *sqlstore.Container {
	globalStateMu.RLock()
	defer globalStateMu.RUnlock()
	return db
}

func GetInstanceClientAndDB(instanceID string) (*whatsmeow.Client, *sqlstore.Container) {
	instanceClientsMu.RLock()
	defer instanceClientsMu.RUnlock()
	if ic, ok := instanceClients[instanceID]; ok {
		return ic.Client, ic.DB
	}
	return nil, nil
}

func GetInstanceClient(instanceID string) *whatsmeow.Client {
	c, _ := GetInstanceClientAndDB(instanceID)
	return c
}

func GetInstanceDB(instanceID string) *sqlstore.Container {
	_, d := GetInstanceClientAndDB(instanceID)
	return d
}

func SetInstanceClient(instanceID string, client *whatsmeow.Client, instDB *sqlstore.Container) {
	instanceClientsMu.Lock()
	defer instanceClientsMu.Unlock()

	if existing, ok := instanceClients[instanceID]; ok && existing != nil && existing.Client != nil && existing.Client != client {
		oldDev := ""
		newDev := ""
		if existing.Client.Store != nil && existing.Client.Store.ID != nil {
			oldDev = existing.Client.Store.ID.String()
		}
		if client != nil && client.Store != nil && client.Store.ID != nil {
			newDev = client.Store.ID.String()
		}
		logrus.WithFields(logrus.Fields{
			"instance_id": instanceID,
			"old_device":  oldDev,
			"new_device":  newDev,
		}).Warn("[INSTANCE] Replacing existing WhatsApp client for instance; disconnecting previous client")
		existing.Client.Disconnect()
	}

	instanceClients[instanceID] = &InstanceClient{Client: client, DB: instDB}
}

func GetActiveInstanceID() string {
	activeInstanceMu.RLock()
	defer activeInstanceMu.RUnlock()
	return activeInstanceID
}

func setActiveInstance(id string) {
	activeInstanceMu.Lock()
	activeInstanceID = id
	activeInstanceMu.Unlock()
}

func GetConnectionStatus() (bool, bool, string) {
	client := GetClient()
	if client == nil {
		return false, false, ""
	}
	deviceID := ""
	if client.Store != nil && client.Store.ID != nil {
		deviceID = client.Store.ID.String()
	}
	return client.IsConnected(), client.IsLoggedIn(), deviceID
}

func GetInstanceConnectionStatus(instanceID string) (bool, bool) {
	if c := GetInstanceClient(instanceID); c != nil {
		return c.IsConnected(), c.IsLoggedIn()
	}
	return false, false
}

// getClientForContext returns the correct client based on the context (Instance vs Global)
func getClientForContext(ctx context.Context) *whatsmeow.Client {
	// 1. Check if Context has Instance ID
	if id := GetInstanceIDFromContext(ctx); id != "" {
		if c := GetInstanceClient(id); c != nil {
			return c
		}
		logrus.WithField("instance_id", id).Warn("[INSTANCE] No client found for context instance; falling back to global client")
	}
	// 2. Fallback to Global Client
	return GetClient()
}

// --- Webhooks ---

func SetInstanceWebhookConfig(instanceID string, urls []string, secret string, insecure bool) {
	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return
	}
	cleanURLs := make([]string, 0)
	for _, u := range urls {
		if v := strings.TrimSpace(u); v != "" {
			cleanURLs = append(cleanURLs, v)
		}
	}
	instanceWebhookMu.Lock()
	instanceWebhookByID[trimmed] = WebhookConfig{URLs: cleanURLs, Secret: secret, InsecureSkipVerify: insecure}
	instanceWebhookMu.Unlock()
}

func getWebhookConfigForContext(ctx context.Context) WebhookConfig {
	// 1. Global config priority
	if len(config.WhatsappWebhook) > 0 || config.WhatsappWebhookSecret != "" {
		return WebhookConfig{URLs: config.WhatsappWebhook, Secret: config.WhatsappWebhookSecret, InsecureSkipVerify: config.WhatsappWebhookInsecureSkipVerify}
	}

	instanceWebhookMu.RLock()
	defer instanceWebhookMu.RUnlock()

	// 2. Context Instance
	if id := GetInstanceIDFromContext(ctx); id != "" {
		if cfg, ok := instanceWebhookByID[id]; ok {
			return cfg
		}
	}
	// 3. Active Instance
	if active := GetActiveInstanceID(); active != "" {
		if cfg, ok := instanceWebhookByID[active]; ok {
			return cfg
		}
	}
	// 4. Fallback if single instance exists
	if len(instanceWebhookByID) == 1 {
		for _, cfg := range instanceWebhookByID {
			return cfg
		}
	}
	return WebhookConfig{}
}

// --- Cleanup & Helpers ---

func configureDeviceProps() {
	osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
	store.DeviceProps.PlatformType = &config.AppPlatform
	store.DeviceProps.Os = &osName
}

func syncKeysDevice(ctx context.Context, db, keysDB *sqlstore.Container) {
	if keysDB == nil {
		return
	}
	dev, err := db.GetFirstDevice(ctx)
	if err != nil {
		return
	}
	if dev == nil {
		return
	}

	devs, err := keysDB.GetAllDevices(ctx)
	if err != nil {
		return
	}
	found := false
	for _, d := range devs {
		if d.ID == dev.ID {
			found = true
		} else {
			keysDB.DeleteDevice(ctx, d)
		}
	}
	if !found {
		keysDB.PutDevice(ctx, dev)
	}
}

func CleanupDatabase() error {
	globalStateMu.RLock()
	currentDB, currentKeysDB := db, keysDB
	globalStateMu.RUnlock()

	// Postgres Cleanup
	if strings.HasPrefix(config.DBURI, "postgres:") {
		logrus.Info("[CLEANUP] Postgres: deleting all devices")
		if currentDB != nil {
			devices, _ := currentDB.GetAllDevices(context.Background())
			for _, d := range devices {
				if err := currentDB.DeleteDevice(context.Background(), d); err != nil {
					return err
				}
			}
		}
		if currentKeysDB != nil && currentKeysDB != currentDB {
			devices, _ := currentKeysDB.GetAllDevices(context.Background())
			for _, d := range devices {
				if err := currentKeysDB.DeleteDevice(context.Background(), d); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// SQLite Cleanup
	logrus.Info("[CLEANUP] SQLite: closing and removing files")
	if currentDB != nil {
		currentDB.Close()
	}
	if currentKeysDB != nil && currentKeysDB != currentDB {
		currentKeysDB.Close()
		removeFileIfExists(config.DBKeysURI)
	}
	removeFileIfExists(config.DBURI)
	return nil
}

func removeFileIfExists(uri string) {
	uri = strings.TrimPrefix(uri, "file:")
	path := strings.Split(uri, "?")[0]
	if path == "" || strings.HasPrefix(path, ":memory:") {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logrus.Errorf("[CLEANUP] Failed to remove %s: %v", path, err)
	}
}

func CleanupTemporaryFiles() error {
	removeGlob := func(pattern string, desc string) {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			if strings.Contains(f, ".gitignore") {
				continue
			}
			os.Remove(f)
		}
		logrus.Infof("[CLEANUP] %s cleaned up", desc)
	}

	removeGlob(fmt.Sprintf("./%s/history-*", config.PathStorages), "History files")
	removeGlob(fmt.Sprintf("./%s/scan-*", config.PathQrCode), "QR images")
	removeGlob(fmt.Sprintf("./%s/*", config.PathSendItems), "Send items")
	return nil
}

func CleanupInstanceSession(_ context.Context, instanceID string, _ domainChatStorage.IChatStorageRepository) error {
	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return nil
	}

	instanceClientsMu.Lock()
	ic := instanceClients[trimmed]
	delete(instanceClients, trimmed)
	instanceClientsMu.Unlock()

	if ic != nil && ic.Client != nil {
		ic.Client.Disconnect()
	}
	if ic != nil && ic.DB != nil {
		ic.DB.Close()
	}

	instanceWebhookMu.Lock()
	delete(instanceWebhookByID, trimmed)
	instanceWebhookMu.Unlock()

	dbURI := fmt.Sprintf("file:%s/whatsapp-%s.db", config.PathStorages, trimmed)
	removeFileIfExists(dbURI)
	removeFileIfExists(dbURI + "-wal")
	removeFileIfExists(dbURI + "-shm")

	activeInstanceMu.Lock()
	if activeInstanceID == trimmed {
		activeInstanceID = ""
	}
	activeInstanceMu.Unlock()
	return nil
}

func PerformCleanupAndUpdateGlobals(ctx context.Context, logPrefix string, repo domainChatStorage.IChatStorageRepository) (*sqlstore.Container, *whatsmeow.Client, error) {
	logrus.Infof("[%s] Starting cleanup...", logPrefix)
	if c := GetClient(); c != nil {
		c.Disconnect()
	}
	if repo != nil {
		repo.TruncateAllDataWithLogging(logPrefix)
	}
	if err := CleanupDatabase(); err != nil {
		return nil, nil, err
	}
	CleanupTemporaryFiles()

	// Reinit
	newDB := InitWaDB(ctx, config.DBURI)
	var newKeysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		newKeysDB = InitWaDB(ctx, config.DBKeysURI)
	}
	newCli := InitWaCLI(ctx, newDB, newKeysDB, repo)
	UpdateGlobalClient(newCli, newDB)

	logrus.Infof("[%s] Cleanup finished, ready for login.", logPrefix)
	return newDB, newCli, nil
}

func handleRemoteLogout(ctx context.Context, repo domainChatStorage.IChatStorageRepository) {
	logrus.Info("[REMOTE_LOGOUT] User logged out, cleaning up...")
	PerformCleanupAndUpdateGlobals(ctx, "REMOTE_LOGOUT", repo)
}

// --- Event Handlers ---

func handler(ctx context.Context, rawEvt any, repo domainChatStorage.IChatStorageRepository) {
	switch evt := rawEvt.(type) {
	case *events.DeleteForMe:
		handleDeleteForMe(ctx, evt, repo)
	case *events.ChatPresence:
		instanceID := strings.TrimSpace(GetInstanceIDFromContext(ctx))
		if (instanceID == "" || instanceID == "global") && evt != nil {
			instanceID = resolveInstanceIDForAI(ctx)
		}
		if instanceID == "" || instanceID == "global" || evt == nil {
			return
		}
		if evt.IsFromMe {
			return
		}
		chatJID := strings.TrimSpace(evt.Chat.String())
		if chatJID == "" || utils.IsGroupJID(chatJID) || strings.HasPrefix(chatJID, "status@") || strings.HasSuffix(chatJID, "@broadcast") {
			return
		}
		chatpresence.Update(instanceID, chatJID, evt.State, evt.Media)
	case *events.AppStateSyncComplete:
		if cli := getClientForContext(ctx); cli != nil && len(cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			cli.SendPresence(context.Background(), types.PresenceAvailable)
		}
	case *events.PairSuccess:
		websocket.Broadcast <- websocket.BroadcastMessage{Code: "LOGIN_SUCCESS", Message: fmt.Sprintf("Successfully pair with %s", evt.ID.String())}
		globalStateMu.RLock() // Safe read global vars
		gDB, gKDB := db, keysDB
		globalStateMu.RUnlock()
		syncKeysDevice(ctx, gDB, gKDB)
	case *events.LoggedOut:
		if instID := strings.TrimSpace(GetInstanceIDFromContext(ctx)); instID != "" {
			CleanupInstanceSession(ctx, instID, repo)
		} else {
			handleRemoteLogout(ctx, repo)
		}
		websocket.Broadcast <- websocket.BroadcastMessage{Code: "LOGOUT_COMPLETE", Message: "Remote logout cleanup completed"}
	case *events.Connected, *events.PushNameSetting:
		if cli := getClientForContext(ctx); cli != nil && len(cli.Store.PushName) > 0 {
			cli.SendPresence(context.Background(), types.PresenceAvailable)
		}
	case *events.StreamReplaced:
		os.Exit(0)
	case *events.Message:
		handleMessage(ctx, evt, repo)
	case *events.Receipt:
		handleReceipt(ctx, evt)
	case *events.Presence:
		status := "online"
		if evt.Unavailable {
			status = "offline"
		}
		log.Infof("%s is now %s", evt.From, status)
	case *events.HistorySync:
		handleHistorySync(ctx, evt, repo)
	case *events.AppState:
		log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
	case *events.GroupInfo:
		handleGroupInfo(ctx, evt)
	}
}

func handleDeleteForMe(ctx context.Context, evt *events.DeleteForMe, repo domainChatStorage.IChatStorageRepository) {
	log.Infof("Deleted message %s for %s", evt.MessageID, evt.SenderJID.String())
	msg, _ := repo.GetMessageByID(evt.MessageID)
	if msg == nil {
		return
	}
	repo.DeleteMessage(evt.MessageID, msg.ChatJID)
	if len(config.WhatsappWebhook) > 0 {
		go forwardDeleteToWebhook(ctx, evt, msg)
	}
}

func handleMessage(ctx context.Context, evt *events.Message, repo domainChatStorage.IChatStorageRepository) {
	log.Infof("Msg %s from %s: type=%s", evt.Info.ID, evt.Info.SourceString(), evt.Info.Type)

	// Captura opcional del evento de mensaje para debugging/tests.
	flag := viper.GetString("capture_whatsapp_events")
	if flag == "" {
		flag = viper.GetString("CAPTURE_WHATSAPP_EVENTS")
	}
	if flag == "1" {
		payload := map[string]interface{}{
			"id":             evt.Info.ID,
			"from":           evt.Info.Sender.String(),
			"chat":           evt.Info.Chat.String(),
			"type":           evt.Info.Type,
			"push_name":      evt.Info.PushName,
			"source":         evt.Info.SourceString(),
			"is_from_me":     evt.Info.IsFromMe,
			"is_broadcast":   evt.Info.IsIncomingBroadcast(),
			"normalized_jid": utils.FormatJID(evt.Info.Sender.String()).String(),
			"text":           utils.ExtractMessageTextFromProto(evt.Message),
		}
		if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
			logrus.Info("[WHATSAPP_CAPTURE] message event")
			logrus.Info(string(data))
		}
	}

	if err := repo.CreateMessage(ctx, evt); err != nil {
		log.Errorf("Failed to store message %s: %v", evt.Info.ID, err)
	}

	// Image download
	if config.WhatsappAutoDownloadMedia {
		if img := evt.Message.GetImageMessage(); img != nil {
			if client := getClientForContext(ctx); client != nil {
				if path, err := utils.ExtractMedia(ctx, client, config.PathStorages, img); err == nil {
					log.Infof("Image downloaded to %s", path)
				}
			}
		}
	}

	// Auto-read
	if config.WhatsappAutoMarkRead && !evt.Info.IsFromMe {
		if client := getClientForContext(ctx); client != nil {
			client.MarkRead(context.Background(), []types.MessageID{evt.Info.ID}, time.Now(), evt.Info.Chat, evt.Info.Sender)
		}
	}

	// Dispatch ALL message processing to worker pool for controlled concurrency
	if msgWorkerPool != nil {
		chatJID := evt.Info.Chat.String()
		instanceID := resolveInstanceIDForAI(ctx)
		if instanceID == "" {
			instanceID = "global"
		}

		capturedInstanceID := instanceID
		msgWorkerPool.Dispatch(msgworker.MessageJob{
			InstanceID: capturedInstanceID,
			ChatJID:    chatJID,
			Handler: func(workerCtx context.Context) error {
				jobCtx := ContextWithInstanceID(workerCtx, capturedInstanceID)
				aiInstanceID := capturedInstanceID
				if aiInstanceID != "" && aiInstanceID != "global" {
					phone := NormalizePhoneForChatwoot(jobCtx, evt)
					enqueueAIReplyDebounced(workerCtx, aiInstanceID, phone, evt)
				}

				// 2. AutoReply
				handleAutoReply(jobCtx, evt, repo)

				// 3. Webhook forwarding (includes Chatwoot)
				handleWebhookForward(jobCtx, evt)

				return nil
			},
		})
	} else {
		// Fallback to old behavior if pool not initialized
		client := getClientForContext(ctx)
		if client != nil {
			if instanceID := resolveInstanceIDForAI(ctx); instanceID != "" && instanceID != "global" {
				phone := NormalizePhoneForChatwoot(ctx, evt)
				enqueueAIReplyDebounced(ctx, instanceID, phone, evt)
			}
		}
		handleAutoReply(ctx, evt, repo)
		handleWebhookForward(ctx, evt)
	}
}

func handleAutoReply(ctx context.Context, evt *events.Message, repo domainChatStorage.IChatStorageRepository) {
	autoReply := strings.TrimSpace(config.WhatsappAutoReplyMessage)
	autoReply = strings.Trim(autoReply, "\"'")
	if autoReply == "" || strings.EqualFold(autoReply, "Auto reply message") || utils.IsGroupJID(evt.Info.Chat.String()) || evt.Info.IsIncomingBroadcast() || evt.Info.IsFromMe || evt.Info.Chat.Server != types.DefaultUserServer {
		return
	}
	// Filters
	src := evt.Info.SourceString()
	if strings.Contains(src, "broadcast") || strings.HasSuffix(evt.Info.Chat.String(), "@broadcast") || strings.HasPrefix(evt.Info.Chat.String(), "status@") {
		return
	}

	// Unwrap FutureProof messages to check for text
	innerMsg := evt.Message
	unwrap := func(m *waE2E.Message) *waE2E.Message {
		if v := m.GetViewOnceMessage(); v != nil {
			return v.GetMessage()
		}
		if v := m.GetEphemeralMessage(); v != nil {
			return v.GetMessage()
		}
		if v := m.GetViewOnceMessageV2(); v != nil {
			return v.GetMessage()
		}
		if v := m.GetViewOnceMessageV2Extension(); v != nil {
			return v.GetMessage()
		}
		return nil
	}
	for i := 0; i < 3; i++ {
		if next := unwrap(innerMsg); next != nil {
			innerMsg = next
		} else {
			break
		}
	}

	hasText := innerMsg.GetConversation() != "" || (innerMsg.GetExtendedTextMessage() != nil && innerMsg.GetExtendedTextMessage().GetText() != "")
	if !hasText && innerMsg.GetProtocolMessage() != nil && innerMsg.GetProtocolMessage().GetEditedMessage() != nil {
		ed := innerMsg.GetProtocolMessage().GetEditedMessage()
		hasText = ed.GetConversation() != "" || (ed.GetExtendedTextMessage() != nil && ed.GetExtendedTextMessage().GetText() != "")
	}

	if !hasText {
		return
	}

	client := getClientForContext(ctx)
	if client == nil {
		return
	}
	recipientJID := utils.FormatJID(evt.Info.Sender.String())
	resp, err := client.SendMessage(ctx, recipientJID, &waE2E.Message{Conversation: proto.String(autoReply)})
	if err != nil {
		log.Errorf("Auto-reply fail: %v", err)
		return
	}

	if repo != nil && client.Store.ID != nil {
		repo.StoreSentMessageWithContext(ctx, resp.ID, client.Store.ID.String(), recipientJID.String(), autoReply, resp.Timestamp)
	}
}

func handleWebhookForward(ctx context.Context, evt *events.Message) {
	if pm := evt.Message.GetProtocolMessage(); pm != nil && pm.GetType().String() == "EPHEMERAL_SYNC_RESPONSE" {
		return
	}
	if strings.Contains(evt.Info.SourceString(), "broadcast") {
		return
	}

	if cfg := getWebhookConfigForContext(ctx); len(cfg.URLs) > 0 {
		go func() {
			if err := forwardMessageToWebhook(ctx, evt); err != nil {
				logrus.Error("Webhook forward fail: ", err)
			}
		}()
	}

	if instanceID := GetInstanceIDFromContext(ctx); instanceID != "" {
		if phone := NormalizePhoneForChatwoot(ctx, evt); phone != "" {
			go chatwoot.ForwardWhatsAppMessage(ctx, instanceID, phone, evt)
		}
	}
}

func handleReceipt(ctx context.Context, evt *events.Receipt) {
	if (evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf || evt.Type == types.ReceiptTypeDelivered) && len(config.WhatsappWebhook) > 0 {
		log.Infof("Receipt %s for %v", evt.Type, evt.MessageIDs)
		go forwardReceiptToWebhook(ctx, evt)
	}
}

func handleHistorySync(ctx context.Context, evt *events.HistorySync, repo domainChatStorage.IChatStorageRepository) {
	client := getClientForContext(ctx)
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return
	}
	id := atomic.AddInt32(&historySyncID, 1)
	fname := fmt.Sprintf("%s/history-%d-%s-%d-%s.json", config.PathStorages, startupTime, client.Store.ID.String(), id, evt.Data.SyncType)

	if f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE, 0600); err == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.Encode(evt.Data)
		f.Close()
		log.Infof("Wrote history sync: %s", fname)
	}

	if repo == nil || evt.Data == nil {
		return
	}

	switch evt.Data.GetSyncType() {
	case waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT:
		for _, conv := range evt.Data.GetConversations() {
			processConversation(conv, repo, client)
		}
	case waHistorySync.HistorySync_PUSH_NAME:
		for _, pn := range evt.Data.GetPushnames() {
			if chat, _ := repo.GetChat(pn.GetID()); chat != nil && chat.Name != pn.GetPushname() {
				chat.Name = pn.GetPushname()
				repo.StoreChat(chat)
			}
		}
	}
}
func processConversation(conv *waHistorySync.Conversation, repo domainChatStorage.IChatStorageRepository, client *whatsmeow.Client) {
	chatJID := conv.GetID()
	if chatJID == "" {
		return
	}
	jid, _ := types.ParseJID(chatJID)

	chatName := repo.GetChatNameWithPushName(jid, chatJID, "", conv.GetDisplayName())
	var batch []*domainChatStorage.Message
	var lastTime time.Time

	for _, hm := range conv.GetMessages() {
		if hm == nil || hm.Message == nil {
			continue
		}
		msg := hm.Message
		key := msg.GetKey()
		if key == nil || key.GetID() == "" {
			continue
		}

		content := utils.ExtractMessageTextFromProto(msg.GetMessage())
		mType, fname, url, mKey, sha, encSha, fLen := utils.ExtractMediaInfo(msg.GetMessage())
		if content == "" && mType == "" {
			continue
		}

		sender := ""
		if key.GetFromMe() {
			if client.Store.ID != nil {
				sender = client.Store.ID.String()
			}
		} else {
			if p := key.GetParticipant(); p != "" {
				if sJID, err := types.ParseJID(p); err == nil {
					sender = sJID.String()
				} else {
					sender = p
				}
			} else {
				sender = jid.String()
			}
		}
		if sender == "" {
			continue
		}

		ts := time.Unix(int64(msg.GetMessageTimestamp()), 0)
		if ts.After(lastTime) {
			lastTime = ts
		}

		batch = append(batch, &domainChatStorage.Message{
			ID: key.GetID(), ChatJID: chatJID, Sender: sender, Content: content, Timestamp: ts,
			IsFromMe: key.GetFromMe(), MediaType: mType, Filename: fname, URL: url,
			MediaKey: mKey, FileSHA256: sha, FileEncSHA256: encSha, FileLength: fLen,
		})
	}

	if len(batch) > 0 {
		repo.StoreChat(&domainChatStorage.Chat{JID: chatJID, Name: chatName, LastMessageTime: lastTime, EphemeralExpiration: conv.GetEphemeralExpiration()})
		repo.StoreMessagesBatch(batch)
	}
}

func handleGroupInfo(ctx context.Context, evt *events.GroupInfo) {
	if len(evt.Join)+len(evt.Leave)+len(evt.Promote)+len(evt.Demote) == 0 && evt.Name == nil && evt.Topic == nil && evt.Locked == nil && evt.Announce == nil {
		return
	}
	if len(config.WhatsappWebhook) > 0 {
		go forwardGroupInfoToWebhook(ctx, evt)
	}
}

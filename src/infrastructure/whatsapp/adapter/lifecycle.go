package adapter

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/AzielCF/az-wap/pkg/chatpresence"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Status returns the connection status
func (wa *WhatsAppAdapter) Status() channel.ChannelStatus {
	if wa.client == nil {
		return channel.ChannelStatusDisconnected
	}

	wa.hibMu.RLock()
	isHib := wa.hibernating
	wa.hibMu.RUnlock()

	if !wa.client.IsConnected() {
		if isHib {
			return channel.ChannelStatusHibernating
		}
		return channel.ChannelStatusDisconnected
	}
	if !wa.client.IsLoggedIn() {
		// If DB session is missing or invalid, it's disconnected
		return channel.ChannelStatusDisconnected
	}
	return channel.ChannelStatusConnected
}

// IsLoggedIn returns true if the client is authenticated
func (wa *WhatsAppAdapter) IsLoggedIn() bool {
	if wa.client == nil {
		return false
	}
	return wa.client.IsLoggedIn()
}

// Start ensures the client is connected
func (wa *WhatsAppAdapter) Start(ctx context.Context, config channel.ChannelConfig) error {
	wa.config = config

	// 1. Si ya tenemos cliente, solo aseguramos conexión
	if wa.client != nil {
		if !wa.client.IsConnected() {
			return wa.client.Connect()
		}
		return nil
	}

	// 2. Si NO tenemos cliente, inicializamos uno nuevo (Modo Autónomo / Workspace Nativo)
	// Usar siempre el ID del canal para consistencia entre whatsapp-{id}.db y chat-{id}.db
	dbKey := wa.channelID

	// Ensure storage directory exists
	if err := os.MkdirAll("storages", 0755); err != nil {
		return fmt.Errorf("failed to create storage dir: %w", err)
	}

	dbPath := fmt.Sprintf("storages/whatsapp-%s.db?_foreign_keys=on", dbKey)
	dbLog := waLog.Stdout("DB-"+dbKey[:8], "INFO", true)

	container, err := sqlstore.New(ctx, "sqlite3", "file:"+dbPath, dbLog)
	if err != nil {
		return fmt.Errorf("failed to init channel db: %w", err)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	// Si es login nuevo
	if device == nil {
		device = container.NewDevice()
	}

	// Configurar props del dispositivo desde variables de entorno
	// PlatformType: 1=Chrome
	chromePlatform := waCompanionReg.DeviceProps_CHROME

	// Check for platform override
	if pType := os.Getenv("APP_PLATFORM_TYPE"); pType != "" {
		// Logic to parse enum could go here, but for now stick to Chrome as base
	}

	osName := os.Getenv("APP_OS")
	if osName == "" {
		osName = "Linux"
	}

	store.DeviceProps.PlatformType = &chromePlatform
	store.DeviceProps.Os = &osName

	clientLog := waLog.Stdout("Client-"+wa.channelID[:8], "INFO", true)
	wa.client = whatsmeow.NewClient(device, clientLog)
	wa.client.EnableAutoReconnect = config.AutoReconnect
	wa.client.AutoTrustIdentity = true

	// Registrar handlers internos
	wa.handlerID = wa.client.AddEventHandler(wa.handleEvent)

	if err := wa.client.Connect(); err != nil {
		return fmt.Errorf("failed to connect new client: %w", err)
	}

	// 6. Start Hibernation Polling Sync (Every 15 minutes)
	go wa.startHibernationSync()

	return nil
}

// SetOnline sets the global presence (Available/Unavailable) without closing the socket
func (wa *WhatsAppAdapter) SetOnline(ctx context.Context, online bool) error {
	if wa.client == nil {
		return nil
	}

	// Wait briefly if we just connected
	if !wa.client.IsConnected() {
		logrus.Debugf("[WHATSAPP] SetOnline(%v) skipped: client not connected for %s", online, wa.channelID)
		return nil
	}

	presence := types.PresenceAvailable
	if !online {
		presence = types.PresenceUnavailable
	}

	logrus.Infof("[WHATSAPP] Setting visual presence to %v for channel %s", online, wa.channelID)
	err := wa.client.SendPresence(ctx, presence)
	if err != nil {
		logrus.WithError(err).Errorf("[WHATSAPP] Failed to set visual presence for channel %s", wa.channelID)
	}
	return err
}

// Stop removes the event handler
func (wa *WhatsAppAdapter) Stop(ctx context.Context) error {
	if wa.client != nil && wa.handlerID != 0 {
		wa.client.RemoveEventHandler(wa.handlerID)
		wa.handlerID = 0
	}
	// Stop polling loop
	if wa.stopSync != nil {
		select {
		case wa.stopSync <- struct{}{}:
		default:
		}
	}
	return nil
}

func (wa *WhatsAppAdapter) startHibernationSync() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wa.hibMu.RLock()
			isHib := wa.hibernating
			wa.hibMu.RUnlock()

			if isHib {
				logrus.Infof("[WHATSAPP] Periodic Sync Wakeup for channel %s", wa.channelID)
				_ = wa.Resume(context.Background())

				// Wait 60s to receive anything pending (offline messages)
				time.Sleep(1 * time.Minute)

				// After sync, only go back to sleep if there is NO active chats
				// AND NO recent activity resumed by HandleIncomingActivity
				wa.hibMu.RLock()
				stillIdle := !wa.hibernating // if it was resumed by activity, hibernating will be false
				wa.hibMu.RUnlock()

				if stillIdle && wa.manager != nil && len(wa.manager.GetActiveChats(wa.channelID)) == 0 {
					logrus.Infof("[WHATSAPP] Periodic Sync Done. Returning to hibernation for channel %s", wa.channelID)
					_ = wa.Hibernate(context.Background())
				} else {
					logrus.Infof("[WHATSAPP] Periodic Sync detected activity or manual resume. Staying ONLINE for channel %s", wa.channelID)
				}
			}
		case <-wa.stopSync:
			logrus.Infof("[WHATSAPP] Polling sync stopped for channel %s", wa.channelID)
			return
		}
	}
}

func (wa *WhatsAppAdapter) WaitIdle(ctx context.Context, chatID string, duration time.Duration) error {
	logrus.Debugf("[WHATSAPP] Waiting for idle in chat %s (timeout: %v)", chatID, duration)
	// We use the unified ID to check for presence
	unifiedID := chatID // In this adapter, chatID should already be unified
	chatpresence.WaitIdle(ctx, wa.channelID, unifiedID, duration)
	return nil
}

// Hibernate physically closes the socket to keep a low profile on WhatsApp servers
func (wa *WhatsAppAdapter) Hibernate(ctx context.Context) error {
	wa.hibMu.Lock()
	defer wa.hibMu.Unlock()

	if wa.client == nil || !wa.client.IsConnected() {
		wa.hibernating = true
		return nil
	}

	logrus.Infof("[WHATSAPP] Hibernating channel %s (Closing socket)", wa.channelID)
	wa.client.Disconnect()
	wa.hibernating = true
	return nil
}

// Resume physically reconnects the socket if it was hibernated
func (wa *WhatsAppAdapter) Resume(ctx context.Context) error {
	wa.hibMu.Lock()
	wa.hibernating = false
	wa.hibMu.Unlock()

	if wa.client == nil {
		return fmt.Errorf("client not initialized")
	}

	if !wa.client.IsConnected() {
		logrus.Infof("[WHATSAPP] Resuming socket for channel %s...", wa.channelID)
		if err := wa.client.Connect(); err != nil {
			return fmt.Errorf("failed to resume connection: %w", err)
		}
	}

	return nil
}

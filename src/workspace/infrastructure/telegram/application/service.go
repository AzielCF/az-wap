package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/infrastructure/telegram/domain"
	"github.com/sirupsen/logrus"
)

type WebhookConfig struct {
	Enabled bool
	BaseURL string
}

// TelegramService es el caso de uso que orquestas la lógica de un bot de Telegram
type TelegramService struct {
	client domain.ITelegramClient

	statusMu sync.RWMutex
	status   channel.ChannelStatus
	loggedIn bool
	botInfo  map[string]interface{}

	onMessage func(message.IncomingMessage)
	cancel    context.CancelFunc

	startMu    sync.Mutex
	isStarting bool

	webhook WebhookConfig
}

func NewTelegramService() *TelegramService {
	return &TelegramService{
		status: channel.ChannelStatusDisconnected,
	}
}

// SetClient inyecta la infraestructura (HTTP) en el dominio
func (s *TelegramService) SetClient(client domain.ITelegramClient) {
	s.client = client
}

func (s *TelegramService) SetWebhookConfig(cfg WebhookConfig) {
	s.webhook = cfg
}

// StartBot orquestas el arranque del bot (Validación y estado)
func (s *TelegramService) StartBot(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.StopBot() // Asegurar que no haya bucles previos

	if s.client == nil {
		return fmt.Errorf("telegram client not initialized (missing token)")
	}

	s.updateStatus(channel.ChannelStatusPending, false)

	// La lógica de "Cómo validar un bot" vive aquí
	me, err := s.client.GetMe(ctx)
	if err != nil {
		s.updateStatus(channel.ChannelStatusError, false)
		return fmt.Errorf("telegram gateway: failed to validate identity: %w", err)
	}

	username, _ := me["username"].(string)
	logrus.Infof("[TELEGRAM-SERVICE] Instance online: @%s", username)

	s.statusMu.Lock()
	s.botInfo = me
	s.status = channel.ChannelStatusConnected
	s.loggedIn = true

	// Gestión de Webhook vs Polling
	if s.webhook.Enabled && s.webhook.BaseURL != "" {
		// En producción, configuramos Webhook
		webhookURL := fmt.Sprintf("%s/api/v1/telegram/webhook/%s", s.webhook.BaseURL, ctx.Value("channel_id"))
		if err := s.client.SetWebhook(ctx, webhookURL); err != nil {
			s.statusMu.Unlock()
			return fmt.Errorf("failed to set telegram webhook: %w", err)
		}
		logrus.Infof("[TELEGRAM-SERVICE] Webhook set: %s", webhookURL)
	} else {
		// En desarrollo (localhost), usamos Polling
		if err := s.client.DeleteWebhook(ctx); err != nil {
			logrus.WithError(err).Warn("[TELEGRAM-SERVICE] Failed to delete webhook, polling might fail")
		}

		pollCtx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.pollingLoop(pollCtx)
		logrus.Info("[TELEGRAM-SERVICE] Polling loop started")
	}

	s.statusMu.Unlock()
	return nil
}

func (s *TelegramService) pollingLoop(ctx context.Context) {
	logrus.Infof("[TELEGRAM-SERVICE] Starting message polling...")
	offset := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
			updates, err := s.client.GetUpdates(ctx, offset)
			if err != nil {
				logrus.WithError(err).Error("[TELEGRAM-SERVICE] Error polling updates")
				time.Sleep(5 * time.Second)
				continue
			}

			for _, upd := range updates {
				if upd.UpdateID >= offset {
					offset = upd.UpdateID + 1
				}
				s.handleIncomingUpdate(upd)
			}
		}
	}
}

func (s *TelegramService) handleIncomingUpdate(upd domain.Update) {
	if upd.Message == nil || s.onMessage == nil {
		return
	}

	msg := upd.Message
	text := msg.Text
	if text == "" && msg.Caption != "" {
		text = msg.Caption
	}

	if msg.ReplyToMessage != nil {
		quotedText := msg.ReplyToMessage.Text
		if quotedText == "" && msg.ReplyToMessage.Caption != "" {
			quotedText = msg.ReplyToMessage.Caption
		}

		if quotedText == "" {
			if len(msg.ReplyToMessage.Photo) > 0 {
				quotedText = "🖼️ Image"
			} else if msg.ReplyToMessage.Video != nil {
				quotedText = "🎥 Video"
			} else if msg.ReplyToMessage.Document != nil {
				quotedText = "📄 Document"
			} else if msg.ReplyToMessage.Audio != nil {
				quotedText = "🎧 Audio"
			} else if msg.ReplyToMessage.Voice != nil {
				quotedText = "🎤 Voice Message"
			}
		}

		if quotedText != "" {
			text = fmt.Sprintf("[Replying to message: \"%s\"]\n%s", quotedText, text)
		}
	}

	incoming := message.IncomingMessage{
		ChatID:   fmt.Sprintf("%d", msg.Chat.ID),
		SenderID: fmt.Sprintf("%d", msg.From.ID),
		Text:     text,
		Metadata: map[string]any{
			"message_id": fmt.Sprintf("tg_%d", msg.MessageID),
			"first_name": msg.From.FirstName,
			"username":   msg.From.Username,
			"chat_type":  msg.Chat.Type,
		},
	}

	// Media Detection
	extractMedia(msg, incoming.Metadata, "tg")

	if msg.ReplyToMessage != nil {
		extractMedia(msg.ReplyToMessage, incoming.Metadata, "tg_reply")
	}

	s.onMessage(incoming)
}

func extractMedia(msg *domain.Message, metadata map[string]any, prefix string) {
	if len(msg.Photo) > 0 {
		// Telegram sends multiple sizes, take the largest one
		largest := msg.Photo[len(msg.Photo)-1]
		metadata[prefix+"_media_type"] = "image"
		metadata[prefix+"_file_id"] = largest.FileID
		metadata[prefix+"_mime_type"] = "image/jpeg"
		metadata[prefix+"_file_size"] = largest.FileSize
	} else if msg.Voice != nil {
		metadata[prefix+"_media_type"] = "audio" // Use audio for voice notes in engine
		metadata[prefix+"_file_id"] = msg.Voice.FileID
		metadata[prefix+"_mime_type"] = msg.Voice.MimeType
		metadata[prefix+"_file_size"] = msg.Voice.FileSize
		metadata[prefix+"_is_voice"] = true
	} else if msg.Audio != nil {
		metadata[prefix+"_media_type"] = "audio"
		metadata[prefix+"_file_id"] = msg.Audio.FileID
		metadata[prefix+"_mime_type"] = msg.Audio.MimeType
		metadata[prefix+"_file_size"] = msg.Audio.FileSize
	} else if msg.Video != nil {
		metadata[prefix+"_media_type"] = "video"
		metadata[prefix+"_file_id"] = msg.Video.FileID
		metadata[prefix+"_mime_type"] = msg.Video.MimeType
		metadata[prefix+"_file_size"] = msg.Video.FileSize
	} else if msg.Document != nil {
		metadata[prefix+"_media_type"] = "document"
		metadata[prefix+"_file_id"] = msg.Document.FileID
		metadata[prefix+"_mime_type"] = msg.Document.MimeType
		metadata[prefix+"_file_size"] = msg.Document.FileSize
		metadata[prefix+"_file_name"] = msg.Document.FileName
	}
}

func (s *TelegramService) OnMessage(handler func(message.IncomingMessage)) {
	s.onMessage = handler
}

func (s *TelegramService) StopBot() {
	s.statusMu.Lock()
	wasRunning := s.cancel != nil
	if wasRunning {
		s.cancel()
		s.cancel = nil
	}
	s.statusMu.Unlock()

	if wasRunning {
		// Dar un breve respiro para que la conexión de red se cierre realmente
		// y evitar el error 409 Conflict de Telegram al reiniciar inmediatamente.
		time.Sleep(500 * time.Millisecond)
	}

	s.updateStatus(channel.ChannelStatusDisconnected, false)
}

func (s *TelegramService) SendMessage(ctx context.Context, chatID interface{}, text string) (string, error) {
	if !s.loggedIn || s.client == nil {
		return "", fmt.Errorf("telegram service not connected")
	}
	return s.client.SendMessage(ctx, chatID, text)
}

func (s *TelegramService) SendPresence(ctx context.Context, chatID interface{}, typing bool, isAudio bool) error {
	if !s.loggedIn || s.client == nil || !typing {
		return nil
	}
	action := "typing"
	if isAudio {
		action = "record_voice"
	}
	return s.client.SendChatAction(ctx, chatID, action)
}

func (s *TelegramService) Status() channel.ChannelStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

func (s *TelegramService) IsLoggedIn() bool {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.loggedIn
}

func (s *TelegramService) updateStatus(status channel.ChannelStatus, loggedIn bool) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status = status
	s.loggedIn = loggedIn
}

func (s *TelegramService) GetFile(ctx context.Context, fileID string) (domain.File, error) {
	if s.client == nil {
		return domain.File{}, fmt.Errorf("client not initialized")
	}
	return s.client.GetFile(ctx, fileID)
}

func (s *TelegramService) DownloadFile(ctx context.Context, filePath string) ([]byte, error) {
	if s.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	return s.client.DownloadFile(ctx, filePath)
}

func (s *TelegramService) GetBotInfo() map[string]interface{} {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.botInfo
}

func (s *TelegramService) ProcessUpdate(upd domain.Update) {
	s.handleIncomingUpdate(upd)
}

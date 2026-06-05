package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/core/config"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/infrastructure/telegram/application"
	tgDomain "github.com/AzielCF/az-wap/workspace/infrastructure/telegram/domain"
	"github.com/AzielCF/az-wap/workspace/infrastructure/telegram/infrastructure"
	"github.com/sirupsen/logrus"
)

// TelegramAdapter es solo el adaptador de infraestructura del módulo WORKSPACE.
// No contiene lógica, solo delega al servicio de aplicación de Telegram.
type TelegramAdapter struct {
	channelID   string
	workspaceID string

	// Delegación al servicio de aplicación (Clean Architecture)
	service *application.TelegramService
	manager *workspace.Manager

	onMessage func(message.IncomingMessage)

	configMu sync.RWMutex
	config   channel.ChannelConfig
}

func NewAdapter(channelID, workspaceID, token string, manager *workspace.Manager) *TelegramAdapter {
	adapter := &TelegramAdapter{
		channelID:   channelID,
		workspaceID: workspaceID,
		manager:     manager,
		service:     application.NewTelegramService(),
	}

	// Inyectamos la infraestructura inicial si el token existe
	if token != "" {
		adapter.service.SetClient(infrastructure.NewTelegramHTTPClient(token))
	}

	// Configuración de Webhook desde el Config Global
	if config.Global != nil {
		adapter.service.SetWebhookConfig(application.WebhookConfig{
			Enabled: config.Global.Telegram.WebhookEnabled,
			BaseURL: config.Global.Telegram.WebhookURL,
		})
	}

	// Conectamos el servicio con el adapter para los eventos de mensajes
	adapter.service.OnMessage(func(msg message.IncomingMessage) {
		msg.ChannelID = channelID
		msg.WorkspaceID = workspaceID

		adapter.configMu.RLock()
		conf := adapter.config
		adapter.configMu.RUnlock()

		msg.Media = processTelegramMedia(adapter, conf, &msg, "tg")

		if replyMedia := processTelegramMedia(adapter, conf, &msg, "tg_reply"); replyMedia != nil {
			msg.Medias = append(msg.Medias, replyMedia)
		}

		if adapter.onMessage != nil {
			adapter.onMessage(msg)
		}
	})

	return adapter
}

func processTelegramMedia(adapter *TelegramAdapter, conf channel.ChannelConfig, msg *message.IncomingMessage, prefix string) *message.IncomingMedia {
	fileID, _ := msg.Metadata[prefix+"_file_id"].(string)
	mediaType, _ := msg.Metadata[prefix+"_media_type"].(string)

	if fileID == "" || mediaType == "" || adapter.manager == nil {
		return nil
	}

	isAllowed := true
	switch mediaType {
	case "image":
		isAllowed = conf.AllowImages
	case "audio":
		isAllowed = conf.AllowAudio
	case "video":
		isAllowed = conf.AllowVideo
	case "document":
		isAllowed = conf.AllowDocuments
	}

	if !isAllowed {
		reason := fmt.Sprintf("Downloading %s is disabled in channel settings", mediaType)
		logrus.Warnf("[TELEGRAM-ADAPTER] Media %s blocked by config for channel %s", mediaType, adapter.channelID)
		return &message.IncomingMedia{MimeType: mediaType, Blocked: true, BlockReason: reason}
	}

	var fileSize int64
	if fs, ok := msg.Metadata[prefix+"_file_size"].(float64); ok {
		fileSize = int64(fs)
	} else if fs, ok := msg.Metadata[prefix+"_file_size"].(int); ok {
		fileSize = int64(fs)
	} else if fs, ok := msg.Metadata[prefix+"_file_size"].(int64); ok {
		fileSize = fs
	}

	maxSize := conf.MaxDownloadSize
	if maxSize <= 0 {
		maxSize = 20 * 1024 * 1024 // 20MB default
	}

	if fileSize > maxSize {
		reason := fmt.Sprintf("File size (%d bytes) exceeds the maximum allowed limit (%d bytes)", fileSize, maxSize)
		logrus.Warnf("[TELEGRAM-ADAPTER] Media %s exceeds size limit (%d > %d) for channel %s", mediaType, fileSize, maxSize, adapter.channelID)
		return &message.IncomingMedia{MimeType: mediaType, Blocked: true, BlockReason: reason}
	}

	ctx := context.Background()
	mimeType, _ := msg.Metadata[prefix+"_mime_type"].(string)
	fileName, _ := msg.Metadata[prefix+"_file_name"].(string)

	targetPath, err := adapter.downloadMedia(ctx, fileID, mediaType, msg.ChatID, msg.SenderID, mimeType, fileName)
	if err == nil {
		return &message.IncomingMedia{
			Path:     targetPath,
			MimeType: mimeType,
		}
	}

	logrus.WithError(err).Errorf("[TELEGRAM-ADAPTER] Failed to download media for fileID %s", fileID)
	return &message.IncomingMedia{MimeType: mediaType, Blocked: true, BlockReason: "Failed to download media from Telegram server"}
}

func (ta *TelegramAdapter) downloadMedia(ctx context.Context, fileID, mediaType, chatID, senderID, mimeType, friendlyName string) (string, error) {
	tgFile, err := ta.service.GetFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	sessionKey := ta.channelID + "|" + chatID + "|" + senderID
	ext := filepath.Ext(tgFile.FilePath)
	if ext == "" {
		if mediaType == "image" {
			ext = ".jpg"
		} else if mediaType == "audio" {
			ext = ".ogg"
		}
	}

	fileName := tgFile.FileUniqueID + ext
	if friendlyName == "" {
		friendlyName = fileName
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	targetPath, errPrep := ta.manager.PrepareSessionFile(
		ta.workspaceID, ta.channelID, sessionKey, fileName, friendlyName,
		mimeType, tgFile.FileUniqueID,
	)

	if errPrep != nil {
		return "", errPrep
	}

	if _, errStat := os.Stat(targetPath); errStat != nil {
		data, errDown := ta.service.DownloadFile(ctx, tgFile.FilePath)
		if errDown != nil {
			return "", errDown
		}
		errWrite := os.WriteFile(targetPath, data, 0644)
		if errWrite != nil {
			return "", errWrite
		}
	}

	return targetPath, nil
}

// Implementación de Identidad
func (ta *TelegramAdapter) ID() string                    { return ta.channelID }
func (ta *TelegramAdapter) Type() channel.ChannelType     { return channel.ChannelTypeTelegram }
func (ta *TelegramAdapter) Status() channel.ChannelStatus { return ta.service.Status() }
func (ta *TelegramAdapter) IsLoggedIn() bool              { return ta.service.IsLoggedIn() }

// Ciclo de Vida (Mapeado directo al servicio de aplicación)
func (ta *TelegramAdapter) Start(ctx context.Context, config channel.ChannelConfig) error {
	ta.configMu.Lock()
	ta.config = config
	ta.configMu.Unlock()

	token, _ := config.Settings["token"].(string)
	if token != "" {
		ta.service.SetClient(infrastructure.NewTelegramHTTPClient(token))
	}

	// Añadimos el ID del canal al contexto para que el servicio pueda armar la URL del webhook si es necesario
	startCtx := context.WithValue(ctx, "channel_id", ta.channelID)
	return ta.service.StartBot(startCtx)
}

func (ta *TelegramAdapter) Stop(ctx context.Context) error {
	ta.service.StopBot()
	return nil
}

func (ta *TelegramAdapter) UpdateConfig(config channel.ChannelConfig) {
	ta.configMu.Lock()
	ta.config = config
	ta.configMu.Unlock()

	token, _ := config.Settings["token"].(string)
	if token != "" {
		ta.service.SetClient(infrastructure.NewTelegramHTTPClient(token))
		// Reiniciar para aplicar el nuevo token/configuración
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = ta.Start(ctx, config)
		}()
	}
}

// Mensajería (Solo delegación)
func (ta *TelegramAdapter) SendMessage(ctx context.Context, chatID, text, quoteID string) (common.SendResponse, error) {
	msgID, err := ta.service.SendMessage(ctx, chatID, text)
	if err != nil {
		return common.SendResponse{}, err
	}
	return common.SendResponse{MessageID: "tg_" + msgID, Timestamp: time.Now()}, nil
}

// Resto de Stubs (Solo delegan o están vacíos hasta ser implementados en el servicio de aplicación)
func (ta *TelegramAdapter) Cleanup(ctx context.Context) error                { return nil }
func (ta *TelegramAdapter) Hibernate(ctx context.Context) error              { return nil }
func (ta *TelegramAdapter) Resume(ctx context.Context) error                 { return nil }
func (ta *TelegramAdapter) CloseSession(ctx context.Context, s string) error { return nil }
func (ta *TelegramAdapter) SetOnline(ctx context.Context, online bool) error { return nil }
func (ta *TelegramAdapter) SendMedia(ctx context.Context, chatID string, media common.MediaUpload, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendPresence(ctx context.Context, chatID string, typing bool, isAudio bool) error {
	return ta.service.SendPresence(ctx, chatID, typing, isAudio)
}
func (ta *TelegramAdapter) SendContact(ctx context.Context, chatID, name, phone, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendLocation(ctx context.Context, chatID string, lat, lng float64, _ string, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendGroupInvite(ctx context.Context, chatID, groupJID, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendPoll(ctx context.Context, chatID, question string, options []string, maxSelections int, quoteMessageID string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendLink(ctx context.Context, chatID, link, cap, title, desc string, thumb []byte, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) CreateGroup(ctx context.Context, n string, p []string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetGroupInfo(ctx context.Context, g string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (ta *TelegramAdapter) GetGroupInfoFromLink(ctx context.Context, link string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (ta *TelegramAdapter) GetGroupInviteLink(ctx context.Context, groupID string, reset bool) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) JoinGroupWithLink(ctx context.Context, link string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) LeaveGroup(ctx context.Context, groupID string) error { return nil }
func (ta *TelegramAdapter) GetJoinedGroups(ctx context.Context) ([]common.GroupInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) UpdateGroupParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error {
	return nil
}
func (ta *TelegramAdapter) GetGroupRequestParticipants(ctx context.Context, groupID string) ([]common.GroupRequestParticipant, error) {
	return nil, nil
}
func (ta *TelegramAdapter) UpdateGroupRequestParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error {
	return nil
}
func (ta *TelegramAdapter) SetGroupName(ctx context.Context, g, n string) error { return nil }
func (ta *TelegramAdapter) SetGroupLocked(ctx context.Context, groupID string, locked bool) error {
	return nil
}
func (ta *TelegramAdapter) SetGroupAnnounce(ctx context.Context, groupID string, announce bool) error {
	return nil
}
func (ta *TelegramAdapter) SetGroupTopic(ctx context.Context, groupID string, topic string) error {
	return nil
}
func (ta *TelegramAdapter) GetPrivacySettings(ctx context.Context) (common.PrivacySettings, error) {
	return common.PrivacySettings{}, nil
}
func (ta *TelegramAdapter) GetUserInfo(ctx context.Context, jids []string) ([]common.ContactInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) GetProfilePictureInfo(ctx context.Context, jid string, preview bool) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) RevokeMessage(ctx context.Context, chatID, messageID string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) UnfollowNewsletter(ctx context.Context, jid string) error { return nil }
func (ta *TelegramAdapter) LoginWithCode(ctx context.Context, phone string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) WaitIdle(ctx context.Context, chatID string, duration time.Duration) error {
	return nil
}
func (ta *TelegramAdapter) ResolveIdentity(ctx context.Context, identifier string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) SetGroupPhoto(ctx context.Context, id string, p []byte) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) SetProfileName(ctx context.Context, n string) error   { return nil }
func (ta *TelegramAdapter) SetProfileStatus(ctx context.Context, s string) error { return nil }
func (ta *TelegramAdapter) SetProfilePhoto(ctx context.Context, p []byte) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetBusinessProfile(ctx context.Context, jid string) (common.BusinessProfile, error) {
	return common.BusinessProfile{}, nil
}
func (ta *TelegramAdapter) GetContact(ctx context.Context, jid string) (common.ContactInfo, error) {
	return common.ContactInfo{}, nil
}
func (ta *TelegramAdapter) OnMessage(fn func(message.IncomingMessage)) { ta.onMessage = fn }
func (ta *TelegramAdapter) OnDisconnect(fn func(string))               {}
func (ta *TelegramAdapter) OnLogin(fn func(string))                    {}

func (ta *TelegramAdapter) GetMessages(ctx context.Context, c string, l int) ([]message.IncomingMessage, error) {
	return nil, nil
}
func (ta *TelegramAdapter) GetContactStatus(ctx context.Context, c string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetContactInfo(ctx context.Context, c string) (common.ContactInfo, error) {
	return common.ContactInfo{}, nil
}
func (ta *TelegramAdapter) GetAllContacts(ctx context.Context) ([]common.ContactInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) MarkRead(ctx context.Context, c string, m []string) error { return nil }
func (ta *TelegramAdapter) ReactMessage(ctx context.Context, c, m, e string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) DeleteMessage(ctx context.Context, c, m string, a bool) error {
	return nil
}
func (ta *TelegramAdapter) DeleteMessageForMe(ctx context.Context, c, m string) error  { return nil }
func (ta *TelegramAdapter) StarMessage(ctx context.Context, c, m string, s bool) error { return nil }
func (ta *TelegramAdapter) DownloadMedia(ctx context.Context, mediaID, chatID string) (string, error) {
	// Attempt download. We don't have mediaType or mimeType here, so we let downloadMedia infer via extension
	return ta.downloadMedia(ctx, mediaID, "", chatID, "any", "application/octet-stream", "")
}
func (ta *TelegramAdapter) IsOnWhatsApp(ctx context.Context, p string) (bool, error) {
	return false, nil
}
func (ta *TelegramAdapter) FetchNewsletters(ctx context.Context) ([]common.NewsletterInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) SubscribeNewsletter(ctx context.Context, j string) error { return nil }
func (ta *TelegramAdapter) SendNewsletterMessage(ctx context.Context, n, t, p string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) PinChat(ctx context.Context, c string, p bool) error     { return nil }
func (ta *TelegramAdapter) GetQRChannel(ctx context.Context) (<-chan string, error) { return nil, nil }
func (ta *TelegramAdapter) Login(ctx context.Context) error                         { return nil }
func (ta *TelegramAdapter) Logout(ctx context.Context) error {
	ta.UpdateConfig(channel.ChannelConfig{})
	return nil
}
func (ta *TelegramAdapter) GetStatus() channel.ChannelStatus {
	return ta.service.Status()
}
func (ta *TelegramAdapter) GetMe() (common.ContactInfo, error) {
	info := ta.service.GetBotInfo()
	name := "Telegram Bot"
	if info != nil {
		if first, ok := info["first_name"].(string); ok {
			name = first
			if last, ok := info["last_name"].(string); ok {
				name += " " + last
			}
		} else if user, ok := info["username"].(string); ok {
			name = "@" + user
		}
	}
	return common.ContactInfo{JID: ta.ID(), Name: name}, nil
}

func (ta *TelegramAdapter) ProcessTelegramUpdate(upd tgDomain.Update) {
	ta.service.ProcessUpdate(upd)
}

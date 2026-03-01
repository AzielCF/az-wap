package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/sirupsen/logrus"
)

type TelegramAdapter struct {
	channelID   string
	workspaceID string
	token       string
	client      *TelegramClient
	manager     *workspace.Manager

	statusMu sync.RWMutex
	status   channel.ChannelStatus
	loggedIn bool

	configMu sync.RWMutex
	config   channel.ChannelConfig

	onMessage func(message.IncomingMessage)
}

func NewAdapter(channelID, workspaceID, token string, manager *workspace.Manager) *TelegramAdapter {
	return &TelegramAdapter{
		channelID:   channelID,
		workspaceID: workspaceID,
		token:       token,
		client:      NewTelegramClient(token),
		manager:     manager,
		status:      channel.ChannelStatusDisconnected,
	}
}

func (ta *TelegramAdapter) ID() string                { return ta.channelID }
func (ta *TelegramAdapter) Type() channel.ChannelType { return channel.ChannelTypeTelegram }

func (ta *TelegramAdapter) Status() channel.ChannelStatus {
	ta.statusMu.RLock()
	defer ta.statusMu.RUnlock()
	return ta.status
}

func (ta *TelegramAdapter) IsLoggedIn() bool {
	ta.statusMu.RLock()
	defer ta.statusMu.RUnlock()
	return ta.loggedIn
}

func (ta *TelegramAdapter) Start(ctx context.Context, config channel.ChannelConfig) error {
	ta.configMu.Lock()
	ta.config = config
	ta.configMu.Unlock()

	ta.statusMu.Lock()
	ta.status = channel.ChannelStatusPending
	ta.statusMu.Unlock()

	// Probar conexión
	me, err := ta.client.GetMe(ctx)
	if err != nil {
		ta.statusMu.Lock()
		ta.status = channel.ChannelStatusError
		ta.statusMu.Unlock()
		return fmt.Errorf("failed to validate telegram token: %w", err)
	}

	username, _ := me["username"].(string)
	logrus.Infof("[TELEGRAM] Logged in as @%s", username)

	ta.statusMu.Lock()
	ta.status = channel.ChannelStatusConnected
	ta.loggedIn = true
	ta.statusMu.Unlock()

	return nil
}

func (ta *TelegramAdapter) Stop(ctx context.Context) error {
	ta.statusMu.Lock()
	ta.status = channel.ChannelStatusDisconnected
	ta.loggedIn = false
	ta.statusMu.Unlock()
	return nil
}

func (ta *TelegramAdapter) Cleanup(ctx context.Context) error                { return nil }
func (ta *TelegramAdapter) Hibernate(ctx context.Context) error              { return nil }
func (ta *TelegramAdapter) Resume(ctx context.Context) error                 { return nil }
func (ta *TelegramAdapter) SetOnline(ctx context.Context, online bool) error { return nil }

func (ta *TelegramAdapter) UpdateConfig(config channel.ChannelConfig) {
	ta.configMu.Lock()
	defer ta.configMu.Unlock()
	ta.config = config
}

func (ta *TelegramAdapter) SendMessage(ctx context.Context, chatID, text, quoteID string) (common.SendResponse, error) {
	err := ta.client.SendMessage(ctx, chatID, text)
	if err != nil {
		return common.SendResponse{}, err
	}
	return common.SendResponse{MessageID: "tg_" + time.Now().Format("150405"), Timestamp: time.Now()}, nil
}

// Stubs para cumplir con la interfaz (se implementarán después)
func (ta *TelegramAdapter) SendMedia(ctx context.Context, chatID string, media common.MediaUpload, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendPresence(ctx context.Context, chatID string, typing, isAudio bool) error {
	return nil
}
func (ta *TelegramAdapter) SendContact(ctx context.Context, chatID, name, phone, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendLocation(ctx context.Context, chatID string, lat, lon float64, addr, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendPoll(ctx context.Context, chatID, q string, opt []string, max int, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}
func (ta *TelegramAdapter) SendLink(ctx context.Context, chatID, link, cap, title, desc string, thumb []byte, quote string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}

func (ta *TelegramAdapter) CreateGroup(ctx context.Context, n string, p []string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) JoinGroupWithLink(ctx context.Context, l string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) LeaveGroup(ctx context.Context, id string) error { return nil }
func (ta *TelegramAdapter) GetGroupInfo(ctx context.Context, id string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (ta *TelegramAdapter) UpdateGroupParticipants(ctx context.Context, id string, p []string, act common.ParticipantAction) error {
	return nil
}
func (ta *TelegramAdapter) GetGroupInviteLink(ctx context.Context, id string, r bool) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetJoinedGroups(ctx context.Context) ([]common.GroupInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) GetGroupInfoFromLink(ctx context.Context, l string) (common.GroupInfo, error) {
	return common.GroupInfo{}, nil
}
func (ta *TelegramAdapter) GetGroupRequestParticipants(ctx context.Context, id string) ([]common.GroupRequestParticipant, error) {
	return nil, nil
}
func (ta *TelegramAdapter) UpdateGroupRequestParticipants(ctx context.Context, id string, p []string, act common.ParticipantAction) error {
	return nil
}
func (ta *TelegramAdapter) SetGroupName(ctx context.Context, id, n string) error          { return nil }
func (ta *TelegramAdapter) SetGroupLocked(ctx context.Context, id string, l bool) error   { return nil }
func (ta *TelegramAdapter) SetGroupAnnounce(ctx context.Context, id string, a bool) error { return nil }
func (ta *TelegramAdapter) SetGroupTopic(ctx context.Context, id, t string) error         { return nil }
func (ta *TelegramAdapter) SetGroupPhoto(ctx context.Context, id string, p []byte) (string, error) {
	return "", nil
}

func (ta *TelegramAdapter) SetProfileName(ctx context.Context, n string) error   { return nil }
func (ta *TelegramAdapter) SetProfileStatus(ctx context.Context, s string) error { return nil }
func (ta *TelegramAdapter) SetProfilePhoto(ctx context.Context, p []byte) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetContact(ctx context.Context, j string) (common.ContactInfo, error) {
	return common.ContactInfo{}, nil
}
func (ta *TelegramAdapter) GetPrivacySettings(ctx context.Context) (common.PrivacySettings, error) {
	return common.PrivacySettings{}, nil
}
func (ta *TelegramAdapter) GetUserInfo(ctx context.Context, j []string) ([]common.ContactInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) GetProfilePictureInfo(ctx context.Context, j string, pr bool) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) GetBusinessProfile(ctx context.Context, j string) (common.BusinessProfile, error) {
	return common.BusinessProfile{}, nil
}
func (ta *TelegramAdapter) GetAllContacts(ctx context.Context) ([]common.ContactInfo, error) {
	return nil, nil
}

func (ta *TelegramAdapter) MarkRead(ctx context.Context, c string, m []string) error { return nil }
func (ta *TelegramAdapter) ReactMessage(ctx context.Context, c, m, e string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) RevokeMessage(ctx context.Context, c, m string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) DeleteMessageForMe(ctx context.Context, c, m string) error  { return nil }
func (ta *TelegramAdapter) StarMessage(ctx context.Context, c, m string, s bool) error { return nil }
func (ta *TelegramAdapter) DownloadMedia(ctx context.Context, m, c string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) IsOnWhatsApp(ctx context.Context, p string) (bool, error) {
	return false, nil
}

func (ta *TelegramAdapter) FetchNewsletters(ctx context.Context) ([]common.NewsletterInfo, error) {
	return nil, nil
}
func (ta *TelegramAdapter) UnfollowNewsletter(ctx context.Context, j string) error { return nil }
func (ta *TelegramAdapter) SendNewsletterMessage(ctx context.Context, n, t, p string) (common.SendResponse, error) {
	return common.SendResponse{}, nil
}

func (ta *TelegramAdapter) PinChat(ctx context.Context, c string, p bool) error     { return nil }
func (ta *TelegramAdapter) GetQRChannel(ctx context.Context) (<-chan string, error) { return nil, nil }
func (ta *TelegramAdapter) Login(ctx context.Context) error                         { return nil }
func (ta *TelegramAdapter) LoginWithCode(ctx context.Context, p string) (string, error) {
	return "", nil
}
func (ta *TelegramAdapter) Logout(ctx context.Context) error                              { return nil }
func (ta *TelegramAdapter) WaitIdle(ctx context.Context, c string, d time.Duration) error { return nil }
func (ta *TelegramAdapter) CloseSession(ctx context.Context, c string) error              { return nil }
func (ta *TelegramAdapter) OnMessage(handler func(message.IncomingMessage))               { ta.onMessage = handler }
func (ta *TelegramAdapter) ResolveIdentity(ctx context.Context, i string) (string, error) {
	return i, nil
}
func (ta *TelegramAdapter) GetMe() (common.ContactInfo, error) {
	return common.ContactInfo{JID: ta.ID(), Name: "Telegram Bot"}, nil
}

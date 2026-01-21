package channel

import (
	"context"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/message"
)

// ChannelAdapter define la interfaz que todas las implementaciones de canales deben satisfacer
type ChannelAdapter interface {
	// Identidad
	ID() string
	Type() ChannelType
	Status() ChannelStatus
	IsLoggedIn() bool

	// Ciclo de vida
	Start(ctx context.Context, config ChannelConfig) error
	Stop(ctx context.Context) error
	Cleanup(ctx context.Context) error // Deletes all persistent data (DBs, files)
	UpdateConfig(config ChannelConfig)
	Hibernate(ctx context.Context) error
	Resume(ctx context.Context) error
	SetOnline(ctx context.Context, online bool) error

	// Mensajería
	SendMessage(ctx context.Context, chatID, text, quoteMessageID string) (common.SendResponse, error)
	SendMedia(ctx context.Context, chatID string, media common.MediaUpload, quoteMessageID string) (common.SendResponse, error)
	SendPresence(ctx context.Context, chatID string, typing bool, isAudio bool) error
	SendContact(ctx context.Context, chatID, contactName, contactPhone string, quoteMessageID string) (common.SendResponse, error)
	SendLocation(ctx context.Context, chatID string, lat, long float64, address string, quoteMessageID string) (common.SendResponse, error)
	SendPoll(ctx context.Context, chatID, question string, options []string, maxSelections int, quoteMessageID string) (common.SendResponse, error)
	SendLink(ctx context.Context, chatID, link, caption, title, description string, thumbnail []byte, quoteMessageID string) (common.SendResponse, error)

	// Gestión de Grupos
	CreateGroup(ctx context.Context, name string, participants []string) (string, error)
	JoinGroupWithLink(ctx context.Context, link string) (string, error)
	LeaveGroup(ctx context.Context, groupID string) error
	GetGroupInfo(ctx context.Context, groupID string) (common.GroupInfo, error)
	UpdateGroupParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error
	GetGroupInviteLink(ctx context.Context, groupID string, reset bool) (string, error)
	GetJoinedGroups(ctx context.Context) ([]common.GroupInfo, error)
	GetGroupInfoFromLink(ctx context.Context, link string) (common.GroupInfo, error)
	GetGroupRequestParticipants(ctx context.Context, groupID string) ([]common.GroupRequestParticipant, error)
	UpdateGroupRequestParticipants(ctx context.Context, groupID string, participants []string, action common.ParticipantAction) error
	SetGroupName(ctx context.Context, groupID string, name string) error
	SetGroupLocked(ctx context.Context, groupID string, locked bool) error
	SetGroupAnnounce(ctx context.Context, groupID string, announce bool) error
	SetGroupTopic(ctx context.Context, groupID string, topic string) error
	SetGroupPhoto(ctx context.Context, groupID string, photo []byte) (string, error)

	// Perfil, Presencia y Privacidad
	SetProfileName(ctx context.Context, name string) error
	SetProfileStatus(ctx context.Context, status string) error
	SetProfilePhoto(ctx context.Context, photo []byte) (string, error)
	GetContact(ctx context.Context, jid string) (common.ContactInfo, error)
	GetPrivacySettings(ctx context.Context) (common.PrivacySettings, error)
	GetUserInfo(ctx context.Context, jids []string) ([]common.ContactInfo, error)
	GetProfilePictureInfo(ctx context.Context, jid string, preview bool) (string, error)
	GetBusinessProfile(ctx context.Context, jid string) (common.BusinessProfile, error)
	GetAllContacts(ctx context.Context) ([]common.ContactInfo, error)

	// Gestión de Mensajes
	MarkRead(ctx context.Context, chatID string, messageIDs []string) error
	ReactMessage(ctx context.Context, chatID, messageID, emoji string) (string, error)
	RevokeMessage(ctx context.Context, chatID, messageID string) (string, error)
	DeleteMessageForMe(ctx context.Context, chatID, messageID string) error
	StarMessage(ctx context.Context, chatID, messageID string, starred bool) error
	DownloadMedia(ctx context.Context, messageID, chatID string) (string, error)

	// Utils y Checks
	IsOnWhatsApp(ctx context.Context, phone string) (bool, error)

	// Newsletters
	FetchNewsletters(ctx context.Context) ([]common.NewsletterInfo, error)
	UnfollowNewsletter(ctx context.Context, jid string) error
	SendNewsletterMessage(ctx context.Context, newsletterID, text string, mediaPath string) (common.SendResponse, error)

	// Gestión de Chats
	PinChat(ctx context.Context, chatID string, pinned bool) error

	// Gestión de Sesión
	GetQRChannel(ctx context.Context) (<-chan string, error)
	Login(ctx context.Context) error
	LoginWithCode(ctx context.Context, phone string) (string, error)
	Logout(ctx context.Context) error

	// WaitIdle espera hasta que el usuario ya no esté activo en el chat
	WaitIdle(ctx context.Context, chatID string, duration time.Duration) error

	// CloseSession finaliza explícitamente una sesión de interacción
	CloseSession(ctx context.Context, chatID string) error

	// Manejo de eventos
	OnMessage(handler func(message.IncomingMessage))

	// Resolución de Identidad
	ResolveIdentity(ctx context.Context, identifier string) (string, error)
	GetMe() (common.ContactInfo, error)
}

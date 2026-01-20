package common

import "time"

// MediaType define los tipos de media soportados de forma genérica
type MediaType string

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeDocument MediaType = "document"
	MediaTypeSticker  MediaType = "sticker"
)

// MediaUpload representa un archivo multimedia genérico para subida
type MediaUpload struct {
	Data     []byte
	FileName string
	MimeType string
	Caption  string
	ViewOnce bool
	PTT      bool
	Type     MediaType
}

// SendResponse representa una respuesta genérica tras enviar un mensaje
type SendResponse struct {
	MessageID string
	Timestamp time.Time
}

// GroupParticipant representa un miembro de grupo genérico
type GroupParticipant struct {
	JID          string
	IsAdmin      bool
	IsSuperAdmin bool
	DisplayName  string
	LID          string
}

// GroupInfo representa información de grupo genérica
type GroupInfo struct {
	JID          string
	OwnerJID     string
	Name         string // Renombrado de Subject para compatibilidad con GroupTools
	SubjectOwner string
	SubjectTime  time.Time
	CreateTime   time.Time
	IsReadOnly   bool
	IsAnnounce   bool
	IsEphemeral  bool
	IsLocked     bool
	IsCommunity  bool
	Participants []GroupParticipant
}

// GroupRequestParticipant representa un usuario solicitando unirse a un grupo
type GroupRequestParticipant struct {
	JID         string
	RequestedAt time.Time
}

// ContactInfo representa información de contacto genérica
type ContactInfo struct {
	JID    string
	Name   string
	Status string
	LID    string
}

// ParticipantAction define acciones para gestión de grupos
type ParticipantAction string

const (
	ParticipantActionAdd     ParticipantAction = "add"
	ParticipantActionRemove  ParticipantAction = "remove"
	ParticipantActionPromote ParticipantAction = "promote"
	ParticipantActionDemote  ParticipantAction = "demote"
	ParticipantActionApprove ParticipantAction = "approve"
	ParticipantActionReject  ParticipantAction = "reject"
)

// PrivacySettings representa configuraciones de privacidad del usuario
type PrivacySettings struct {
	GroupAdd     string `json:"group_add"`
	Status       string `json:"status"`
	ReadReceipts string `json:"read_receipts"`
	Profile      string `json:"profile"`
}

// NewsletterInfo representa información básica de newsletter
type NewsletterInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Subscribers  int       `json:"subscribers"`
	Role         string    `json:"role"`
	Subscription string    `json:"subscription"`
	CreatedAt    time.Time `json:"created_at"`
}

// BusinessProfile representa un perfil de WhatsApp Business
type BusinessProfile struct {
	JID                   string             `json:"jid"`
	Email                 string             `json:"email"`
	Address               string             `json:"address"`
	Description           string             `json:"description"`
	Categories            []BusinessCategory `json:"categories"`
	BusinessHoursTimeZone string             `json:"business_hours_timezone"`
	BusinessHours         []BusinessHourDay  `json:"business_hours"`
	Website               []string           `json:"website"`
}

type BusinessCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BusinessHourDay struct {
	DayOfWeek string `json:"day_of_week"`
	Mode      string `json:"mode"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
}

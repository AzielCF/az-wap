package channel

import "time"

type Channel struct {
	ID              string             `json:"id"`
	WorkspaceID     string             `json:"workspace_id"`
	Type            ChannelType        `json:"type"`
	Name            string             `json:"name"`
	Enabled         bool               `json:"enabled"`
	Config          ChannelConfig      `json:"config"`
	Status          ChannelStatus      `json:"status"`
	ExternalRef     string             `json:"external_ref"` // WhatsApp instance ID, Telegram bot token, etc.
	LastSeen        *time.Time         `json:"last_seen,omitempty"`
	AccumulatedCost float64            `json:"accumulated_cost"`         // Costo acumulado en USD (Total legacy)
	CostBreakdown   map[string]float64 `json:"cost_breakdown,omitempty"` // Desglose: "bot_id:model" -> cost
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type ChannelType string

const (
	ChannelTypeWhatsApp ChannelType = "whatsapp"
	ChannelTypeTelegram ChannelType = "telegram"
	ChannelTypeWebChat  ChannelType = "webchat"
	ChannelTypeAPI      ChannelType = "api"
)

type AccessMode string

const (
	AccessModePrivate AccessMode = "private"
	AccessModePublic  AccessMode = "public"
)

type ChannelStatus string

const (
	ChannelStatusPending      ChannelStatus = "pending"
	ChannelStatusConnected    ChannelStatus = "connected"
	ChannelStatusDisconnected ChannelStatus = "disconnected"
	ChannelStatusHibernating  ChannelStatus = "hibernating"
	ChannelStatusError        ChannelStatus = "error"
)

type ChannelConfig struct {
	Settings              map[string]interface{}   `json:"settings"`
	WebhookURL            string                   `json:"webhook_url,omitempty"`
	WebhookSecret         string                   `json:"webhook_secret,omitempty"`
	BotID                 string                   `json:"bot_id,omitempty"`
	DefaultLanguage       string                   `json:"default_language,omitempty"`
	Timezone              string                   `json:"timezone,omitempty"`
	SkipTLSVerification   bool                     `json:"skip_tls_verification"`
	AutoReconnect         bool                     `json:"auto_reconnect"`
	Chatwoot              *ChatwootConfig          `json:"chatwoot,omitempty"`
	Credentials           map[string]string        `json:"credentials,omitempty"`
	AccessMode            AccessMode               `json:"access_mode,omitempty"`
	AllowImages           bool                     `json:"allow_images"`
	AllowAudio            bool                     `json:"allow_audio"`
	AllowVideo            bool                     `json:"allow_video"`
	AllowDocuments        bool                     `json:"allow_documents"`
	AllowStickers         bool                     `json:"allow_stickers"`
	VoiceNotesOnly        bool                     `json:"voice_notes_only"`
	AllowedExtensions     []string                 `json:"allowed_extensions"`
	MaxDownloadSize       int64                    `json:"max_download_size"` // in bytes
	InactivityWarning     *InactivityWarningConfig `json:"inactivity_warning,omitempty"`
	SessionClosing        *SessionClosingConfig    `json:"session_closing,omitempty"`
	SessionTimeout        int                      `json:"session_timeout,omitempty"`         // Minutes. Default: 4
	InactivityWarningTime int                      `json:"inactivity_warning_time,omitempty"` // Minutes. When to alert. Must be >= 80% of total
	MaxHistoryLimit       int                      `json:"max_history_limit,omitempty"`       // Max messages in context. 0 = 10, -1 = Unlimited
}

type SessionClosingConfig struct {
	Enabled     bool              `json:"enabled"`
	Templates   map[string]string `json:"templates"`    // "en", "es", "fr", "ru"
	DefaultLang string            `json:"default_lang"` // Default "en"
}

type InactivityWarningConfig struct {
	Enabled     bool              `json:"enabled"`
	Templates   map[string]string `json:"templates"`    // "en", "es", "fr"
	DefaultLang string            `json:"default_lang"` // Default "en"
}

type ChatwootConfig struct {
	Enabled         bool   `json:"enabled"`
	AccountID       int    `json:"account_id"`
	InboxID         int    `json:"inbox_id"`
	Token           string `json:"token"`
	URL             string `json:"url"`
	BotToken        string `json:"bot_token,omitempty"`        // Bot token for this instance
	InboxIdentifier string `json:"inbox_identifier,omitempty"` // API channel identifier
	CredentialID    string `json:"credential_id,omitempty"`    // Link to a reusable Chatwoot credential
	WebhookURL      string `json:"webhook_url,omitempty"`      // Read-only: URL to configure in Chatwoot
}

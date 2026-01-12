package domain

import "time"

type Channel struct {
	ID          string        `json:"id"`
	WorkspaceID string        `json:"workspace_id"`
	Type        ChannelType   `json:"type"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	Config      ChannelConfig `json:"config"`
	Status      ChannelStatus `json:"status"`
	ExternalRef string        `json:"external_ref"` // WhatsApp instance ID, Telegram bot token, etc.
	LastSeen    *time.Time    `json:"last_seen,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type ChannelType string

const (
	ChannelTypeWhatsApp ChannelType = "whatsapp"
	ChannelTypeTelegram ChannelType = "telegram"
	ChannelTypeWebChat  ChannelType = "webchat"
	ChannelTypeAPI      ChannelType = "api"
)

type ChannelStatus string

const (
	ChannelStatusPending      ChannelStatus = "pending"
	ChannelStatusConnected    ChannelStatus = "connected"
	ChannelStatusDisconnected ChannelStatus = "disconnected"
	ChannelStatusError        ChannelStatus = "error"
)

type ChannelConfig struct {
	Settings    map[string]interface{} `json:"settings"`
	WebhookURL  string                 `json:"webhook_url,omitempty"`
	Credentials map[string]string      `json:"credentials,omitempty"`
}

package domain

import "time"

// ClientTier representa el nivel de un cliente en el sistema
type ClientTier string

const (
	TierFree       ClientTier = "free"
	TierTrial      ClientTier = "trial"
	TierStandard   ClientTier = "standard"
	TierPremium    ClientTier = "premium"
	TierVIP        ClientTier = "vip"
	TierEnterprise ClientTier = "enterprise"
)

// PlatformType representa el tipo de plataforma de origen del cliente
type PlatformType string

const (
	PlatformWhatsApp PlatformType = "whatsapp"
	PlatformTelegram PlatformType = "telegram"
	PlatformWebChat  PlatformType = "webchat"
	PlatformAPI      PlatformType = "api"
)

// Client representa un cliente global en el sistema
type Client struct {
	ID                    string         `json:"id"`
	PlatformID            string         `json:"platform_id"` // LID (WhatsApp), Telegram ID, etc.
	PlatformType          PlatformType   `json:"platform_type"`
	DisplayName           string         `json:"display_name"`
	Email                 string         `json:"email,omitempty"`
	Phone                 string         `json:"phone,omitempty"`
	Tier                  ClientTier     `json:"tier"`
	Tags                  []string       `json:"tags"`
	Metadata              map[string]any `json:"metadata"`
	Notes                 string         `json:"notes,omitempty"`
	Language              string         `json:"language,omitempty"`                // Idioma preferido (es, en, etc.)
	Timezone              string         `json:"timezone,omitempty"`                // IANA timezone (e.g. America/Lima)
	Country               string         `json:"country,omitempty"`                 // ISO 3166-1 alpha-2 (e.g. PE, US, DO)
	AllowedBots           []string       `json:"allowed_bots"`                      // IDs de bots permitidos para este cliente
	SessionTimeout        int            `json:"session_timeout,omitempty"`         // Minutos (Override)
	InactivityWarningTime int            `json:"inactivity_warning_time,omitempty"` // Minutos (Override)
	Enabled               bool           `json:"enabled"`
	LastInteraction       *time.Time     `json:"last_interaction,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// IsVIP retorna true si el cliente tiene tier VIP o superior
func (c *Client) IsVIP() bool {
	return c.Tier == TierVIP || c.Tier == TierEnterprise
}

// IsPremium retorna true si el cliente tiene tier Premium o superior
func (c *Client) IsPremium() bool {
	return c.Tier == TierPremium || c.IsVIP()
}

// HasTag verifica si el cliente tiene un tag específico
func (c *Client) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

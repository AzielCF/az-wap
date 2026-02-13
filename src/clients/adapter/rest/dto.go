package rest

import (
	"time"
)

// CreateClientRequest representa la petición para crear un cliente
type CreateClientRequest struct {
	PlatformID   string         `json:"platform_id"`
	PlatformType string         `json:"platform_type"`
	DisplayName  string         `json:"display_name"`
	Email        string         `json:"email"`
	Phone        string         `json:"phone"`
	Tier         string         `json:"tier"`
	Tags         []string       `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	Notes        string         `json:"notes"`
	Language     string         `json:"language"`
	Timezone     string         `json:"timezone"`
	Country      string         `json:"country"`
	AllowedBots  []string       `json:"allowed_bots"`
	IsTester     bool           `json:"is_tester"`
}

// UpdateClientRequest representa la petición para actualizar un cliente
type UpdateClientRequest struct {
	PlatformID  *string        `json:"platform_id"`
	DisplayName *string        `json:"display_name"`
	Email       *string        `json:"email"`
	Phone       *string        `json:"phone"`
	Tier        *string        `json:"tier"`
	Tags        []string       `json:"tags"`
	Metadata    map[string]any `json:"metadata"`
	Notes       *string        `json:"notes"`
	Language    *string        `json:"language"`
	Timezone    *string        `json:"timezone"`
	Country     *string        `json:"country"`
	AllowedBots []string       `json:"allowed_bots"`
	IsTester    *bool          `json:"is_tester"`
}

// CreateSubscriptionRequest representa la petición para crear una suscripción
type CreateSubscriptionRequest struct {
	ChannelID             string         `json:"channel_id"`
	CustomBotID           string         `json:"custom_bot_id"`
	CustomSystemPrompt    string         `json:"custom_system_prompt"`
	CustomConfig          map[string]any `json:"custom_config"`
	Priority              int            `json:"priority"`
	ExpiresAt             *time.Time     `json:"expires_at"`
	SessionTimeout        int            `json:"session_timeout"`
	InactivityWarningTime int            `json:"inactivity_warning_time"`
	MaxHistoryLimit       *int           `json:"max_history_limit"`
}

// UpdateSubscriptionRequest representa la petición para actualizar una suscripción
type UpdateSubscriptionRequest struct {
	CustomBotID            *string        `json:"custom_bot_id"`
	CustomSystemPrompt     *string        `json:"custom_system_prompt"`
	CustomConfig           map[string]any `json:"custom_config"`
	Priority               *int           `json:"priority"`
	Status                 *string        `json:"status"`
	ExpiresAt              *time.Time     `json:"expires_at"`
	ClearExpiresAt         bool           `json:"clear_expires_at"`
	SessionTimeout         *int           `json:"session_timeout"`
	InactivityWarningTime  *int           `json:"inactivity_warning_time"`
	MaxHistoryLimit        *int           `json:"max_history_limit"`
	ClearSessionTimeout    bool           `json:"clear_session_timeout"`
	ClearInactivityWarning bool           `json:"clear_inactivity_warning"`
	ClearMaxHistoryLimit   bool           `json:"clear_max_history_limit"`
}

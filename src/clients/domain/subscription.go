package domain

import "time"

// SubscriptionStatus representa el estado de una suscripción
type SubscriptionStatus string

const (
	SubscriptionActive  SubscriptionStatus = "active"
	SubscriptionPaused  SubscriptionStatus = "paused"
	SubscriptionExpired SubscriptionStatus = "expired"
	SubscriptionRevoked SubscriptionStatus = "revoked"
)

// ClientSubscription representa el vínculo entre un cliente y un canal
type ClientSubscription struct {
	ID                    string             `json:"id"`
	ClientID              string             `json:"client_id"`
	ChannelID             string             `json:"channel_id"`
	CustomBotID           string             `json:"custom_bot_id,omitempty"`
	CustomSystemPrompt    string             `json:"custom_system_prompt,omitempty"`
	CustomConfig          map[string]any     `json:"custom_config"`
	Priority              int                `json:"priority"`
	SessionTimeout        int                `json:"session_timeout,omitempty"`         // Minutos (Override per subscription)
	InactivityWarningTime int                `json:"inactivity_warning_time,omitempty"` // Minutos (Override per subscription)
	MaxHistoryLimit       *int               `json:"max_history_limit,omitempty"`       // Override limit. Nil = Unlimited, >0 = Limit
	Status                SubscriptionStatus `json:"status"`
	ExpiresAt             *time.Time         `json:"expires_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

// IsActive verifica si la suscripción está activa y no expirada
func (s *ClientSubscription) IsActive() bool {
	if s.Status != SubscriptionActive {
		return false
	}
	if s.ExpiresAt != nil && s.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

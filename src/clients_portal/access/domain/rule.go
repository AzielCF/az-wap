package domain

import (
	"time"

	"github.com/google/uuid"
)

// RuleType defines how the rule is evaluated
type RuleType string

const (
	RuleTypeRouting    RuleType = "ROUTING"    // Redirect to a specific bot
	RuleTypePermission RuleType = "PERMISSION" // Allow/Deny feature access
	RuleTypeLimit      RuleType = "LIMIT"      // Usage thresholds
)

// AccessRule represents a dynamic configuration for a client's workspace
type AccessRule struct {
	ID           string `json:"id" gorm:"primaryKey"`
	PortalUserID string `json:"portal_user_id" gorm:"index;not null"` // Owner of the rule
	ClientID     string `json:"client_id" gorm:"index"`               // Associated CRM Client (optional)

	Type     RuleType `json:"type" gorm:"not null"`
	TargetID string   `json:"target_id"` // ID of Bot, Channel or Feature

	// Criteria (Condition for the rule to apply)
	ConditionKey   string `json:"condition_key"`   // e.g., "phone_number", "platform"
	ConditionValue string `json:"condition_value"` // e.g., "51999888777", "whatsapp"

	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewAccessRule(portalUserID, clientID string, ruleType RuleType, targetID string) *AccessRule {
	return &AccessRule{
		ID:           uuid.New().String(),
		PortalUserID: portalUserID,
		ClientID:     clientID,
		Type:         ruleType,
		TargetID:     targetID,
		Enabled:      true,
	}
}

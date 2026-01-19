package common

import "time"

type AccessAction string

const (
	AccessActionAllow AccessAction = "ALLOW"
	AccessActionDeny  AccessAction = "DENY"
)

type AccessRule struct {
	ID        string       `json:"id"`
	ChannelID string       `json:"channel_id"`
	Identity  string       `json:"identity"` // Pure ID (e.g. JID in WhatsApp)
	Action    AccessAction `json:"action"`
	Label     string       `json:"label"` // Friendly name
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

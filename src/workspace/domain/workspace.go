package domain

import "time"

type Workspace struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	OwnerID     string          `json:"owner_id"`
	Config      WorkspaceConfig `json:"config"`
	Limits      WorkspaceLimits `json:"limits"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type WorkspaceConfig struct {
	Timezone        string            `json:"timezone"`
	DefaultLanguage string            `json:"default_language"`
	Metadata        map[string]string `json:"metadata"`
}

type WorkspaceLimits struct {
	MaxMessagesPerDay  int `json:"max_messages_per_day"`
	MaxChannels        int `json:"max_channels"`
	MaxBots            int `json:"max_bots"`
	RateLimitPerMinute int `json:"rate_limit_per_minute"`
}

// DefaultLimits for new workspaces
var DefaultLimits = WorkspaceLimits{
	MaxMessagesPerDay:  10000,
	MaxChannels:        5,
	MaxBots:            10,
	RateLimitPerMinute: 60,
}

package domain

import "context"

// Setting represents a dynamic configuration value stored in the database.
type Setting struct {
	Key   string
	Value string
}

// ISettingsRepository defines the contract for persisting dynamic settings.
type ISettingsRepository interface {
	// Basic CRUD
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error

	// InitSchema creates the necessary tables
	InitSchema(ctx context.Context) error
}

// Common Keys defined in the system
const (
	KeyAIGlobalSystemPrompt    = "ai_global_system_prompt"
	KeyAITimezone              = "ai_timezone"
	KeyAIDebounceMs            = "ai_debounce_ms"
	KeyAIWaitContactIdleMs     = "ai_wait_contact_idle_ms"
	KeyAITypingEnabled         = "ai_typing_enabled"
	KeyWhatsappMaxDownloadSize = "whatsapp_max_download_size"
	KeyCacheEnabled            = "cache_enabled"
	KeyCacheMaxAgeDays         = "cache_max_age_days"
	KeyCacheMaxSizeMB          = "cache_max_size_mb"
	KeyCacheCleanupInterval    = "cache_cleanup_interval"
)

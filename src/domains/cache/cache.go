package cache

import "context"

type CacheStats struct {
	TotalSize int64  `json:"total_size"`
	HumanSize string `json:"human_size"`
}

type CacheSettings struct {
	Enabled         bool  `json:"enabled"`
	MaxAgeDays      int   `json:"max_age_days"`
	MaxSizeMB       int64 `json:"max_size_mb"`
	CleanupInterval int   `json:"cleanup_interval_mins"` // in minutes
}

type ICacheUsecase interface {
	GetGlobalStats(ctx context.Context) (CacheStats, error)
	ClearGlobalCache(ctx context.Context) error
	GetInstanceStats(ctx context.Context, instanceID string) (CacheStats, error)
	ClearInstanceCache(ctx context.Context, instanceID string) error

	GetSettings(ctx context.Context) (CacheSettings, error)
	SaveSettings(ctx context.Context, settings CacheSettings) error
	StartBackgroundCleanup(ctx context.Context)
}

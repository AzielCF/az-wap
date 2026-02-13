package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AzielCF/az-wap/core/settings/domain"
	"github.com/AzielCF/az-wap/core/settings/infrastructure"
	"gorm.io/gorm"
)

type SettingsService struct {
	repo domain.ISettingsRepository
}

func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{
		repo: infrastructure.NewGlobalSettingsGormRepository(db),
	}
}

type DynamicSettings struct {
	AIGlobalSystemPrompt    string
	AITimezone              string
	AIDebounceMs            *int
	AIWaitContactIdleMs     *int
	AITypingEnabled         *bool
	WhatsappMaxDownloadSize *int64
	CacheEnabled            *bool
	CacheMaxAgeDays         *int
	CacheMaxSizeMB          *int64
	CacheCleanupInterval    *int
}

func (s *SettingsService) GetDynamicSettings(ctx context.Context) (*DynamicSettings, error) {
	if err := s.repo.InitSchema(ctx); err != nil {
		return nil, err
	}

	ds := &DynamicSettings{}

	if val, _ := s.repo.Get(ctx, domain.KeyAIGlobalSystemPrompt); val != "" {
		ds.AIGlobalSystemPrompt = val
	}
	if val, _ := s.repo.Get(ctx, domain.KeyAITimezone); val != "" {
		ds.AITimezone = val
	}
	if val, _ := s.repo.Get(ctx, domain.KeyAIDebounceMs); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			ds.AIDebounceMs = &n
		}
	}
	if val, _ := s.repo.Get(ctx, domain.KeyAIWaitContactIdleMs); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			ds.AIWaitContactIdleMs = &n
		}
	}
	if val, _ := s.repo.Get(ctx, domain.KeyAITypingEnabled); val != "" {
		vLower := strings.ToLower(val)
		isOn := vLower == "1" || vLower == "true" || vLower == "yes" || vLower == "on"
		ds.AITypingEnabled = &isOn
	}
	if val, _ := s.repo.Get(ctx, domain.KeyWhatsappMaxDownloadSize); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
			ds.WhatsappMaxDownloadSize = &n
		}
	}
	if val, _ := s.repo.Get(ctx, domain.KeyCacheEnabled); val != "" {
		vLower := strings.ToLower(val)
		isOn := vLower == "1" || vLower == "true" || vLower == "yes" || vLower == "on"
		ds.CacheEnabled = &isOn
	}
	if val, _ := s.repo.Get(ctx, domain.KeyCacheMaxAgeDays); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			ds.CacheMaxAgeDays = &n
		}
	}
	if val, _ := s.repo.Get(ctx, domain.KeyCacheMaxSizeMB); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
			ds.CacheMaxSizeMB = &n
		}
	}
	if val, _ := s.repo.Get(ctx, domain.KeyCacheCleanupInterval); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			ds.CacheCleanupInterval = &n
		}
	}
	return ds, nil
}

func (s *SettingsService) SetSystemPrompt(ctx context.Context, v string) error {
	return s.repo.Set(ctx, domain.KeyAIGlobalSystemPrompt, strings.TrimSpace(v))
}

func (s *SettingsService) SetTimezone(ctx context.Context, v string) error {
	return s.repo.Set(ctx, domain.KeyAITimezone, strings.TrimSpace(v))
}

func (s *SettingsService) SetDebounce(ctx context.Context, v int) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyAIDebounceMs, fmt.Sprintf("%d", v))
}

func (s *SettingsService) SetContactIdle(ctx context.Context, v int) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyAIWaitContactIdleMs, fmt.Sprintf("%d", v))
}

func (s *SettingsService) SetTypingEnabled(ctx context.Context, v bool) error {
	val := "0"
	if v {
		val = "1"
	}
	return s.repo.Set(ctx, domain.KeyAITypingEnabled, val)
}

func (s *SettingsService) SetMaxDownloadSize(ctx context.Context, v int64) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyWhatsappMaxDownloadSize, fmt.Sprintf("%d", v))
}

func (s *SettingsService) SetCacheEnabled(ctx context.Context, v bool) error {
	val := "0"
	if v {
		val = "1"
	}
	return s.repo.Set(ctx, domain.KeyCacheEnabled, val)
}

func (s *SettingsService) SetCacheMaxAge(ctx context.Context, v int) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyCacheMaxAgeDays, fmt.Sprintf("%d", v))
}

func (s *SettingsService) SetCacheMaxSize(ctx context.Context, v int64) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyCacheMaxSizeMB, fmt.Sprintf("%d", v))
}

func (s *SettingsService) SetCacheCleanupInterval(ctx context.Context, v int) error {
	if v < 0 {
		v = 0
	}
	return s.repo.Set(ctx, domain.KeyCacheCleanupInterval, fmt.Sprintf("%d", v))
}

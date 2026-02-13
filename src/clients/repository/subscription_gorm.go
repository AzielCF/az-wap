package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// --- Persistence Model ---

type subscriptionModel struct {
	ID                    string `gorm:"primaryKey"`
	ClientID              string `gorm:"index:idx_subscriptions_client;index:idx_unique_sub,unique;not null"`
	ChannelID             string `gorm:"index:idx_subscriptions_channel;index:idx_unique_sub,unique;not null"` // FK logic handled by DB constraints ideally
	CustomBotID           string
	CustomSystemPrompt    string
	CustomConfig          string     `gorm:"type:text;default:'{}'"` // JSON
	Priority              int        `gorm:"default:0"`
	Status                string     `gorm:"index:idx_subscriptions_status;default:'active'"`
	ExpiresAt             *time.Time `gorm:"index"` // Index useful for expiration checks
	CreatedAt             time.Time  `gorm:"not null"`
	UpdatedAt             time.Time  `gorm:"not null"`
	SessionTimeout        int        `gorm:"default:0"`
	InactivityWarningTime int        `gorm:"default:0"`
	MaxHistoryLimit       *int
	MaxRecurringReminders *int `gorm:"default:5"`
}

func (subscriptionModel) TableName() string {
	return "client_subscriptions"
}

// --- Repository Implementation ---

type SubscriptionGormRepository struct {
	db *gorm.DB
}

func NewSubscriptionGormRepository(db *gorm.DB) *SubscriptionGormRepository {
	return &SubscriptionGormRepository{db: db}
}

func (r *SubscriptionGormRepository) InitSchema(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&subscriptionModel{})
}

// CRUD

func (r *SubscriptionGormRepository) Create(ctx context.Context, sub *domain.ClientSubscription) error {
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	now := time.Now()
	// Set default times if zero
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = now
	}
	sub.UpdatedAt = now

	model, err := toSubscriptionModel(sub)
	if err != nil {
		return err
	}

	result := r.db.WithContext(ctx).Create(&model)

	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") || strings.Contains(result.Error.Error(), "duplicate key value") {
			return domain.ErrDuplicateSubscription
		}
		return result.Error
	}
	return nil
}

func (r *SubscriptionGormRepository) GetByID(ctx context.Context, id string) (*domain.ClientSubscription, error) {
	var m subscriptionModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrSubscriptionNotFound
		}
		return nil, err
	}
	return fromSubscriptionModel(m)
}

func (r *SubscriptionGormRepository) Update(ctx context.Context, sub *domain.ClientSubscription) error {
	sub.UpdatedAt = time.Now()
	model, err := toSubscriptionModel(sub)
	if err != nil {
		return err
	}

	// Strict Update: only if exists
	result := r.db.WithContext(ctx).Model(&subscriptionModel{ID: sub.ID}).Select("*").Updates(&model)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}

func (r *SubscriptionGormRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&subscriptionModel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}
	return nil
}

// Queries

func (r *SubscriptionGormRepository) GetByClientAndChannel(ctx context.Context, clientID, channelID string) (*domain.ClientSubscription, error) {
	var m subscriptionModel
	if err := r.db.WithContext(ctx).Where("client_id = ? AND channel_id = ?", clientID, channelID).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrSubscriptionNotFound
		}
		return nil, err
	}
	return fromSubscriptionModel(m)
}

func (r *SubscriptionGormRepository) ListByClient(ctx context.Context, clientID string) ([]*domain.ClientSubscription, error) {
	var models []subscriptionModel
	if err := r.db.WithContext(ctx).Where("client_id = ?", clientID).Order("priority DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	return fromSubscriptionModels(models)
}

func (r *SubscriptionGormRepository) ListByChannel(ctx context.Context, channelID string) ([]*domain.ClientSubscription, error) {
	var models []subscriptionModel
	if err := r.db.WithContext(ctx).Where("channel_id = ?", channelID).Order("priority DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	return fromSubscriptionModels(models)
}

func (r *SubscriptionGormRepository) GetActiveSubscription(ctx context.Context, clientID, channelID string) (*domain.ClientSubscription, error) {
	var m subscriptionModel
	now := time.Now()

	// Query: client=X AND channel=Y AND status='active' AND (expires_at IS NULL OR expires_at > now)
	// Order by priority DESC, limit 1
	err := r.db.WithContext(ctx).
		Where("client_id = ? AND channel_id = ? AND status = ?", clientID, channelID, "active").
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("priority DESC").
		First(&m).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrSubscriptionNotFound
		}
		return nil, err
	}
	return fromSubscriptionModel(m)
}

// Maintenance & Stats

func (r *SubscriptionGormRepository) ExpireOldSubscriptions(ctx context.Context) (int, error) {
	now := time.Now()
	// UPDATE client_subscriptions SET status = 'expired', updated_at = now ...
	result := r.db.WithContext(ctx).Model(&subscriptionModel{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", "active", now).
		Updates(map[string]interface{}{
			"status":     "expired",
			"updated_at": now,
		})

	return int(result.RowsAffected), result.Error
}

func (r *SubscriptionGormRepository) DeleteByClientID(ctx context.Context, clientID string) error {
	return r.db.WithContext(ctx).Where("client_id = ?", clientID).Delete(&subscriptionModel{}).Error
}

func (r *SubscriptionGormRepository) CountByChannel(ctx context.Context, channelID string) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&subscriptionModel{}).Where("channel_id = ?", channelID).Count(&count).Error
	return int(count), err
}

func (r *SubscriptionGormRepository) CountActiveByChannel(ctx context.Context, channelID string) (int, error) {
	var count int64
	now := time.Now()
	err := r.db.WithContext(ctx).Model(&subscriptionModel{}).
		Where("channel_id = ? AND status = ?", channelID, "active").
		Where("expires_at IS NULL OR expires_at > ?", now).
		Count(&count).Error
	return int(count), err
}

// --- Mappers ---

func toSubscriptionModel(s *domain.ClientSubscription) (subscriptionModel, error) {
	config := s.CustomConfig
	if config == nil {
		config = make(map[string]any)
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return subscriptionModel{}, fmt.Errorf("marshal custom_config: %w", err)
	}

	return subscriptionModel{
		ID:                    s.ID,
		ClientID:              s.ClientID,
		ChannelID:             s.ChannelID,
		CustomBotID:           s.CustomBotID,
		CustomSystemPrompt:    s.CustomSystemPrompt,
		CustomConfig:          string(configJSON),
		Priority:              s.Priority,
		Status:                string(s.Status),
		ExpiresAt:             s.ExpiresAt,
		CreatedAt:             s.CreatedAt,
		UpdatedAt:             s.UpdatedAt,
		SessionTimeout:        s.SessionTimeout,
		InactivityWarningTime: s.InactivityWarningTime,
		MaxHistoryLimit:       s.MaxHistoryLimit,
		MaxRecurringReminders: s.MaxRecurringReminders,
	}, nil
}

func fromSubscriptionModel(m subscriptionModel) (*domain.ClientSubscription, error) {
	s := &domain.ClientSubscription{
		ID:                    m.ID,
		ClientID:              m.ClientID,
		ChannelID:             m.ChannelID,
		CustomBotID:           m.CustomBotID,
		CustomSystemPrompt:    m.CustomSystemPrompt,
		Priority:              m.Priority,
		Status:                domain.SubscriptionStatus(m.Status),
		ExpiresAt:             m.ExpiresAt,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
		SessionTimeout:        m.SessionTimeout,
		InactivityWarningTime: m.InactivityWarningTime,
		MaxHistoryLimit:       m.MaxHistoryLimit,
		MaxRecurringReminders: m.MaxRecurringReminders,
	}

	// Default value for MaxRecurringReminders if nil (as per SQLite logic)
	// SQLite logic: 'DEFAULT 5' in schema, but also handled in code.
	// Since GORM model has `default:5`, read should be fine. But if explicit nil comes from DB for some reason?
	if s.MaxRecurringReminders == nil {
		defaultVal := 5
		s.MaxRecurringReminders = &defaultVal
	}

	if m.CustomConfig != "" {
		_ = json.Unmarshal([]byte(m.CustomConfig), &s.CustomConfig)
	}
	if s.CustomConfig == nil {
		s.CustomConfig = make(map[string]any)
	}

	return s, nil
}

func fromSubscriptionModels(models []subscriptionModel) ([]*domain.ClientSubscription, error) {
	res := make([]*domain.ClientSubscription, len(models))
	for i, m := range models {
		s, err := fromSubscriptionModel(m)
		if err != nil {
			return nil, err
		}
		res[i] = s
	}
	return res, nil
}

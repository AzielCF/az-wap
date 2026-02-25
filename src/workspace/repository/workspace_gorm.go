package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
	"gorm.io/gorm"
)

// --- Persistence Models ---

type workspaceModel struct {
	ID                    string         `gorm:"primaryKey;column:id"`
	Name                  string         `gorm:"column:name;not null"`
	Description           sql.NullString `gorm:"column:description"`
	OwnerID               string         `gorm:"column:owner_id;not null"`
	ConfigTimezone        string         `gorm:"column:config_timezone;default:'UTC'"`
	ConfigDefaultLanguage string         `gorm:"column:config_default_language;default:'en'"`
	ConfigMetadata        sql.NullString `gorm:"column:config_metadata"` // JSON
	MaxMessagesPerDay     int            `gorm:"column:limits_max_messages_per_day;default:10000"`
	MaxChannels           int            `gorm:"column:limits_max_channels;default:5"`
	MaxBots               int            `gorm:"column:limits_max_bots;default:10"`
	RateLimitPerMinute    int            `gorm:"column:limits_rate_limit_per_minute;default:60"`
	Enabled               bool           `gorm:"column:enabled;default:true"`
	CreatedAt             time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time      `gorm:"column:updated_at;not null"`
}

func (workspaceModel) TableName() string { return "workspaces" }

type channelModel struct {
	ID              string         `gorm:"primaryKey;column:id"`
	WorkspaceID     string         `gorm:"column:workspace_id;not null;index"`
	OwnerID         sql.NullString `gorm:"column:owner_id;index"` // Client ID
	Type            string         `gorm:"column:type;not null"`
	Name            string         `gorm:"column:name;not null"`
	Enabled         bool           `gorm:"column:enabled;default:false"`
	Config          sql.NullString `gorm:"column:config;type:text"` // JSON
	Status          string         `gorm:"column:status;default:'pending'"`
	ExternalRef     *string        `gorm:"column:external_ref;uniqueIndex"`
	LastSeen        *time.Time     `gorm:"column:last_seen"`
	AccumulatedCost float64        `gorm:"column:accumulated_cost;default:0"`
	CostBreakdown   sql.NullString `gorm:"column:cost_breakdown;default:'{}'"` // JSON
	CreatedAt       time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;not null"`
}

func (channelModel) TableName() string { return "channels" }

type accessRuleModel struct {
	ID        string `gorm:"primaryKey"`
	ChannelID string `gorm:"column:channel_id;not null;index;uniqueIndex:idx_channel_identity"`
	Identity  string `gorm:"not null;uniqueIndex:idx_channel_identity"`
	Action    string `gorm:"not null"`
	Label     sql.NullString
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (accessRuleModel) TableName() string { return "access_rules" }

type scheduledPostModel struct {
	ID             string         `gorm:"primaryKey"`
	ChannelID      string         `gorm:"column:channel_id;not null;index"`
	TargetID       string         `gorm:"column:target_id;not null"`
	SenderID       sql.NullString `gorm:"column:sender_id"`
	Text           sql.NullString
	MediaPath      sql.NullString `gorm:"column:media_path"`
	MediaType      sql.NullString `gorm:"column:media_type"`
	ScheduledAt    time.Time      `gorm:"column:scheduled_at;not null;index"`
	Status         string         `gorm:"default:'pending';index"`
	Error          sql.NullString
	RecurrenceDays sql.NullString `gorm:"column:recurrence_days"`
	OriginalTime   sql.NullString `gorm:"column:original_time"`
	ExecutionCount int            `gorm:"column:execution_count;default:0"`
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
}

func (scheduledPostModel) TableName() string { return "scheduled_posts" }

type clientWorkspaceModel struct {
	ID          string         `gorm:"primaryKey;column:id"`
	OwnerID     string         `gorm:"column:owner_id;not null;index"`
	Name        string         `gorm:"column:name;not null"`
	Description sql.NullString `gorm:"column:description"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;not null"`
}

func (clientWorkspaceModel) TableName() string { return "client_workspaces" }

type clientWorkspaceChannelModel struct {
	ClientWorkspaceID string    `gorm:"primaryKey;column:client_workspace_id"`
	ChannelID         string    `gorm:"primaryKey;column:channel_id"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
}

func (clientWorkspaceChannelModel) TableName() string { return "client_workspace_channels" }

type clientWorkspaceGuestModel struct {
	ID                  string    `gorm:"primaryKey;column:id"`
	OwnerID             string    `gorm:"column:owner_id;not null;index"`
	ClientWorkspaceID   string    `gorm:"column:client_workspace_id;not null;index"`
	Name                string    `gorm:"column:name;not null"`
	BotID               string    `gorm:"column:bot_id;not null"`
	BotTemplateID       string    `gorm:"column:bot_template_id;not null"`
	PlatformIdentifiers string    `gorm:"column:platform_identifiers;type:text"` // JSON
	CreatedAt           time.Time `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null"`
}

func (clientWorkspaceGuestModel) TableName() string { return "client_workspace_guests" }

// --- Repository Implementation ---

type WorkspaceGormRepository struct {
	db *gorm.DB
}

func NewWorkspaceGormRepository(db *gorm.DB) *WorkspaceGormRepository {
	return &WorkspaceGormRepository{db: db}
}

func (r *WorkspaceGormRepository) Init(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(
		&workspaceModel{},
		&channelModel{},
		&accessRuleModel{},
		&scheduledPostModel{},
		&clientWorkspaceModel{},
		&clientWorkspaceChannelModel{},
		&clientWorkspaceGuestModel{},
	)
}

// Workspace CRUD

func (r *WorkspaceGormRepository) Create(ctx context.Context, ws workspace.Workspace) error {
	model := toWorkspaceModel(ws)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *WorkspaceGormRepository) GetByID(ctx context.Context, id string) (workspace.Workspace, error) {
	var m workspaceModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return workspace.Workspace{}, common.ErrWorkspaceNotFound
		}
		return workspace.Workspace{}, err
	}
	return fromWorkspaceModel(m), nil
}

func (r *WorkspaceGormRepository) List(ctx context.Context) ([]workspace.Workspace, error) {
	var models []workspaceModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]workspace.Workspace, len(models))
	for i, m := range models {
		res[i] = fromWorkspaceModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) Update(ctx context.Context, ws workspace.Workspace) error {
	model := toWorkspaceModel(ws)
	return r.db.WithContext(ctx).Save(&model).Error
}

func (r *WorkspaceGormRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&workspaceModel{}, "id = ?", id).Error
}

// Channel CRUD

func (r *WorkspaceGormRepository) CreateChannel(ctx context.Context, ch channel.Channel) error {
	model := toChannelModel(ch)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *WorkspaceGormRepository) GetChannel(ctx context.Context, channelID string) (channel.Channel, error) {
	var m channelModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", channelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return channel.Channel{}, common.ErrChannelNotFound
		}
		return channel.Channel{}, err
	}
	return fromChannelModel(m), nil
}

func (r *WorkspaceGormRepository) ListChannels(ctx context.Context, workspaceID string) ([]channel.Channel, error) {
	var models []channelModel
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]channel.Channel, len(models))
	for i, m := range models {
		res[i] = fromChannelModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) ListChannelsByOwnerID(ctx context.Context, ownerID string) ([]channel.Channel, error) {
	var models []channelModel
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]channel.Channel, len(models))
	for i, m := range models {
		res[i] = fromChannelModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) UpdateChannel(ctx context.Context, ch channel.Channel) error {
	model := toChannelModel(ch)
	return r.db.WithContext(ctx).Save(&model).Error
}

func (r *WorkspaceGormRepository) DeleteChannel(ctx context.Context, channelID string) error {
	return r.db.WithContext(ctx).Delete(&channelModel{}, "id = ?", channelID).Error
}

func (r *WorkspaceGormRepository) GetChannelByExternalRef(ctx context.Context, externalRef string) (channel.Channel, error) {
	var m channelModel
	if err := r.db.WithContext(ctx).Where("external_ref = ?", externalRef).First(&m).Error; err != nil {
		return channel.Channel{}, err
	}
	return fromChannelModel(m), nil
}

// Access Rules

func (r *WorkspaceGormRepository) GetAccessRules(ctx context.Context, channelID string) ([]common.AccessRule, error) {
	var models []accessRuleModel
	if err := r.db.WithContext(ctx).Where("channel_id = ?", channelID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]common.AccessRule, len(models))
	for i, m := range models {
		res[i] = fromAccessRuleModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) AddAccessRule(ctx context.Context, rule common.AccessRule) error {
	model := toAccessRuleModel(rule)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *WorkspaceGormRepository) DeleteAccessRule(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&accessRuleModel{}, "id = ?", id).Error
}

func (r *WorkspaceGormRepository) DeleteAllAccessRules(ctx context.Context, channelID string) error {
	return r.db.WithContext(ctx).Where("channel_id = ?", channelID).Delete(&accessRuleModel{}).Error
}

// Costs

func (r *WorkspaceGormRepository) AddChannelCost(ctx context.Context, channelID string, cost float64) error {
	return r.db.WithContext(ctx).Model(&channelModel{}).Where("id = ?", channelID).
		Update("accumulated_cost", gorm.Expr("accumulated_cost + ?", cost)).Error
}

func (r *WorkspaceGormRepository) AddChannelComplexCost(ctx context.Context, channelID string, total float64, details map[string]float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m channelModel
		if err := tx.First(&m, "id = ?", channelID).Error; err != nil {
			return err
		}

		breakdown := make(map[string]float64)
		configJSON := nullStringValue(m.CostBreakdown)
		if configJSON != "" && configJSON != "null" {
			_ = json.Unmarshal([]byte(configJSON), &breakdown)
		}

		for k, v := range details {
			breakdown[k] += v
		}

		newJSON, _ := json.Marshal(breakdown)
		return tx.Model(&m).Updates(map[string]interface{}{
			"accumulated_cost": gorm.Expr("accumulated_cost + ?", total),
			"cost_breakdown":   string(newJSON),
			"updated_at":       time.Now(),
		}).Error
	})
}

// Scheduled Posts

func (r *WorkspaceGormRepository) CreateScheduledPost(ctx context.Context, post common.ScheduledPost) error {
	model := toScheduledPostModel(post)
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *WorkspaceGormRepository) GetScheduledPost(ctx context.Context, id string) (common.ScheduledPost, error) {
	var m scheduledPostModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return common.ScheduledPost{}, err
	}
	return fromScheduledPostModel(m), nil
}

func (r *WorkspaceGormRepository) ListScheduledPosts(ctx context.Context, channelID string) ([]common.ScheduledPost, error) {
	var models []scheduledPostModel
	if err := r.db.WithContext(ctx).Where("channel_id = ?", channelID).Order("scheduled_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]common.ScheduledPost, len(models))
	for i, m := range models {
		res[i] = fromScheduledPostModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) ListPendingScheduledPosts(ctx context.Context) ([]common.ScheduledPost, error) {
	var models []scheduledPostModel
	if err := r.db.WithContext(ctx).Where("status = ? AND scheduled_at <= ?", "pending", time.Now()).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]common.ScheduledPost, len(models))
	for i, m := range models {
		res[i] = fromScheduledPostModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) ListUpcomingScheduledPosts(ctx context.Context, limitTime time.Time) ([]common.ScheduledPost, error) {
	var models []scheduledPostModel
	if err := r.db.WithContext(ctx).Where("status IN ? AND scheduled_at <= ?", []string{"pending", "enqueued"}, limitTime).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]common.ScheduledPost, len(models))
	for i, m := range models {
		res[i] = fromScheduledPostModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) UpdateScheduledPost(ctx context.Context, post common.ScheduledPost) error {
	model := toScheduledPostModel(post)
	return r.db.WithContext(ctx).Save(&model).Error
}

func (r *WorkspaceGormRepository) DeleteScheduledPost(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&scheduledPostModel{}, "id = ?", id).Error
}

func (r *WorkspaceGormRepository) CountPendingScheduledPosts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&scheduledPostModel{}).Where("status IN ?", []string{"pending", "processing"}).Count(&count).Error
	return count, err
}

// Client Workspace CRUD

func (r *WorkspaceGormRepository) CreateClientWorkspace(ctx context.Context, ws workspace.ClientWorkspace) error {
	m := toClientWorkspaceModel(ws)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *WorkspaceGormRepository) GetClientWorkspace(ctx context.Context, id string) (workspace.ClientWorkspace, error) {
	var m clientWorkspaceModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return workspace.ClientWorkspace{}, err
	}
	return fromClientWorkspaceModel(m), nil
}

func (r *WorkspaceGormRepository) ListClientWorkspaces(ctx context.Context, ownerID string) ([]workspace.ClientWorkspace, error) {
	var models []clientWorkspaceModel
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]workspace.ClientWorkspace, len(models))
	for i, m := range models {
		res[i] = fromClientWorkspaceModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) UpdateClientWorkspace(ctx context.Context, ws workspace.ClientWorkspace) error {
	m := toClientWorkspaceModel(ws)
	return r.db.WithContext(ctx).Save(&m).Error
}

func (r *WorkspaceGormRepository) DeleteClientWorkspace(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clean up links and guests first? Or let DB cascades handle it if configured.
		// Manual cleanup to be safe:
		if err := tx.Where("client_workspace_id = ?", id).Delete(&clientWorkspaceChannelModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("client_workspace_id = ?", id).Delete(&clientWorkspaceGuestModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&clientWorkspaceModel{}, "id = ?", id).Error
	})
}

// Client Workspace Channels

func (r *WorkspaceGormRepository) LinkChannelToClientWorkspace(ctx context.Context, workspaceID, channelID string) error {
	var count int64
	r.db.WithContext(ctx).Model(&clientWorkspaceChannelModel{}).
		Where("client_workspace_id = ? AND channel_id = ?", workspaceID, channelID).
		Count(&count)
	if count > 0 {
		return nil // Already linked, no error
	}

	m := clientWorkspaceChannelModel{
		ClientWorkspaceID: workspaceID,
		ChannelID:         channelID,
		CreatedAt:         time.Now(),
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *WorkspaceGormRepository) UnlinkChannelFromClientWorkspace(ctx context.Context, workspaceID, channelID string) error {
	return r.db.WithContext(ctx).Delete(&clientWorkspaceChannelModel{}, "client_workspace_id = ? AND channel_id = ?", workspaceID, channelID).Error
}

func (r *WorkspaceGormRepository) ListChannelsInClientWorkspace(ctx context.Context, workspaceID string) ([]channel.Channel, error) {
	var models []channelModel
	err := r.db.WithContext(ctx).
		Table("channels").
		Joins("JOIN client_workspace_channels ON client_workspace_channels.channel_id = channels.id").
		Where("client_workspace_channels.client_workspace_id = ?", workspaceID).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	res := make([]channel.Channel, len(models))
	for i, m := range models {
		res[i] = fromChannelModel(m)
	}
	return res, nil
}

// Client Workspace Guests

func (r *WorkspaceGormRepository) CreateGuest(ctx context.Context, guest workspace.ClientWorkspaceGuest) error {
	m := toClientWorkspaceGuestModel(guest)
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *WorkspaceGormRepository) GetGuest(ctx context.Context, id string) (workspace.ClientWorkspaceGuest, error) {
	var m clientWorkspaceGuestModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return workspace.ClientWorkspaceGuest{}, err
	}
	return fromClientWorkspaceGuestModel(m), nil
}

func (r *WorkspaceGormRepository) ListGuestsInClientWorkspace(ctx context.Context, workspaceID string) ([]workspace.ClientWorkspaceGuest, error) {
	var models []clientWorkspaceGuestModel
	if err := r.db.WithContext(ctx).Where("client_workspace_id = ?", workspaceID).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]workspace.ClientWorkspaceGuest, len(models))
	for i, m := range models {
		res[i] = fromClientWorkspaceGuestModel(m)
	}
	return res, nil
}

func (r *WorkspaceGormRepository) UpdateGuest(ctx context.Context, guest workspace.ClientWorkspaceGuest) error {
	m := toClientWorkspaceGuestModel(guest)
	return r.db.WithContext(ctx).Save(&m).Error
}

func (r *WorkspaceGormRepository) DeleteGuest(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&clientWorkspaceGuestModel{}, "id = ?", id).Error
}

func (r *WorkspaceGormRepository) ListGuestsByOwnerID(ctx context.Context, ownerID string) ([]workspace.ClientWorkspaceGuest, error) {
	var models []clientWorkspaceGuestModel
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]workspace.ClientWorkspaceGuest, len(models))
	for i, m := range models {
		res[i] = fromClientWorkspaceGuestModel(m)
	}
	return res, nil
}

// --- Mappers ---

func toWorkspaceModel(ws workspace.Workspace) workspaceModel {
	metadata, _ := json.Marshal(ws.Config.Metadata)
	return workspaceModel{
		ID:                 ws.ID,
		Name:               ws.Name,
		Description:        sql.NullString{String: ws.Description, Valid: ws.Description != ""},
		OwnerID:            ws.OwnerID,
		ConfigTimezone:     ws.Config.Timezone,
		ConfigMetadata:     sql.NullString{String: string(metadata), Valid: true},
		MaxMessagesPerDay:  ws.Limits.MaxMessagesPerDay,
		MaxChannels:        ws.Limits.MaxChannels,
		MaxBots:            ws.Limits.MaxBots,
		RateLimitPerMinute: ws.Limits.RateLimitPerMinute,
		Enabled:            ws.Enabled,
		CreatedAt:          ws.CreatedAt,
		UpdatedAt:          ws.UpdatedAt,
	}
}

func fromWorkspaceModel(m workspaceModel) workspace.Workspace {
	var metadata map[string]string
	configJSON := nullStringValue(m.ConfigMetadata)
	if configJSON != "" {
		_ = json.Unmarshal([]byte(configJSON), &metadata)
	}
	return workspace.Workspace{
		ID:          m.ID,
		Name:        m.Name,
		Description: nullStringValue(m.Description),
		OwnerID:     m.OwnerID,
		Config: workspace.WorkspaceConfig{
			Timezone: m.ConfigTimezone,
			Metadata: metadata,
		},
		Limits: workspace.WorkspaceLimits{
			MaxMessagesPerDay:  m.MaxMessagesPerDay,
			MaxChannels:        m.MaxChannels,
			MaxBots:            m.MaxBots,
			RateLimitPerMinute: m.RateLimitPerMinute,
		},
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toChannelModel(ch channel.Channel) channelModel {
	config, _ := json.Marshal(ch.Config)
	breakdown, _ := json.Marshal(ch.CostBreakdown)
	var extRef *string
	if ch.ExternalRef != "" {
		ref := ch.ExternalRef
		extRef = &ref
	}
	return channelModel{
		ID:              ch.ID,
		WorkspaceID:     ch.WorkspaceID,
		OwnerID:         sql.NullString{String: ch.OwnerID, Valid: ch.OwnerID != ""},
		Type:            string(ch.Type),
		Name:            ch.Name,
		Enabled:         ch.Enabled,
		Config:          sql.NullString{String: string(config), Valid: true},
		Status:          string(ch.Status),
		ExternalRef:     extRef,
		LastSeen:        ch.LastSeen,
		AccumulatedCost: ch.AccumulatedCost,
		CostBreakdown:   sql.NullString{String: string(breakdown), Valid: true},
		CreatedAt:       ch.CreatedAt,
		UpdatedAt:       ch.UpdatedAt,
	}
}

func fromChannelModel(m channelModel) channel.Channel {
	var config channel.ChannelConfig
	configJSON := nullStringValue(m.Config)
	if configJSON != "" {
		_ = json.Unmarshal([]byte(configJSON), &config)
	}
	var breakdown map[string]float64
	breakdownJSON := nullStringValue(m.CostBreakdown)
	if breakdownJSON != "" {
		_ = json.Unmarshal([]byte(breakdownJSON), &breakdown)
	}
	var extRef string
	if m.ExternalRef != nil {
		extRef = *m.ExternalRef
	}
	return channel.Channel{
		ID:              m.ID,
		WorkspaceID:     m.WorkspaceID,
		OwnerID:         nullStringValue(m.OwnerID),
		Type:            channel.ChannelType(m.Type),
		Name:            m.Name,
		Enabled:         m.Enabled,
		Config:          config,
		Status:          channel.ChannelStatus(m.Status),
		ExternalRef:     extRef,
		LastSeen:        m.LastSeen,
		AccumulatedCost: m.AccumulatedCost,
		CostBreakdown:   breakdown,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func toAccessRuleModel(r common.AccessRule) accessRuleModel {
	return accessRuleModel{
		ID:        r.ID,
		ChannelID: r.ChannelID,
		Identity:  r.Identity,
		Action:    string(r.Action),
		Label:     sql.NullString{String: r.Label, Valid: r.Label != ""},
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func fromAccessRuleModel(m accessRuleModel) common.AccessRule {
	return common.AccessRule{
		ID:        m.ID,
		ChannelID: m.ChannelID,
		Identity:  m.Identity,
		Action:    common.AccessAction(m.Action),
		Label:     nullStringValue(m.Label),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toScheduledPostModel(p common.ScheduledPost) scheduledPostModel {
	return scheduledPostModel{
		ID:             p.ID,
		ChannelID:      p.ChannelID,
		TargetID:       p.TargetID,
		SenderID:       sql.NullString{String: p.SenderID, Valid: p.SenderID != ""},
		Text:           sql.NullString{String: p.Text, Valid: p.Text != ""},
		MediaPath:      sql.NullString{String: p.MediaPath, Valid: p.MediaPath != ""},
		MediaType:      sql.NullString{String: string(p.MediaType), Valid: p.MediaType != ""},
		ScheduledAt:    p.ScheduledAt,
		Status:         string(p.Status),
		Error:          sql.NullString{String: p.Error, Valid: p.Error != ""},
		RecurrenceDays: sql.NullString{String: p.RecurrenceDays, Valid: p.RecurrenceDays != ""},
		OriginalTime:   sql.NullString{String: p.OriginalTime, Valid: p.OriginalTime != ""},
		ExecutionCount: p.ExecutionCount,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func fromScheduledPostModel(m scheduledPostModel) common.ScheduledPost {
	return common.ScheduledPost{
		ID:             m.ID,
		ChannelID:      m.ChannelID,
		TargetID:       m.TargetID,
		SenderID:       nullStringValue(m.SenderID),
		Text:           nullStringValue(m.Text),
		MediaPath:      nullStringValue(m.MediaPath),
		MediaType:      common.MediaType(nullStringValue(m.MediaType)),
		ScheduledAt:    m.ScheduledAt,
		Status:         common.ScheduledPostStatus(m.Status),
		Error:          nullStringValue(m.Error),
		RecurrenceDays: nullStringValue(m.RecurrenceDays),
		OriginalTime:   nullStringValue(m.OriginalTime),
		ExecutionCount: m.ExecutionCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// nullStringValue returns a trimmed string or empty if null to prevent legacy data panics.
func nullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return strings.TrimSpace(ns.String)
}

func toClientWorkspaceModel(ws workspace.ClientWorkspace) clientWorkspaceModel {
	return clientWorkspaceModel{
		ID:          ws.ID,
		OwnerID:     ws.OwnerID,
		Name:        ws.Name,
		Description: sql.NullString{String: ws.Description, Valid: ws.Description != ""},
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func fromClientWorkspaceModel(m clientWorkspaceModel) workspace.ClientWorkspace {
	return workspace.ClientWorkspace{
		ID:          m.ID,
		OwnerID:     m.OwnerID,
		Name:        m.Name,
		Description: nullStringValue(m.Description),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func toClientWorkspaceGuestModel(g workspace.ClientWorkspaceGuest) clientWorkspaceGuestModel {
	idents, _ := json.Marshal(g.PlatformIdentifiers)
	return clientWorkspaceGuestModel{
		ID:                  g.ID,
		OwnerID:             g.OwnerID,
		ClientWorkspaceID:   g.ClientWorkspaceID,
		Name:                g.Name,
		BotID:               g.BotID,
		BotTemplateID:       g.BotTemplateID,
		PlatformIdentifiers: string(idents),
		CreatedAt:           g.CreatedAt,
		UpdatedAt:           g.UpdatedAt,
	}
}

func fromClientWorkspaceGuestModel(m clientWorkspaceGuestModel) workspace.ClientWorkspaceGuest {
	var idents workspace.PlatformIdentifiers
	_ = json.Unmarshal([]byte(m.PlatformIdentifiers), &idents)
	return workspace.ClientWorkspaceGuest{
		ID:                  m.ID,
		OwnerID:             m.OwnerID,
		ClientWorkspaceID:   m.ClientWorkspaceID,
		Name:                m.Name,
		BotID:               m.BotID,
		BotTemplateID:       m.BotTemplateID,
		PlatformIdentifiers: idents,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

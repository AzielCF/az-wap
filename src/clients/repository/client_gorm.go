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

type clientModel struct {
	ID                    string `gorm:"primaryKey"`
	PlatformID            string `gorm:"index:idx_clients_platform,priority:1;not null"`
	PlatformType          string `gorm:"index:idx_clients_platform,priority:2;not null"`
	DisplayName           string `gorm:"index:idx_clients_display_name"` // Agrego index para search si no existe
	Email                 string `gorm:"index:idx_clients_email"`
	Phone                 string `gorm:"index:idx_clients_phone"`
	Tier                  string `gorm:"index:idx_clients_tier;default:'standard'"`
	Tags                  string `gorm:"type:text;default:'[]'"` // JSON
	Metadata              string `gorm:"type:text;default:'{}'"` // JSON
	Notes                 string
	Language              string `gorm:"default:'en'"`
	Timezone              string
	Country               string
	AllowedBots           string     `gorm:"type:text;default:'[]'"` // JSON
	SessionTimeout        int        `gorm:"default:0"`
	InactivityWarningTime int        `gorm:"default:0"`
	Enabled               bool       `gorm:"default:true"`
	LastInteraction       *time.Time `gorm:"column:last_interaction"`
	CreatedAt             time.Time  `gorm:"not null"`
	UpdatedAt             time.Time  `gorm:"not null"`
}

func (clientModel) TableName() string {
	return "clients"
}

// --- Repository Implementation ---

type ClientGormRepository struct {
	db *gorm.DB
}

func NewClientGormRepository(db *gorm.DB) *ClientGormRepository {
	return &ClientGormRepository{db: db}
}

func (r *ClientGormRepository) InitSchema(ctx context.Context) error {
	// GORM AutoMigrate handles creation and schema updates
	return r.db.WithContext(ctx).AutoMigrate(&clientModel{})
}

// CRUD

func (r *ClientGormRepository) Create(ctx context.Context, client *domain.Client) error {
	if client.ID == "" {
		client.ID = uuid.New().String()
	}
	now := time.Now()
	// Si no vienen seteadas, las ponemos
	if client.CreatedAt.IsZero() {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	model, err := toClientModel(client)
	if err != nil {
		return err
	}

	result := r.db.WithContext(ctx).Create(&model)
	if result.Error != nil {
		// Duplicates detection
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") || strings.Contains(result.Error.Error(), "duplicate key value") {
			return domain.ErrDuplicateClient
		}
		return result.Error
	}
	return nil
}

func (r *ClientGormRepository) GetByID(ctx context.Context, id string) (*domain.Client, error) {
	var m clientModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrClientNotFound
		}
		return nil, err
	}
	return fromClientModel(m)
}

func (r *ClientGormRepository) GetByPlatform(ctx context.Context, platformID string, platformType domain.PlatformType) (*domain.Client, error) {
	var m clientModel
	if err := r.db.WithContext(ctx).Where("platform_id = ? AND platform_type = ?", platformID, string(platformType)).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrClientNotFound
		}
		return nil, err
	}
	return fromClientModel(m)
}

func (r *ClientGormRepository) GetByPhone(ctx context.Context, phone string) (*domain.Client, error) {
	// Logic from SQLite repo: clean phone for search
	cleanPhone := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	var m clientModel
	// Intentar con LIKE para manejar prefijos (ej: +51...) OR exact match
	if err := r.db.WithContext(ctx).Where("phone LIKE ? OR phone = ?", "%"+cleanPhone+"%", phone).First(&m).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrClientNotFound
		}
		return nil, err
	}
	return fromClientModel(m)
}

func (r *ClientGormRepository) Update(ctx context.Context, client *domain.Client) error {
	client.UpdatedAt = time.Now()
	model, err := toClientModel(client)
	if err != nil {
		return err
	}

	// Usamos Save, pero para garantizar que actualice solo si existe y manejar errores de duplicados (unique constraints)
	// GORM Save hace upsert normalmente.
	// Si queremos comportamiento estricto de Update (retornar not found si no existe):
	result := r.db.WithContext(ctx).Model(&clientModel{ID: client.ID}).Select("*").Updates(&model)

	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") || strings.Contains(result.Error.Error(), "duplicate key value") {
			return domain.ErrDuplicateClient
		}
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrClientNotFound
	}

	return nil
}

func (r *ClientGormRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&clientModel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrClientNotFound
	}
	return nil
}

// List & Search

func (r *ClientGormRepository) List(ctx context.Context, filter domain.ClientFilter) ([]*domain.Client, error) {
	var models []clientModel
	query := r.db.WithContext(ctx).Model(&clientModel{})

	if filter.Tier != nil {
		query = query.Where("tier = ?", *filter.Tier)
	}

	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("display_name LIKE ? OR email LIKE ? OR phone LIKE ?", searchPattern, searchPattern, searchPattern)
	}

	// Order
	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	orderDir := "ASC"
	if filter.OrderDesc {
		orderDir = "DESC"
	}
	query = query.Order(fmt.Sprintf("%s %s", orderBy, orderDir))

	// Pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	clients := make([]*domain.Client, len(models))
	for i, m := range models {
		c, err := fromClientModel(m)
		if err != nil {
			return nil, err
		}
		clients[i] = c
	}
	return clients, nil
}

func (r *ClientGormRepository) ListByTier(ctx context.Context, tier domain.ClientTier) ([]*domain.Client, error) {
	return r.List(ctx, domain.ClientFilter{Tier: &tier})
}

func (r *ClientGormRepository) ListByTag(ctx context.Context, tag string) ([]*domain.Client, error) {
	var models []clientModel
	// SQLite usaba LIKE '%"tag"%'. GORM:
	if err := r.db.WithContext(ctx).Where("tags LIKE ?", "%\""+tag+"\"%").Find(&models).Error; err != nil {
		return nil, err
	}

	clients := make([]*domain.Client, len(models))
	for i, m := range models {
		c, err := fromClientModel(m)
		if err != nil {
			return nil, err
		}
		clients[i] = c
	}
	return clients, nil
}

func (r *ClientGormRepository) Search(ctx context.Context, query string) ([]*domain.Client, error) {
	return r.List(ctx, domain.ClientFilter{Search: query, Limit: 50})
}

func (r *ClientGormRepository) CountByTier(ctx context.Context) (map[domain.ClientTier]int, error) {
	var results []struct {
		Tier  string
		Count int
	}
	if err := r.db.WithContext(ctx).Model(&clientModel{}).Select("tier, count(*) as count").Group("tier").Scan(&results).Error; err != nil {
		return nil, err
	}

	counts := make(map[domain.ClientTier]int)
	for _, r := range results {
		counts[domain.ClientTier(r.Tier)] = r.Count
	}
	return counts, nil
}

// Partial Updates

func (r *ClientGormRepository) UpdateLastInteraction(ctx context.Context, id string, t time.Time) error {
	result := r.db.WithContext(ctx).Model(&clientModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_interaction": t,
		"updated_at":       time.Now(),
	})
	return result.Error
}

func (r *ClientGormRepository) AddTag(ctx context.Context, id string, tag string) error {
	client, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Check duplicates
	if client.HasTag(tag) {
		return nil
	}

	client.Tags = append(client.Tags, tag)
	return r.Update(ctx, client)
}

func (r *ClientGormRepository) RemoveTag(ctx context.Context, id string, tag string) error {
	client, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	newTags := []string{}
	for _, t := range client.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	client.Tags = newTags
	return r.Update(ctx, client)
}

// --- Mappers ---

func toClientModel(c *domain.Client) (clientModel, error) {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return clientModel{}, fmt.Errorf("marshal tags: %w", err)
	}

	metadata := c.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return clientModel{}, fmt.Errorf("marshal metadata: %w", err)
	}

	bots := c.AllowedBots
	if bots == nil {
		bots = []string{}
	}
	allowedBotsJSON, err := json.Marshal(bots)
	if err != nil {
		return clientModel{}, fmt.Errorf("marshal allowed_bots: %w", err)
	}

	return clientModel{
		ID:                    c.ID,
		PlatformID:            c.PlatformID,
		PlatformType:          string(c.PlatformType),
		DisplayName:           c.DisplayName,
		Email:                 c.Email,
		Phone:                 c.Phone,
		Tier:                  string(c.Tier),
		Tags:                  string(tagsJSON),
		Metadata:              string(metadataJSON),
		Notes:                 c.Notes,
		Language:              c.Language,
		Timezone:              c.Timezone,
		Country:               c.Country,
		AllowedBots:           string(allowedBotsJSON),
		SessionTimeout:        c.SessionTimeout,
		InactivityWarningTime: c.InactivityWarningTime,
		Enabled:               c.Enabled,
		LastInteraction:       c.LastInteraction,
		CreatedAt:             c.CreatedAt,
		UpdatedAt:             c.UpdatedAt,
	}, nil
}

func fromClientModel(m clientModel) (*domain.Client, error) {
	c := &domain.Client{
		ID:                    m.ID,
		PlatformID:            m.PlatformID,
		PlatformType:          domain.PlatformType(m.PlatformType),
		DisplayName:           m.DisplayName,
		Email:                 m.Email,
		Phone:                 m.Phone,
		Tier:                  domain.ClientTier(m.Tier),
		Notes:                 m.Notes,
		Language:              m.Language,
		Timezone:              m.Timezone,
		Country:               m.Country,
		SessionTimeout:        m.SessionTimeout,
		InactivityWarningTime: m.InactivityWarningTime,
		Enabled:               m.Enabled,
		LastInteraction:       m.LastInteraction,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}

	if m.Tags != "" {
		_ = json.Unmarshal([]byte(m.Tags), &c.Tags)
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}

	if m.Metadata != "" {
		_ = json.Unmarshal([]byte(m.Metadata), &c.Metadata)
	}
	if c.Metadata == nil {
		c.Metadata = make(map[string]any)
	}

	if m.AllowedBots != "" {
		_ = json.Unmarshal([]byte(m.AllowedBots), &c.AllowedBots)
	}
	if c.AllowedBots == nil {
		c.AllowedBots = []string{}
	}

	return c, nil
}

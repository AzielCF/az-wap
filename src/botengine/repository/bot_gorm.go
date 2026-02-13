package repository

import (
	"context"
	"strings"
	"time"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"gorm.io/gorm"
)

// botModel es el modelo de persistencia para GORM.
// Mantiene el dominio puro al no añadir tags de GORM en el struct de dominio.
type botModel struct {
	ID                   string `gorm:"primaryKey"`
	Name                 string
	Description          string
	Provider             string
	Enabled              bool   `gorm:"not null;default:true"`
	APIKey               string `gorm:"column:api_key"`
	Model                string
	SystemPrompt         string
	KnowledgeBase        string
	AudioEnabled         bool      `gorm:"column:audio_enabled;not null;default:false"`
	ImageEnabled         bool      `gorm:"column:image_enabled;not null;default:false"`
	VideoEnabled         bool      `gorm:"column:video_enabled;not null;default:false"`
	DocumentEnabled      bool      `gorm:"column:document_enabled;not null;default:false"`
	MemoryEnabled        bool      `gorm:"column:memory_enabled;not null;default:false"`
	MindsetModel         string    `gorm:"column:mindset_model"`
	MultimodalModel      string    `gorm:"column:multimodal_model"`
	CredentialID         string    `gorm:"column:credential_id"`
	ChatwootCredentialID string    `gorm:"column:chatwoot_credential_id"`
	ChatwootBotToken     string    `gorm:"column:chatwoot_bot_token"`
	Whitelist            string    `gorm:"column:whitelist"` // CSV string
	CreatedAt            time.Time `gorm:"autoCreateTime"`
	UpdatedAt            time.Time `gorm:"autoUpdateTime"`
}

// TableName especifica el nombre de la tabla para GORM.
func (botModel) TableName() string {
	return "bots"
}

// BotGormRepository implementa IBotRepository usando GORM.
type BotGormRepository struct {
	db *gorm.DB
}

// NewBotGormRepository crea una nueva instancia del repositorio GORM.
func NewBotGormRepository(db *gorm.DB) *BotGormRepository {
	return &BotGormRepository{db: db}
}

// Init inicializa el esquema usando AutoMigrate.
func (r *BotGormRepository) Init(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&botModel{})
}

// Create inserta un nuevo bot.
func (r *BotGormRepository) Create(ctx context.Context, bot domainBot.Bot) error {
	model := toBotModel(bot)
	return r.db.WithContext(ctx).Create(&model).Error
}

// GetByID busca un bot por ID.
func (r *BotGormRepository) GetByID(ctx context.Context, id string) (domainBot.Bot, error) {
	var model botModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainBot.Bot{}, pkgError.NotFoundError("bot not found")
		}
		return domainBot.Bot{}, err
	}
	return fromBotModel(model), nil
}

// List retorna todos los bots ordenados por nombre.
func (r *BotGormRepository) List(ctx context.Context) ([]domainBot.Bot, error) {
	var models []botModel
	err := r.db.WithContext(ctx).Order("name ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}

	result := make([]domainBot.Bot, len(models))
	for i, m := range models {
		result[i] = fromBotModel(m)
	}
	return result, nil
}

// Update actualiza un bot.
func (r *BotGormRepository) Update(ctx context.Context, bot domainBot.Bot) error {
	model := toBotModel(bot)
	return r.db.WithContext(ctx).Save(&model).Error
}

// Delete elimina un bot.
func (r *BotGormRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&botModel{}, "id = ?", id).Error
}

// Mappers manuales para mantener la pureza del dominio.
func toBotModel(b domainBot.Bot) botModel {
	return botModel{
		ID:                   b.ID,
		Name:                 b.Name,
		Description:          b.Description,
		Provider:             string(b.Provider),
		Enabled:              b.Enabled,
		APIKey:               b.APIKey,
		Model:                b.Model,
		SystemPrompt:         b.SystemPrompt,
		KnowledgeBase:        b.KnowledgeBase,
		AudioEnabled:         b.AudioEnabled,
		ImageEnabled:         b.ImageEnabled,
		VideoEnabled:         b.VideoEnabled,
		DocumentEnabled:      b.DocumentEnabled,
		MemoryEnabled:        b.MemoryEnabled,
		MindsetModel:         b.MindsetModel,
		MultimodalModel:      b.MultimodalModel,
		CredentialID:         b.CredentialID,
		ChatwootCredentialID: b.ChatwootCredentialID,
		ChatwootBotToken:     b.ChatwootBotToken,
		Whitelist:            strings.Join(b.Whitelist, ","),
	}
}

func fromBotModel(m botModel) domainBot.Bot {
	var whitelist []string
	trimmed := strings.TrimSpace(m.Whitelist)
	if trimmed != "" {
		whitelist = strings.Split(trimmed, ",")
	}

	return domainBot.Bot{
		ID:                   m.ID,
		Name:                 m.Name,
		Description:          m.Description,
		Provider:             domainBot.Provider(m.Provider),
		Enabled:              m.Enabled,
		APIKey:               m.APIKey,
		Model:                m.Model,
		SystemPrompt:         m.SystemPrompt,
		KnowledgeBase:        m.KnowledgeBase,
		AudioEnabled:         m.AudioEnabled,
		ImageEnabled:         m.ImageEnabled,
		VideoEnabled:         m.VideoEnabled,
		DocumentEnabled:      m.DocumentEnabled,
		MemoryEnabled:        m.MemoryEnabled,
		MindsetModel:         m.MindsetModel,
		MultimodalModel:      m.MultimodalModel,
		CredentialID:         strings.TrimSpace(m.CredentialID),
		ChatwootCredentialID: strings.TrimSpace(m.ChatwootCredentialID),
		ChatwootBotToken:     strings.TrimSpace(m.ChatwootBotToken),
		Whitelist:            whitelist,
	}
}

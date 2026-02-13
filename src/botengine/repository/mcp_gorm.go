package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/pkg/crypto"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Persistence Models ---

type mcpServerModel struct {
	ID             string `gorm:"primaryKey"`
	Name           string `gorm:"not null"`
	Description    sql.NullString
	Type           string `gorm:"not null"`
	URL            sql.NullString
	Command        sql.NullString
	Args           sql.NullString `gorm:"type:text"` // JSON
	Env            sql.NullString `gorm:"type:text"` // JSON
	Headers        sql.NullString `gorm:"type:text"` // Encrypted JSON
	Tools          sql.NullString `gorm:"type:text"` // JSON
	Enabled        bool           `gorm:"default:true"`
	IsTemplate     bool           `gorm:"default:false"`
	TemplateConfig sql.NullString `gorm:"type:text"` // JSON
	Instructions   sql.NullString
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (mcpServerModel) TableName() string {
	return "mcp_servers"
}

type botMCPConfigModel struct {
	BotID      string         `gorm:"primaryKey;column:bot_id"`
	ServerID   string         `gorm:"primaryKey;column:server_id"`
	ConfigJSON sql.NullString `gorm:"column:config_json;type:text"`
	Enabled    bool           `gorm:"default:true"`
}

func (botMCPConfigModel) TableName() string {
	return "bot_mcp_configs"
}

// --- Repository Implementation ---

type MCPGormRepository struct {
	db *gorm.DB
}

func NewMCPGormRepository(db *gorm.DB) *MCPGormRepository {
	return &MCPGormRepository{db: db}
}

func (r *MCPGormRepository) Init(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(
		&mcpServerModel{},
		&botMCPConfigModel{},
	)
}

// === MCP Servers ===

func (r *MCPGormRepository) AddServer(ctx context.Context, server domainMCP.MCPServer) error {
	model, err := toMCPServerModel(server)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *MCPGormRepository) ListServers(ctx context.Context) ([]domainMCP.MCPServer, error) {
	var models []mcpServerModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}

	result := make([]domainMCP.MCPServer, len(models))
	for i, m := range models {
		s, err := fromMCPServerModel(m)
		if err != nil {
			return nil, err
		}
		result[i] = s
	}
	return result, nil
}

func (r *MCPGormRepository) GetServer(ctx context.Context, id string) (domainMCP.MCPServer, error) {
	var m mcpServerModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return domainMCP.MCPServer{}, err
	}
	return fromMCPServerModel(m)
}

func (r *MCPGormRepository) UpdateServer(ctx context.Context, id string, server domainMCP.MCPServer) error {
	// Preservar ID original si no viene en el objeto (aunque debería)
	if server.ID == "" {
		server.ID = id
	}

	model, err := toMCPServerModel(server)
	if err != nil {
		return err
	}

	// Usamos Model().Select("*").Updates() para forzar UPDATE de todos los campos (incluyendo zero values)
	// y evitar la operación UPSERT por defecto de Save(). Esto replica el comportamiento de SQL UPDATE.
	result := r.db.WithContext(ctx).Model(&mcpServerModel{ID: id}).Select("*").Updates(&model)
	return result.Error
}

func (r *MCPGormRepository) DeleteServer(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&mcpServerModel{}, "id = ?", id).Error
}

func (r *MCPGormRepository) UpdateServerTools(ctx context.Context, serverID string, tools []domainMCP.Tool) error {
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&mcpServerModel{}).Where("id = ?", serverID).Update("tools", sql.NullString{String: string(toolsJSON), Valid: true}).Error
}

// === Bot MCP Configs ===

func (r *MCPGormRepository) GetBotServerIDs(ctx context.Context, botID string) ([]string, error) {
	var serverIDs []string
	err := r.db.WithContext(ctx).Model(&botMCPConfigModel{}).
		Where("bot_id = ? AND enabled = ?", botID, true).
		Pluck("server_id", &serverIDs).Error
	return serverIDs, err
}

func (r *MCPGormRepository) GetBotMCPConfig(ctx context.Context, botID, serverID string) (domainMCP.BotMCPConfigDB, error) {
	var m botMCPConfigModel
	err := r.db.WithContext(ctx).Where("bot_id = ? AND server_id = ?", botID, serverID).Limit(1).Find(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Si no existe configuración, retornamos valores por defecto (disabled)
			return domainMCP.BotMCPConfigDB{Enabled: false, ConfigJSON: ""}, nil
		}
		return domainMCP.BotMCPConfigDB{}, err
	}

	return domainMCP.BotMCPConfigDB{
		Enabled:    m.Enabled,
		ConfigJSON: nullStringValue(m.ConfigJSON),
	}, nil
}

func (r *MCPGormRepository) ListBotsUsingServer(ctx context.Context, serverID string) ([]string, error) {
	var botIDs []string
	err := r.db.WithContext(ctx).Model(&botMCPConfigModel{}).
		Where("server_id = ? AND enabled = ?", serverID, true).
		Pluck("bot_id", &botIDs).Error
	return botIDs, err
}

func (r *MCPGormRepository) ToggleServerForBot(ctx context.Context, botID, serverID string, enabled bool) error {
	// Upsert: Si existe actualiza, si no crea.
	// En SQLite ON CONFLICT. Gorm lo maneja con Clauses.
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bot_id"}, {Name: "server_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"enabled": enabled}),
	}).Create(&botMCPConfigModel{
		BotID:    botID,
		ServerID: serverID,
		Enabled:  enabled,
	}).Error
}

func (r *MCPGormRepository) SaveBotMCPConfig(ctx context.Context, botID, serverID string, enabled bool, configJSON string) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "bot_id"}, {Name: "server_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"enabled":     enabled,
			"config_json": configJSON,
		}),
	}).Create(&botMCPConfigModel{
		BotID:      botID,
		ServerID:   serverID,
		Enabled:    enabled,
		ConfigJSON: sql.NullString{String: configJSON, Valid: configJSON != ""},
	}).Error
}

// --- Mappers ---

func toMCPServerModel(s domainMCP.MCPServer) (mcpServerModel, error) {
	argsJSON, _ := json.Marshal(s.Args)
	if len(s.Args) == 0 {
		argsJSON = []byte("[]")
	}

	envJSON, _ := json.Marshal(s.Env)
	if s.Env == nil {
		envJSON = []byte("{}")
	}

	// Encrypt Headers
	headersJSON := []byte("{}")
	if s.Headers != nil {
		hJSON, err := json.Marshal(s.Headers)
		if err != nil {
			return mcpServerModel{}, fmt.Errorf("marshal headers: %w", err)
		}
		// Si falla encriptación, logueamos pero guardamos {} (comportamiento original sqlite: fallback o error)
		// El original intenta encriptar, si falla loguea error y guarda {}.
		// Aquí vamos a intentar ser consistentes.
		encrypted, err := crypto.Encrypt(string(hJSON))
		if err != nil {
			logrus.WithError(err).Error("[MCPRepo] failed to encrypt headers")
		} else {
			headersJSON = []byte(encrypted)
		}
	}

	toolsJSON := "[]"
	if s.Tools != nil {
		b, _ := json.Marshal(s.Tools)
		toolsJSON = string(b)
	}

	templateConfigJSON := "{}"
	if s.TemplateConfig != nil {
		b, _ := json.Marshal(s.TemplateConfig)
		templateConfigJSON = string(b)
	}

	return mcpServerModel{
		ID:             s.ID,
		Name:           s.Name,
		Description:    sql.NullString{String: s.Description, Valid: s.Description != ""},
		Type:           string(s.Type),
		URL:            sql.NullString{String: s.URL, Valid: s.URL != ""},
		Command:        sql.NullString{String: s.Command, Valid: s.Command != ""},
		Args:           sql.NullString{String: string(argsJSON), Valid: true},
		Env:            sql.NullString{String: string(envJSON), Valid: true},
		Headers:        sql.NullString{String: string(headersJSON), Valid: true},
		Tools:          sql.NullString{String: toolsJSON, Valid: true},
		Enabled:        s.Enabled,
		IsTemplate:     s.IsTemplate,
		TemplateConfig: sql.NullString{String: templateConfigJSON, Valid: true},
		Instructions:   sql.NullString{String: s.Instructions, Valid: s.Instructions != ""},
	}, nil
}

func fromMCPServerModel(m mcpServerModel) (domainMCP.MCPServer, error) {
	s := domainMCP.MCPServer{
		ID:           m.ID,
		Name:         m.Name,
		Description:  nullStringValue(m.Description),
		Type:         domainMCP.ConnectionType(m.Type),
		URL:          nullStringValue(m.URL),
		Command:      nullStringValue(m.Command),
		Enabled:      m.Enabled,
		IsTemplate:   m.IsTemplate,
		Instructions: nullStringValue(m.Instructions),
	}

	// JSON unmarshals
	argsJSON := nullStringValue(m.Args)
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &s.Args)
	}
	envJSON := nullStringValue(m.Env)
	if envJSON != "" {
		_ = json.Unmarshal([]byte(envJSON), &s.Env)
	}

	// Decrypt Headers
	headersJSON := nullStringValue(m.Headers)
	if headersJSON != "" && headersJSON != "{}" {
		decrypted, err := crypto.Decrypt(headersJSON)
		if err == nil {
			_ = json.Unmarshal([]byte(decrypted), &s.Headers)
		}
	}

	toolsJSON := nullStringValue(m.Tools)
	if toolsJSON != "" {
		_ = json.Unmarshal([]byte(toolsJSON), &s.Tools)
	}
	templateConfigJSON := nullStringValue(m.TemplateConfig)
	if templateConfigJSON != "" {
		_ = json.Unmarshal([]byte(templateConfigJSON), &s.TemplateConfig)
	}

	return s, nil
}

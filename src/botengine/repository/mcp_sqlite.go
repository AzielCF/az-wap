package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	coreDB "github.com/AzielCF/az-wap/core/database"
	"github.com/AzielCF/az-wap/pkg/crypto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// MCPSQLiteRepository implementa IMCPRepository usando SQLite.
type MCPSQLiteRepository struct {
	db *sql.DB
}

// NewMCPSQLiteRepository crea una nueva instancia del repositorio.
func NewMCPSQLiteRepository() (*MCPSQLiteRepository, error) {
	db, err := coreDB.GetLegacyDB()
	if err != nil {
		return nil, err
	}
	repo := &MCPSQLiteRepository{db: db}
	if err := repo.Init(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

// NewMCPSQLiteRepositoryWithDB crea una instancia con DB proporcionada (para tests).
func NewMCPSQLiteRepositoryWithDB(db *sql.DB) (*MCPSQLiteRepository, error) {
	repo := &MCPSQLiteRepository{db: db}
	if err := repo.Init(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

// Init inicializa las tablas y migraciones. (Paridad exacta con mcp.go initMCPStorageDB)
func (r *MCPSQLiteRepository) Init(ctx context.Context) error {
	// Table for global MCP servers
	createServersTable := `
		CREATE TABLE IF NOT EXISTS mcp_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			type TEXT NOT NULL,
			url TEXT,
			command TEXT,
			args TEXT,
			env TEXT,
			headers TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			tools TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	// Table to map which bots use which MCP servers
	createBotMCPTable := `
		CREATE TABLE IF NOT EXISTS bot_mcp_configs (
			bot_id TEXT NOT NULL,
			server_id TEXT NOT NULL,
			config_json TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (bot_id, server_id),
			FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
		);
	`

	if _, err := r.db.ExecContext(ctx, createServersTable); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, createBotMCPTable); err != nil {
		return err
	}

	return r.runMigrations(ctx)
}

func (r *MCPSQLiteRepository) runMigrations(ctx context.Context) error {
	// Migraciones de mcp_servers
	migrations := []struct {
		table  string
		column string
		ddl    string
	}{
		{"mcp_servers", "headers", "ALTER TABLE mcp_servers ADD COLUMN headers TEXT"},
		{"mcp_servers", "tools", "ALTER TABLE mcp_servers ADD COLUMN tools TEXT"},
		{"mcp_servers", "is_template", "ALTER TABLE mcp_servers ADD COLUMN is_template INTEGER DEFAULT 0"},
		{"mcp_servers", "template_config", "ALTER TABLE mcp_servers ADD COLUMN template_config TEXT"},
		{"mcp_servers", "instructions", "ALTER TABLE mcp_servers ADD COLUMN instructions TEXT"},
		{"bot_mcp_configs", "config_json", "ALTER TABLE bot_mcp_configs ADD COLUMN config_json TEXT"},
	}

	for _, m := range migrations {
		var exists bool
		query := "SELECT count(*) FROM pragma_table_info('" + m.table + "') WHERE name='" + m.column + "'"
		_ = r.db.QueryRowContext(ctx, query).Scan(&exists)
		if !exists {
			if _, err := r.db.ExecContext(ctx, m.ddl); err != nil {
				logrus.WithError(err).Warnf("[MCPRepo] Failed to add column %s.%s", m.table, m.column)
			}
		}
	}

	return nil
}

// AddServer añade un nuevo servidor MCP.
func (r *MCPSQLiteRepository) AddServer(ctx context.Context, server domainMCP.MCPServer) error {
	argsJSON, _ := json.Marshal(server.Args)
	if len(server.Args) == 0 {
		argsJSON = []byte("[]")
	}
	envJSON, _ := json.Marshal(server.Env)
	if server.Env == nil {
		envJSON = []byte("{}")
	}
	headersJSON, _ := json.Marshal(server.Headers)
	if server.Headers == nil {
		headersJSON = []byte("{}")
	}
	toolsJSON := "[]"
	if server.Tools != nil {
		b, _ := json.Marshal(server.Tools)
		toolsJSON = string(b)
	}
	templateConfigJSON := "{}"
	if server.TemplateConfig != nil {
		b, _ := json.Marshal(server.TemplateConfig)
		templateConfigJSON = string(b)
	}
	isTemplateInt := 0
	if server.IsTemplate {
		isTemplateInt = 1
	}

	query := `INSERT INTO mcp_servers (id, name, description, type, url, command, args, env, headers, tools, enabled, is_template, template_config, instructions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, server.ID, server.Name, server.Description, string(server.Type), server.URL, server.Command, string(argsJSON), string(envJSON), string(headersJSON), string(toolsJSON), 1, isTemplateInt, templateConfigJSON, server.Instructions)
	return err
}

// ListServers retorna todos los servidores MCP.
func (r *MCPSQLiteRepository) ListServers(ctx context.Context) ([]domainMCP.MCPServer, error) {
	query := `SELECT id, name, description, type, url, command, args, env, headers, tools, COALESCE(is_template, 0), COALESCE(template_config, '{}'), COALESCE(instructions, '') FROM mcp_servers`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []domainMCP.MCPServer
	for rows.Next() {
		srv, err := r.scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

// GetServer obtiene un servidor por su ID.
func (r *MCPSQLiteRepository) GetServer(ctx context.Context, id string) (domainMCP.MCPServer, error) {
	query := `SELECT id, name, description, type, url, command, args, env, headers, tools, COALESCE(is_template, 0), COALESCE(template_config, '{}'), COALESCE(instructions, '') FROM mcp_servers WHERE id = ?`
	return r.scanServer(r.db.QueryRowContext(ctx, query, id))
}

func (r *MCPSQLiteRepository) scanServer(scanner interface{ Scan(...any) error }) (domainMCP.MCPServer, error) {
	var srv domainMCP.MCPServer
	var typeStr, argsJSON, envJSON, headersJSON, toolsJSON, templateConfigJSON string
	var isTemplateInt int

	err := scanner.Scan(&srv.ID, &srv.Name, &srv.Description, &typeStr, &srv.URL, &srv.Command, &argsJSON, &envJSON, &headersJSON, &toolsJSON, &isTemplateInt, &templateConfigJSON, &srv.Instructions)
	if err != nil {
		return srv, err
	}

	srv.Type = domainMCP.ConnectionType(typeStr)
	json.Unmarshal([]byte(argsJSON), &srv.Args)
	json.Unmarshal([]byte(envJSON), &srv.Env)

	// Decrypt headers
	if headersJSON != "" {
		decrypted, _ := crypto.Decrypt(headersJSON)
		json.Unmarshal([]byte(decrypted), &srv.Headers)
	}

	json.Unmarshal([]byte(toolsJSON), &srv.Tools)
	json.Unmarshal([]byte(templateConfigJSON), &srv.TemplateConfig)
	srv.IsTemplate = isTemplateInt != 0

	return srv, nil
}

// UpdateServer actualiza un servidor MCP.
func (r *MCPSQLiteRepository) UpdateServer(ctx context.Context, id string, server domainMCP.MCPServer) error {
	argsJSON, _ := json.Marshal(server.Args)
	if len(server.Args) == 0 {
		argsJSON = []byte("[]")
	}
	envJSON, _ := json.Marshal(server.Env)
	if server.Env == nil {
		envJSON = []byte("{}")
	}
	// Encrypt headers
	headersJSON := []byte("{}")
	if server.Headers != nil {
		hJSON, _ := json.Marshal(server.Headers)
		encrypted, err := crypto.Encrypt(string(hJSON))
		if err == nil {
			headersJSON = []byte(encrypted)
		} else {
			logrus.WithError(err).Error("[MCPRepo] failed to encrypt headers in UpdateServer")
		}
	}
	toolsJSON := "[]"
	if server.Tools != nil {
		b, _ := json.Marshal(server.Tools)
		toolsJSON = string(b)
	}
	templateConfigJSON := "{}"
	if server.TemplateConfig != nil {
		b, _ := json.Marshal(server.TemplateConfig)
		templateConfigJSON = string(b)
	}
	isTemplateInt := 0
	if server.IsTemplate {
		isTemplateInt = 1
	}

	query := `UPDATE mcp_servers 
			  SET name=?, description=?, type=?, url=?, command=?, args=?, env=?, headers=?, tools=?, enabled=?, is_template=?, template_config=?, instructions=?
			  WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, server.Name, server.Description, string(server.Type), server.URL, server.Command, string(argsJSON), string(envJSON), string(headersJSON), string(toolsJSON), 1, isTemplateInt, templateConfigJSON, server.Instructions, id)
	return err
}

// DeleteServer elimina un servidor por ID.
func (r *MCPSQLiteRepository) DeleteServer(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM mcp_servers WHERE id = ?", id)
	return err
}

// UpdateServerTools actualiza el cache de tools de un servidor.
func (r *MCPSQLiteRepository) UpdateServerTools(ctx context.Context, serverID string, tools []domainMCP.Tool) error {
	toolsJSON, _ := json.Marshal(tools)
	_, err := r.db.ExecContext(ctx, "UPDATE mcp_servers SET tools = ? WHERE id = ?", string(toolsJSON), serverID)
	return err
}

// === Bot MCP Configs ===

// GetBotServerIDs retorna los IDs de servidores habilitados para un bot.
func (r *MCPSQLiteRepository) GetBotServerIDs(ctx context.Context, botID string) ([]string, error) {
	query := `SELECT server_id FROM bot_mcp_configs WHERE bot_id = ? AND enabled = 1`
	rows, err := r.db.QueryContext(ctx, query, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var serverIDs []string
	for rows.Next() {
		var serverID string
		if err := rows.Scan(&serverID); err == nil {
			serverIDs = append(serverIDs, serverID)
		}
	}
	return serverIDs, nil
}

// GetBotMCPConfig obtiene la configuración de un bot para un servidor específico.
func (r *MCPSQLiteRepository) GetBotMCPConfig(ctx context.Context, botID, serverID string) (domainMCP.BotMCPConfigDB, error) {
	var cfg domainMCP.BotMCPConfigDB
	var enabled int
	var configJSON sql.NullString

	query := `SELECT enabled, config_json FROM bot_mcp_configs WHERE bot_id = ? AND server_id = ?`
	err := r.db.QueryRowContext(ctx, query, botID, serverID).Scan(&enabled, &configJSON)
	if err != nil {
		return cfg, err
	}

	cfg.Enabled = enabled != 0
	if configJSON.Valid {
		cfg.ConfigJSON = configJSON.String
	}
	return cfg, nil
}

// ListBotsUsingServer retorna los IDs de bots que usan un servidor.
func (r *MCPSQLiteRepository) ListBotsUsingServer(ctx context.Context, serverID string) ([]string, error) {
	query := `SELECT bot_id FROM bot_mcp_configs WHERE server_id = ? AND enabled = 1`
	rows, err := r.db.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			bots = append(bots, id)
		}
	}
	return bots, nil
}

// ToggleServerForBot habilita/deshabilita un servidor para un bot.
func (r *MCPSQLiteRepository) ToggleServerForBot(ctx context.Context, botID, serverID string, enabled bool) error {
	status := 0
	if enabled {
		status = 1
	}

	query := `INSERT INTO bot_mcp_configs (bot_id, server_id, enabled) VALUES (?, ?, ?)
			  ON CONFLICT(bot_id, server_id) DO UPDATE SET enabled = ?`
	_, err := r.db.ExecContext(ctx, query, botID, serverID, status, status)
	return err
}

// SaveBotMCPConfig guarda la configuración de un bot para un servidor.
func (r *MCPSQLiteRepository) SaveBotMCPConfig(ctx context.Context, botID, serverID string, enabled bool, configJSON string) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	query := `INSERT INTO bot_mcp_configs (bot_id, server_id, enabled, config_json) VALUES (?, ?, ?, ?)
			  ON CONFLICT(bot_id, server_id) DO UPDATE SET enabled = ?, config_json = ?`
	_, err := r.db.ExecContext(ctx, query, botID, serverID, enabledInt, configJSON, enabledInt, configJSON)
	return err
}

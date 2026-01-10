package usecase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"time"

	"github.com/AzielCF/az-wap/config"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	domainMCP "github.com/AzielCF/az-wap/domains/mcp"
	"github.com/AzielCF/az-wap/pkg/crypto"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

type mcpService struct {
	db     *sql.DB
	health domainHealth.IHealthUsecase
}

func initMCPStorageDB() (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

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
			config_json TEXT, -- Optional credentials or specific params for the bot
			enabled INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (bot_id, server_id),
			FOREIGN KEY (server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
		);
	`

	if _, err := db.Exec(createServersTable); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createBotMCPTable); err != nil {
		return nil, err
	}

	// Internal migration for headers column
	var hasHeaders bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('mcp_servers') WHERE name='headers'").Scan(&hasHeaders)
	if !hasHeaders {
		_, _ = db.Exec("ALTER TABLE mcp_servers ADD COLUMN headers TEXT")
	}

	var hasTools bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('mcp_servers') WHERE name='tools'").Scan(&hasTools)
	if !hasTools {
		_, _ = db.Exec("ALTER TABLE mcp_servers ADD COLUMN tools TEXT")
	}

	var hasConfigJson bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('bot_mcp_configs') WHERE name='config_json'").Scan(&hasConfigJson)
	if !hasConfigJson {
		_, _ = db.Exec("ALTER TABLE bot_mcp_configs ADD COLUMN config_json TEXT")
	}

	// Internal migration for template columns
	var hasIsTemplate bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('mcp_servers') WHERE name='is_template'").Scan(&hasIsTemplate)
	if !hasIsTemplate {
		_, _ = db.Exec("ALTER TABLE mcp_servers ADD COLUMN is_template INTEGER DEFAULT 0")
	}

	var hasTemplateConfig bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('mcp_servers') WHERE name='template_config'").Scan(&hasTemplateConfig)
	if !hasTemplateConfig {
		_, _ = db.Exec("ALTER TABLE mcp_servers ADD COLUMN template_config TEXT")
	}

	var hasInstructions bool
	_ = db.QueryRow("SELECT count(*) FROM pragma_table_info('mcp_servers') WHERE name='instructions'").Scan(&hasInstructions)
	if !hasInstructions {
		_, _ = db.Exec("ALTER TABLE mcp_servers ADD COLUMN instructions TEXT")
	}

	return db, nil
}

func NewMCPService() domainMCP.IMCPUsecase {
	// Initialize crypto
	if err := crypto.SetEncryptionKey(config.AppSecretKey); err != nil {
		logrus.WithError(err).Error("[MCP] failed to set encryption key")
	}

	db, err := initMCPStorageDB()
	if err != nil {
		logrus.WithError(err).Error("[MCP] failed to initialize storage")
		return &mcpService{db: nil}
	}
	return &mcpService{db: db}
}

func (s *mcpService) ensureDB() error {
	if s.db == nil {
		return fmt.Errorf("mcp storage not initialized")
	}
	return nil
}

func (s *mcpService) AddServer(ctx context.Context, server domainMCP.MCPServer) (domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return server, err
	}

	if server.ID == "" {
		server.ID = uuid.NewString()
	}

	// Stdio is now allowed as per user request
	if server.Type == domainMCP.ConnTypeSSE {
		allowInsecure := os.Getenv("MCP_ALLOW_INSECURE_HTTP") == "true"
		if !allowInsecure && !strings.HasPrefix(server.URL, "https://") {
			return server, fmt.Errorf("insecure HTTP is not allowed. Use HTTPS or enable MCP_ALLOW_INSECURE_HTTP for local development")
		}
	}

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

	// PROACTIVE VALIDATION
	var err error
	if server.IsTemplate {
		err = s.checkAvailability(ctx, server)
	} else {
		err = s.performFullValidation(ctx, server)
	}

	if s.health != nil {
		if err != nil {
			s.health.ReportFailure(ctx, domainHealth.EntityMCP, server.ID, err.Error())
		} else {
			s.health.ReportSuccess(ctx, domainHealth.EntityMCP, server.ID)
		}
	}

	if err != nil {
		return server, fmt.Errorf("validation failed: %w", err)
	}

	query := `INSERT INTO mcp_servers (id, name, description, type, url, command, args, env, headers, tools, enabled, is_template, template_config, instructions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = s.db.ExecContext(ctx, query, server.ID, server.Name, server.Description, string(server.Type), server.URL, server.Command, string(argsJSON), string(envJSON), string(headersJSON), string(toolsJSON), 1, isTemplateInt, templateConfigJSON, server.Instructions)
	return server, err
}

func (s *mcpService) ListServers(ctx context.Context) ([]domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	query := `SELECT id, name, description, type, url, command, args, env, headers, tools, COALESCE(is_template, 0), COALESCE(template_config, '{}'), COALESCE(instructions, '') FROM mcp_servers`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []domainMCP.MCPServer
	for rows.Next() {
		var srv domainMCP.MCPServer
		var typeStr, argsJSON, envJSON, headersJSON, toolsJSON, templateConfigJSON string
		var isTemplateInt int
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Description, &typeStr, &srv.URL, &srv.Command, &argsJSON, &envJSON, &headersJSON, &toolsJSON, &isTemplateInt, &templateConfigJSON, &srv.Instructions); err != nil {
			return nil, err
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
		servers = append(servers, srv)
	}
	return servers, nil
}

func (s *mcpService) GetServer(ctx context.Context, id string) (domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return domainMCP.MCPServer{}, err
	}
	var srv domainMCP.MCPServer
	var typeStr, argsJSON, envJSON, headersJSON, toolsJSON, templateConfigJSON string
	var isTemplateInt int
	query := `SELECT id, name, description, type, url, command, args, env, headers, tools, COALESCE(is_template, 0), COALESCE(template_config, '{}'), COALESCE(instructions, '') FROM mcp_servers WHERE id = ?`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&srv.ID, &srv.Name, &srv.Description, &typeStr, &srv.URL, &srv.Command, &argsJSON, &envJSON, &headersJSON, &toolsJSON, &isTemplateInt, &templateConfigJSON, &srv.Instructions)
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

func (s *mcpService) DeleteServer(ctx context.Context, id string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_servers WHERE id = ?", id)
	return err
}

func (s *mcpService) UpdateServer(ctx context.Context, id string, server domainMCP.MCPServer) (domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return server, err
	}
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
			logrus.WithError(err).Error("[MCP] failed to encrypt headers in UpdateServer")
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

	// PROACTIVE VALIDATION
	var err error
	if server.IsTemplate {
		err = s.checkAvailability(ctx, server)
	} else {
		err = s.performFullValidation(ctx, server)
	}

	if s.health != nil {
		if err != nil {
			s.health.ReportFailure(ctx, domainHealth.EntityMCP, id, err.Error())
		} else {
			s.health.ReportSuccess(ctx, domainHealth.EntityMCP, id)
		}
	}

	if err != nil {
		return server, fmt.Errorf("validation failed: %w", err)
	}

	query := `UPDATE mcp_servers 
			  SET name=?, description=?, type=?, url=?, command=?, args=?, env=?, headers=?, tools=?, enabled=?, is_template=?, template_config=?, instructions=?
			  WHERE id = ?`
	_, err = s.db.ExecContext(ctx, query, server.Name, server.Description, string(server.Type), server.URL, server.Command, string(argsJSON), string(envJSON), string(headersJSON), string(toolsJSON), 1, isTemplateInt, templateConfigJSON, server.Instructions, id)
	return server, err
}

func (s *mcpService) getClient(ctx context.Context, server domainMCP.MCPServer) (*client.Client, error) {
	var mcpClient *client.Client
	var err error

	switch server.Type {
	case domainMCP.ConnTypeStdio:
		return nil, fmt.Errorf("MCP stdio connections are disabled for security reasons")
	case domainMCP.ConnTypeHTTP:
		if strings.TrimSpace(server.URL) == "" {
			return nil, fmt.Errorf("MCP HTTP server '%s' has no URL configured", server.Name)
		}
		var opts []transport.StreamableHTTPCOption
		if len(server.Headers) > 0 {
			opts = append(opts, transport.WithHTTPHeaders(server.Headers))
		}
		mcpClient, err = client.NewStreamableHttpClient(server.URL, opts...)
	default: // ConnTypeSSE or fallback
		if strings.TrimSpace(server.URL) == "" {
			return nil, fmt.Errorf("MCP SSE server '%s' has no URL configured", server.Name)
		}
		var opts []transport.ClientOption
		if len(server.Headers) > 0 {
			opts = append(opts, client.WithHeaders(server.Headers))
		}
		mcpClient, err = client.NewSSEMCPClient(server.URL, opts...)
	}

	if err != nil {
		return nil, err
	}

	if err := mcpClient.Start(ctx); err != nil {
		return nil, err
	}

	return mcpClient, nil
}

func (s *mcpService) ListTools(ctx context.Context, serverID string) ([]domainMCP.Tool, error) {
	server, err := s.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// Create MCP Client
	location := server.URL
	if server.Type == domainMCP.ConnTypeStdio {
		location = server.Command
	}
	logrus.Infof("[MCP] Connecting to server %s (%s) at %s", server.Name, server.Type, location)
	mcpClient, err := s.getClient(ctx, server)
	if err != nil {
		logrus.WithError(err).Errorf("[MCP] Failed to create client for %s", serverID)
		return nil, err
	}
	defer mcpClient.Close()

	logrus.Infof("[MCP] Initializing session for %s", server.Name)
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "az-wap-bot", Version: "1.0.0"}

	// Retry initialization for SSE servers as they might take a moment to establish the session
	var initErr error
	for i := 0; i < 5; i++ {
		_, initErr = mcpClient.Initialize(ctx, initReq)
		if initErr == nil {
			break
		}
		errStr := initErr.Error()
		if strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "session") {
			time.Sleep(500 * time.Millisecond)
			logrus.Warnf("[MCP] Session not ready for %s, retrying (%d/5)...", server.Name, i+1)
			continue
		}
		break
	}

	if initErr != nil {
		logrus.WithError(initErr).Errorf("[MCP] Failed to initialize server %s", serverID)
		return nil, initErr
	}

	logrus.Infof("[MCP] Listing tools for %s", server.Name)
	toolsRes, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		logrus.WithError(err).Errorf("[MCP] Failed to list tools for server %s", serverID)
		return nil, err
	}

	var results []domainMCP.Tool
	for _, t := range toolsRes.Tools {
		results = append(results, domainMCP.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	// Cache tools in database
	if len(results) > 0 {
		toolsJSON, _ := json.Marshal(results)
		_, _ = s.db.ExecContext(ctx, "UPDATE mcp_servers SET tools = ? WHERE id = ?", string(toolsJSON), serverID)
	}

	return results, nil
}

func (s *mcpService) CallTool(ctx context.Context, botID string, req domainMCP.CallToolRequest) (domainMCP.CallToolResult, error) {
	logrus.Infof("[MCP] Bot %s calling tool %s on server %s", botID, req.ToolName, req.ServerID)
	server, err := s.GetServer(ctx, req.ServerID)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	// Fetch per-bot config to check for custom headers
	var configJSON sql.NullString
	_ = s.db.QueryRowContext(ctx, "SELECT config_json FROM bot_mcp_configs WHERE bot_id = ? AND server_id = ?", botID, req.ServerID).Scan(&configJSON)

	if configJSON.Valid && configJSON.String != "" {
		var cfg struct {
			CustomHeaders map[string]string `json:"custom_headers"`
		}
		if err := json.Unmarshal([]byte(configJSON.String), &cfg); err == nil {
			if server.Headers == nil {
				server.Headers = make(map[string]string)
			}
			for k, v := range cfg.CustomHeaders {
				// Decrypt value
				if decrypted, err := crypto.Decrypt(v); err == nil {
					server.Headers[k] = decrypted
				} else {
					server.Headers[k] = v // Fallback
				}
			}
		}
	}

	mcpClient, err := s.getClient(ctx, server)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}
	defer mcpClient.Close()

	// Initialize
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "az-wap-bot", Version: "1.0.0"}

	// Retry initialization
	for i := 0; i < 5; i++ {
		_, err = mcpClient.Initialize(ctx, initReq)
		if err == nil {
			break
		}
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "session") {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = req.ToolName
	callReq.Params.Arguments = req.Arguments

	res, err := mcpClient.CallTool(ctx, callReq)
	if err != nil {
		logrus.WithError(err).Errorf("[MCP] Tool call failed for %s", req.ToolName)
		if s.health != nil {
			s.health.ReportFailure(ctx, domainHealth.EntityMCP, req.ServerID, fmt.Sprintf("Tool call failed: %v", err))
		}
		return domainMCP.CallToolResult{}, err
	}

	if s.health != nil {
		s.health.ReportSuccess(ctx, domainHealth.EntityMCP, req.ServerID)
	}

	logrus.Infof("[MCP] Tool call %s successful (IsError: %v)", req.ToolName, res.IsError)
	var result domainMCP.CallToolResult
	result.IsError = res.IsError
	for _, content := range res.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			result.Content = append(result.Content, struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				Type: "text",
				Text: textContent.Text,
			})
		}
	}

	return result, nil
}

func (s *mcpService) GetBotTools(ctx context.Context, botID string) ([]domainMCP.Tool, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	query := `SELECT server_id FROM bot_mcp_configs WHERE bot_id = ? AND enabled = 1`
	rows, err := s.db.QueryContext(ctx, query, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allTools []domainMCP.Tool
	for rows.Next() {
		var serverID string
		if err := rows.Scan(&serverID); err == nil {
			// Get server
			srv, err := s.GetServer(ctx, serverID)
			if err != nil {
				continue
			}

			// Get configuration for this bot+server
			var configJSON sql.NullString
			_ = s.db.QueryRowContext(ctx, "SELECT config_json FROM bot_mcp_configs WHERE bot_id = ? AND server_id = ?", botID, serverID).Scan(&configJSON)

			var disabledTools []string
			if configJSON.Valid && configJSON.String != "" {
				var cfg struct {
					DisabledTools []string          `json:"disabled_tools"`
					CustomHeaders map[string]string `json:"custom_headers"` // Encrypted
					Instructions  string            `json:"instructions"`
				}
				if err := json.Unmarshal([]byte(configJSON.String), &cfg); err == nil {
					disabledTools = cfg.DisabledTools
					// Decrypt custom headers values?
					// Wait, the WHOLE CONFIG is JSON. We should encrypt values or the whole blob?
					// Implementation choice: We encrypted just the values map in UpdateBotMCPConfig?
					// Let's re-read UpdateBotMCPConfig...
					// Ah, we haven't updated UpdateBotMCPConfig yet to encrypt.
					// Let's assume we encrypt VALUES of CustomHeaders.

					for k, v := range cfg.CustomHeaders {
						if decrypted, err := crypto.Decrypt(v); err == nil {
							cfg.CustomHeaders[k] = decrypted
						}
					}

					// Merge headers for tool fetching if needed
					if srv.Headers == nil {
						srv.Headers = make(map[string]string)
					}
					for k, v := range cfg.CustomHeaders {
						srv.Headers[k] = v
					}
				}
			}

			disabledMap := make(map[string]bool)
			for _, dt := range disabledTools {
				disabledMap[dt] = true
			}

			// Try to list tools with the bot-aware server config
			// We call getClient directly here to avoid the bot-less ListTools
			tools := srv.Tools
			if len(tools) == 0 {
				mcpClient, err := s.getClient(ctx, srv)
				if err == nil {
					// We just need tool list, but it requires initialization
					initReq := mcp.InitializeRequest{}
					initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
					initReq.Params.ClientInfo = mcp.Implementation{Name: "az-wap-bot", Version: "1.0.0"}
					if _, err := mcpClient.Initialize(ctx, initReq); err == nil {
						if res, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{}); err == nil {
							for _, t := range res.Tools {
								tools = append(tools, domainMCP.Tool{
									Name:        t.Name,
									Description: t.Description,
									InputSchema: t.InputSchema,
								})
							}
						}
					}
					mcpClient.Close()
				}
			}

			for _, t := range tools {
				if !disabledMap[t.Name] {
					allTools = append(allTools, t)
				}
			}
		}
	}
	return allTools, nil
}

func (s *mcpService) ListServersForBot(ctx context.Context, botID string) ([]domainMCP.MCPServer, error) {
	servers, err := s.ListServers(ctx)
	if err != nil {
		return nil, err
	}

	// For each server, check if it's enabled for this bot
	for i := range servers {
		var enabled int
		var configJSON sql.NullString
		query := `SELECT enabled, config_json FROM bot_mcp_configs WHERE bot_id = ? AND server_id = ?`
		err := s.db.QueryRowContext(ctx, query, botID, servers[i].ID).Scan(&enabled, &configJSON)
		if err == nil {
			servers[i].Enabled = enabled != 0
			if configJSON.Valid && configJSON.String != "" {
				var cfg struct {
					DisabledTools []string          `json:"disabled_tools"`
					CustomHeaders map[string]string `json:"custom_headers"`
					Instructions  string            `json:"instructions"`
				}
				json.Unmarshal([]byte(configJSON.String), &cfg)
				servers[i].DisabledTools = cfg.DisabledTools
				servers[i].BotInstructions = cfg.Instructions // Map to dedicated bot-specific field
				// Decrypt custom headers for the UI/Validation
				decryptedHeaders := make(map[string]string)
				for k, v := range cfg.CustomHeaders {
					if dec, err := crypto.Decrypt(v); err == nil {
						decryptedHeaders[k] = dec
					} else {
						decryptedHeaders[k] = v // Fallback
					}
				}
				servers[i].CustomHeaders = decryptedHeaders
			}
		} else {
			servers[i].Enabled = false
		}
	}

	return servers, nil
}

func (s *mcpService) ToggleServerForBot(ctx context.Context, botID string, serverID string, enabled bool) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	status := 0
	if enabled {
		status = 1
	}

	query := `INSERT INTO bot_mcp_configs (bot_id, server_id, enabled) VALUES (?, ?, ?)
			  ON CONFLICT(bot_id, server_id) DO UPDATE SET enabled = ?`
	_, err := s.db.ExecContext(ctx, query, botID, serverID, status, status)
	return err
}

func (s *mcpService) UpdateBotMCPConfig(ctx context.Context, config domainMCP.BotMCPConfig) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	// BOT-SPECIFIC VALIDATION
	// If enabled, we MUST validate that we can connect and load tools using the bot's custom headers
	if config.Enabled {
		srv, err := s.GetServer(ctx, config.ServerID)
		if err == nil {
			if srv.Headers == nil {
				srv.Headers = make(map[string]string)
			}
			for k, v := range config.CustomHeaders {
				srv.Headers[k] = v // Use raw headers for validation here
			}

			if err := s.performFullValidation(ctx, srv); err != nil {
				if s.health != nil {
					s.health.ReportFailure(ctx, domainHealth.EntityMCP, config.ServerID, err.Error())
				}
				return fmt.Errorf("bot validation failed (check your headers): %w", err)
			}
			if s.health != nil {
				s.health.ReportSuccess(ctx, domainHealth.EntityMCP, config.ServerID)
			}
		}
	}

	// Encrypt Custom Headers Values for Storage
	encryptedHeaders := make(map[string]string)
	for k, v := range config.CustomHeaders {
		if enc, err := crypto.Encrypt(v); err == nil {
			encryptedHeaders[k] = enc
		} else {
			encryptedHeaders[k] = v // Should probably error, but fallback for now
		}
	}

	confJSON, _ := json.Marshal(struct {
		DisabledTools []string          `json:"disabled_tools"`
		CustomHeaders map[string]string `json:"custom_headers"`
		Instructions  string            `json:"instructions"`
	}{
		DisabledTools: config.DisabledTools,
		CustomHeaders: encryptedHeaders,
		Instructions:  config.Instructions,
	})

	enabledInt := 0
	if config.Enabled {
		enabledInt = 1
	}

	query := `INSERT INTO bot_mcp_configs (bot_id, server_id, enabled, config_json) VALUES (?, ?, ?, ?)
			  ON CONFLICT(bot_id, server_id) DO UPDATE SET enabled = ?, config_json = ?`
	_, err := s.db.ExecContext(ctx, query, config.BotID, config.ServerID, enabledInt, string(confJSON), enabledInt, string(confJSON))
	return err
}
func (s *mcpService) Validate(ctx context.Context, id string) error {
	server, err := s.GetServer(ctx, id)
	if err != nil {
		return err
	}

	if server.IsTemplate {
		// Templates only check if URL is reachable (Status 200 or accessible)
		// We can use a simple HTTP GET/HEAD or just start the client without full initialization
		return s.checkAvailability(ctx, server)
	}

	// Normal servers perform full initialization and list tools
	err = s.performFullValidation(ctx, server)
	if s.health != nil {
		if err != nil {
			s.health.ReportFailure(ctx, domainHealth.EntityMCP, id, err.Error())
		} else {
			s.health.ReportSuccess(ctx, domainHealth.EntityMCP, id)
		}
	}
	return err
}

func (s *mcpService) SetHealthUsecase(health domainHealth.IHealthUsecase) {
	s.health = health
}

func (s *mcpService) ListBotsUsingServer(ctx context.Context, serverID string) ([]string, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	var bots []string
	query := `SELECT bot_id FROM bot_mcp_configs WHERE server_id = ? AND enabled = 1`
	rows, err := s.db.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			bots = append(bots, id)
		}
	}
	return bots, nil
}

func (s *mcpService) checkAvailability(ctx context.Context, server domainMCP.MCPServer) error {
	if server.URL == "" {
		return fmt.Errorf("URL is required for this connection type")
	}

	// For SSE/HTTP, we perform a standard HTTP request to see if the endpoint is reachable.
	// We accept 200, 401, 403, 405 as "reachable" because a template might require Auth.
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Add basic headers if any
	for k, v := range server.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("server not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("server returned 404 Not Found")
	}

	return nil
}

func (s *mcpService) performFullValidation(ctx context.Context, server domainMCP.MCPServer) error {
	// First check basic connectivity
	if err := s.checkAvailability(ctx, server); err != nil {
		return err
	}

	mcpClient, err := s.getClient(ctx, server)
	if err != nil {
		return err
	}
	defer mcpClient.Close()

	// Initialize
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "az-wap-health-check", Version: "1.0.0"}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Initialize with retry logic for SSE transient errors
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = mcpClient.Initialize(timeoutCtx, initReq)
		if lastErr == nil {
			break
		}
		errStr := lastErr.Error()
		// If it's a 404 or session error, it might be the SSE session not being ready yet
		if strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "session") {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	if lastErr != nil {
		return fmt.Errorf("initialization failed (possible auth error): %w", lastErr)
	}

	// Try to list tools as a final verification
	if _, err := mcpClient.ListTools(timeoutCtx, mcp.ListToolsRequest{}); err != nil {
		return fmt.Errorf("listing tools failed: %w", err)
	}

	return nil
}

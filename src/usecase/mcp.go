package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/AzielCF/az-wap/config"
	domainMCP "github.com/AzielCF/az-wap/domains/mcp"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

type mcpService struct {
	db *sql.DB
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
			enabled INTEGER NOT NULL DEFAULT 1,
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

	return db, nil
}

func NewMCPService() domainMCP.IMCPUsecase {
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

	// Strict security check: Stdio is disabled by default
	if server.Type == domainMCP.ConnTypeStdio {
		return server, fmt.Errorf("system command execution (stdio) is strictly disabled for security reasons")
	}

	if server.Type == domainMCP.ConnTypeSSE {
		allowInsecure := os.Getenv("MCP_ALLOW_INSECURE_HTTP") == "true"
		if !allowInsecure && !strings.HasPrefix(server.URL, "https://") {
			return server, fmt.Errorf("insecure HTTP is not allowed. Use HTTPS or enable MCP_ALLOW_INSECURE_HTTP for local development")
		}
	}

	argsJSON := "[]" // simplified for now
	envJSON := "{}"

	query := `INSERT INTO mcp_servers (id, name, description, type, url, command, args, env, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, server.ID, server.Name, server.Description, string(server.Type), server.URL, server.Command, argsJSON, envJSON, 1)
	return server, err
}

func (s *mcpService) ListServers(ctx context.Context) ([]domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT id, name, description, type, url, command, enabled FROM mcp_servers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []domainMCP.MCPServer
	for rows.Next() {
		var srv domainMCP.MCPServer
		var enabledVal int
		var typeStr string
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Description, &typeStr, &srv.URL, &srv.Command, &enabledVal); err != nil {
			return nil, err
		}
		srv.Type = domainMCP.ConnectionType(typeStr)
		srv.Enabled = enabledVal != 0
		servers = append(servers, srv)
	}
	return servers, nil
}

func (s *mcpService) GetServer(ctx context.Context, id string) (domainMCP.MCPServer, error) {
	if err := s.ensureDB(); err != nil {
		return domainMCP.MCPServer{}, err
	}
	var srv domainMCP.MCPServer
	var enabledVal int
	var typeStr string
	query := "SELECT id, name, description, type, url, command, enabled FROM mcp_servers WHERE id = ?"
	err := s.db.QueryRowContext(ctx, query, id).Scan(&srv.ID, &srv.Name, &srv.Description, &typeStr, &srv.URL, &srv.Command, &enabledVal)
	srv.Type = domainMCP.ConnectionType(typeStr)
	srv.Enabled = enabledVal != 0
	return srv, err
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
	query := `UPDATE mcp_servers SET name = ?, description = ?, url = ?, enabled = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, server.Name, server.Description, server.URL, 1, id)
	return server, err
}

func (s *mcpService) ListTools(ctx context.Context, serverID string) ([]domainMCP.Tool, error) {
	server, err := s.GetServer(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if server.Type != domainMCP.ConnTypeSSE {
		return nil, fmt.Errorf("only SSE servers are currently supported for tool listing")
	}

	// Create MCP Client
	mcpClient, err := client.NewSSEMCPClient(server.URL)
	if err != nil {
		return nil, err
	}

	if err := mcpClient.Start(ctx); err != nil {
		return nil, err
	}
	defer mcpClient.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	initReq.Params.ClientInfo = mcp.Implementation{Name: "az-wap-bot", Version: "1.0.0"}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		return nil, err
	}

	toolsRes, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
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

	return results, nil
}

func (s *mcpService) CallTool(ctx context.Context, botID string, req domainMCP.CallToolRequest) (domainMCP.CallToolResult, error) {
	server, err := s.GetServer(ctx, req.ServerID)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	mcpClient, err := client.NewSSEMCPClient(server.URL)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

	if err := mcpClient.Start(ctx); err != nil {
		return domainMCP.CallToolResult{}, err
	}
	defer mcpClient.Close()

	// Initialize
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	_, _ = mcpClient.Initialize(ctx, initReq)

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = req.ToolName
	callReq.Params.Arguments = req.Arguments

	res, err := mcpClient.CallTool(ctx, callReq)
	if err != nil {
		return domainMCP.CallToolResult{}, err
	}

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
			tools, _ := s.ListTools(ctx, serverID)
			allTools = append(allTools, tools...)
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
		query := `SELECT enabled FROM bot_mcp_configs WHERE bot_id = ? AND server_id = ?`
		err := s.db.QueryRowContext(ctx, query, botID, servers[i].ID).Scan(&enabled)
		if err == nil {
			servers[i].Enabled = enabled != 0
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

package mcp

import (
	"context"

	domainHealth "github.com/AzielCF/az-wap/domains/health"
)

type ConnectionType string

const (
	ConnTypeStdio ConnectionType = "stdio"
	ConnTypeSSE   ConnectionType = "sse"
	ConnTypeHTTP  ConnectionType = "http"
)

type MCPServer struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Type            ConnectionType    `json:"type"`
	URL             string            `json:"url,omitempty"`     // For SSE
	Command         string            `json:"command,omitempty"` // For Stdio (disabled by default)
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Enabled         bool              `json:"enabled"`
	Tools           []Tool            `json:"tools,omitempty"`
	DisabledTools   []string          `json:"disabled_tools,omitempty"`
	CustomHeaders   map[string]string `json:"custom_headers,omitempty"`   // Per-bot header overrides
	IsTemplate      bool              `json:"is_template"`                // Defines if this server requires per-bot config
	TemplateConfig  map[string]string `json:"template_config,omitempty"`  // Required headers for template (Key: HeaderName, Value: HelperText)
	Instructions    string            `json:"instructions,omitempty"`     // Global instructions for this MCP server
	BotInstructions string            `json:"bot_instructions,omitempty"` // Bot-specific instructions for this MCP server
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type CallToolRequest struct {
	ServerID  string                 `json:"server_id"`
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type CallToolResult struct {
	Content []CallToolContent `json:"content"`
	IsError bool              `json:"is_error"`
}

type CallToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type BotMCPConfig struct {
	BotID         string            `json:"bot_id"`
	ServerID      string            `json:"server_id"`
	Enabled       bool              `json:"enabled"`
	DisabledTools []string          `json:"disabled_tools"` // List of tool names to hide from this bot
	CustomHeaders map[string]string `json:"custom_headers"` // Bot-specific headers (auth, etc)
	Instructions  string            `json:"instructions"`   // Bot-specific instructions for this MCP server
}

// BotMCPConfigJSON define el esquema exacto de config_json en la BD.
type BotMCPConfigJSON struct {
	DisabledTools []string          `json:"disabled_tools"`
	CustomHeaders map[string]string `json:"custom_headers"`
	Instructions  string            `json:"instructions"`
}

type IMCPUsecase interface {
	// Server Management
	AddServer(ctx context.Context, server MCPServer) (MCPServer, error)
	ListServers(ctx context.Context) ([]MCPServer, error)
	GetServer(ctx context.Context, id string) (MCPServer, error)
	DeleteServer(ctx context.Context, id string) error
	UpdateServer(ctx context.Context, id string, server MCPServer) (MCPServer, error)

	// Tools Interaction
	ListTools(ctx context.Context, serverID string) ([]Tool, error)
	CallTool(ctx context.Context, botID string, req CallToolRequest) (CallToolResult, error)

	// Bot specific
	GetBotTools(ctx context.Context, botID string) ([]Tool, error)
	ListServersForBot(ctx context.Context, botID string) ([]MCPServer, error)
	ToggleServerForBot(ctx context.Context, botID string, serverID string, enabled bool) error
	UpdateBotMCPConfig(ctx context.Context, config BotMCPConfig) error
	Validate(ctx context.Context, id string) error
	ListBotsUsingServer(ctx context.Context, serverID string) ([]string, error)
	SetHealthUsecase(health domainHealth.IHealthUsecase)
	Shutdown()
}

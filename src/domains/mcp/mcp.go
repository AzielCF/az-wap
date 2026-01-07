package mcp

import (
	"context"
)

type ConnectionType string

const (
	ConnTypeStdio ConnectionType = "stdio"
	ConnTypeSSE   ConnectionType = "sse"
)

type MCPServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        ConnectionType    `json:"type"`
	URL         string            `json:"url,omitempty"`     // For SSE
	Command     string            `json:"command,omitempty"` // For Stdio (disabled by default)
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Enabled     bool              `json:"enabled"`
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
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"is_error"`
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
}

package mcp

import (
	"context"
)

// IMCPRepository define el contrato para el acceso a datos de servidores MCP.
// Ubicado en el dominio (Clean Architecture / DIP).
type IMCPRepository interface {
	// Init inicializa el esquema de tablas y migraciones.
	Init(ctx context.Context) error

	// === MCP Servers ===

	// AddServer añade un nuevo servidor MCP.
	AddServer(ctx context.Context, server MCPServer) error

	// ListServers retorna todos los servidores MCP.
	ListServers(ctx context.Context) ([]MCPServer, error)

	// GetServer obtiene un servidor por su ID.
	GetServer(ctx context.Context, id string) (MCPServer, error)

	// UpdateServer actualiza un servidor MCP.
	UpdateServer(ctx context.Context, id string, server MCPServer) error

	// DeleteServer elimina un servidor por ID.
	DeleteServer(ctx context.Context, id string) error

	// UpdateServerTools actualiza el cache de tools de un servidor.
	UpdateServerTools(ctx context.Context, serverID string, tools []Tool) error

	// === Bot MCP Configs ===

	// GetBotServerIDs retorna los IDs de servidores habilitados para un bot.
	GetBotServerIDs(ctx context.Context, botID string) ([]string, error)

	// GetBotMCPConfig obtiene la configuración de un bot para un servidor específico.
	GetBotMCPConfig(ctx context.Context, botID, serverID string) (BotMCPConfigDB, error)

	// ListBotsUsingServer retorna los IDs de bots que usan un servidor.
	ListBotsUsingServer(ctx context.Context, serverID string) ([]string, error)

	// ToggleServerForBot habilita/deshabilita un servidor para un bot.
	ToggleServerForBot(ctx context.Context, botID, serverID string, enabled bool) error

	// SaveBotMCPConfig guarda la configuración de un bot para un servidor.
	SaveBotMCPConfig(ctx context.Context, botID, serverID string, enabled bool, configJSON string) error
}

// BotMCPConfigDB representa la fila de bot_mcp_configs en la base de datos.
type BotMCPConfigDB struct {
	Enabled    bool
	ConfigJSON string
}

package mcp

import "context"

// IMCPProvider define el puerto de comunicación con servidores MCP externos.
// Aísla el sistema de la librería mcp-go y protocolos específicos.
type IMCPProvider interface {
	// ListTools solicita la lista de herramientas a un servidor.
	ListTools(ctx context.Context, server MCPServer) ([]Tool, error)

	// CallTool ejecuta una herramienta en un servidor.
	CallTool(ctx context.Context, server MCPServer, toolName string, args map[string]interface{}) (CallToolResult, error)

	// Validate verifica la conectividad y disponibilidad de un servidor.
	Validate(ctx context.Context, server MCPServer, fullHandshake bool) ([]Tool, error)

	// Shutdown cierra todas las conexiones activas.
	Shutdown()
}

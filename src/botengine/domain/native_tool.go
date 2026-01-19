package domain

import (
	"context"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// NativeTool extiende la definición de MCP Tool con una función de ejecución local
type NativeTool struct {
	domainMCP.Tool
	Handler func(ctx context.Context, context map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error)
}

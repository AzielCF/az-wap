package domain

import (
	"context"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// ToolVisibilityCondition define si una herramienta debe ser visible para la IA en el contexto actual
type ToolVisibilityCondition func(input BotInput) bool

// NativeTool extiende la definición de MCP Tool con una función de ejecución local
type NativeTool struct {
	domainMCP.Tool
	Handler   func(ctx context.Context, context map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error)
	IsVisible ToolVisibilityCondition
}

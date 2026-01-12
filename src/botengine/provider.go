package botengine

import (
	"context"

	"github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// AIProvider es la interfaz que debe implementar cualquier proveedor de IA (Gemini, OpenAI, etc.)
type AIProvider interface {
	// GenerateReply genera una respuesta basada en el contexto y las herramientas MCP disponibles
	GenerateReply(ctx context.Context, b bot.Bot, input BotInput, tools []mcp.Tool) (BotOutput, error)
}

package domain

import (
	"context"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMcp "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// ChatRequest es una petición agnóstica de chat
type ChatRequest struct {
	SystemPrompt   string
	DynamicContext string // Contexto dinámico que cambia en cada mensaje (hora, focus score, etc.)
	History        []ChatTurn
	Tools          []domainMcp.Tool
	UserText       string
	Model          string
	ChatKey        string // Identificador único de sesión (ej: InstanceID|ChatID)
}

// UsageStats contiene estadísticas de tokens y costo de una respuesta
type UsageStats struct {
	Model         string  `json:"model"` // Nombre del modelo que generó el gasto
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	CachedTokens  int     `json:"cached_tokens"` // Tokens recuperados de la caché
	SystemTokens  int     `json:"system_tokens,omitempty"`
	UserTokens    int     `json:"user_tokens,omitempty"`
	HistoryTokens int     `json:"history_tokens,omitempty"`
	SystemCached  bool    `json:"system_cached,omitempty"` // True if system prompt came from cache
	CostUSD       float64 `json:"cost_usd"`
}

// ChatResponse es la respuesta agnóstica de un proveedor de IA
type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
	// RawContent almacena el contenido original del proveedor para iteraciones de herramientas.
	// Esto permite al Orchestrator re-inyectar el contenido exacto en la siguiente iteración.
	RawContent interface{}
	// Usage contiene estadísticas de tokens y costo estimado
	Usage *UsageStats
}

// MultimodalResult contiene el resultado del análisis de diversos medios
type MultimodalResult struct {
	Transcriptions []string
	Descriptions   []string
	Summaries      []string
	VideoSummaries []string
}

// MultimodalInterpreter es la interfaz que deben implementar los proveedores
// para analizar archivos multimedia.
type MultimodalInterpreter interface {
	Interpret(ctx context.Context, apiKey string, model string, userText string, language string, medias []*BotMedia) (*MultimodalResult, *UsageStats, error)
}

// AIProvider es la interfaz delgada que deben implementar los modelos
type AIProvider interface {
	// Chat envía el contexto y herramientas a la IA y devuelve texto o llamadas a herramientas
	Chat(ctx context.Context, b domainBot.Bot, req ChatRequest) (ChatResponse, error)

	// PreAnalyzeMindset analiza rápidamente el sentimiento y esfuerzo requerido.
	PreAnalyzeMindset(ctx context.Context, b domainBot.Bot, input BotInput, history []ChatTurn) (*Mindset, *UsageStats, error)
}

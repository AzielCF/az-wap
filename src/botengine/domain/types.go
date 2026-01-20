package domain

import "time"

// ToolCall representa una intención de la IA de llamar a una herramienta
type ToolCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// ToolResponse representa el resultado de la ejecución de una herramienta
type ToolResponse struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Data any    `json:"data"`
}

// ChatTurn represents a single turn in a conversation
type ChatTurn struct {
	Role          string         `json:"role"`
	Text          string         `json:"text,omitempty"`
	ToolCalls     []ToolCall     `json:"tool_calls,omitempty"`
	ToolResponses []ToolResponse `json:"tool_responses,omitempty"`
	// RawContent almacena el contenido original del proveedor (ej: *genai.Content)
	// para ser re-inyectado en iteraciones subsecuentes del bucle de herramientas.
	RawContent interface{} `json:"-"`
}

// Platform representa la plataforma de origen del mensaje
type Platform string

const (
	PlatformWhatsApp Platform = "whatsapp"
	PlatformTest     Platform = "test"
	PlatformWeb      Platform = "web"
)

// MediaState define el estado de procesamiento del medio
type MediaState string

const (
	MediaStateAvailable MediaState = "available" // Recurso disponible en disco pero no leído por la IA
	MediaStateAnalyzed  MediaState = "analyzed"  // Contenido enviado y procesado por la IA
	MediaStateBlocked   MediaState = "blocked"   // Descarga bloqueada por el canal
)

// BotMedia representa un archivo adjunto (imagen, audio, etc.)
type BotMedia struct {
	Data      []byte     `json:"-"`
	MimeType  string     `json:"mime_type"`
	FileName  string     `json:"file_name,omitempty"`
	LocalPath string     `json:"local_path,omitempty"`
	State     MediaState `json:"state"`
}

// BotInput es la estructura agnóstica de entrada para el motor del bot
type BotInput struct {
	BotID         string         `json:"bot_id"`
	WorkspaceID   string         `json:"workspace_id"` // Adding WorkspaceID for scoped memory
	SenderID      string         `json:"sender_id"`    // JID o ID de usuario único en la plataforma
	ChatID        string         `json:"chat_id"`      // ID de la conversación
	Platform      Platform       `json:"platform"`
	Text          string         `json:"text"`
	History       []ChatTurn     `json:"history,omitempty"` // Conversation history from the channel session
	Media         *BotMedia      `json:"media,omitempty"`   // Deprecated: Use Medias
	Medias        []*BotMedia    `json:"medias,omitempty"`
	InstanceID    string         `json:"instance_id"` // Útil para saber qué instancia física recibió el mensaje
	TraceID       string         `json:"trace_id,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	OnChatOpen    func()         `json:"-"` // Callback para notificar apertura visual del chat
	FocusScore    int            `json:"focus_score"`
	LastMindset   *Mindset       `json:"last_mindset,omitempty"`
	PendingTasks  []string       `json:"pending_tasks,omitempty"`
	LastReplyTime time.Time      `json:"last_reply_time,omitempty"`
	Language      string         `json:"language,omitempty"` // Idioma resuelto para la respuesta (es, en, etc.)

	// Client Context - Información del cliente registrado (si existe)
	ClientContext *ClientContext `json:"client_context,omitempty"`
}

// Mindset representa la mentalidad situacional de la IA para una respuesta
type Mindset struct {
	Pace            string `json:"pace"`            // fast, steady, deep
	Focus           bool   `json:"focus"`           // true si debe entrar en modo enfoque
	Work            bool   `json:"work"`            // true si realizó una tarea pesada
	Acknowledgement string `json:"acknowledgement"` // Respuesta rápida inmediata (ej: "Un momento...")
	ShouldRespond   bool   `json:"should_respond"`  // Determina si realmente vale la pena responder
	EnqueueTask     string `json:"enqueue_task"`    // Descripción de una tarea para la cola de espera
	ClearTasks      bool   `json:"clear_tasks"`     // True si las tareas pendientes han sido resueltas
}

// ExecutionCost representa el costo de una parte de la ejecución (vía un modelo específico)
type ExecutionCost struct {
	BotID string  `json:"bot_id"`
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
}

// BotOutput es la estructura de respuesta generada por el cerebro del bot
type BotOutput struct {
	Text        string          `json:"text"`
	Action      string          `json:"action,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	Mindset     *Mindset        `json:"mindset,omitempty"`
	TotalCost   float64         `json:"total_cost,omitempty"`   // Costo acumulado de esta ejecución en USD
	CostDetails []ExecutionCost `json:"cost_details,omitempty"` // Desglose por bot/modelo
}

// PresenceConfig centraliza los tiempos y umbrales de la humanización situacional
type PresenceConfig struct {
	ImmediateReadWindow  time.Duration // Tiempo tras responder donde el visto es instantáneo
	HighFocusThreshold   int           // Score para entrar en modo enfoque alto
	MediumFocusThreshold int           // Score para enfoque moderado
	NoticeDelayBase      time.Duration // Tiempo base para "abrir" el chat
}

var DefaultPresenceConfig = PresenceConfig{
	ImmediateReadWindow:  5 * time.Second,
	HighFocusThreshold:   70,
	MediumFocusThreshold: 40,
	NoticeDelayBase:      1000 * time.Millisecond,
}

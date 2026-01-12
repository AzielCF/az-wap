package botengine

// Platform representa la plataforma de origen del mensaje
type Platform string

const (
	PlatformWhatsApp Platform = "whatsapp"
	PlatformTest     Platform = "test"
	PlatformWeb      Platform = "web"
)

// BotMedia representa un archivo adjunto (imagen, audio, etc.)
type BotMedia struct {
	Data     []byte `json:"-"`
	MimeType string `json:"mime_type"`
	FileName string `json:"file_name,omitempty"`
}

// BotInput es la estructura agnóstica de entrada para el motor del bot
type BotInput struct {
	BotID       string         `json:"bot_id"`
	WorkspaceID string         `json:"workspace_id"` // Adding WorkspaceID for scoped memory
	SenderID    string         `json:"sender_id"`    // JID o ID de usuario único en la plataforma
	ChatID      string         `json:"chat_id"`      // ID de la conversación
	Platform    Platform       `json:"platform"`
	Text        string         `json:"text"`
	Media       *BotMedia      `json:"media,omitempty"`
	InstanceID  string         `json:"instance_id"` // Útil para saber qué instancia física recibió el mensaje
	TraceID     string         `json:"trace_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// BotOutput es la estructura de respuesta generada por el cerebro del bot
type BotOutput struct {
	Text     string         `json:"text"`
	Action   string         `json:"action,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

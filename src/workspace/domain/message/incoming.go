package message

// IncomingMedia representa el contenido multimedia recibido de un canal
type IncomingMedia struct {
	Path     string `json:"path"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption,omitempty"`
	Blocked  bool   `json:"blocked,omitempty"` // Indica si la descarga fue bloqueada
}

// IncomingMessage representa un mensaje recibido de cualquier canal
type IncomingMessage struct {
	WorkspaceID string
	ChannelID   string
	ChatID      string
	SenderID    string
	IsStatus    bool // True si el mensaje es una historia/status
	Text        string
	Language    string // Idioma resuelto (es, en, etc.)
	Media       *IncomingMedia
	Medias      []*IncomingMedia // Para mensajes agrupados
	Metadata    map[string]any
}

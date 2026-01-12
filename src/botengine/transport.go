package botengine

import "context"

// Transport define los métodos que una plataforma (ej. WhatsApp) debe implementar
// para que el bot pueda interactuar con ella de forma agnóstica.
type Transport interface {
	// ID devuelve el identificador único del transporte (nombre de la instancia o plataforma)
	ID() string

	// SendMessage envía un mensaje de texto plano
	SendMessage(ctx context.Context, chatID string, text string) error

	// SendPresence envía estados como "composing" o "paused"
	SendPresence(ctx context.Context, chatID string, isTyping bool) error
}

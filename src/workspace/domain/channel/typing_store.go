package channel

import (
	"context"
	"time"
)

// TypingMedia define el tipo de contenido que se está componiendo.
type TypingMedia string

const (
	TypingMediaText  TypingMedia = "text"
	TypingMediaAudio TypingMedia = "audio"
)

// TypingState representa quién está escribiendo en qué chat.
type TypingState struct {
	ChannelID string      `json:"channel_id"`
	ChatID    string      `json:"chat_id"`
	Media     TypingMedia `json:"media"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// TypingStore define el contrato para persistir estados temporales de escritura.
type TypingStore interface {
	// Update registra o actualiza el estado de escritura de un usuario.
	Update(ctx context.Context, channelID, chatID string, isTyping bool, media TypingMedia) error

	// Get recupera el estado de escritura actual de un chat.
	// Devuelve nil si no está escribiendo o si expiró.
	Get(ctx context.Context, channelID, chatID string) (*TypingState, error)

	// GetAll devuelve todos los estados de escritura activos y no expirados.
	GetAll(ctx context.Context) ([]TypingState, error)
}

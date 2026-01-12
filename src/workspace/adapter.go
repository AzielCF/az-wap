package workspace

import (
	"context"

	"github.com/AzielCF/az-wap/workspace/domain"
)

// ChannelAdapter defines the interface that all channel implementations must satisfy
type ChannelAdapter interface {
	// Identity
	ID() string
	Type() domain.ChannelType
	Status() domain.ChannelStatus

	// Lifecycle
	Start(ctx context.Context, config domain.ChannelConfig) error
	Stop(ctx context.Context) error

	// Messaging
	SendMessage(ctx context.Context, chatID, text string) error
	SendPresence(ctx context.Context, chatID string, typing bool) error

	// Event handling
	OnMessage(handler func(IncomingMessage))
}

// IncomingMessage represents a message received from any channel
type IncomingMessage struct {
	WorkspaceID string
	ChannelID   string
	ChatID      string
	SenderID    string
	Text        string
	Media       any // Platform-specific media
	Metadata    map[string]any
}

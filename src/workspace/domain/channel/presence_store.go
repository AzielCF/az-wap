package channel

import (
	"context"
	"time"
)

// ChannelPresence representa el estado de presencia y hibernación de un canal.
type ChannelPresence struct {
	ChannelID string    `json:"channel_id"`
	LastSeen  time.Time `json:"last_seen"`

	// Tiempos objetivos para acciones automáticas
	VisualOfflineAt time.Time `json:"visual_offline_at"`
	DeepHibernateAt time.Time `json:"deep_hibernate_at"`

	// Flags de estado
	IsVisuallyOnline  bool `json:"is_visually_online"`
	IsSocketConnected bool `json:"is_socket_connected"`
}

// PresenceStore define el contrato para persistir el estado de presencia de los canales.
type PresenceStore interface {
	// Save guarda o actualiza el estado de presencia de un canal.
	Save(ctx context.Context, presence *ChannelPresence) error

	// Get recupera el estado de presencia de un canal.
	Get(ctx context.Context, channelID string) (*ChannelPresence, error)

	// Delete elimina el registro de presencia de un canal.
	Delete(ctx context.Context, channelID string) error

	// GetAll devuelve todos los estados de presencia registrados.
	GetAll(ctx context.Context) ([]*ChannelPresence, error)
}

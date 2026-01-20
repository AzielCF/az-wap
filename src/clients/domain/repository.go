package domain

import (
	"context"
	"time"
)

// ClientFilter define los criterios de filtrado para listar clientes
type ClientFilter struct {
	Tier      *ClientTier
	Tags      []string
	Enabled   *bool
	Search    string
	Limit     int
	Offset    int
	OrderBy   string
	OrderDesc bool
}

// ClientRepository define las operaciones de persistencia para clientes (app.db)
type ClientRepository interface {
	// CRUD básico
	Create(ctx context.Context, client *Client) error
	GetByID(ctx context.Context, id string) (*Client, error)
	GetByPlatform(ctx context.Context, platformID string, platformType PlatformType) (*Client, error)
	GetByPhone(ctx context.Context, phone string) (*Client, error)
	Update(ctx context.Context, client *Client) error
	Delete(ctx context.Context, id string) error

	// Listados
	List(ctx context.Context, filter ClientFilter) ([]*Client, error)
	ListByTier(ctx context.Context, tier ClientTier) ([]*Client, error)
	ListByTag(ctx context.Context, tag string) ([]*Client, error)

	// Búsqueda
	Search(ctx context.Context, query string) ([]*Client, error)

	// Estadísticas
	CountByTier(ctx context.Context) (map[ClientTier]int, error)

	// Actualizaciones parciales
	UpdateLastInteraction(ctx context.Context, id string, t time.Time) error
	AddTag(ctx context.Context, id string, tag string) error
	RemoveTag(ctx context.Context, id string, tag string) error
}

// SubscriptionRepository define las operaciones de persistencia para suscripciones (workspaces.db)
type SubscriptionRepository interface {
	// CRUD
	Create(ctx context.Context, sub *ClientSubscription) error
	GetByID(ctx context.Context, id string) (*ClientSubscription, error)
	Update(ctx context.Context, sub *ClientSubscription) error
	Delete(ctx context.Context, id string) error

	// Consultas clave
	GetByClientAndChannel(ctx context.Context, clientID, channelID string) (*ClientSubscription, error)
	ListByClient(ctx context.Context, clientID string) ([]*ClientSubscription, error)
	ListByChannel(ctx context.Context, channelID string) ([]*ClientSubscription, error)

	// Consulta optimizada para resolución en runtime
	GetActiveSubscription(ctx context.Context, clientID, channelID string) (*ClientSubscription, error)

	// Mantenimiento
	ExpireOldSubscriptions(ctx context.Context) (int, error)
	DeleteByClientID(ctx context.Context, clientID string) error

	// Estadísticas
	CountByChannel(ctx context.Context, channelID string) (int, error)
	CountActiveByChannel(ctx context.Context, channelID string) (int, error)
}

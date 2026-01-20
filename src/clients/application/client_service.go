package application

import (
	"context"
	"time"

	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/google/uuid"
)

// ClientService contiene la lógica de negocio para la gestión de clientes
type ClientService struct {
	clientRepo domain.ClientRepository
	subRepo    domain.SubscriptionRepository
}

// NewClientService crea una nueva instancia de ClientService
func NewClientService(clientRepo domain.ClientRepository, subRepo domain.SubscriptionRepository) *ClientService {
	return &ClientService{
		clientRepo: clientRepo,
		subRepo:    subRepo,
	}
}

// Create crea un nuevo cliente
func (s *ClientService) Create(ctx context.Context, client *domain.Client) error {
	if client.ID == "" {
		client.ID = uuid.New().String()
	}
	if client.Tier == "" {
		client.Tier = domain.TierStandard
	}
	if client.Tags == nil {
		client.Tags = []string{}
	}
	if client.Metadata == nil {
		client.Metadata = make(map[string]any)
	}
	client.Enabled = true
	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()

	return s.clientRepo.Create(ctx, client)
}

// GetByID obtiene un cliente por su ID
func (s *ClientService) GetByID(ctx context.Context, id string) (*domain.Client, error) {
	return s.clientRepo.GetByID(ctx, id)
}

// GetByPlatform obtiene un cliente por su identificador de plataforma
func (s *ClientService) GetByPlatform(ctx context.Context, platformID string, platformType domain.PlatformType) (*domain.Client, error) {
	return s.clientRepo.GetByPlatform(ctx, platformID, platformType)
}

// Update actualiza un cliente existente
func (s *ClientService) Update(ctx context.Context, client *domain.Client) error {
	client.UpdatedAt = time.Now()
	return s.clientRepo.Update(ctx, client)
}

// Delete elimina un cliente y todas sus suscripciones
func (s *ClientService) Delete(ctx context.Context, id string) error {
	// Primero eliminar suscripciones (cross-db, por eso se hace manualmente)
	if err := s.subRepo.DeleteByClientID(ctx, id); err != nil {
		return err
	}
	return s.clientRepo.Delete(ctx, id)
}

// List obtiene una lista de clientes con filtros
func (s *ClientService) List(ctx context.Context, filter domain.ClientFilter) ([]*domain.Client, error) {
	return s.clientRepo.List(ctx, filter)
}

// Search busca clientes por texto libre
func (s *ClientService) Search(ctx context.Context, query string) ([]*domain.Client, error) {
	return s.clientRepo.Search(ctx, query)
}

// Enable habilita un cliente
func (s *ClientService) Enable(ctx context.Context, id string) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	client.Enabled = true
	return s.clientRepo.Update(ctx, client)
}

// Disable deshabilita un cliente
func (s *ClientService) Disable(ctx context.Context, id string) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	client.Enabled = false
	return s.clientRepo.Update(ctx, client)
}

// UpdateTier actualiza el tier de un cliente
func (s *ClientService) UpdateTier(ctx context.Context, id string, tier domain.ClientTier) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	client.Tier = tier
	return s.clientRepo.Update(ctx, client)
}

// AddTag agrega un tag a un cliente
func (s *ClientService) AddTag(ctx context.Context, id string, tag string) error {
	return s.clientRepo.AddTag(ctx, id, tag)
}

// RemoveTag elimina un tag de un cliente
func (s *ClientService) RemoveTag(ctx context.Context, id string, tag string) error {
	return s.clientRepo.RemoveTag(ctx, id, tag)
}

// GetStats obtiene estadísticas de clientes por tier
func (s *ClientService) GetStats(ctx context.Context) (map[domain.ClientTier]int, error) {
	return s.clientRepo.CountByTier(ctx)
}

// RecordInteraction registra la última interacción de un cliente
func (s *ClientService) RecordInteraction(ctx context.Context, id string) error {
	return s.clientRepo.UpdateLastInteraction(ctx, id, time.Now())
}

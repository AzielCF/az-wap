package application

import (
	"context"
	"time"

	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/google/uuid"
)

// SubscriptionService contiene la lógica de negocio para la gestión de suscripciones
type SubscriptionService struct {
	subRepo    domain.SubscriptionRepository
	clientRepo domain.ClientRepository
}

// NewSubscriptionService crea una nueva instancia de SubscriptionService
func NewSubscriptionService(subRepo domain.SubscriptionRepository, clientRepo domain.ClientRepository) *SubscriptionService {
	return &SubscriptionService{
		subRepo:    subRepo,
		clientRepo: clientRepo,
	}
}

// Create crea una nueva suscripción
func (s *SubscriptionService) Create(ctx context.Context, sub *domain.ClientSubscription) error {
	// Validar que el cliente existe
	client, err := s.clientRepo.GetByID(ctx, sub.ClientID)
	if err != nil {
		return err
	}
	if !client.Enabled {
		return domain.ErrClientDisabled
	}

	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	if sub.Status == "" {
		sub.Status = domain.SubscriptionActive
	}
	if sub.CustomConfig == nil {
		sub.CustomConfig = make(map[string]any)
	}
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()

	return s.subRepo.Create(ctx, sub)
}

// GetByID obtiene una suscripción por su ID
func (s *SubscriptionService) GetByID(ctx context.Context, id string) (*domain.ClientSubscription, error) {
	return s.subRepo.GetByID(ctx, id)
}

// Update actualiza una suscripción existente
func (s *SubscriptionService) Update(ctx context.Context, sub *domain.ClientSubscription) error {
	sub.UpdatedAt = time.Now()
	return s.subRepo.Update(ctx, sub)
}

// Delete elimina una suscripción
func (s *SubscriptionService) Delete(ctx context.Context, id string) error {
	return s.subRepo.Delete(ctx, id)
}

// ListByClient lista todas las suscripciones de un cliente
func (s *SubscriptionService) ListByClient(ctx context.Context, clientID string) ([]*domain.ClientSubscription, error) {
	return s.subRepo.ListByClient(ctx, clientID)
}

// ListByChannel lista todas las suscripciones de un canal
func (s *SubscriptionService) ListByChannel(ctx context.Context, channelID string) ([]*domain.ClientSubscription, error) {
	return s.subRepo.ListByChannel(ctx, channelID)
}

// Pause pausal una suscripción
func (s *SubscriptionService) Pause(ctx context.Context, id string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.Status = domain.SubscriptionPaused
	return s.subRepo.Update(ctx, sub)
}

// Resume reactiva una suscripción pausada
func (s *SubscriptionService) Resume(ctx context.Context, id string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.Status = domain.SubscriptionActive
	return s.subRepo.Update(ctx, sub)
}

// Revoke revoca una suscripción (permanente)
func (s *SubscriptionService) Revoke(ctx context.Context, id string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.Status = domain.SubscriptionRevoked
	return s.subRepo.Update(ctx, sub)
}

// SetExpiration establece la fecha de expiración de una suscripción
func (s *SubscriptionService) SetExpiration(ctx context.Context, id string, expiresAt *time.Time) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.ExpiresAt = expiresAt
	return s.subRepo.Update(ctx, sub)
}

// SetCustomBot asigna un bot personalizado a una suscripción
func (s *SubscriptionService) SetCustomBot(ctx context.Context, id string, botID string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.CustomBotID = botID
	return s.subRepo.Update(ctx, sub)
}

// SetCustomPrompt asigna un prompt personalizado a una suscripción
func (s *SubscriptionService) SetCustomPrompt(ctx context.Context, id string, prompt string) error {
	sub, err := s.subRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sub.CustomSystemPrompt = prompt
	return s.subRepo.Update(ctx, sub)
}

// ExpireOldSubscriptions marca suscripciones expiradas
func (s *SubscriptionService) ExpireOldSubscriptions(ctx context.Context) (int, error) {
	return s.subRepo.ExpireOldSubscriptions(ctx)
}

// GetChannelStats obtiene estadísticas de un canal
func (s *SubscriptionService) GetChannelStats(ctx context.Context, channelID string) (total int, active int, err error) {
	total, err = s.subRepo.CountByChannel(ctx, channelID)
	if err != nil {
		return 0, 0, err
	}
	active, err = s.subRepo.CountActiveByChannel(ctx, channelID)
	if err != nil {
		return 0, 0, err
	}
	return total, active, nil
}

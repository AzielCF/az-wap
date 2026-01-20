package application

import (
	"context"
	"errors"
	"strings"
	"time"

	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/clients/domain"
	channelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/sirupsen/logrus"
)

// ChannelResolver es una interfaz para resolver información del canal
type ChannelResolver interface {
	GetChannel(ctx context.Context, channelID string) (channelDomain.Channel, error)
}

// ClientResolver resuelve el contexto de un cliente para un mensaje entrante
type ClientResolver struct {
	clientRepo  domain.ClientRepository
	subRepo     domain.SubscriptionRepository
	channelRepo ChannelResolver
}

// NewClientResolver crea una nueva instancia del resolver
func NewClientResolver(clientRepo domain.ClientRepository, subRepo domain.SubscriptionRepository, channelRepo ChannelResolver) *ClientResolver {
	return &ClientResolver{
		clientRepo:  clientRepo,
		subRepo:     subRepo,
		channelRepo: channelRepo,
	}
}

// Resolve determina el contexto completo para un mensaje entrante
func (r *ClientResolver) Resolve(ctx context.Context, platformID, secondaryID, platformType string, channelID string) (*botengineDomain.ClientContext, string, error) {
	// 1. Obtener info del canal
	channel, err := r.channelRepo.GetChannel(ctx, channelID)
	if err != nil {
		return nil, "", err
	}

	// Bot default del canal
	resolvedBotID := channel.Config.BotID

	// 2. Buscar cliente global
	client, err := r.clientRepo.GetByPlatform(ctx, platformID, domain.PlatformType(platformType))
	if err != nil && !errors.Is(err, domain.ErrClientNotFound) {
		return nil, "", err
	}

	// Fallback 1: Intentar con secondaryID si existe
	if client == nil && secondaryID != "" && secondaryID != platformID {
		client, err = r.clientRepo.GetByPlatform(ctx, secondaryID, domain.PlatformType(platformType))
		if err != nil && !errors.Is(err, domain.ErrClientNotFound) {
			return nil, "", err
		}
	}

	// Fallback 2: Intentar por teléfono extraído de platformID o secondaryID
	if client == nil {
		phoneCandidate := platformID
		if strings.Contains(secondaryID, "@") && !strings.Contains(platformID, "@") {
			// Prefer JID over LID for phone extraction
			phoneCandidate = secondaryID
		}

		client, err = r.clientRepo.GetByPhone(ctx, phoneCandidate)
		if err != nil && !errors.Is(err, domain.ErrClientNotFound) {
			return nil, "", err
		}
	}

	clientCtx := &domain.ClientContext{
		ResolvedBotID: resolvedBotID,
	}

	if client != nil && client.Enabled {
		logrus.Infof("[ClientResolver] Client FOUND: %s (ID: %s, Tier: %s)", client.DisplayName, client.ID, client.Tier)

		// AUTOMATIC MIGRATION: If we found via phone/JID fallback but msg has a WhatsApp LID,
		// update the client's platform_id to the LID to make it the primary identifier.
		if platformType == "whatsapp" && strings.HasSuffix(platformID, "@lid") && client.PlatformID != platformID {
			logrus.Infof("[ClientResolver] Migrating client %s (%s) from legacy ID %s to new LID %s",
				client.DisplayName, client.ID, client.PlatformID, platformID)
			client.PlatformID = platformID
			_ = r.clientRepo.Update(ctx, client)
		}

		clientCtx.Client = client
		clientCtx.IsRegistered = true
		clientCtx.IsVIP = client.IsVIP()
		clientCtx.IsPremium = client.IsPremium()

		// Actualizar última interacción
		now := time.Now()
		_ = r.clientRepo.UpdateLastInteraction(ctx, client.ID, now)

		// 3. Buscar suscripción activa en este canal
		sub, err := r.subRepo.GetActiveSubscription(ctx, client.ID, channelID)
		if err == nil && sub != nil && sub.IsActive() {
			logrus.Infof("[ClientResolver] Active subscription FOUND for client %s in channel %s", client.ID, channelID)
			clientCtx.Subscription = sub
			clientCtx.HasSubscription = true
			if sub.CustomBotID != "" {
				resolvedBotID = sub.CustomBotID
				clientCtx.ResolvedBotID = resolvedBotID
				logrus.Infof("[ClientResolver] Applying custom bot ID: %s", resolvedBotID)
			}

			// Override de system prompt si existe
			if sub.CustomSystemPrompt != "" {
				clientCtx.AdditionalPrompt = sub.CustomSystemPrompt
				logrus.Infof("[ClientResolver] Applying custom system prompt (length: %d)", len(sub.CustomSystemPrompt))
			}
		} else {
			logrus.Warnf("[ClientResolver] No active subscription found for client %s in channel %s", client.ID, channelID)
		}
	}

	botCtx := ToBotEngineContext(clientCtx)
	if botCtx != nil {
		if clientCtx.Client != nil && clientCtx.Client.Language != "" {
			botCtx.Language = clientCtx.Client.Language
		} else if channel.Config.DefaultLanguage != "" {
			botCtx.Language = channel.Config.DefaultLanguage
		}
	}

	return botCtx, resolvedBotID, nil
}

// ResolveQuick hace una resolución rápida sin buscar suscripción (para verificaciones básicas)
func (r *ClientResolver) ResolveQuick(ctx context.Context, platformID string, platformType domain.PlatformType) (*domain.Client, error) {
	return r.clientRepo.GetByPlatform(ctx, platformID, platformType)
}

// ToBotEngineContext convierte el ClientContext del módulo clients al tipo del botengine
func ToBotEngineContext(ctx *domain.ClientContext) *botengineDomain.ClientContext {
	if ctx == nil || !ctx.IsRegistered {
		return nil
	}

	result := &botengineDomain.ClientContext{
		IsRegistered:    ctx.IsRegistered,
		HasSubscription: ctx.HasSubscription,
		IsVIP:           ctx.IsVIP,
		IsPremium:       ctx.IsPremium,
	}

	if ctx.Client != nil {
		result.ClientID = ctx.Client.ID
		result.DisplayName = ctx.Client.DisplayName
		result.Email = ctx.Client.Email
		result.Phone = ctx.Client.Phone
		result.Tier = string(ctx.Client.Tier)
		result.Tags = ctx.Client.Tags
		result.Language = ctx.Client.Language
		result.AllowedBots = ctx.Client.AllowedBots
		result.Metadata = ctx.Client.Metadata

		// Extraer nombre social de metadata
		if ctx.Client.Metadata != nil {
			if name, ok := ctx.Client.Metadata["name"].(string); ok {
				result.SocialName = name
			}
		}
	}

	if ctx.Subscription != nil {
		result.CustomSystemPrompt = ctx.Subscription.CustomSystemPrompt
	}

	return result
}

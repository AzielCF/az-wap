package rest

import (
	"github.com/AzielCF/az-wap/clients/application"
	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/gofiber/fiber/v2"
)

// ClientHandler maneja las peticiones REST para clientes
type ClientHandler struct {
	clientService *application.ClientService
	subService    *application.SubscriptionService
}

// NewClientHandler crea una nueva instancia del handler
func NewClientHandler(clientService *application.ClientService, subService *application.SubscriptionService) *ClientHandler {
	return &ClientHandler{
		clientService: clientService,
		subService:    subService,
	}
}

// RegisterRoutes registra las rutas de clientes en el router de Fiber
func (h *ClientHandler) RegisterRoutes(router fiber.Router) {
	clients := router.Group("/clients")

	// CRUD de clientes
	clients.Get("/", h.ListClients)
	clients.Post("/", h.CreateClient)
	clients.Get("/search", h.SearchClients)
	clients.Get("/stats", h.GetStats)
	clients.Get("/:id", h.GetClient)
	clients.Put("/:id", h.UpdateClient)
	clients.Delete("/:id", h.DeleteClient)

	// Tags
	clients.Post("/:id/tags", h.AddTag)
	clients.Delete("/:id/tags/:tag", h.RemoveTag)

	// Suscripciones del cliente
	clients.Get("/:id/subscriptions", h.ListClientSubscriptions)
	clients.Post("/:id/subscriptions", h.CreateSubscription)
	clients.Put("/:id/subscriptions/:subId", h.UpdateSubscription)
	clients.Delete("/:id/subscriptions/:subId", h.DeleteSubscription)

	// Tier management
	clients.Put("/:id/tier", h.UpdateTier)
	clients.Put("/:id/enable", h.EnableClient)
	clients.Put("/:id/disable", h.DisableClient)

	// Suscripciones por canal
	router.Get("/channels/:channelId/subscribers", h.ListChannelSubscribers)
}

// ListClients lista clientes con filtros
func (h *ClientHandler) ListClients(c *fiber.Ctx) error {
	filter := domain.ClientFilter{
		Search:    c.Query("search"),
		Limit:     c.QueryInt("limit", 50),
		Offset:    c.QueryInt("offset", 0),
		OrderBy:   c.Query("order_by", "created_at"),
		OrderDesc: c.QueryBool("order_desc", true),
	}

	if tier := c.Query("tier"); tier != "" {
		t := domain.ClientTier(tier)
		filter.Tier = &t
	}

	if enabled := c.Query("enabled"); enabled != "" {
		e := enabled == "true"
		filter.Enabled = &e
	}

	clients, err := h.clientService.List(c.Context(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": clients, "count": len(clients)})
}

// CreateClient crea un nuevo cliente
func (h *ClientHandler) CreateClient(c *fiber.Ctx) error {
	var req CreateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	client := &domain.Client{
		PlatformID:   req.PlatformID,
		PlatformType: domain.PlatformType(req.PlatformType),
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Phone:        req.Phone,
		Tier:         domain.ClientTier(req.Tier),
		Tags:         req.Tags,
		Metadata:     req.Metadata,
		Notes:        req.Notes,
		Language:     req.Language,
		AllowedBots:  req.AllowedBots,
	}

	if err := h.clientService.Create(c.Context(), client); err != nil {
		if err == domain.ErrDuplicateClient {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(client)
}

// GetClient obtiene un cliente por ID
func (h *ClientHandler) GetClient(c *fiber.Ctx) error {
	id := c.Params("id")

	client, err := h.clientService.GetByID(c.Context(), id)
	if err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(client)
}

// UpdateClient actualiza un cliente
func (h *ClientHandler) UpdateClient(c *fiber.Ctx) error {
	id := c.Params("id")

	client, err := h.clientService.GetByID(c.Context(), id)
	if err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req UpdateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Actualizar campos
	if req.PlatformID != nil {
		client.PlatformID = *req.PlatformID
	}
	if req.DisplayName != nil {
		client.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		client.Email = *req.Email
	}
	if req.Phone != nil {
		client.Phone = *req.Phone
	}
	if req.Tier != nil {
		client.Tier = domain.ClientTier(*req.Tier)
	}
	if req.Tags != nil {
		client.Tags = req.Tags
	}
	if req.Metadata != nil {
		client.Metadata = req.Metadata
	}
	if req.Notes != nil {
		client.Notes = *req.Notes
	}
	if req.Language != nil {
		client.Language = *req.Language
	}
	if req.AllowedBots != nil {
		client.AllowedBots = req.AllowedBots
	}

	if err := h.clientService.Update(c.Context(), client); err != nil {
		if err == domain.ErrDuplicateClient {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(client)
}

// DeleteClient elimina un cliente
func (h *ClientHandler) DeleteClient(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.clientService.Delete(c.Context(), id); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// SearchClients busca clientes por texto
func (h *ClientHandler) SearchClients(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Query parameter 'q' is required"})
	}

	clients, err := h.clientService.Search(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": clients, "count": len(clients)})
}

// GetStats obtiene estadísticas
func (h *ClientHandler) GetStats(c *fiber.Ctx) error {
	stats, err := h.clientService.GetStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"by_tier": stats})
}

// AddTag agrega un tag a un cliente
func (h *ClientHandler) AddTag(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		Tag string `json:"tag"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.clientService.AddTag(c.Context(), id, req.Tag); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveTag elimina un tag de un cliente
func (h *ClientHandler) RemoveTag(c *fiber.Ctx) error {
	id := c.Params("id")
	tag := c.Params("tag")

	if err := h.clientService.RemoveTag(c.Context(), id, tag); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateTier actualiza el tier de un cliente
func (h *ClientHandler) UpdateTier(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		Tier string `json:"tier"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := h.clientService.UpdateTier(c.Context(), id, domain.ClientTier(req.Tier)); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// EnableClient habilita un cliente
func (h *ClientHandler) EnableClient(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.clientService.Enable(c.Context(), id); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// DisableClient deshabilita un cliente
func (h *ClientHandler) DisableClient(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.clientService.Disable(c.Context(), id); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListClientSubscriptions lista las suscripciones de un cliente
func (h *ClientHandler) ListClientSubscriptions(c *fiber.Ctx) error {
	clientID := c.Params("id")

	subs, err := h.subService.ListByClient(c.Context(), clientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": subs, "count": len(subs)})
}

// CreateSubscription crea una suscripción para un cliente
func (h *ClientHandler) CreateSubscription(c *fiber.Ctx) error {
	clientID := c.Params("id")

	var req CreateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	sub := &domain.ClientSubscription{
		ClientID:           clientID,
		ChannelID:          req.ChannelID,
		CustomBotID:        req.CustomBotID,
		CustomSystemPrompt: req.CustomSystemPrompt,
		CustomConfig:       req.CustomConfig,
		Priority:           req.Priority,
		ExpiresAt:          req.ExpiresAt,
	}

	if err := h.subService.Create(c.Context(), sub); err != nil {
		if err == domain.ErrClientNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Client not found"})
		}
		if err == domain.ErrDuplicateSubscription {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		if err == domain.ErrClientDisabled {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(sub)
}

// DeleteSubscription elimina una suscripción
func (h *ClientHandler) DeleteSubscription(c *fiber.Ctx) error {
	subID := c.Params("subId")

	if err := h.subService.Delete(c.Context(), subID); err != nil {
		if err == domain.ErrSubscriptionNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListChannelSubscribers lista los clientes suscritos a un canal
func (h *ClientHandler) ListChannelSubscribers(c *fiber.Ctx) error {
	channelID := c.Params("channelId")

	subs, err := h.subService.ListByChannel(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type SubscriberInfo struct {
		Subscription *domain.ClientSubscription `json:"subscription"`
		Client       *domain.Client             `json:"client"`
	}

	results := make([]SubscriberInfo, 0, len(subs))
	for _, sub := range subs {
		client, err := h.clientService.GetByID(c.Context(), sub.ClientID)
		if err == nil {
			results = append(results, SubscriberInfo{
				Subscription: sub,
				Client:       client,
			})
		}
	}

	return c.JSON(fiber.Map{"data": results, "count": len(results)})
}

// UpdateSubscription actualiza una suscripción
func (h *ClientHandler) UpdateSubscription(c *fiber.Ctx) error {
	id := c.Params("id")
	subID := c.Params("subId")

	// Verificar si existe la suscripción
	sub, err := h.subService.GetByID(c.Context(), subID)
	if err != nil {
		if err == domain.ErrSubscriptionNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Verificar que coincida el clientID
	if sub.ClientID != id {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Subscription does not belong to this client"})
	}

	var req UpdateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Actualizar campos
	if req.CustomBotID != nil {
		sub.CustomBotID = *req.CustomBotID
	}
	if req.CustomSystemPrompt != nil {
		sub.CustomSystemPrompt = *req.CustomSystemPrompt
	}
	if req.CustomConfig != nil {
		sub.CustomConfig = req.CustomConfig
	}
	if req.Priority != nil {
		sub.Priority = *req.Priority
	}
	if req.Status != nil {
		sub.Status = domain.SubscriptionStatus(*req.Status)
	}
	if req.ClearExpiresAt {
		sub.ExpiresAt = nil
	} else if req.ExpiresAt != nil {
		sub.ExpiresAt = req.ExpiresAt
	}

	if err := h.subService.Update(c.Context(), sub); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(sub)
}

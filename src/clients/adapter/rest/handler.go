package rest

import (
	"github.com/AzielCF/az-wap/clients/application"
	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/AzielCF/az-wap/core/pkg/utils"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	wsRepo "github.com/AzielCF/az-wap/workspace/repository"
	wsUcase "github.com/AzielCF/az-wap/workspace/usecase"
	"github.com/gofiber/fiber/v2"
)

// ClientHandler maneja las peticiones REST para clientes
type ClientHandler struct {
	clientService *application.ClientService
	subService    *application.SubscriptionService
	wsRepo        wsRepo.IWorkspaceRepository
	wsUc          *wsUcase.WorkspaceUsecase
}

// NewClientHandler crea una nueva instancia del handler
func NewClientHandler(clientService *application.ClientService, subService *application.SubscriptionService, wsRepo wsRepo.IWorkspaceRepository, wsUc *wsUcase.WorkspaceUsecase) *ClientHandler {
	return &ClientHandler{
		clientService: clientService,
		subService:    subService,
		wsRepo:        wsRepo,
		wsUc:          wsUc,
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

	// Canales del cliente
	clients.Get("/:id/channels", h.ListClientChannels)

	// Workspaces y Guests de cliente (Admin Access)
	clients.Get("/:id/workspaces", h.ListClientWorkspaces)
	clients.Post("/:id/workspaces", h.CreateClientWorkspace)
	clients.Get("/:id/workspaces/:wid", h.GetClientWorkspace)
	clients.Put("/:id/workspaces/:wid", h.UpdateClientWorkspace)
	clients.Delete("/:id/workspaces/:wid", h.DeleteClientWorkspace)

	// Canales en Workspace
	clients.Get("/:id/workspaces/:wid/channels", h.ListWorkspaceChannels)
	clients.Post("/:id/workspaces/:wid/channels/:cid", h.LinkChannel)
	clients.Delete("/:id/workspaces/:wid/channels/:cid", h.UnlinkChannel)

	// Guests en Workspace
	clients.Get("/:id/workspaces/:wid/guests", h.ListWorkspaceGuests)
	clients.Post("/:id/workspaces/:wid/guests", h.CreateWorkspaceGuest)
	clients.Put("/:id/workspaces/:wid/guests/:gid", h.UpdateWorkspaceGuest)
	clients.Delete("/:id/workspaces/:wid/guests/:gid", h.DeleteWorkspaceGuest)
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
		PlatformID:    req.PlatformID,
		PlatformType:  domain.PlatformType(req.PlatformType),
		DisplayName:   req.DisplayName,
		Email:         req.Email,
		Phone:         req.Phone,
		Tier:          domain.ClientTier(req.Tier),
		Tags:          req.Tags,
		Metadata:      req.Metadata,
		Notes:         req.Notes,
		Language:      req.Language,
		Timezone:      req.Timezone,
		Country:       req.Country,
		AllowedBots:   req.AllowedBots,
		OwnedChannels: req.OwnedChannels,
		IsTester:      req.IsTester,
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
	if req.Timezone != nil {
		client.Timezone = *req.Timezone
	}
	if req.Country != nil {
		client.Country = *req.Country
	}
	if req.AllowedBots != nil {
		client.AllowedBots = req.AllowedBots
	}
	if req.OwnedChannels != nil {
		client.OwnedChannels = req.OwnedChannels
	}
	if req.IsTester != nil {
		client.IsTester = *req.IsTester
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
		ClientID:              clientID,
		ChannelID:             req.ChannelID,
		CustomBotID:           req.CustomBotID,
		CustomSystemPrompt:    req.CustomSystemPrompt,
		CustomConfig:          req.CustomConfig,
		Priority:              req.Priority,
		ExpiresAt:             req.ExpiresAt,
		SessionTimeout:        req.SessionTimeout,
		InactivityWarningTime: req.InactivityWarningTime,
		MaxHistoryLimit:       req.MaxHistoryLimit,
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

	if req.ClearSessionTimeout {
		sub.SessionTimeout = 0
	} else if req.SessionTimeout != nil {
		sub.SessionTimeout = *req.SessionTimeout
	}

	if req.ClearInactivityWarning {
		sub.InactivityWarningTime = 0
	} else if req.InactivityWarningTime != nil {
		sub.InactivityWarningTime = *req.InactivityWarningTime
	}

	if req.ClearMaxHistoryLimit {
		sub.MaxHistoryLimit = nil
	} else if req.MaxHistoryLimit != nil {
		sub.MaxHistoryLimit = req.MaxHistoryLimit
	}

	if err := h.subService.Update(c.Context(), sub); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(sub)
}

// --- Client Workspace Handlers (Admin) ---

func (h *ClientHandler) ListClientWorkspaces(c *fiber.Ctx) error {
	clientID := c.Params("id")
	workspaces, err := h.wsUc.ListClientWorkspaces(c.Context(), clientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(workspaces)
}

func (h *ClientHandler) CreateClientWorkspace(c *fiber.Ctx) error {
	clientID := c.Params("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ws, err := h.wsUc.CreateClientWorkspace(c.Context(), clientID, req.Name, req.Description)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ws)
}

func (h *ClientHandler) GetClientWorkspace(c *fiber.Ctx) error {
	wid := c.Params("wid")
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "workspace not found"})
	}
	return c.JSON(ws)
}

func (h *ClientHandler) UpdateClientWorkspace(c *fiber.Ctx) error {
	wid := c.Params("wid")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "workspace not found"})
	}

	ws.Name = req.Name
	ws.Description = req.Description
	if err := h.wsRepo.UpdateClientWorkspace(c.Context(), ws); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ws)
}

func (h *ClientHandler) DeleteClientWorkspace(c *fiber.Ctx) error {
	wid := c.Params("wid")
	if err := h.wsUc.DeleteClientWorkspace(c.Context(), wid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ClientHandler) ListClientChannels(c *fiber.Ctx) error {
	clientID := c.Params("id")
	channels, err := h.wsRepo.ListChannelsByOwnerID(c.Context(), clientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(channels)
}

func (h *ClientHandler) ListWorkspaceChannels(c *fiber.Ctx) error {
	wid := c.Params("wid")
	channels, err := h.wsRepo.ListChannelsInClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(channels)
}

func (h *ClientHandler) LinkChannel(c *fiber.Ctx) error {
	wid := c.Params("wid")
	cid := c.Params("cid")
	if err := h.wsUc.LinkChannelToClientWorkspace(c.Context(), wid, cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ClientHandler) UnlinkChannel(c *fiber.Ctx) error {
	wid := c.Params("wid")
	cid := c.Params("cid")
	if err := h.wsUc.UnlinkChannelFromClientWorkspace(c.Context(), wid, cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ClientHandler) ListWorkspaceGuests(c *fiber.Ctx) error {
	wid := c.Params("wid")
	guests, err := h.wsRepo.ListGuestsInClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(guests)
}

func (h *ClientHandler) CreateWorkspaceGuest(c *fiber.Ctx) error {
	clientID := c.Params("id")
	wid := c.Params("wid")
	var guest wsDomain.ClientWorkspaceGuest
	if err := c.BodyParser(&guest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	guest.OwnerID = clientID
	guest.ClientWorkspaceID = wid

	// Validate that the guest does not have the same identifier as the workspace owner
	client, err := h.clientService.GetByID(c.Context(), clientID)
	if err == nil && client != nil {
		guestPhone := guest.PlatformIdentifiers["whatsapp"]
		if utils.MatchWhatsAppIdentities(guestPhone, client.Phone) || utils.MatchWhatsAppIdentities(guestPhone, client.PlatformID) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "the owner client cannot be added as a guest in their own workspace"})
		}
	}
	newGuest, err := h.wsUc.CreateGuest(c.Context(), guest)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(newGuest)
}

func (h *ClientHandler) UpdateWorkspaceGuest(c *fiber.Ctx) error {
	wid := c.Params("wid")
	gid := c.Params("gid")
	var req wsDomain.ClientWorkspaceGuest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	req.ID = gid
	req.ClientWorkspaceID = wid

	// Validar que el invitado no tenga el mismo número que el dueño del workspace
	client, err := h.clientService.GetByID(c.Context(), c.Params("id"))

	if err == nil && client != nil {
		guestPhone := req.PlatformIdentifiers["whatsapp"]
		if utils.MatchWhatsAppIdentities(guestPhone, client.Phone) || utils.MatchWhatsAppIdentities(guestPhone, client.PlatformID) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "the owner client cannot be added as a guest in their own workspace"})
		}
	}

	if err := h.wsUc.UpdateGuest(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(req)
}

func (h *ClientHandler) DeleteWorkspaceGuest(c *fiber.Ctx) error {
	gid := c.Params("gid")
	if err := h.wsUc.DeleteGuest(c.Context(), gid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

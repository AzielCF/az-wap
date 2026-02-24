package infrastructure

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain/bot"
	clientsApp "github.com/AzielCF/az-wap/clients/application"
	clientsDomain "github.com/AzielCF/az-wap/clients/domain"
	portalDomain "github.com/AzielCF/az-wap/clients_portal/auth/domain"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	wsRepo "github.com/AzielCF/az-wap/workspace/repository"
	wsUsecase "github.com/AzielCF/az-wap/workspace/usecase"
	"github.com/gofiber/fiber/v2"
)

type FeaturesHandler struct {
	subService    *clientsApp.SubscriptionService
	clientService *clientsApp.ClientService
	newsletter    domainNewsletter.INewsletterUsecase
	wsRepo        wsRepo.IWorkspaceRepository
	botUsecase    bot.IBotUsecase
	wsUc          *wsUsecase.WorkspaceUsecase
	wm            *workspace.Manager
}

func NewFeaturesHandler(
	subService *clientsApp.SubscriptionService,
	clientService *clientsApp.ClientService,
	newsletter domainNewsletter.INewsletterUsecase,
	wsRepo wsRepo.IWorkspaceRepository,
	botUsecase bot.IBotUsecase,
	wsUc *wsUsecase.WorkspaceUsecase,
	wm *workspace.Manager,
) *FeaturesHandler {
	return &FeaturesHandler{
		subService:    subService,
		clientService: clientService,
		newsletter:    newsletter,
		wsRepo:        wsRepo,
		botUsecase:    botUsecase,
		wsUc:          wsUc,
		wm:            wm,
	}
}

func (h *FeaturesHandler) EnforceWorkspaceFeature(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	client, err := h.clientService.GetByID(c.Context(), user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "client not found"})
	}

	if !client.HasTag(clientsDomain.TagEnableWorkspaces) && !client.IsVIP() {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Workspaces management not allowed for this account"})
	}

	return c.Next()
}

type ReminderView struct {
	ID             string    `json:"id"`
	Text           string    `json:"text"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	Status         string    `json:"status"`
	RecurrenceDays string    `json:"recurrence_days,omitempty"`
	ChannelType    string    `json:"channel_type"`
	BotName        string    `json:"bot_name"`
}

func (h *FeaturesHandler) ListReminders(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	ctxStd := context.Background()

	subs, err := h.subService.ListByClient(ctxStd, user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch subscriptions"})
	}

	var allReminders []ReminderView = make([]ReminderView, 0)

	for _, sub := range subs {
		if !sub.IsActive() {
			continue
		}

		ch, err := h.wsRepo.GetChannel(ctxStd, sub.ChannelID)
		botName := "Asistente"
		channelType := "desconocido"
		if err == nil {
			channelType = string(ch.Type)
			if ch.Config.BotID != "" {
				if b, errBot := h.botUsecase.GetByID(ctxStd, ch.Config.BotID); errBot == nil {
					botName = b.Name
				} else {
					botName = ch.Name
				}
			} else {
				botName = ch.Name
			}
		}

		posts, err := h.newsletter.ListScheduled(ctxStd, sub.ChannelID)
		if err != nil {
			continue
		}

		for _, p := range posts {
			allReminders = append(allReminders, ReminderView{
				ID:             p.ID,
				Text:           p.Text,
				ScheduledAt:    p.ScheduledAt,
				Status:         string(p.Status),
				RecurrenceDays: p.RecurrenceDays,
				ChannelType:    channelType,
				BotName:        botName,
			})
		}
	}

	sort.Slice(allReminders, func(i, j int) bool {
		return allReminders[i].ScheduledAt.Before(allReminders[j].ScheduledAt)
	})

	return c.JSON(allReminders)
}

type SubscriptionInfo struct {
	ChannelType    string `json:"channel_type"`
	BotName        string `json:"bot_name"`
	BotDescription string `json:"bot_description,omitempty"`
	Phone          string `json:"phone,omitempty"`
}

func (h *FeaturesHandler) GetGeneralInfo(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	ctxStd := context.Background()

	subs, err := h.subService.ListByClient(ctxStd, user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch subscriptions"})
	}

	var activeSubs []SubscriptionInfo = make([]SubscriptionInfo, 0)
	for _, sub := range subs {
		if !sub.IsActive() {
			continue
		}

		ch, err := h.wsRepo.GetChannel(ctxStd, sub.ChannelID)
		if err == nil {
			botID := ch.Config.BotID
			// Priority: Subscription override > Channel config
			if sub.CustomBotID != "" {
				botID = sub.CustomBotID
			}

			botName := ch.Name
			botDesc := ""
			if botID != "" {
				if b, errBot := h.botUsecase.GetByID(ctxStd, botID); errBot == nil {
					botName = b.Name
					botDesc = b.Description
				}
			}

			phone := ""
			if ch.Type == "whatsapp" {
				// We prioritize ExternalRef if it looks like a phone, otherwise we could look in Settings
				if ch.ExternalRef != "" && !strings.Contains(ch.ExternalRef, "-") && len(ch.ExternalRef) <= 15 {
					phone = ch.ExternalRef
				} else {
					// Check if phone exists in settings
					if p, ok := ch.Config.Settings["phone"].(string); ok {
						phone = p
					}
				}
			}

			activeSubs = append(activeSubs, SubscriptionInfo{
				ChannelType:    string(ch.Type),
				BotName:        botName,
				BotDescription: botDesc,
				Phone:          phone,
			})
		}
	}

	return c.JSON(fiber.Map{
		"subscriptions": activeSubs,
	})
}

// GetOwnedChannels returns the channels owned by this client regardless of subscription
func (h *FeaturesHandler) GetOwnedChannels(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	ctxStd := context.Background()

	channels, err := h.wsRepo.ListChannelsByOwnerID(ctxStd, user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch owned channels"})
	}

	type SafeChannel struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		Type            string  `json:"type"`
		Status          string  `json:"status"`
		BotName         string  `json:"bot_name,omitempty"`
		BotDescription  string  `json:"bot_description,omitempty"`
		AccumulatedCost float64 `json:"accumulated_cost,omitempty"`
	}

	// 1. Get all active subscriptions for this client to find bot overrides
	subs, err := h.subService.ListByClient(ctxStd, user.ClientID)
	subBotMap := make(map[string]string) // ChannelID -> BotID
	if err == nil {
		for _, s := range subs {
			if s.IsActive() && s.CustomBotID != "" {
				subBotMap[s.ChannelID] = s.CustomBotID
			}
		}
	}

	var safeChannels []SafeChannel
	for _, ch := range channels {
		sc := SafeChannel{
			ID:     ch.ID,
			Name:   ch.Name,
			Type:   string(ch.Type),
			Status: string(ch.Status),
		}

		// 2. Determine Bot ID (Subscription override > Channel config)
		botID := ch.Config.BotID
		if sBotID, ok := subBotMap[ch.ID]; ok {
			botID = sBotID
		}

		if botID != "" {
			if b, errBot := h.botUsecase.GetByID(ctxStd, botID); errBot == nil {
				sc.BotName = b.Name
				sc.BotDescription = b.Description
			}
		}

		client, err := h.clientService.GetByID(ctxStd, user.ClientID)
		if err == nil && client.IsTester {
			sc.AccumulatedCost = ch.AccumulatedCost
		}

		safeChannels = append(safeChannels, sc)
	}

	return c.JSON(safeChannels)
}

func (h *FeaturesHandler) GetAuthorizedAgents(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	client, err := h.clientService.GetByID(c.Context(), user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "client not found"})
	}

	var agents []fiber.Map
	for _, botID := range client.AllowedBots {
		// Use botUsecase to get the human readable bot name, fallback to ID
		bot, err := h.botUsecase.GetByID(c.Context(), botID)
		name := botID
		desc := ""
		capabilities := fiber.Map{
			"audio":    false,
			"image":    false,
			"video":    false,
			"document": false,
		}
		if err == nil && bot.ID != "" {
			name = bot.Name
			desc = bot.Description
			capabilities["audio"] = bot.AudioEnabled
			capabilities["image"] = bot.ImageEnabled
			capabilities["video"] = bot.VideoEnabled
			capabilities["document"] = bot.DocumentEnabled
		}
		agents = append(agents, fiber.Map{
			"id":           botID,
			"name":         name,
			"description":  desc,
			"capabilities": capabilities,
		})
	}

	if agents == nil {
		agents = []fiber.Map{}
	}

	return c.JSON(agents)
}

// Access Rules for portal

func (h *FeaturesHandler) GetChannelAccessRules(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	cid := c.Params("cid")
	// Verify possession
	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	rules, err := h.wsUc.GetAccessRules(c.Context(), cid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type SafeRule struct {
		ID       string `json:"id"`
		Identity string `json:"identity"`
		Action   string `json:"action"`
		Label    string `json:"label"`
	}

	var safeRules []SafeRule
	for _, r := range rules {
		identity := r.Identity
		if idx := strings.Index(identity, "@"); idx != -1 {
			identity = identity[:idx]
		}
		safeRules = append(safeRules, SafeRule{
			ID:       r.ID,
			Identity: identity,
			Action:   string(r.Action),
			Label:    r.Label,
		})
	}

	return c.JSON(safeRules)
}

func (h *FeaturesHandler) UpdateChannelName(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	cid := c.Params("cid")
	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	ch.Name = req.Name
	if err := h.wsRepo.UpdateChannel(c.Context(), ch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not update channel"})
	}

	return c.JSON(fiber.Map{"status": "updated", "name": ch.Name})
}

func (h *FeaturesHandler) AddChannelAccessRule(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	cid := c.Params("cid")
	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	var req struct {
		Identity string              `json:"identity"`
		Action   common.AccessAction `json:"action"`
		Label    string              `json:"label"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.Identity == "" || req.Action == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "identity and action are required"})
	}

	adapter, ok := h.wm.GetAdapter(cid)
	if ok {
		resolved, err := adapter.ResolveIdentity(c.Context(), req.Identity)
		if err == nil && resolved != "" {
			req.Identity = resolved
		}
	} else if string(ch.Type) == "whatsapp" {
		if !strings.Contains(req.Identity, "@") {
			phoneNumber := strings.TrimLeft(req.Identity, "+")
			phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")
			phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
			req.Identity = phoneNumber + "@s.whatsapp.net"
		}
	}

	err = h.wsUc.AddAccessRule(c.Context(), cid, req.Identity, req.Action, req.Label)
	if err != nil {
		if err == common.ErrDuplicateRule {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "duplicate_entry"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "created", "resolved_identity": req.Identity})
}

func (h *FeaturesHandler) DeleteChannelAccessRule(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	cid := c.Params("cid")
	rid := c.Params("rid")

	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	if err := h.wsUc.DeleteAccessRule(c.Context(), rid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *FeaturesHandler) ResolveIdentity(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	cid := c.Params("cid")
	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	identity := c.Query("identity")
	if identity == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "identity is required"})
	}

	adapter, ok := h.wm.GetAdapter(cid)
	if !ok {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "channel adapter not running"})
	}

	resolved, err := adapter.ResolveIdentity(c.Context(), identity)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "could_not_resolve"})
	}

	phone := ""
	if !strings.Contains(identity, "@") {
		phone = identity
	}

	name := ""
	if contact, err := adapter.GetContact(c.Context(), resolved); err == nil {
		name = contact.Name
	}
	return c.JSON(fiber.Map{
		"resolved_identity": resolved,
		"phone":             phone,
		"name":              name,
		"status":            "verified",
	})
}

// --- Client Workspace Handlers ---

func (h *FeaturesHandler) ListWorkspaces(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	workspaces, err := h.wsUc.ListClientWorkspaces(c.Context(), user.ClientID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if workspaces == nil {
		workspaces = []wsDomain.ClientWorkspace{}
	}

	return c.JSON(workspaces)
}

func (h *FeaturesHandler) CreateWorkspace(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ws, err := h.wsUc.CreateClientWorkspace(c.Context(), user.ClientID, req.Name, req.Description)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(ws)
}

func (h *FeaturesHandler) UpdateWorkspace(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	ws.Name = req.Name
	ws.Description = req.Description
	ws.UpdatedAt = time.Now().UTC()

	if err := h.wsRepo.UpdateClientWorkspace(c.Context(), ws); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(ws)
}

func (h *FeaturesHandler) DeleteWorkspace(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	if err := h.wsUc.DeleteClientWorkspace(c.Context(), wid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- Workspace Channels ---

func (h *FeaturesHandler) ListWorkspaceChannels(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	channels, err := h.wsRepo.ListChannelsInClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if channels == nil {
		channels = []channel.Channel{}
	}

	return c.JSON(channels)
}

func (h *FeaturesHandler) LinkChannel(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	cid := c.Params("cid")

	// Verify ownership of both
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden (workspace)"})
	}

	ch, err := h.wsRepo.GetChannel(c.Context(), cid)
	if err != nil || ch.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden (channel)"})
	}

	if err := h.wsUc.LinkChannelToClientWorkspace(c.Context(), wid, cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *FeaturesHandler) UnlinkChannel(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	cid := c.Params("cid")

	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	if err := h.wsUc.UnlinkChannelFromClientWorkspace(c.Context(), wid, cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// --- Workspace Guests ---

func (h *FeaturesHandler) ListGuests(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	guests, err := h.wsRepo.ListGuestsInClientWorkspace(c.Context(), wid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if guests == nil {
		guests = []wsDomain.ClientWorkspaceGuest{}
	}

	for i := range guests {
		if wa, ok := guests[i].PlatformIdentifiers["whatsapp"]; ok {
			guests[i].PlatformIdentifiers["whatsapp"] = utils.NormalizeWhatsAppIdentity(wa)
			guests[i].PlatformIdentifiers["whatsapp_number"] = utils.NormalizeWhatsAppIdentity(wa)
		}
	}

	return c.JSON(guests)
}

func (h *FeaturesHandler) CreateGuest(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	// Verify ownership
	ws, err := h.wsRepo.GetClientWorkspace(c.Context(), wid)
	if err != nil || ws.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	var guest wsDomain.ClientWorkspaceGuest
	if err := c.BodyParser(&guest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	guest.OwnerID = user.ClientID
	guest.ClientWorkspaceID = wid

	// Validate against owner client
	client, err := h.clientService.GetByID(c.Context(), user.ClientID)
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

func (h *FeaturesHandler) UpdateGuest(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	wid := c.Params("wid")
	gid := c.Params("gid")

	// Verify ownership of guest
	guest, err := h.wsRepo.GetGuest(c.Context(), gid)
	if err != nil || guest.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	var req wsDomain.ClientWorkspaceGuest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	req.ID = gid
	req.OwnerID = user.ClientID
	req.ClientWorkspaceID = wid // Ensure it stays in this workspace

	// Validate against owner client
	client, err := h.clientService.GetByID(c.Context(), user.ClientID)
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

func (h *FeaturesHandler) DeleteGuest(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*portalDomain.PortalUser)
	if !ok || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	gid := c.Params("gid")

	// Verify ownership
	guest, err := h.wsRepo.GetGuest(c.Context(), gid)
	if err != nil || guest.OwnerID != user.ClientID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
	}

	if err := h.wsUc.DeleteGuest(c.Context(), gid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

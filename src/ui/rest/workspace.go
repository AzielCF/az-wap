package rest

import (
	"fmt"
	"os"
	"strings"

	domainApp "github.com/AzielCF/az-wap/domains/app"
	"github.com/AzielCF/az-wap/infrastructure/chatstorage"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type WorkspaceHandler struct {
	uc  *usecase.WorkspaceUsecase
	wm  *workspace.Manager
	app domainApp.IAppUsecase
}

func InitRestWorkspace(app fiber.Router, uc *usecase.WorkspaceUsecase, wm *workspace.Manager, appUc domainApp.IAppUsecase) WorkspaceHandler {
	handler := WorkspaceHandler{uc: uc, wm: wm, app: appUc}

	g := app.Group("/workspaces")
	g.Post("/", handler.CreateWorkspace)
	g.Get("/", handler.ListWorkspaces)
	g.Get("/active-sessions", handler.GetActiveSessions)
	g.Get("/active-typing", handler.GetActiveTyping)
	g.Get("/:id", handler.GetWorkspace)
	g.Put("/:id", handler.UpdateWorkspace)
	g.Delete("/:id", handler.DeleteWorkspace)

	g.Post("/:id/channels", handler.CreateChannel)
	g.Get("/:id/channels", handler.ListChannels)
	g.Post("/:id/channels/:cid/enable", handler.EnableChannel)
	g.Post("/:id/channels/:cid/disable", handler.DisableChannel)
	g.Put("/:id/channels/:cid/config", handler.UpdateChannelConfig)
	g.Put("/:id/channels/:cid", handler.UpdateChannel)
	g.Delete("/:id/channels/:cid", handler.DeleteChannel)
	g.Post("/:id/channels/:cid/chatwoot/webhook", handler.ChatwootWebhook)

	// Access Rules
	g.Get("/:id/channels/:cid/access-rules", handler.ListAccessRules)
	g.Post("/:id/channels/:cid/access-rules", handler.AddAccessRule)
	g.Delete("/:id/channels/:cid/access-rules", handler.DeleteAllAccessRules)
	g.Delete("/:id/channels/:cid/access-rules/:rid", handler.DeleteAccessRule)
	g.Get("/:id/channels/:cid/resolve-identity", handler.ResolveIdentity)

	// WhatsApp Control
	g.Get("/:id/channels/:cid/whatsapp/status", handler.GetWhatsAppStatus)
	g.Get("/:id/channels/:cid/whatsapp/login", handler.WhatsAppLogin)
	g.Get("/:id/channels/:cid/whatsapp/logout", handler.WhatsAppLogout)
	g.Get("/:id/channels/:cid/whatsapp/reconnect", handler.WhatsAppReconnect)
	g.Post("/:id/channels/:cid/whatsapp/login-code", handler.WhatsAppLoginWithCode)

	// Bot Control in Channel
	g.Post("/:id/channels/:cid/bot-memory/clear", handler.ClearChannelBotMemory)

	return handler
}

func (h *WorkspaceHandler) GetActiveSessions(c *fiber.Ctx) error {
	return c.JSON(h.wm.GetActiveSessions())
}

func (h *WorkspaceHandler) GetActiveTyping(c *fiber.Ctx) error {
	active, _ := h.wm.GetActiveTyping(c.Context())
	return c.JSON(active)
}

// ... existing code ...

func (h *WorkspaceHandler) ClearChannelBotMemory(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	channelID := c.Params("cid")

	// 1. Get Channel to verify workspace
	ch, err := h.uc.GetChannel(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel not found"})
	}

	if ch.WorkspaceID != workspaceID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "channel does not belong to this workspace"})
	}

	// 2. Get BotID from Channel Config
	botID := ch.Config.BotID
	if botID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "channel has no assigned bot"})
	}

	// 3. Clear memory scoped to this workspace and bot
	if h.wm != nil {
		h.wm.ClearWorkspaceBotMemory(workspaceID, botID)
	}

	return c.JSON(fiber.Map{"status": "memory_cleared", "scope": "workspace_bot"})
}

func (h *WorkspaceHandler) CreateWorkspace(c *fiber.Ctx) error {
	type req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     string `json:"owner_id"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if r.Name == "" || r.OwnerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and owner_id are required"})
	}

	ws, err := h.uc.CreateWorkspace(c.Context(), r.Name, r.Description, r.OwnerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ws)
}

func (h *WorkspaceHandler) ListWorkspaces(c *fiber.Ctx) error {
	workspaces, err := h.uc.ListWorkspaces(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(workspaces)
}

func (h *WorkspaceHandler) GetWorkspace(c *fiber.Ctx) error {
	id := c.Params("id")
	ws, err := h.uc.GetWorkspace(c.Context(), id)
	if err == common.ErrWorkspaceNotFound {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "workspace not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ws)
}

func (h *WorkspaceHandler) CreateChannel(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	type req struct {
		Type channel.ChannelType `json:"type"`
		Name string              `json:"name"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if r.Name == "" || r.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and type are required"})
	}

	ch, err := h.uc.CreateChannel(c.Context(), workspaceID, r.Type, r.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ch)
}

func (h *WorkspaceHandler) UpdateWorkspace(c *fiber.Ctx) error {
	id := c.Params("id")
	type req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	ws, err := h.uc.UpdateWorkspace(c.Context(), id, r.Name, r.Description)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ws)
}

func (h *WorkspaceHandler) DeleteWorkspace(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.uc.DeleteWorkspace(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WorkspaceHandler) ListChannels(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	channels, err := h.uc.ListChannels(c.Context(), workspaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(channels)
}

func (h *WorkspaceHandler) EnableChannel(c *fiber.Ctx) error {
	cid := c.Params("cid")
	if err := h.uc.EnableChannel(c.Context(), cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// Start in manager
	if err := h.wm.StartChannel(c.Context(), cid); err != nil {
		logrus.WithError(err).Warn("[REST] Failed to start channel in manager after enabling")
	}
	return c.JSON(fiber.Map{"status": "enabled"})
}

func (h *WorkspaceHandler) DisableChannel(c *fiber.Ctx) error {
	cid := c.Params("cid")
	if err := h.uc.DisableChannel(c.Context(), cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	// Stop in manager
	h.wm.UnregisterAdapter(cid)
	return c.JSON(fiber.Map{"status": "disabled"})
}

func (h *WorkspaceHandler) DeleteChannel(c *fiber.Ctx) error {
	cid := c.Params("cid")

	// 1. Stop and remove from manager
	h.wm.UnregisterAdapter(cid)

	// 2. Cleanup chat storage files (handles .db, .shm, .wal)
	_ = chatstorage.CleanupInstanceRepository(cid)

	// 3. Cleanup whatsapp storage files (internal library files)
	dbPath := fmt.Sprintf("storages/whatsapp-%s.db", cid)
	files := []string{dbPath, dbPath + "-shm", dbPath + "-wal"}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			if !os.IsNotExist(err) {
				logrus.Errorf("[REST] Failed to remove whatsapp storage file %s: %v", f, err)
			}
		}
	}

	if err := h.uc.DeleteChannel(c.Context(), cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WorkspaceHandler) UpdateChannelConfig(c *fiber.Ctx) error {
	cid := c.Params("cid")
	var cfg channel.ChannelConfig
	if err := c.BodyParser(&cfg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	// First get the channel to ensure it exists
	ch, err := h.uc.GetChannel(c.Context(), cid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel not found"})
	}

	// Update only the config
	ch.Config = cfg

	// Sync ExternalRef for bypass logic if WhatsApp
	if ch.Type == channel.ChannelTypeWhatsApp {
		if instID, ok := cfg.Settings["instance_id"].(string); ok && instID != "" {
			ch.ExternalRef = instID
		}
	}

	if err := h.uc.UpdateChannel(c.Context(), ch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Restart if already running to apply new config
	if _, ok := h.wm.GetAdapter(cid); ok {
		h.wm.UnregisterAdapter(cid)
		if err := h.wm.StartChannel(c.Context(), cid); err != nil {
			logrus.WithError(err).Warn("[REST] Failed to restart channel after config update")
		}
	}

	return c.JSON(ch)
}

func (h *WorkspaceHandler) UpdateChannel(c *fiber.Ctx) error {
	cid := c.Params("cid")
	var req channel.Channel
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	// First get the channel to ensure it exists
	ch, err := h.uc.GetChannel(c.Context(), cid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel not found"})
	}

	// Update params
	if req.Name != "" {
		ch.Name = req.Name
	}
	// We assume if Config is passed, we update it.
	// To be safer, we could check emptiness, but in Go zero value is empty strict.
	// Since it's a PUT, full replacement of sent fields is expected.
	ch.Config = req.Config

	// Sync ExternalRef for bypass logic if WhatsApp
	if ch.Type == channel.ChannelTypeWhatsApp {
		if instID, ok := ch.Config.Settings["instance_id"].(string); ok && instID != "" {
			ch.ExternalRef = instID
		}
	}

	if err := h.uc.UpdateChannel(c.Context(), ch); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Restart if already running to apply new config
	if _, ok := h.wm.GetAdapter(cid); ok {
		h.wm.UnregisterAdapter(cid)
		if err := h.wm.StartChannel(c.Context(), cid); err != nil {
			logrus.WithError(err).Warn("[REST] Failed to restart channel after update")
		}
	}

	return c.JSON(ch)
}

func (h *WorkspaceHandler) ChatwootWebhook(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	channelID := c.Params("cid")

	// 1. Get Channel Config
	ch, err := h.uc.GetChannel(c.Context(), channelID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel not found"})
	}

	if ch.WorkspaceID != workspaceID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "channel does not belong to this workspace"})
	}

	// 2. Parse Chatwoot Webhook Payload
	var payload map[string]any
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	event, _ := payload["event"].(string)
	if event != "message_created" {
		return c.SendStatus(fiber.StatusOK) // Ignore other events
	}

	msgData, _ := payload["message_type"].(string)
	if msgData == "incoming" {
		return c.SendStatus(fiber.StatusOK) // Ignore customer messages
	}

	// 3. Extract target (phone) and message
	content, _ := payload["content"].(string)

	// Complex extraction for phone (Chatwoot source_id or phone)
	var phone string
	if sender, ok := payload["sender"].(map[string]any); ok {
		phone, _ = sender["phone_number"].(string)
	}

	if contact, ok := payload["contact"].(map[string]any); ok {
		if phone == "" {
			phone, _ = contact["phone_number"].(string)
		}
		if phone == "" {
			phone, _ = contact["identifier"].(string)
		}
	}

	if phone == "" {
		if conv, ok := payload["conversation"].(map[string]any); ok {
			if contact, ok := conv["contact"].(map[string]any); ok {
				phone, _ = contact["phone_number"].(string)
			}
		}
	}

	phone = strings.TrimPrefix(phone, "+")
	if phone == "" {
		logrus.WithField("payload", payload).Warn("[CHATWOOT_WEBHOOK] phone not found in payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "phone not found"})
	}

	// 4. Send via Adapter
	adapter, ok := h.wm.GetAdapter(channelID)
	if !ok {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "channel adapter not running"})
	}

	if content != "" {
		if _, err := adapter.SendMessage(c.Context(), phone, content, ""); err != nil {
			logrus.WithError(err).Error("[CHATWOOT_WEBHOOK] failed to send message")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if attachments, ok := payload["attachments"].([]any); ok && len(attachments) > 0 {
		logrus.Infof("[CHATWOOT_WEBHOOK] received %d attachments, forwarding not fully implemented in adapter yet", len(attachments))
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *WorkspaceHandler) GetWhatsAppStatus(c *fiber.Ctx) error {
	cid := c.Params("cid")

	adapter, ok := h.wm.GetAdapter(cid)
	if !ok {
		// If adapter not running, check DB status for a "dry" status report
		ch, err := h.uc.GetChannel(c.Context(), cid)
		if err == nil {
			return c.JSON(fiber.Map{
				"is_connected": false,
				"is_logged_in": ch.Status == channel.ChannelStatusConnected,
				"status":       ch.Status,
				"channel_id":   cid,
				"is_paused":    !ch.Enabled,
			})
		}

		return c.JSON(fiber.Map{
			"is_connected": false,
			"is_logged_in": false,
			"status":       "disconnected",
		})
	}

	if adapter.Type() != channel.ChannelTypeWhatsApp {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "not a whatsapp channel"})
	}

	status := adapter.Status()
	isConnected := status == channel.ChannelStatusConnected
	isHibernating := status == channel.ChannelStatusHibernating
	isLoggedIn := adapter.IsLoggedIn()

	// If manual sync requested and we are hibernating, try to resume
	if c.Query("resume") == "true" && isHibernating {
		logrus.Infof("[REST] Force Status Sync: Resuming hibernated channel %s", cid)
		_ = adapter.Resume(c.Context())
		// Refresh status after resume
		status = adapter.Status()
		isConnected = status == channel.ChannelStatusConnected
		isHibernating = status == channel.ChannelStatusHibernating
	}

	// Sync local DB if there's a discrepancy (e.g. manual DB deletion or hibernation)
	ch, err := h.uc.GetChannel(c.Context(), cid)
	if err == nil && (ch.Status != status) {
		ch.Status = status
		_ = h.uc.UpdateChannel(c.Context(), ch)
	}

	return c.JSON(fiber.Map{
		"is_connected":   isConnected,
		"is_logged_in":   isLoggedIn,
		"is_hibernating": isHibernating,
		"status":         status,
		"channel_id":     cid,
	})
}

func (h *WorkspaceHandler) WhatsAppLogin(c *fiber.Ctx) error {
	cid := c.Params("cid")
	adapter, ok := h.wm.GetAdapter(cid)
	if !ok {
		// Try auto-start if not running
		if err := h.wm.StartChannel(c.Context(), cid); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("channel start failed: %v", err)})
		}
		// Try get again
		adapter, ok = h.wm.GetAdapter(cid)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "adapter failed to initialize"})
		}
	}

	if err := adapter.Login(c.Context()); err != nil {
		// If already connected but we are here, something is out of sync.
		// Forced disconnect and retry once.
		if strings.Contains(err.Error(), "already connected") {
			logrus.Warnf("[REST] Adapter %s reports already connected, forcing reset...", cid)
			_ = adapter.Logout(c.Context())
			if err := adapter.Login(c.Context()); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to reset connection: " + err.Error()})
			}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	qrChan, err := adapter.GetQRChannel(c.Context())
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	// Block and wait for 1 QR code (or timeout managed by frontend polling)
	// PROPER WAY: Stream it. But current frontend expects REST return.
	// We will wait for the first code and return it.

	select {
	case code, open := <-qrChan:
		if !open {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "qr channel closed"})
		}
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "SUCCESS",
			Message: "QR Generated",
			Results: fiber.Map{
				"qr_link":     code,
				"qr_duration": 60, // approximate
			},
		})
	case <-c.Context().Done():
		return c.Status(fiber.StatusRequestTimeout).JSON(fiber.Map{"error": "timeout waiting for qr"})
	}
}

func (h *WorkspaceHandler) WhatsAppLoginWithCode(c *fiber.Ctx) error {
	cid := c.Params("cid")

	type req struct {
		PhoneNumber string `json:"phone_number"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if r.PhoneNumber == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "phone_number is required"})
	}

	// Sanitize phone number (remove +, spaces, dashes)
	logrus.WithField("phone_raw", r.PhoneNumber).Info("[REST] LoginWithCode Request")

	phone := strings.ReplaceAll(r.PhoneNumber, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.TrimSpace(phone)

	logrus.WithField("phone_sanitized", phone).Info("[REST] LoginWithCode Sanitized")

	adapter, ok := h.wm.GetAdapter(cid)
	if !ok {
		// Try auto-start if not running
		if err := h.wm.StartChannel(c.Context(), cid); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("channel start failed: %v", err)})
		}
		adapter, ok = h.wm.GetAdapter(cid)
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel adapter not available"})
		}
	}

	code, err := adapter.LoginWithCode(c.Context(), phone)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"code":   code,
		"status": "pairing_code_generated",
	})
}

func (h *WorkspaceHandler) WhatsAppLogout(c *fiber.Ctx) error {
	cid := c.Params("cid")
	adapter, ok := h.wm.GetAdapter(cid)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "channel not running"})
	}

	logrus.Infof("[REST] Logging out channel %s", cid)

	if err := adapter.Logout(c.Context()); err != nil {
		logrus.Warnf("[REST] Logout failed for %s, but syncing state anyway: %v", cid, err)
	}

	// Update DB status - Always sync to Disconnected on logout attempt
	ch, err := h.uc.GetChannel(c.Context(), cid)
	if err == nil {
		ch.Status = channel.ChannelStatusDisconnected
		_ = h.uc.UpdateChannel(c.Context(), ch)
	}

	return c.JSON(fiber.Map{"status": "logged_out"})
}

func (h *WorkspaceHandler) WhatsAppReconnect(c *fiber.Ctx) error {
	cid := c.Params("cid")

	// Restarting the channel effectively reconnects it
	h.wm.UnregisterAdapter(cid)
	if err := h.wm.StartChannel(c.Context(), cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "reconnected"})
}

// Access Rules Handlers

func (h *WorkspaceHandler) ListAccessRules(c *fiber.Ctx) error {
	cid := c.Params("cid")
	rules, err := h.uc.GetAccessRules(c.Context(), cid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(rules)
}

func (h *WorkspaceHandler) AddAccessRule(c *fiber.Ctx) error {
	cid := c.Params("cid")

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

	// Normalize Identity via Adapter if active
	adapter, ok := h.wm.GetAdapter(cid)
	if ok {
		resolved, err := adapter.ResolveIdentity(c.Context(), req.Identity)
		if err == nil && resolved != "" {
			req.Identity = resolved
		}
	} else {
		// Fallback normalization if adapter is offline
		ch, err := h.uc.GetChannel(c.Context(), cid)
		if err == nil && ch.Type == channel.ChannelTypeWhatsApp {
			if !strings.Contains(req.Identity, "@") {
				phoneNumber := strings.TrimLeft(req.Identity, "+")
				phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")
				phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
				req.Identity = phoneNumber + "@s.whatsapp.net"
			}
		}
	}

	err := h.uc.AddAccessRule(c.Context(), cid, req.Identity, req.Action, req.Label)
	if err != nil {
		if err == common.ErrDuplicateRule {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":   "duplicate_entry",
				"message": "This identity is already in the list",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "created", "resolved_identity": req.Identity})
}

func (h *WorkspaceHandler) DeleteAccessRule(c *fiber.Ctx) error {
	rid := c.Params("rid")
	if err := h.uc.DeleteAccessRule(c.Context(), rid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WorkspaceHandler) DeleteAllAccessRules(c *fiber.Ctx) error {
	cid := c.Params("cid")
	if err := h.uc.DeleteAllAccessRules(c.Context(), cid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
func (h *WorkspaceHandler) ResolveIdentity(c *fiber.Ctx) error {
	cid := c.Params("cid")
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
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "could_not_resolve",
			"message": err.Error(),
		})
	}

	// Check for duplicates in DB immediately
	rules, err := h.uc.GetAccessRules(c.Context(), cid)
	if err == nil {
		for _, r := range rules {
			if r.Identity == resolved {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"error":             "duplicate_entry",
					"message":           "This identity is already in the list",
					"resolved_identity": resolved,
				})
			}
		}
	}

	// Attempt to get name if possible
	name := ""
	if contact, err := adapter.GetContact(c.Context(), resolved); err == nil {
		name = contact.Name
	}

	return c.JSON(fiber.Map{
		"resolved_identity": resolved,
		"name":              name,
		"status":            "verified",
	})
}

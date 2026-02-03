package rest

import (
	"net/url"

	botdomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
	"github.com/gofiber/fiber/v2"
)

type MonitoringHandler struct {
	store    monitoring.MonitoringStore
	wm       *workspace.Manager
	aiCaches botdomain.ContextCacheStore
}

// InitRestMonitoring registra los endpoints unificados de monitoreo del sistema
func InitRestMonitoring(app fiber.Router, store monitoring.MonitoringStore, wm *workspace.Manager, aiCaches botdomain.ContextCacheStore) {
	h := &MonitoringHandler{store: store, wm: wm, aiCaches: aiCaches}

	g := app.Group("/monitoring")

	// Estado del Cluster
	g.Get("/servers", h.GetServers)
	g.Get("/cluster-activity", h.GetClusterActivity)
	g.Get("/stats", h.GetGlobalStats)
	g.Get("/typing", h.GetTypingStatus)

	// Feed de eventos (mantenemos botmonitor por ahora para el log de eventos recientes)
	g.Get("/events", h.GetRecentEvents)

	// AI Cache Inspector
	g.Get("/ai-caches", h.GetAICaches)

	// Admin Controls
	g.Delete("/sessions/:channelID/:chatID", h.KillSession)
}

func (h *MonitoringHandler) GetServers(c *fiber.Ctx) error {
	servers, err := h.store.GetActiveServers(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(servers)
}

func (h *MonitoringHandler) GetClusterActivity(c *fiber.Ctx) error {
	activity, err := h.store.GetClusterActivity(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(activity)
}

func (h *MonitoringHandler) GetGlobalStats(c *fiber.Ctx) error {
	stats, err := h.store.GetGlobalStats(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(stats)
}

func (h *MonitoringHandler) GetRecentEvents(c *fiber.Ctx) error {
	// Obtenemos los eventos de botmonitor (log en vivo)
	stats := botmonitor.GetStats()
	return c.JSON(stats)
}

func (h *MonitoringHandler) GetTypingStatus(c *fiber.Ctx) error {
	active, err := h.wm.GetActiveTyping(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(active)
}

// GetAICaches returns all active AI provider caches for inspection
func (h *MonitoringHandler) GetAICaches(c *fiber.Ctx) error {
	if h.aiCaches == nil {
		return c.JSON([]interface{}{})
	}

	caches, err := h.aiCaches.List(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(caches)
}

func (h *MonitoringHandler) KillSession(c *fiber.Ctx) error {
	channelID := c.Params("channelID")
	chatID, _ := url.QueryUnescape(c.Params("chatID"))

	if channelID == "" || chatID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "channelID and chatID are required"})
	}

	err := h.wm.CloseSession(c.UserContext(), channelID, chatID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "Activity killed successfully"})
}

package rest

import (
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
	"github.com/gofiber/fiber/v2"
)

type MonitoringHandler struct {
	store monitoring.MonitoringStore
}

// InitRestMonitoring registra los endpoints unificados de monitoreo del sistema
func InitRestMonitoring(app fiber.Router, store monitoring.MonitoringStore) {
	h := &MonitoringHandler{store: store}

	g := app.Group("/monitoring")

	// Estado del Cluster
	g.Get("/servers", h.GetServers)
	g.Get("/cluster-activity", h.GetClusterActivity)
	g.Get("/stats", h.GetGlobalStats)

	// Feed de eventos (mantenemos botmonitor por ahora para el log de eventos recientes)
	g.Get("/events", h.GetRecentEvents)
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

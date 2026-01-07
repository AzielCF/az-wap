package rest

import (
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/gofiber/fiber/v2"
)

// GetBotMonitorStats returns real-time bot/AI monitor statistics
func GetBotMonitorStats(c *fiber.Ctx) error {
	return c.JSON(botmonitor.GetStats())
}

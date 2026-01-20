package rest

import (
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/gofiber/fiber/v2"
)

// GetWorkerPoolStats returns real-time worker pool statistics
func GetWorkerPoolStats(c *fiber.Ctx) error {
	stats := msgworker.GetGlobalStats()
	return c.JSON(stats)
}

// GetBotWebhookPoolStats returns real-time bot webhook worker pool statistics
func GetBotWebhookPoolStats(c *fiber.Ctx) error {
	if botWebhookPool == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Bot webhook worker pool not initialized",
		})
	}

	stats := botWebhookPool.GetStats()
	return c.JSON(stats)
}

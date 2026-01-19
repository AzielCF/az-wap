package rest

import (
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/AzielCF/az-wap/pkg/chatpresence"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

// GetBotMonitorStats returns real-time bot/AI monitor statistics
func GetBotMonitorStats(c *fiber.Ctx) error {
	return c.JSON(botmonitor.GetStats())
}

// GetTypingStatus returns the list of chats currently typing across all instances
func GetTypingStatus(c *fiber.Ctx) error {
	active := chatpresence.GetActiveTyping()
	if len(active) > 0 {
		logrus.Infof("[MONITOR] Serving %d active typing events", len(active))
	}
	return c.JSON(active)
}

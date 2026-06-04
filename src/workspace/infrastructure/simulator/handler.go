package simulator

import (
	"context"
	"encoding/json"

	"github.com/AzielCF/az-wap/botengine"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/sirupsen/logrus"
)

func InitRestSimulator(router fiber.Router, engine *botengine.Engine, repo repository.IWorkspaceRepository) {
	router.Use("/ws/simulator", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	router.Get("/ws/simulator/:channelId", websocket.New(func(c *websocket.Conn) {
		token := c.Query("token")
		if token == "" {
			logrus.Warn("[Simulator] Rejected connection: missing token")
			c.Close()
			return
		}
		
		channelID := c.Params("channelId")
		// Usamos un instanceID simulado para que no sobreescriba el transport real
		simInstanceID := "sim_" + channelID
		transport := NewTransport(simInstanceID, c)

		engine.RegisterTransport(transport)
		defer engine.UnregisterTransport(simInstanceID)
		defer c.Close()

		ctx := context.Background()
		ch, err := repo.GetChannel(ctx, channelID)
		if err != nil {
			logrus.Errorf("[Simulator] Failed to get channel %s: %v", channelID, err)
			return
		}

		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				logrus.Infof("[Simulator] WebSocket closed for channel %s", channelID)
				break
			}

			if mt == websocket.TextMessage {
				var data map[string]interface{}
				if err := json.Unmarshal(msg, &data); err != nil {
					logrus.Warnf("[Simulator] Failed to parse message: %v", err)
					continue
				}

				if msgType, ok := data["type"].(string); ok && msgType == "message" {
					text, _ := data["text"].(string)

					botInput := botengineDomain.BotInput{
						BotID:       ch.Config.BotID,
						WorkspaceID: ch.WorkspaceID,
						InstanceID:  simInstanceID,
						SenderID:    "sim_user",
						ChatID:      "sim_user",
						Platform:    botengineDomain.PlatformTest,
						Text:        text,
						Metadata: map[string]any{
							"phone": "sim_user",
							"name":  "Simulator Tester",
						},
					}

					// Ejecutamos en una goroutine
					go func() {
						_, err := engine.Process(context.Background(), botInput)
						if err != nil {
							logrus.Errorf("[Simulator] Engine processing failed: %v", err)
						}
					}()
				}
			}
		}
	}))
}

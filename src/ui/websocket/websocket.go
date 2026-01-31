package websocket

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"

	domainApp "github.com/AzielCF/az-wap/domains/app"
	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	valkeylib "github.com/valkey-io/valkey-go"
)

type client struct{}

type BroadcastMessage struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Token    string `json:"token,omitempty"`
	Result   any    `json:"result"`
	SenderID string `json:"sender_id,omitempty"`
}

var (
	Clients    = make(map[*websocket.Conn]client)
	Register   = make(chan *websocket.Conn)
	Broadcast  = make(chan BroadcastMessage)
	Unregister = make(chan *websocket.Conn)

	vkClient *valkey.Client
	wsChan   = "azwap:ws_broadcast"
	localID  string
)

// SetValkeyClient initializes the distributed broadcast system
func SetValkeyClient(client *valkey.Client, serverID string) {
	vkClient = client
	localID = serverID
}

func handleRegister(conn *websocket.Conn) {
	Clients[conn] = client{}
	logrus.Debug("[WS] Connection registered")
}

func handleUnregister(conn *websocket.Conn) {
	delete(Clients, conn)
	logrus.Debug("[WS] Connection unregistered")
}

func broadcastToLocal(message BroadcastMessage) {
	marshalMessage, err := json.Marshal(message)
	if err != nil {
		logrus.Errorf("[WS] Marshal error: %v", err)
		return
	}

	for conn := range Clients {
		if err := conn.WriteMessage(websocket.TextMessage, marshalMessage); err != nil {
			logrus.Errorf("[WS] Write error: %v", err)
			closeConnection(conn)
		}
	}
}

func publishToValkey(message BroadcastMessage) {
	if vkClient == nil {
		return
	}

	// Attach local ID as sender
	message.SenderID = localID

	data, err := json.Marshal(message)
	if err != nil {
		return
	}

	ctx := context.Background()
	cmd := vkClient.Inner().B().Publish().Channel(wsChan).Message(string(data)).Build()
	if err := vkClient.Inner().Do(ctx, cmd).Error(); err != nil {
		logrus.Errorf("[WS] Failed to publish to Valkey: %v", err)
	}
}

func startValkeySubscriber() {
	if vkClient == nil {
		return
	}

	logrus.Info("[WS] Starting Valkey Pub/Sub subscriber for distributed events")
	go func() {
		err := vkClient.Inner().Receive(context.Background(), vkClient.Inner().B().Subscribe().Channel(wsChan).Build(), func(msg valkeylib.PubSubMessage) {
			var broadcastMsg BroadcastMessage
			if err := json.Unmarshal([]byte(msg.Message), &broadcastMsg); err == nil {
				// Avoid loops: ignore messages sent by this same instance
				if broadcastMsg.SenderID == localID {
					return
				}
				broadcastToLocal(broadcastMsg)
			}
		})
		if err != nil {
			logrus.Errorf("[WS] Valkey subscriber failed: %v", err)
		}
	}()
}

func closeConnection(conn *websocket.Conn) {
	_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
	_ = conn.Close()
	delete(Clients, conn)
}

func RunHub() {
	// If Valkey is enabled, start the subscriber
	if vkClient != nil {
		startValkeySubscriber()
	}

	for {
		select {
		case conn := <-Register:
			handleRegister(conn)

		case conn := <-Unregister:
			handleUnregister(conn)

		case message := <-Broadcast:
			// 1. Send to local clients immediately
			broadcastToLocal(message)

			// 2. If Valkey is active, propagate to other servers
			if vkClient != nil {
				publishToValkey(message)
			}
		}
	}
}

func RegisterRoutes(app fiber.Router, service domainApp.IAppUsecase) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		defer func() {
			Unregister <- conn
			_ = conn.Close()
		}()

		Register <- conn

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.Println("read error:", err)
				}
				return
			}

			if messageType == websocket.TextMessage {
				var messageData BroadcastMessage
				if err := json.Unmarshal(message, &messageData); err != nil {
					logrus.Println("unmarshal error:", err)
					return
				}

				if messageData.Code == "FETCH_DEVICES" {
					devices, _ := service.FetchDevices(context.Background(), messageData.Token)
					Broadcast <- BroadcastMessage{
						Code:    "LIST_DEVICES",
						Message: "Device found",
						Result:  devices,
					}
				}
			} else {
				logrus.Println("unsupported message type:", messageType)
			}
		}
	}))
}

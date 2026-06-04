package simulator

import (
	"context"
	"encoding/json"

	"github.com/gofiber/websocket/v2"
	"github.com/sirupsen/logrus"
)

type Transport struct {
	instanceID string
	conn       *websocket.Conn
}

func NewTransport(instanceID string, conn *websocket.Conn) *Transport {
	return &Transport{
		instanceID: instanceID,
		conn:       conn,
	}
}

func (t *Transport) ID() string {
	return t.instanceID
}

func (t *Transport) SendMessage(ctx context.Context, chatID string, text string, quoteMessageID string) error {
	msg := map[string]interface{}{
		"type": "message",
		"text": text,
	}
	data, _ := json.Marshal(msg)
	logrus.Infof("[SimulatorTransport] Sending message: %s", text)
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

func (t *Transport) SendPresence(ctx context.Context, chatID string, isTyping bool, isAudio bool) error {
	msg := map[string]interface{}{
		"type":      "presence",
		"is_typing": isTyping,
	}
	data, _ := json.Marshal(msg)
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

func (t *Transport) MarkRead(ctx context.Context, chatID string, messageIDs []string) error {
	msg := map[string]interface{}{
		"type": "read",
	}
	data, _ := json.Marshal(msg)
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

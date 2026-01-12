package whatsapp

import (
	"context"
	"fmt"

	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type WhatsAppAdapter struct {
	channelID    string
	workspaceID  string
	client       *whatsmeow.Client
	eventHandler func(workspace.IncomingMessage)
}

// NewAdapter creates a new adapter from an existing client or prepares a new one
func NewAdapter(channelID, workspaceID string, client *whatsmeow.Client) *WhatsAppAdapter {
	return &WhatsAppAdapter{
		channelID:   channelID,
		workspaceID: workspaceID,
		client:      client,
	}
}

// ID returns the channel ID
func (wa *WhatsAppAdapter) ID() string {
	return wa.channelID
}

// Type returns the channel type
func (wa *WhatsAppAdapter) Type() domain.ChannelType {
	return domain.ChannelTypeWhatsApp
}

// Status returns the connection status
func (wa *WhatsAppAdapter) Status() domain.ChannelStatus {
	if wa.client == nil {
		return domain.ChannelStatusDisconnected
	}
	if wa.client.IsConnected() {
		return domain.ChannelStatusConnected
	}
	return domain.ChannelStatusDisconnected
}

// Start ensures the client is connected
func (wa *WhatsAppAdapter) Start(ctx context.Context, config domain.ChannelConfig) error {
	if wa.client == nil {
		return fmt.Errorf("whatsapp client not initialized")
	}
	if !wa.client.IsConnected() {
		return wa.client.Connect()
	}
	return nil
}

// Stop disconnects the client
func (wa *WhatsAppAdapter) Stop(ctx context.Context) error {
	if wa.client != nil {
		wa.client.Disconnect()
	}
	return nil
}

// SendMessage sends a text message
func (wa *WhatsAppAdapter) SendMessage(ctx context.Context, chatID, text string) error {
	if wa.client == nil {
		return fmt.Errorf("no client")
	}

	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	}

	_, err = wa.client.SendMessage(ctx, jid, msg)
	return err
}

// SendPresence sends typing status
func (wa *WhatsAppAdapter) SendPresence(ctx context.Context, chatID string, typing bool) error {
	if wa.client == nil {
		return fmt.Errorf("no client")
	}

	jid, err := types.ParseJID(chatID)
	if err != nil {
		return fmt.Errorf("invalid JID: %w", err)
	}

	state := types.ChatPresenceComposing
	if !typing {
		state = types.ChatPresencePaused
	}
	return wa.client.SendChatPresence(ctx, jid, state, types.ChatPresenceMediaText)
}

// OnMessage registers a handler for incoming messages
func (wa *WhatsAppAdapter) OnMessage(handler func(workspace.IncomingMessage)) {
	wa.eventHandler = handler
	wa.client.AddEventHandler(wa.handleEvent)
}

// handleEvent converts whatsmeow events to workspace events
func (wa *WhatsAppAdapter) handleEvent(evt interface{}) {
	if wa.eventHandler == nil {
		return
	}

	switch v := evt.(type) {
	case *events.Message:
		// Ignore messages from self if needed, or handle them based on logic
		// Using pkg/utils to extract text
		text := utils.ExtractMessageTextFromEvent(v)

		// Basic metadata extraction
		metadata := map[string]any{
			"platform":   "whatsapp",
			"message_id": v.Info.ID,
			"timestamp":  v.Info.Timestamp.Unix(),
			"from_me":    v.Info.IsFromMe,
		}

		if v.Info.PushName != "" {
			metadata["sender_name"] = v.Info.PushName
		}

		msg := workspace.IncomingMessage{
			WorkspaceID: wa.workspaceID,
			ChannelID:   wa.channelID,
			ChatID:      v.Info.Chat.String(),
			SenderID:    v.Info.Sender.String(),
			Text:        text,
			Media:       nil, // TODO: Handle media using utils.ExtractMedia if needed later
			Metadata:    metadata,
		}
		wa.eventHandler(msg)
	}
}

package infrastructure

import (
	"context"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
)

// BotTransportAdapter puentea ChannelAdapter a botengine.Transport
type BotTransportAdapter struct {
	Adapter channel.ChannelAdapter
}

func (a *BotTransportAdapter) ID() string {
	return a.Adapter.ID()
}

func (a *BotTransportAdapter) SendMessage(ctx context.Context, chatID string, text string, quoteMessageID string) error {
	_, err := a.Adapter.SendMessage(ctx, chatID, text, quoteMessageID)
	return err
}

func (a *BotTransportAdapter) SendPresence(ctx context.Context, chatID string, isTyping bool, isAudio bool) error {
	return a.Adapter.SendPresence(ctx, chatID, isTyping, isAudio)
}

func (a *BotTransportAdapter) MarkRead(ctx context.Context, chatID string, messageIDs []string) error {
	return a.Adapter.MarkRead(ctx, chatID, messageIDs)
}

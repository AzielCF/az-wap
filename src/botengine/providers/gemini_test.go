package providers

import (
	"context"
	"testing"

	"github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/botengine/domain/bot"
)

func TestGeminiProvider_GenerateReply_Basic(t *testing.T) {
	ctx := context.Background()
	p := NewGeminiProvider(nil)

	b := bot.Bot{
		ID:            "bot-1",
		Enabled:       true,
		APIKey:        "FAKE_KEY",
		MemoryEnabled: true,
	}

	input := domain.BotInput{
		Text: "hola",
		// Injecting history directly to test stateless behavior
		History: []domain.ChatTurn{
			{Role: "user", Text: "Hello"},
			{Role: "assistant", Text: "Hi there!"},
		},
	}

	req := domain.ChatRequest{
		History:  input.History,
		UserText: input.Text,
		Model:    "gemini-pro",
	}

	_, err := p.Chat(ctx, b, req)
	if err == nil {
		t.Errorf("expected error for fake api key, got nil")
	}
}

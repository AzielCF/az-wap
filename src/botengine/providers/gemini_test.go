package providers

import (
	"context"
	"testing"

	"github.com/AzielCF/az-wap/botengine"
	"github.com/AzielCF/az-wap/botengine/domain/bot"
)

func TestGeminiProvider_GenerateReply_Basic(t *testing.T) {
	ctx := context.Background()
	memory := botengine.NewMemoryStore()
	p := NewGeminiProvider(nil, memory)

	b := bot.Bot{
		ID:            "bot-1",
		Enabled:       true,
		APIKey:        "FAKE_KEY", // No llamaremos a la API real si mockeamos o si el test falla antes
		MemoryEnabled: true,
	}

	input := botengine.BotInput{
		BotID:    "bot-1",
		SenderID: "user-1",
		Text:     "hola",
	}

	// Como no queremos llamar a la red en unit tests, aquí solo validamos la lógica previa a la red
	// o usamos un mock del cliente genai si fuera necesario.
	// Por ahora verificamos que falle por API KEY inválida (comportamiento esperado sin red).
	_, err := p.GenerateReply(ctx, b, input, nil)
	if err == nil {
		t.Errorf("expected error for fake api key, got nil")
	}
}

func TestGeminiProvider_MemoryStore_Integration(t *testing.T) {
	memory := botengine.NewMemoryStore()
	key := "bot|bot-1|user-1"

	turn := botengine.ChatTurn{Role: "user", Text: "hi"}
	memory.Save(key, turn, 10)

	history := memory.Get(key)
	if len(history) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(history))
	}
	if history[0].Text != "hi" {
		t.Fatalf("expected 'hi', got %q", history[0].Text)
	}

	memory.Clear(key)
	if len(memory.Get(key)) != 0 {
		t.Fatalf("expected 0 turns after clear")
	}
}

func TestGeminiProvider_MemoryPrefixClear(t *testing.T) {
	memory := botengine.NewMemoryStore()
	memory.Save("bot|b1|u1", botengine.ChatTurn{Text: "1"}, 10)
	memory.Save("bot|b1|u2", botengine.ChatTurn{Text: "2"}, 10)
	memory.Save("bot|b2|u1", botengine.ChatTurn{Text: "3"}, 10)

	memory.ClearPrefix("bot|b1|")

	if len(memory.Get("bot|b1|u1")) != 0 {
		t.Errorf("expected b1|u1 to be cleared")
	}
	if len(memory.Get("bot|b1|u2")) != 0 {
		t.Errorf("expected b1|u2 to be cleared")
	}
	if len(memory.Get("bot|b2|u1")) != 1 {
		t.Errorf("expected b2|u1 to remain")
	}
}

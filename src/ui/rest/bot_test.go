package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	domainBot "github.com/AzielCF/az-wap/domains/bot"
	"github.com/gofiber/fiber/v2"
)

// fakeBotService implementa IBotUsecase pero solo las partes necesarias para este test e2e.
type fakeBotService struct{}

func (f *fakeBotService) Create(ctx context.Context, req domainBot.CreateBotRequest) (domainBot.Bot, error) {
	return domainBot.Bot{}, nil
}

func (f *fakeBotService) List(ctx context.Context) ([]domainBot.Bot, error) {
	return nil, nil
}

func (f *fakeBotService) GetByID(ctx context.Context, id string) (domainBot.Bot, error) {
	return domainBot.Bot{ID: id}, nil
}

func (f *fakeBotService) Update(ctx context.Context, id string, req domainBot.UpdateBotRequest) (domainBot.Bot, error) {
	return domainBot.Bot{}, nil
}

func (f *fakeBotService) Delete(ctx context.Context, id string) error {
	return nil
}

func TestBotHandleWebhook_E2E(t *testing.T) {
	app := fiber.New()

	// Stub de GenerateBotTextReply para no llamar a la API real de Gemini.
	var (
		gotBotID  string
		gotMemID  string
		gotInput  string
		stubReply = "respuesta-e2e"
	)

	origGen := generateBotTextReplyFunc
	t.Cleanup(func() { generateBotTextReplyFunc = origGen })

	generateBotTextReplyFunc = func(ctx context.Context, botID string, memoryID string, input string) (string, error) {
		gotBotID = botID
		gotMemID = memoryID
		gotInput = input
		return stubReply, nil
	}

	service := &fakeBotService{}
	InitRestBot(app, service, nil)

	body := []byte(`{"memory_id":"mem-123","input":"  hola mundo  "}`)
	req := httptest.NewRequest(http.MethodPost, "/bots/bot-123/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d, body=%s", resp.StatusCode, string(b))
	}

	// utils.ResponseData serializa solo code, message y results; Status no va en el JSON.
	var envelope struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Results map[string]interface{} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if envelope.Code != "SUCCESS" || envelope.Message != "Bot reply generated" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}

	if v, ok := envelope.Results["bot_id"].(string); !ok || v != "bot-123" {
		t.Fatalf("expected bot_id 'bot-123', got %#v", envelope.Results["bot_id"])
	}
	if v, ok := envelope.Results["memory_id"].(string); !ok || v != "mem-123" {
		t.Fatalf("expected memory_id 'mem-123', got %#v", envelope.Results["memory_id"])
	}
	if v, ok := envelope.Results["input"].(string); !ok || v != "hola mundo" {
		t.Fatalf("expected input 'hola mundo', got %#v", envelope.Results["input"])
	}
	if v, ok := envelope.Results["reply"].(string); !ok || v != stubReply {
		t.Fatalf("expected reply %q, got %#v", stubReply, envelope.Results["reply"])
	}

	// Verificamos que el stub de Gemini recibió los parámetros correctos.
	if gotBotID != "bot-123" {
		t.Fatalf("expected botID 'bot-123', got %q", gotBotID)
	}
	if gotMemID != "mem-123" {
		t.Fatalf("expected memoryID 'mem-123', got %q", gotMemID)
	}
	if gotInput != "hola mundo" {
		t.Fatalf("expected input 'hola mundo', got %q", gotInput)
	}
}

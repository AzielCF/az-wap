package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/gofiber/fiber/v2"
)

func TestGetBotWebhookPoolStats_Uninitialized(t *testing.T) {
	app := fiber.New()
	app.Get("/api/bot-webhook-pool/stats", GetBotWebhookPoolStats)

	origPool := botWebhookPool
	t.Cleanup(func() { botWebhookPool = origPool })
	botWebhookPool = nil

	req := httptest.NewRequest(http.MethodGet, "/api/bot-webhook-pool/stats", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

func TestGetBotWebhookPoolStats_Initialized(t *testing.T) {
	app := fiber.New()
	app.Get("/api/bot-webhook-pool/stats", GetBotWebhookPoolStats)

	ctx, cancel := context.WithCancel(context.Background())
	pool := msgworker.NewMessageWorkerPool(2, 10)
	pool.Start(ctx)

	origPool := botWebhookPool
	t.Cleanup(func() {
		cancel()
		pool.Stop()
		botWebhookPool = origPool
	})
	botWebhookPool = pool

	req := httptest.NewRequest(http.MethodGet, "/api/bot-webhook-pool/stats", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}

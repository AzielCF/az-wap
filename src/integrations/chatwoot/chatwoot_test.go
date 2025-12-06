package chatwoot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSendTextToConversation_UsesBotTokenWhenFromBot(t *testing.T) {
	ctx := context.Background()
	cfg := &instanceChatwootConfig{
		InstanceID:   "inst-1",
		BaseURL:      "https://chatwoot.test",
		AccountID:    1,
		InboxID:      2,
		AccountToken: "acc-token",
		BotToken:     "bot-token",
		Enabled:      true,
	}

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	var (
		gotMethod string
		gotURL    string
		gotToken  string
		gotBody   []byte
	)

	httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotMethod = req.Method
			gotURL = req.URL.String()
			gotToken = req.Header.Get("api_access_token")
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				gotBody = b
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	attrs := map[string]interface{}{"from_bot": true}
	if err := sendTextToConversation(ctx, cfg, 123, "hello", "incoming", attrs); err != nil {
		t.Fatalf("sendTextToConversation() unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %q", gotMethod)
	}
	wantURL := "https://chatwoot.test/api/v1/accounts/1/conversations/123/messages"
	if gotURL != wantURL {
		t.Fatalf("unexpected URL: got %q, want %q", gotURL, wantURL)
	}
	if gotToken != "bot-token" {
		t.Fatalf("expected bot token %q, got %q", "bot-token", gotToken)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if v, ok := payload["content"].(string); !ok || v != "hello" {
		t.Fatalf("unexpected content: %#v", payload["content"])
	}
	if v, ok := payload["message_type"].(string); !ok || v != "incoming" {
		t.Fatalf("unexpected message_type: %#v", payload["message_type"])
	}

	attrsAny, ok := payload["content_attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("content_attributes not a map: %#v", payload["content_attributes"])
	}
	if v, ok := attrsAny["from_bot"].(bool); !ok || !v {
		t.Fatalf("expected content_attributes.from_bot=true, got %#v", attrsAny["from_bot"])
	}
}

// --- Bot AI Flow (solo WhatsApp) ---

// TestBotAIFlow_InboundUsesAccountToken verifica la parte 1 del flujo:
// el mensaje que llega desde WhatsApp hacia Chatwoot (channel) usa el AccountToken
// y se marca como message_type=incoming.
func TestBotAIFlow_InboundUsesAccountToken(t *testing.T) {
	ctx := context.Background()
	cfg := &instanceChatwootConfig{
		InstanceID:   "inst-flow",
		BaseURL:      "https://chatwoot.flow",
		AccountID:    10,
		InboxID:      20,
		AccountToken: "CHANNEL_TOKEN",
		BotToken:     "BOT_TOKEN",
		Enabled:      true,
	}

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	var (
		gotToken string
		gotBody  []byte
	)

	httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotToken = req.Header.Get("api_access_token")
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				gotBody = b
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if err := sendTextToConversation(ctx, cfg, 999, "hola", "incoming", nil); err != nil {
		t.Fatalf("sendTextToConversation() unexpected error: %v", err)
	}

	if gotToken != "CHANNEL_TOKEN" {
		t.Fatalf("expected AccountToken %q, got %q", "CHANNEL_TOKEN", gotToken)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if v, ok := payload["message_type"].(string); !ok || v != "incoming" {
		t.Fatalf("expected message_type 'incoming', got %#v", payload["message_type"])
	}
}

// TestBotAIFlow_OutboundUsesBotToken verifica la parte 2 del flujo:
// la respuesta de la IA que se envía a Chatwoot debe usar el BotToken y marcar from_bot=true.
func TestBotAIFlow_OutboundUsesBotToken(t *testing.T) {
	ctx := context.Background()
	cfg := &instanceChatwootConfig{
		InstanceID:   "inst-flow",
		BaseURL:      "https://chatwoot.flow",
		AccountID:    10,
		InboxID:      20,
		AccountToken: "CHANNEL_TOKEN",
		BotToken:     "BOT_TOKEN",
		Enabled:      true,
	}

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	var (
		gotToken string
		gotBody  []byte
	)

	httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotToken = req.Header.Get("api_access_token")
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				gotBody = b
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// Simulamos la llamada que hace forwardBotTextMessage/sendTextToConversation
	attrs := map[string]interface{}{"from_bot": true}
	if err := sendTextToConversation(ctx, cfg, 1000, "respuesta IA", "outgoing", attrs); err != nil {
		t.Fatalf("sendTextToConversation() unexpected error: %v", err)
	}

	if gotToken != "BOT_TOKEN" {
		t.Fatalf("expected BotToken %q, got %q", "BOT_TOKEN", gotToken)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if v, ok := payload["message_type"].(string); !ok || v != "outgoing" {
		t.Fatalf("expected message_type 'outgoing', got %#v", payload["message_type"])
	}
	attrsAny, ok := payload["content_attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("content_attributes not a map: %#v", payload["content_attributes"])
	}
	if v, ok := attrsAny["from_bot"].(bool); !ok || !v {
		t.Fatalf("expected content_attributes.from_bot=true, got %#v", attrsAny["from_bot"])
	}
}

func TestSendTextToConversation_OutgoingSetsFromBotAndBotToken(t *testing.T) {
	ctx := context.Background()
	cfg := &instanceChatwootConfig{
		InstanceID:   "inst-1",
		BaseURL:      "https://chatwoot.test",
		AccountID:    1,
		InboxID:      2,
		AccountToken: "acc-token",
		BotToken:     "bot-token",
		Enabled:      true,
	}

	origClient := httpClient
	t.Cleanup(func() { httpClient = origClient })

	var (
		gotToken string
		gotBody  []byte
	)

	httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			gotToken = req.Header.Get("api_access_token")
			if req.Body != nil {
				b, _ := io.ReadAll(req.Body)
				gotBody = b
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	attrs := map[string]interface{}{}
	if err := sendTextToConversation(ctx, cfg, 456, "hello", "outgoing", attrs); err != nil {
		t.Fatalf("sendTextToConversation() unexpected error: %v", err)
	}

	if gotToken != "bot-token" {
		t.Fatalf("expected bot token %q, got %q", "bot-token", gotToken)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if v, ok := payload["message_type"].(string); !ok || v != "outgoing" {
		t.Fatalf("unexpected message_type: %#v", payload["message_type"])
	}

	attrsAny, ok := payload["content_attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("content_attributes not a map: %#v", payload["content_attributes"])
	}
	if v, ok := attrsAny["from_bot"].(bool); !ok || !v {
		t.Fatalf("expected content_attributes.from_bot=true, got %#v", attrsAny["from_bot"])
	}
}

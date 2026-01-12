package instance

import "context"

type Status string

const (
	StatusCreated Status = "CREATED"
	StatusOnline  Status = "ONLINE"
	StatusOffline Status = "OFFLINE"
)

type Instance struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Token  string `json:"token,omitempty"`
	Status Status `json:"status"`

	WebhookURLs               []string `json:"webhook_urls,omitempty"`
	WebhookSecret             string   `json:"webhook_secret,omitempty"`
	WebhookInsecureSkipVerify bool     `json:"webhook_insecure_skip_verify,omitempty"`

	ChatwootBaseURL         string `json:"chatwoot_base_url,omitempty"`
	ChatwootAccountToken    string `json:"chatwoot_account_token,omitempty"`
	ChatwootBotToken        string `json:"chatwoot_bot_token,omitempty"`
	ChatwootAccountID       string `json:"chatwoot_account_id,omitempty"`
	ChatwootInboxID         string `json:"chatwoot_inbox_id,omitempty"`
	ChatwootInboxIdentifier string `json:"chatwoot_inbox_identifier,omitempty"`
	ChatwootEnabled         bool   `json:"chatwoot_enabled,omitempty"`
	ChatwootCredentialID    string `json:"chatwoot_credential_id,omitempty"`

	BotID string `json:"bot_id,omitempty"`

	GeminiEnabled       bool   `json:"gemini_enabled,omitempty"`
	GeminiAPIKey        string `json:"gemini_api_key,omitempty"`
	GeminiModel         string `json:"gemini_model,omitempty"`
	GeminiSystemPrompt  string `json:"gemini_system_prompt,omitempty"`
	GeminiKnowledgeBase string `json:"gemini_knowledge_base,omitempty"`
	GeminiTimezone      string `json:"gemini_timezone,omitempty"`
	GeminiAudioEnabled  bool   `json:"gemini_audio_enabled,omitempty"`
	GeminiImageEnabled  bool   `json:"gemini_image_enabled,omitempty"`
	GeminiMemoryEnabled bool   `json:"gemini_memory_enabled,omitempty"`
	AutoReconnect       bool   `json:"auto_reconnect"`
}

type CreateInstanceRequest struct {
	Name string `json:"name" form:"name"`
}

type IInstanceUsecase interface {
	Create(ctx context.Context, request CreateInstanceRequest) (Instance, error)
	List(ctx context.Context) ([]Instance, error)
	GetByID(ctx context.Context, id string) (Instance, error)
	GetByToken(ctx context.Context, token string) (Instance, error)
	Delete(ctx context.Context, id string) error
	UpdateWebhookConfig(ctx context.Context, id string, urls []string, secret string, insecure bool) (Instance, error)
	UpdateChatwootConfig(ctx context.Context, id string, baseURL, accountID, inboxID, inboxIdentifier, accountToken, botToken, credentialID string, enabled bool) (Instance, error)
	UpdateBotConfig(ctx context.Context, id string, botID string) (Instance, error)
	UpdateGeminiConfig(ctx context.Context, id string, enabled bool, apiKey, model, systemPrompt, knowledgeBase, timezone string, audioEnabled, imageEnabled, memoryEnabled bool) (Instance, error)
	UpdateAutoReconnectConfig(ctx context.Context, id string, enabled bool) (Instance, error)
}

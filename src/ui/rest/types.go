package rest

// LegacyInstanceResponse mirrors the old Instance domain model
// to ensure backward compatibility with the frontend.
type LegacyInstanceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Token  string `json:"token,omitempty"`
	Status string `json:"status"`

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

	AIEnabled       bool               `json:"ai_enabled,omitempty"`
	AIAPIKey        string             `json:"ai_api_key,omitempty"`
	AIModel         string             `json:"ai_model,omitempty"`
	AISystemPrompt  string             `json:"ai_system_prompt,omitempty"`
	AIKnowledgeBase string             `json:"ai_knowledge_base,omitempty"`
	AITimezone      string             `json:"ai_timezone,omitempty"`
	AIAudioEnabled  bool               `json:"ai_audio_enabled,omitempty"`
	AIImageEnabled  bool               `json:"ai_image_enabled,omitempty"`
	AIMemoryEnabled bool               `json:"ai_memory_enabled,omitempty"`
	AutoReconnect   bool               `json:"auto_reconnect"`
	AccumulatedCost float64            `json:"accumulated_cost"`
	CostBreakdown   map[string]float64 `json:"cost_breakdown,omitempty"`
}

type CreateInstanceRequest struct {
	Name string `json:"name" form:"name"`
}

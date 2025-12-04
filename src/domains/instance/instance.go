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
}

type CreateInstanceRequest struct {
	Name string `json:"name" form:"name"`
}

type IInstanceUsecase interface {
	Create(ctx context.Context, request CreateInstanceRequest) (Instance, error)
	List(ctx context.Context) ([]Instance, error)
	GetByToken(ctx context.Context, token string) (Instance, error)
	Delete(ctx context.Context, id string) error
	UpdateWebhookConfig(ctx context.Context, id string, urls []string, secret string, insecure bool) (Instance, error)
	UpdateChatwootConfig(ctx context.Context, id string, baseURL, accountID, inboxID, inboxIdentifier, accountToken, botToken string) (Instance, error)
}

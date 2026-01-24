package credential

import "context"

type Kind string

const (
	KindAI       Kind = "ai"
	KindGemini   Kind = "gemini"
	KindOpenAI   Kind = "openai"
	KindClaude   Kind = "claude"
	KindChatwoot Kind = "chatwoot"
)

type Credential struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Kind                 Kind   `json:"kind"`
	AIAPIKey             string `json:"ai_api_key,omitempty"`
	ChatwootBaseURL      string `json:"chatwoot_base_url,omitempty"`
	ChatwootAccountToken string `json:"chatwoot_account_token,omitempty"`
	ChatwootBotToken     string `json:"chatwoot_bot_token,omitempty"`
}

type CreateCredentialRequest struct {
	Name                 string `json:"name"`
	Kind                 Kind   `json:"kind"`
	AIAPIKey             string `json:"ai_api_key"`
	ChatwootBaseURL      string `json:"chatwoot_base_url"`
	ChatwootAccountToken string `json:"chatwoot_account_token"`
	ChatwootBotToken     string `json:"chatwoot_bot_token"`
}

type UpdateCredentialRequest struct {
	Name                 string `json:"name"`
	Kind                 Kind   `json:"kind"`
	AIAPIKey             string `json:"ai_api_key"`
	ChatwootBaseURL      string `json:"chatwoot_base_url"`
	ChatwootAccountToken string `json:"chatwoot_account_token"`
	ChatwootBotToken     string `json:"chatwoot_bot_token"`
}

type ICredentialUsecase interface {
	Create(ctx context.Context, req CreateCredentialRequest) (Credential, error)
	List(ctx context.Context, kind *Kind) ([]Credential, error)
	GetByID(ctx context.Context, id string) (Credential, error)
	Update(ctx context.Context, id string, req UpdateCredentialRequest) (Credential, error)
	Delete(ctx context.Context, id string) error
	Validate(ctx context.Context, id string) error
}

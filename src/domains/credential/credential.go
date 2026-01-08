package credential

import "context"

type Kind string

const (
	KindGemini   Kind = "gemini"
	KindChatwoot Kind = "chatwoot"
)

type Credential struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Kind                 Kind   `json:"kind"`
	GeminiAPIKey         string `json:"gemini_api_key,omitempty"`
	ChatwootBaseURL      string `json:"chatwoot_base_url,omitempty"`
	ChatwootAccountToken string `json:"chatwoot_account_token,omitempty"`
	ChatwootBotToken     string `json:"chatwoot_bot_token,omitempty"`
}

type CreateCredentialRequest struct {
	Name                 string `json:"name"`
	Kind                 Kind   `json:"kind"`
	GeminiAPIKey         string `json:"gemini_api_key"`
	ChatwootBaseURL      string `json:"chatwoot_base_url"`
	ChatwootAccountToken string `json:"chatwoot_account_token"`
	ChatwootBotToken     string `json:"chatwoot_bot_token"`
}

type UpdateCredentialRequest struct {
	Name                 string `json:"name"`
	GeminiAPIKey         string `json:"gemini_api_key"`
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

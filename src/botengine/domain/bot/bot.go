package bot

import (
	"context"

	domainHealth "github.com/AzielCF/az-wap/domains/health"
)

type Provider string

const (
	ProviderAI     Provider = "ai"
	ProviderGemini Provider = "gemini"
	ProviderOpenAI Provider = "openai"
	ProviderClaude Provider = "claude"
)

type Bot struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Provider    Provider `json:"provider"`
	Enabled     bool     `json:"enabled"`

	APIKey        string `json:"api_key,omitempty"`
	Model         string `json:"model,omitempty"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	KnowledgeBase string `json:"knowledge_base,omitempty"`

	AudioEnabled    bool   `json:"audio_enabled,omitempty"`
	ImageEnabled    bool   `json:"image_enabled,omitempty"`
	VideoEnabled    bool   `json:"video_enabled,omitempty"`
	DocumentEnabled bool   `json:"document_enabled,omitempty"`
	MemoryEnabled   bool   `json:"memory_enabled,omitempty"`
	MindsetModel    string `json:"mindset_model,omitempty"`
	MultimodalModel string `json:"multimodal_model,omitempty"`
	CredentialID    string `json:"credential_id,omitempty"`
	// Chatwoot-specific (optional): allow Bot AI to carry Chatwoot config if needed
	ChatwootCredentialID string `json:"chatwoot_credential_id,omitempty"`
	ChatwootBotToken     string `json:"chatwoot_bot_token,omitempty"`
	// New fields added
	ChatwootCredential ChatwootCredential `json:"chatwoot_credential,omitempty"`
	Whitelist          []string           `json:"whitelist,omitempty"`
}

type ChatwootCredential struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type CreateBotRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Provider    Provider `json:"provider"`

	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	SystemPrompt  string `json:"system_prompt"`
	KnowledgeBase string `json:"knowledge_base"`

	AudioEnabled    bool   `json:"audio_enabled"`
	ImageEnabled    bool   `json:"image_enabled"`
	VideoEnabled    bool   `json:"video_enabled"`
	DocumentEnabled bool   `json:"document_enabled"`
	MemoryEnabled   bool   `json:"memory_enabled"`
	MindsetModel    string `json:"mindset_model"`
	MultimodalModel string `json:"multimodal_model"`
	CredentialID    string `json:"credential_id"`
	// Optional Chatwoot config
	ChatwootCredentialID string   `json:"chatwoot_credential_id"`
	ChatwootBotToken     string   `json:"chatwoot_bot_token"`
	Whitelist            []string `json:"whitelist"`
}

type UpdateBotRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Provider    Provider `json:"provider"`

	APIKey        string `json:"api_key"`
	Model         string `json:"model"`
	SystemPrompt  string `json:"system_prompt"`
	KnowledgeBase string `json:"knowledge_base"`

	AudioEnabled    bool   `json:"audio_enabled"`
	ImageEnabled    bool   `json:"image_enabled"`
	VideoEnabled    bool   `json:"video_enabled"`
	DocumentEnabled bool   `json:"document_enabled"`
	MemoryEnabled   bool   `json:"memory_enabled"`
	MindsetModel    string `json:"mindset_model"`
	MultimodalModel string `json:"multimodal_model"`
	CredentialID    string `json:"credential_id"`
	// Optional Chatwoot config
	ChatwootCredentialID string   `json:"chatwoot_credential_id"`
	ChatwootBotToken     string   `json:"chatwoot_bot_token"`
	Whitelist            []string `json:"whitelist"`
}

type IBotUsecase interface {
	Create(ctx context.Context, req CreateBotRequest) (Bot, error)
	List(ctx context.Context) ([]Bot, error)
	GetByID(ctx context.Context, id string) (Bot, error)
	Update(ctx context.Context, id string, req UpdateBotRequest) (Bot, error)
	Delete(ctx context.Context, id string) error

	SetHealthUsecase(h domainHealth.IHealthUsecase)
	Shutdown()
}

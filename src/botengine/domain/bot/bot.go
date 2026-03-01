package bot

import (
	"context"
	"strings"

	domainHealth "github.com/AzielCF/az-wap/core/common/health/domain"
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

	AudioEnabled    bool   `json:"audio_enabled"`
	ImageEnabled    bool   `json:"image_enabled"`
	VideoEnabled    bool   `json:"video_enabled"`
	DocumentEnabled bool   `json:"document_enabled"`
	MemoryEnabled   bool   `json:"memory_enabled"`
	MindsetModel    string `json:"mindset_model,omitempty"`
	MultimodalModel string `json:"multimodal_model,omitempty"`
	CredentialID    string `json:"credential_id,omitempty"`
	// Chatwoot-specific (optional): allow Bot AI to carry Chatwoot config if needed
	ChatwootCredentialID string `json:"chatwoot_credential_id,omitempty"`
	ChatwootBotToken     string `json:"chatwoot_bot_token,omitempty"`
	// New fields added
	ChatwootCredential ChatwootCredential    `json:"chatwoot_credential,omitempty"`
	Whitelist          []string              `json:"whitelist,omitempty"`
	Variants           map[string]BotVariant `json:"variants,omitempty"`
}

type BotVariant struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	SystemPrompt string   `json:"system_prompt"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	AllowedMCPs  []string `json:"allowed_mcps,omitempty"`

	AudioEnabled    *bool `json:"audio_enabled,omitempty"`
	ImageEnabled    *bool `json:"image_enabled,omitempty"`
	VideoEnabled    *bool `json:"video_enabled,omitempty"`
	DocumentEnabled *bool `json:"document_enabled,omitempty"`
	MemoryEnabled   *bool `json:"memory_enabled,omitempty"`
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
	ChatwootCredentialID string                `json:"chatwoot_credential_id"`
	ChatwootBotToken     string                `json:"chatwoot_bot_token"`
	Whitelist            []string              `json:"whitelist"`
	Variants             map[string]BotVariant `json:"variants"`
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
	ChatwootCredentialID string                `json:"chatwoot_credential_id"`
	ChatwootBotToken     string                `json:"chatwoot_bot_token"`
	Whitelist            []string              `json:"whitelist"`
	Variants             map[string]BotVariant `json:"variants"`
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

func (b *Bot) SanitizeVariants() {
	if b.Variants == nil {
		return
	}

	cleaned := make(map[string]BotVariant)
	for key, variant := range b.Variants {
		name := strings.TrimSpace(variant.Name)
		if name == "" {
			continue // Ignorar variantes sin nombre
		}

		variant.Name = name
		variant.Description = strings.TrimSpace(variant.Description)
		variant.SystemPrompt = strings.TrimSpace(variant.SystemPrompt)

		// Clean tools string slice
		var validTools []string
		for _, t := range variant.AllowedTools {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				validTools = append(validTools, trimmed)
			}
		}
		variant.AllowedTools = validTools

		// Clean mcps string slice
		var validMCPs []string
		for _, m := range variant.AllowedMCPs {
			if trimmed := strings.TrimSpace(m); trimmed != "" {
				validMCPs = append(validMCPs, trimmed)
			}
		}
		variant.AllowedMCPs = validMCPs

		cleaned[key] = variant
	}

	if len(cleaned) == 0 {
		b.Variants = nil
	} else {
		b.Variants = cleaned
	}
}

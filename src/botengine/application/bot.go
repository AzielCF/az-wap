package application

import (
	"context"
	"strings"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/botengine/repository"
	globalConfig "github.com/AzielCF/az-wap/config"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type botService struct {
	repo        domainBot.IBotRepository
	credService domainCredential.ICredentialUsecase
	health      domainHealth.IHealthUsecase
}

// NewBotService inicializa el servicio con el repositorio SQLite por defecto.
func NewBotService(credService domainCredential.ICredentialUsecase) domainBot.IBotUsecase {
	repo, err := repository.NewBotSQLiteRepository()
	if err != nil {
		logrus.WithError(err).Error("[BOT] failed to initialize bot storage, bot operations will be disabled")
		return &botService{repo: nil, credService: credService}
	}
	return &botService{repo: repo, credService: credService}
}

// NewBotServiceWithDeps permite inyectar dependencias para tests o configuraciones personalizadas.
func NewBotServiceWithDeps(repo domainBot.IBotRepository, credService domainCredential.ICredentialUsecase) domainBot.IBotUsecase {
	return &botService{
		repo:        repo,
		credService: credService,
	}
}

func (s *botService) ensureRepo() error {
	if s.repo == nil {
		return pkgError.InternalServerError("bot storage is not initialized")
	}
	return nil
}

// === Bot Management ===

func (s *botService) Create(ctx context.Context, req domainBot.CreateBotRequest) (domainBot.Bot, error) {
	if err := s.ensureRepo(); err != nil {
		return domainBot.Bot{}, err
	}

	// Validación y normalización
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domainBot.Bot{}, pkgError.ValidationError("name: cannot be blank.")
	}

	provider := strings.TrimSpace(string(req.Provider))
	if provider == "" {
		provider = string(domainBot.ProviderAI)
	}

	if provider != string(domainBot.ProviderAI) &&
		provider != string(domainBot.ProviderGemini) &&
		provider != string(domainBot.ProviderOpenAI) &&
		provider != string(domainBot.ProviderClaude) {
		return domainBot.Bot{}, pkgError.ValidationError("provider: unsupported provider.")
	}

	id := uuid.NewString()

	// Mapeo a entidad
	bot := domainBot.Bot{
		ID:                   id,
		Name:                 name,
		Description:          strings.TrimSpace(req.Description),
		Provider:             domainBot.Provider(provider),
		Enabled:              true,
		APIKey:               strings.TrimSpace(req.APIKey),
		Model:                strings.TrimSpace(req.Model),
		SystemPrompt:         strings.TrimSpace(req.SystemPrompt),
		KnowledgeBase:        strings.TrimSpace(req.KnowledgeBase),
		Timezone:             strings.TrimSpace(req.Timezone),
		AudioEnabled:         req.AudioEnabled,
		ImageEnabled:         req.ImageEnabled,
		VideoEnabled:         req.VideoEnabled,
		DocumentEnabled:      req.DocumentEnabled,
		MemoryEnabled:        req.MemoryEnabled,
		MindsetModel:         strings.TrimSpace(req.MindsetModel),
		MultimodalModel:      strings.TrimSpace(req.MultimodalModel),
		CredentialID:         strings.TrimSpace(req.CredentialID),
		ChatwootCredentialID: strings.TrimSpace(req.ChatwootCredentialID),
		ChatwootBotToken:     strings.TrimSpace(req.ChatwootBotToken),
		Whitelist:            req.Whitelist,
	}

	if err := s.repo.Create(ctx, bot); err != nil {
		return domainBot.Bot{}, err
	}

	return bot, nil
}

func (s *botService) List(ctx context.Context) ([]domainBot.Bot, error) {
	if err := s.ensureRepo(); err != nil {
		return nil, err
	}
	bots, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve credentials for each bot in the list
	for i := range bots {
		s.resolveCredentials(ctx, &bots[i])
	}

	return bots, nil
}

func (s *botService) GetByID(ctx context.Context, id string) (domainBot.Bot, error) {
	if err := s.ensureRepo(); err != nil {
		return domainBot.Bot{}, err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return domainBot.Bot{}, pkgError.ValidationError("id: cannot be blank.")
	}

	bot, err := s.repo.GetByID(ctx, trimmed)
	if err != nil {
		return domainBot.Bot{}, err
	}

	if bot.Model == "" {
		bot.Model = domainBot.DefaultGeminiModel
	}

	// Resolve Credentials
	s.resolveCredentials(ctx, &bot)

	return bot, nil
}

func (s *botService) Update(ctx context.Context, id string, req domainBot.UpdateBotRequest) (domainBot.Bot, error) {
	if err := s.ensureRepo(); err != nil {
		return domainBot.Bot{}, err
	}

	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return domainBot.Bot{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}

	provider := strings.TrimSpace(string(req.Provider))
	if provider == "" {
		provider = string(existing.Provider)
	}
	if provider != string(domainBot.ProviderAI) &&
		provider != string(domainBot.ProviderGemini) &&
		provider != string(domainBot.ProviderOpenAI) &&
		provider != string(domainBot.ProviderClaude) {
		return domainBot.Bot{}, pkgError.ValidationError("provider: unsupported provider.")
	}

	updated := existing
	updated.Name = name
	updated.Description = strings.TrimSpace(req.Description)
	updated.Provider = domainBot.Provider(provider)
	updated.APIKey = strings.TrimSpace(req.APIKey)
	updated.Model = strings.TrimSpace(req.Model)
	updated.SystemPrompt = strings.TrimSpace(req.SystemPrompt)
	updated.KnowledgeBase = strings.TrimSpace(req.KnowledgeBase)
	updated.Timezone = strings.TrimSpace(req.Timezone)
	updated.AudioEnabled = req.AudioEnabled
	updated.ImageEnabled = req.ImageEnabled
	updated.VideoEnabled = req.VideoEnabled
	updated.DocumentEnabled = req.DocumentEnabled
	updated.MemoryEnabled = req.MemoryEnabled
	updated.MindsetModel = strings.TrimSpace(req.MindsetModel)
	updated.MultimodalModel = strings.TrimSpace(req.MultimodalModel)
	updated.CredentialID = strings.TrimSpace(req.CredentialID)
	updated.ChatwootCredentialID = strings.TrimSpace(req.ChatwootCredentialID)
	updated.ChatwootBotToken = strings.TrimSpace(req.ChatwootBotToken)
	updated.Whitelist = req.Whitelist

	if err := s.repo.Update(ctx, updated); err != nil {
		return domainBot.Bot{}, err
	}

	return updated, nil
}

func (s *botService) Delete(ctx context.Context, id string) error {
	if err := s.ensureRepo(); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return pkgError.ValidationError("id: cannot be blank.")
	}

	return s.repo.Delete(ctx, trimmed)
}

// === Lifecycle & Health ===

func (s *botService) SetHealthUsecase(h domainHealth.IHealthUsecase) {
	s.health = h
}

func (s *botService) Shutdown() {
	// Limpieza de recursos si fuera necesario
	logrus.Info("[BOT] Service shutdown")
}

// === Helpers ===

func (s *botService) resolveCredentials(ctx context.Context, b *domainBot.Bot) {
	if s.credService != nil && b.APIKey == "" && b.CredentialID != "" {
		logrus.Debugf("[BOT] Resolving credential %s for bot %s", b.CredentialID, b.ID)
		cred, err := s.credService.GetByID(ctx, b.CredentialID)
		if err != nil {
			logrus.Errorf("[BOT] Failed to load credential %s for bot %s: %v", b.CredentialID, b.ID, err)
		} else {
			isAI := cred.Kind == domainCredential.KindAI ||
				cred.Kind == domainCredential.KindGemini ||
				cred.Kind == domainCredential.KindOpenAI ||
				cred.Kind == domainCredential.KindClaude

			if isAI && cred.AIAPIKey != "" {
				b.APIKey = cred.AIAPIKey
				logrus.Debugf("[BOT] API Key resolved from credential %s", cred.Name)
			} else if isAI && cred.AIAPIKey == "" {
				logrus.Warnf("[BOT] Credential %s found but AI API Key is empty", cred.Name)
			}
		}
	}

	// Fallback to Config Variables if still empty
	if b.APIKey == "" {
		// Log only once if we are attempting fallback
		hasLoggedFallback := false

		switch b.Provider {
		case domainBot.ProviderGemini, domainBot.ProviderAI:
			if globalConfig.GeminiAPIKey != "" {
				b.APIKey = globalConfig.GeminiAPIKey
				logrus.Infof("[BOT] Using Gemini API Key from config for bot %s", b.ID)
				hasLoggedFallback = true
			}
		case domainBot.ProviderOpenAI:
			if globalConfig.OpenAIAPIKey != "" {
				b.APIKey = globalConfig.OpenAIAPIKey
				logrus.Infof("[BOT] Using OpenAI API Key from config for bot %s", b.ID)
				hasLoggedFallback = true
			}
		case domainBot.ProviderClaude:
			if globalConfig.ClaudeAPIKey != "" {
				b.APIKey = globalConfig.ClaudeAPIKey
				logrus.Infof("[BOT] Using Claude API Key from config for bot %s", b.ID)
				hasLoggedFallback = true
			}
		}

		// Final fallback for generic AI key
		if b.APIKey == "" {
			if globalConfig.AIApiKey != "" {
				b.APIKey = globalConfig.AIApiKey
				logrus.Infof("[BOT] Using General AI API Key from config for bot %s", b.ID)
				hasLoggedFallback = true
			}
		}

		if !hasLoggedFallback && b.APIKey == "" {
			logrus.Errorf("[BOT] Bot %s has no API Key configured (Database, Credential or Config fallback)", b.ID)
		}
	}

	if b.ChatwootCredentialID != "" && s.credService != nil {
		cred, err := s.credService.GetByID(ctx, b.ChatwootCredentialID)
		if err == nil && cred.Kind == domainCredential.KindChatwoot {
			b.ChatwootCredential = domainBot.ChatwootCredential{
				ID:    cred.ID,
				Token: cred.ChatwootAccountToken,
			}
			if b.ChatwootBotToken == "" {
				b.ChatwootBotToken = cred.ChatwootBotToken
			}
		}
	}
}

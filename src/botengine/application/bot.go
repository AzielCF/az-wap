package application

import (
	"context"
	"strings"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/botengine/repository"
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
	return s.repo.List(ctx)
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
	if s.credService == nil {
		return
	}

	if b.APIKey == "" && b.CredentialID != "" {
		cred, err := s.credService.GetByID(ctx, b.CredentialID)
		isAI := cred.Kind == domainCredential.KindAI ||
			cred.Kind == domainCredential.KindGemini ||
			cred.Kind == domainCredential.KindOpenAI ||
			cred.Kind == domainCredential.KindClaude

		if err == nil && isAI {
			b.APIKey = cred.AIAPIKey
		}
	}

	if b.ChatwootCredentialID != "" {
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

package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
	"gorm.io/gorm"
)

// --- Persistence Model ---

type credentialModel struct {
	ID                   string         `gorm:"primaryKey;column:id"`
	Name                 string         `gorm:"column:name;not null"`
	Kind                 string         `gorm:"column:kind;not null"`
	AIAPIKey             sql.NullString `gorm:"column:ai_api_key"`
	ChatwootBaseURL      sql.NullString `gorm:"column:chatwoot_base_url"`
	ChatwootAccountToken sql.NullString `gorm:"column:chatwoot_account_token"`
	ChatwootBotToken     sql.NullString `gorm:"column:chatwoot_bot_token"`
	CreatedAt            time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

func (credentialModel) TableName() string {
	return "credentials"
}

type credentialService struct {
	db *gorm.DB
}

func (s *credentialService) initSchema(ctx context.Context) error {
	// 1. Manual Migration for column rename if needed (SQLite doesn't support RENAME COLUMN easily in old versions)
	sqlDB, err := s.db.DB()
	if err == nil {
		rows, err := sqlDB.Query(`PRAGMA table_info(credentials)`)
		if err == nil {
			defer rows.Close()
			hasGemini := false
			hasAI := false
			for rows.Next() {
				var (
					cid        int
					name       string
					typeName   string
					notNull    int
					defaultVal interface{}
					pk         int
				)
				if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultVal, &pk); err == nil {
					if name == "gemini_api_key" {
						hasGemini = true
					}
					if name == "ai_api_key" {
						hasAI = true
					}
				}
			}
			if hasGemini && !hasAI {
				if err := s.db.Exec(`ALTER TABLE credentials RENAME COLUMN gemini_api_key TO ai_api_key`).Error; err != nil {
					logrus.WithError(err).Warn("[CREDENTIAL] failed to rename gemini_api_key to ai_api_key")
				}
			}
		}
	}

	// 2. GORM AutoMigrate
	return s.db.AutoMigrate(&credentialModel{})
}

func NewCredentialService(db *gorm.DB) domainCredential.ICredentialUsecase {
	s := &credentialService{db: db}
	if db != nil {
		if err := s.initSchema(context.Background()); err != nil {
			logrus.WithError(err).Error("[CREDENTIAL] failed to init schema")
		}
	} else {
		logrus.Error("[CREDENTIAL] GORM DB is nil, service will be disabled")
	}
	return s
}

func (s *credentialService) ensureDB() error {
	if s.db == nil {
		return pkgError.InternalServerError("credential storage is not initialized")
	}
	return nil
}

func (s *credentialService) Create(ctx context.Context, req domainCredential.CreateCredentialRequest) (domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return domainCredential.Credential{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domainCredential.Credential{}, pkgError.ValidationError("name: cannot be blank.")
	}

	kindStr := strings.TrimSpace(string(req.Kind))
	if kindStr == "" {
		return domainCredential.Credential{}, pkgError.ValidationError("kind: cannot be blank.")
	}

	kind := domainCredential.Kind(kindStr)
	if kind != domainCredential.KindAI &&
		kind != domainCredential.KindGemini &&
		kind != domainCredential.KindOpenAI &&
		kind != domainCredential.KindClaude &&
		kind != domainCredential.KindChatwoot {
		return domainCredential.Credential{}, pkgError.ValidationError("kind: unsupported kind.")
	}

	id := uuid.NewString()

	model := credentialModel{
		ID:                   id,
		Name:                 name,
		Kind:                 string(kind),
		AIAPIKey:             sql.NullString{String: strings.TrimSpace(req.AIAPIKey), Valid: req.AIAPIKey != ""},
		ChatwootBaseURL:      sql.NullString{String: strings.TrimSpace(req.ChatwootBaseURL), Valid: req.ChatwootBaseURL != ""},
		ChatwootAccountToken: sql.NullString{String: strings.TrimSpace(req.ChatwootAccountToken), Valid: req.ChatwootAccountToken != ""},
		ChatwootBotToken:     sql.NullString{String: strings.TrimSpace(req.ChatwootBotToken), Valid: req.ChatwootBotToken != ""},
	}

	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domainCredential.Credential{}, err
	}

	return fromModel(model), nil
}

func (s *credentialService) List(ctx context.Context, kind *domainCredential.Kind) ([]domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	var models []credentialModel
	query := s.db.WithContext(ctx).Order("name ASC")

	if kind != nil && *kind != "" {
		query = query.Where("kind = ?", string(*kind))
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	result := make([]domainCredential.Credential, len(models))
	for i, m := range models {
		result[i] = fromModel(m)
	}

	return result, nil
}

func (s *credentialService) GetByID(ctx context.Context, id string) (domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return domainCredential.Credential{}, err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return domainCredential.Credential{}, pkgError.ValidationError("id: cannot be blank.")
	}

	var model credentialModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", trimmed).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainCredential.Credential{}, pkgError.ValidationError("id: credential not found.")
		}
		return domainCredential.Credential{}, err
	}

	return fromModel(model), nil
}

func (s *credentialService) Update(ctx context.Context, id string, req domainCredential.UpdateCredentialRequest) (domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return domainCredential.Credential{}, err
	}

	var model credentialModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainCredential.Credential{}, pkgError.ValidationError("id: credential not found.")
		}
		return domainCredential.Credential{}, err
	}

	if req.Name != "" {
		model.Name = strings.TrimSpace(req.Name)
	}
	if req.Kind != "" {
		model.Kind = string(req.Kind)
	}
	model.AIAPIKey = sql.NullString{String: strings.TrimSpace(req.AIAPIKey), Valid: req.AIAPIKey != ""}
	model.ChatwootBaseURL = sql.NullString{String: strings.TrimSpace(req.ChatwootBaseURL), Valid: req.ChatwootBaseURL != ""}
	model.ChatwootAccountToken = sql.NullString{String: strings.TrimSpace(req.ChatwootAccountToken), Valid: req.ChatwootAccountToken != ""}
	model.ChatwootBotToken = sql.NullString{String: strings.TrimSpace(req.ChatwootBotToken), Valid: req.ChatwootBotToken != ""}

	if err := s.db.WithContext(ctx).Save(&model).Error; err != nil {
		return domainCredential.Credential{}, err
	}

	return fromModel(model), nil
}

func (s *credentialService) Delete(ctx context.Context, id string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return pkgError.ValidationError("id: cannot be blank.")
	}

	result := s.db.WithContext(ctx).Delete(&credentialModel{}, "id = ?", trimmed)
	return result.Error
}

func (s *credentialService) Validate(ctx context.Context, id string) error {
	cred, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	isAI := cred.Kind == domainCredential.KindAI ||
		cred.Kind == domainCredential.KindGemini ||
		cred.Kind == domainCredential.KindOpenAI ||
		cred.Kind == domainCredential.KindClaude

	if isAI {
		if cred.AIAPIKey == "" {
			return fmt.Errorf("missing AI API key")
		}

		if cred.Kind == domainCredential.KindGemini || cred.Kind == domainCredential.KindAI {
			client, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey:  cred.AIAPIKey,
				Backend: genai.BackendGeminiAPI,
			})
			if err != nil {
				return fmt.Errorf("failed to create Gemini client: %w", err)
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			_, err = client.Models.List(timeoutCtx, nil)
			if err != nil {
				return fmt.Errorf("AI API key verification failed: %w", err)
			}
		}

		return nil
	}

	if cred.Kind == domainCredential.KindChatwoot {
		if cred.ChatwootBaseURL == "" || cred.ChatwootAccountToken == "" {
			return fmt.Errorf("missing Chatwoot configuration")
		}
		return nil
	}

	return fmt.Errorf("unsupported credential kind: %s", cred.Kind)
}

// --- Helpers ---

func fromModel(m credentialModel) domainCredential.Credential {
	return domainCredential.Credential{
		ID:                   m.ID,
		Name:                 m.Name,
		Kind:                 domainCredential.Kind(m.Kind),
		AIAPIKey:             nullStringValue(m.AIAPIKey),
		ChatwootBaseURL:      nullStringValue(m.ChatwootBaseURL),
		ChatwootAccountToken: nullStringValue(m.ChatwootAccountToken),
		ChatwootBotToken:     nullStringValue(m.ChatwootBotToken),
	}
}

// nullStringValue returns a trimmed string or empty if null to prevent legacy data panics.
func nullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return strings.TrimSpace(ns.String)
}

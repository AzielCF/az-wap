package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

type credentialService struct {
	db *sql.DB
}

func initCredentialStorageDB() (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	createTable := `
		CREATE TABLE IF NOT EXISTS credentials (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			gemini_api_key TEXT,
			chatwoot_base_url TEXT,
			chatwoot_account_token TEXT,
			chatwoot_bot_token TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	if _, err := db.Exec(createTable); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func NewCredentialService() domainCredential.ICredentialUsecase {
	db, err := initCredentialStorageDB()
	if err != nil {
		logrus.WithError(err).Error("[CREDENTIAL] failed to initialize credential storage, operations will be disabled")
		return &credentialService{db: nil}
	}
	return &credentialService{db: db}
}

func (s *credentialService) ensureDB() error {
	if s.db == nil {
		return fmt.Errorf("credential storage is not initialized")
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
	if kind != domainCredential.KindGemini && kind != domainCredential.KindChatwoot {
		return domainCredential.Credential{}, pkgError.ValidationError("kind: unsupported kind.")
	}

	geminiAPIKey := strings.TrimSpace(req.GeminiAPIKey)
	chatwootBaseURL := strings.TrimSpace(req.ChatwootBaseURL)
	chatwootAccountToken := strings.TrimSpace(req.ChatwootAccountToken)
	chatwootBotToken := strings.TrimSpace(req.ChatwootBotToken)

	if kind == domainCredential.KindGemini {
		if geminiAPIKey == "" {
			return domainCredential.Credential{}, pkgError.ValidationError("gemini_api_key: cannot be blank for gemini credentials.")
		}
	}

	if kind == domainCredential.KindChatwoot {
		if chatwootBaseURL == "" {
			return domainCredential.Credential{}, pkgError.ValidationError("chatwoot_base_url: cannot be blank for chatwoot credentials.")
		}
		if chatwootAccountToken == "" {
			return domainCredential.Credential{}, pkgError.ValidationError("chatwoot_account_token: cannot be blank for chatwoot credentials.")
		}
	}

	id := uuid.NewString()

	cred := domainCredential.Credential{
		ID:                   id,
		Name:                 name,
		Kind:                 kind,
		GeminiAPIKey:         geminiAPIKey,
		ChatwootBaseURL:      chatwootBaseURL,
		ChatwootAccountToken: chatwootAccountToken,
		ChatwootBotToken:     chatwootBotToken,
	}

	query := `
		INSERT INTO credentials (
			id, name, kind, gemini_api_key, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token
		) VALUES (?, ?, ?, ?, ?, ?, ?);
	`

	if _, err := s.db.ExecContext(ctx, query,
		cred.ID, cred.Name, string(cred.Kind),
		cred.GeminiAPIKey, cred.ChatwootBaseURL, cred.ChatwootAccountToken, cred.ChatwootBotToken,
	); err != nil {
		return domainCredential.Credential{}, err
	}

	return cred, nil
}

func (s *credentialService) List(ctx context.Context, kind *domainCredential.Kind) ([]domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	var (
		rows *sql.Rows
		err  error
	)

	if kind != nil && *kind != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, kind, gemini_api_key, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token
			FROM credentials
			WHERE kind = ?
			ORDER BY name ASC;
		`, string(*kind))
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, name, kind, gemini_api_key, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token
			FROM credentials
			ORDER BY name ASC;
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domainCredential.Credential
	for rows.Next() {
		var cred domainCredential.Credential
		var kindStr string
		if err := rows.Scan(&cred.ID, &cred.Name, &kindStr, &cred.GeminiAPIKey, &cred.ChatwootBaseURL, &cred.ChatwootAccountToken, &cred.ChatwootBotToken); err != nil {
			return nil, err
		}
		cred.Kind = domainCredential.Kind(kindStr)
		result = append(result, cred)
	}

	return result, rows.Err()
}

func (s *credentialService) GetByID(ctx context.Context, id string) (domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return domainCredential.Credential{}, err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return domainCredential.Credential{}, pkgError.ValidationError("id: cannot be blank.")
	}

	var cred domainCredential.Credential
	var kindStr string

	query := `
		SELECT id, name, kind, gemini_api_key, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token
		FROM credentials
		WHERE id = ?;
	`

	err := s.db.QueryRowContext(ctx, query, trimmed).Scan(
		&cred.ID, &cred.Name, &kindStr, &cred.GeminiAPIKey, &cred.ChatwootBaseURL, &cred.ChatwootAccountToken, &cred.ChatwootBotToken,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domainCredential.Credential{}, pkgError.ValidationError("id: credential not found.")
		}
		return domainCredential.Credential{}, err
	}

	cred.Kind = domainCredential.Kind(kindStr)
	return cred, nil
}

func (s *credentialService) Update(ctx context.Context, id string, req domainCredential.UpdateCredentialRequest) (domainCredential.Credential, error) {
	if err := s.ensureDB(); err != nil {
		return domainCredential.Credential{}, err
	}

	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return domainCredential.Credential{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}

	updated := existing
	updated.Name = name
	updated.GeminiAPIKey = strings.TrimSpace(req.GeminiAPIKey)
	updated.ChatwootBaseURL = strings.TrimSpace(req.ChatwootBaseURL)
	updated.ChatwootAccountToken = strings.TrimSpace(req.ChatwootAccountToken)
	updated.ChatwootBotToken = strings.TrimSpace(req.ChatwootBotToken)

	query := `
		UPDATE credentials
		SET name = ?, gemini_api_key = ?, chatwoot_base_url = ?, chatwoot_account_token = ?, chatwoot_bot_token = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?;
	`

	if _, err := s.db.ExecContext(ctx, query,
		updated.Name, updated.GeminiAPIKey, updated.ChatwootBaseURL, updated.ChatwootAccountToken, updated.ChatwootBotToken,
		updated.ID,
	); err != nil {
		return domainCredential.Credential{}, err
	}

	return updated, nil
}

func (s *credentialService) Delete(ctx context.Context, id string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return pkgError.ValidationError("id: cannot be blank.")
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = ?;`, trimmed)
	return err
}
func (s *credentialService) Validate(ctx context.Context, id string) error {
	cred, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cred.Kind == domainCredential.KindGemini {
		if cred.GeminiAPIKey == "" {
			return fmt.Errorf("missing Gemini API key")
		}
		// Attempt to list models to verify API Key
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  cred.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return fmt.Errorf("failed to create Gemini client: %w", err)
		}

		// Use a short timeout for health check
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_, err = client.Models.List(timeoutCtx, nil)
		if err != nil {
			return fmt.Errorf("Gemini API key verification failed: %w", err)
		}
		return nil
	}

	if cred.Kind == domainCredential.KindChatwoot {
		if cred.ChatwootBaseURL == "" || cred.ChatwootAccountToken == "" {
			return fmt.Errorf("missing Chatwoot configuration")
		}
		// Simple HTTP check (placeholder for now, just checking reachable)
		return nil
	}

	return fmt.Errorf("unsupported credential kind: %s", cred.Kind)
}

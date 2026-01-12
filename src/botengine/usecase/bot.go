package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/config"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"

	domainCredential "github.com/AzielCF/az-wap/domains/credential"
)

type botService struct {
	db          *sql.DB
	credService domainCredential.ICredentialUsecase
}

func initBotStorageDB() (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	createTable := `
		CREATE TABLE IF NOT EXISTS bots (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			provider TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			api_key TEXT,
			model TEXT,
			system_prompt TEXT,
			knowledge_base TEXT,
			timezone TEXT,
			audio_enabled INTEGER NOT NULL DEFAULT 0,
			image_enabled INTEGER NOT NULL DEFAULT 0,
			memory_enabled INTEGER NOT NULL DEFAULT 0,
			credential_id TEXT,
			chatwoot_credential_id TEXT,
			chatwoot_bot_token TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	if _, err := db.Exec(createTable); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Migración ligera para añadir columna credential_id en bases antiguas.
	rows, err := db.Query(`PRAGMA table_info(bots)`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid        int
			name       string
			typeName   string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultVal, &pk); err != nil {
			_ = db.Close()
			return nil, err
		}
		columns[name] = true
	}
	if !columns["credential_id"] {
		if _, err := db.Exec(`ALTER TABLE bots ADD COLUMN credential_id TEXT`); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if !columns["chatwoot_credential_id"] {
		if _, err := db.Exec(`ALTER TABLE bots ADD COLUMN chatwoot_credential_id TEXT`); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if !columns["chatwoot_bot_token"] {
		if _, err := db.Exec(`ALTER TABLE bots ADD COLUMN chatwoot_bot_token TEXT`); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if !columns["whitelist"] {
		if _, err := db.Exec(`ALTER TABLE bots ADD COLUMN whitelist TEXT`); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return db, nil
}

func NewBotService(credService domainCredential.ICredentialUsecase) domainBot.IBotUsecase {
	db, err := initBotStorageDB()
	if err != nil {
		logrus.WithError(err).Error("[BOT] failed to initialize bot storage, bot operations will be disabled")
		return &botService{db: nil, credService: credService}
	}
	return &botService{db: db, credService: credService}
}

func (s *botService) ensureDB() error {
	if s.db == nil {
		return fmt.Errorf("bot storage is not initialized")
	}
	return nil
}

func (s *botService) Create(ctx context.Context, req domainBot.CreateBotRequest) (domainBot.Bot, error) {
	if err := s.ensureDB(); err != nil {
		return domainBot.Bot{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domainBot.Bot{}, pkgError.ValidationError("name: cannot be blank.")
	}

	provider := strings.TrimSpace(string(req.Provider))
	if provider == "" {
		provider = string(domainBot.ProviderGemini)
	}

	if provider != string(domainBot.ProviderGemini) {
		return domainBot.Bot{}, pkgError.ValidationError("provider: unsupported provider.")
	}

	id := uuid.NewString()

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
		MemoryEnabled:        req.MemoryEnabled,
		CredentialID:         strings.TrimSpace(req.CredentialID),
		ChatwootCredentialID: strings.TrimSpace(req.ChatwootCredentialID),
		ChatwootBotToken:     strings.TrimSpace(req.ChatwootBotToken),
		Whitelist:            req.Whitelist,
	}

	query := `
		INSERT INTO bots (
			id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base, timezone,
			audio_enabled, image_enabled, memory_enabled, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	whitelistStr := strings.Join(bot.Whitelist, ",")

	if _, err := s.db.ExecContext(ctx, query,
		bot.ID, bot.Name, bot.Description, bot.Provider, 1,
		bot.APIKey, bot.Model, bot.SystemPrompt, bot.KnowledgeBase, bot.Timezone,
		boolToInt(bot.AudioEnabled), boolToInt(bot.ImageEnabled), boolToInt(bot.MemoryEnabled), strings.TrimSpace(bot.CredentialID), strings.TrimSpace(bot.ChatwootCredentialID), strings.TrimSpace(bot.ChatwootBotToken),
		whitelistStr,
	); err != nil {
		return domainBot.Bot{}, err
	}

	return bot, nil
}

func (s *botService) List(ctx context.Context) ([]domainBot.Bot, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base, timezone,
			audio_enabled, image_enabled, memory_enabled, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		FROM bots
		ORDER BY name ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domainBot.Bot
	for rows.Next() {
		var (
			b                      domainBot.Bot
			enabledVal             int
			audioEnabledVal        int
			imageEnabledVal        int
			memoryEnabledVal       int
			credID                 sql.NullString
			chatwootCredID, botTkn sql.NullString
			whitelistStr           sql.NullString
		)
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Description, &b.Provider, &enabledVal,
			&b.APIKey, &b.Model, &b.SystemPrompt, &b.KnowledgeBase, &b.Timezone,
			&audioEnabledVal, &imageEnabledVal, &memoryEnabledVal, &credID, &chatwootCredID, &botTkn,
			&whitelistStr,
		); err != nil {
			return nil, err
		}
		b.Enabled = enabledVal != 0
		b.AudioEnabled = audioEnabledVal != 0
		b.ImageEnabled = imageEnabledVal != 0
		b.MemoryEnabled = memoryEnabledVal != 0
		if credID.Valid {
			b.CredentialID = strings.TrimSpace(credID.String)
		} else {
			b.CredentialID = ""
		}
		if chatwootCredID.Valid {
			b.ChatwootCredentialID = strings.TrimSpace(chatwootCredID.String)
		} else {
			b.ChatwootCredentialID = ""
		}
		if botTkn.Valid {
			b.ChatwootBotToken = strings.TrimSpace(botTkn.String)
		} else {
			b.ChatwootBotToken = ""
		}
		if whitelistStr.Valid && strings.TrimSpace(whitelistStr.String) != "" {
			b.Whitelist = strings.Split(whitelistStr.String, ",")
		}
		result = append(result, b)
	}

	return result, rows.Err()
}

func (s *botService) GetByID(ctx context.Context, id string) (domainBot.Bot, error) {
	if err := s.ensureDB(); err != nil {
		return domainBot.Bot{}, err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return domainBot.Bot{}, pkgError.ValidationError("id: cannot be blank.")
	}

	var (
		b                domainBot.Bot
		enabledVal       int
		audioEnabledVal  int
		imageEnabledVal  int
		memoryEnabledVal int
		credID           sql.NullString
		chatwootCredID   sql.NullString
		botTkn           sql.NullString
		whitelistStr     sql.NullString
	)

	query := `
		SELECT id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base, timezone,
			audio_enabled, image_enabled, memory_enabled, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		FROM bots
		WHERE id = ?;
	`

	err := s.db.QueryRowContext(ctx, query, trimmed).Scan(
		&b.ID, &b.Name, &b.Description, &b.Provider, &enabledVal,
		&b.APIKey, &b.Model, &b.SystemPrompt, &b.KnowledgeBase, &b.Timezone,
		&audioEnabledVal, &imageEnabledVal, &memoryEnabledVal, &credID, &chatwootCredID, &botTkn,
		&whitelistStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domainBot.Bot{}, pkgError.NotFoundError("bot not found")
		}
		return domainBot.Bot{}, err
	}
	b.ID = trimmed
	b.Enabled = enabledVal != 0
	b.AudioEnabled = audioEnabledVal != 0
	b.ImageEnabled = imageEnabledVal != 0
	b.MemoryEnabled = memoryEnabledVal != 0
	if credID.Valid {
		b.CredentialID = strings.TrimSpace(credID.String)
	} else {
		b.CredentialID = ""
	}
	if chatwootCredID.Valid {
		b.ChatwootCredentialID = strings.TrimSpace(chatwootCredID.String)
	} else {
		b.ChatwootCredentialID = ""
	}
	if botTkn.Valid {
		b.ChatwootBotToken = strings.TrimSpace(botTkn.String)
	} else {
		b.ChatwootBotToken = ""
	}
	if whitelistStr.Valid && strings.TrimSpace(whitelistStr.String) != "" {
		b.Whitelist = strings.Split(whitelistStr.String, ",")
	}

	if b.Model == "" {
		b.Model = "gemini-2.0-flash-exp"
	}

	// Resolve Credentials
	s.resolveCredentials(ctx, &b)

	return b, nil
}

func (s *botService) resolveCredentials(ctx context.Context, b *domainBot.Bot) {
	if s.credService == nil {
		return
	}

	// 1. AI API Key resolution
	if b.APIKey == "" && b.CredentialID != "" {
		cred, err := s.credService.GetByID(ctx, b.CredentialID)
		if err == nil && cred.Kind == domainCredential.KindGemini {
			b.APIKey = cred.GeminiAPIKey
		}
	}

	// 2. Chatwoot resolution
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

func (s *botService) GetByInstanceID(ctx context.Context, instanceID string) (domainBot.Bot, error) {
	if err := s.ensureDB(); err != nil {
		return domainBot.Bot{}, err
	}

	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return domainBot.Bot{}, pkgError.ValidationError("instanceID: cannot be blank.")
	}

	var enabledVal, audioEnabledVal, imageEnabledVal, memoryEnabledVal int
	var apiKey, model, systemPrompt, knowledgeBase, timezone, botID sql.NullString
	query := `
		SELECT 
			gemini_enabled, gemini_api_key, gemini_model, gemini_system_prompt, gemini_knowledge_base, gemini_timezone, 
			gemini_audio_enabled, gemini_image_enabled, gemini_memory_enabled, bot_id 
		FROM instances WHERE id = ?
	`
	err := s.db.QueryRowContext(ctx, query, trimmed).Scan(
		&enabledVal, &apiKey, &model, &systemPrompt, &knowledgeBase, &timezone,
		&audioEnabledVal, &imageEnabledVal, &memoryEnabledVal, &botID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domainBot.Bot{}, pkgError.NotFoundError("instance not found")
		}
		return domainBot.Bot{}, err
	}

	// Si tiene un bot_id, cargamos ese bot directamente
	if botID.Valid && strings.TrimSpace(botID.String) != "" {
		return s.GetByID(ctx, strings.TrimSpace(botID.String))
	}

	// Si no tiene bot_id pero la IA está habilitada en la instancia (legacy)
	if enabledVal == 0 {
		return domainBot.Bot{Enabled: false}, nil
	}

	b := domainBot.Bot{
		ID:            "legacy-" + trimmed,
		Name:          "Legacy Instance Bot",
		Provider:      domainBot.ProviderGemini,
		Enabled:       true,
		APIKey:        strings.TrimSpace(apiKey.String),
		Model:         strings.TrimSpace(model.String),
		SystemPrompt:  strings.TrimSpace(systemPrompt.String),
		KnowledgeBase: strings.TrimSpace(knowledgeBase.String),
		Timezone:      strings.TrimSpace(timezone.String),
		AudioEnabled:  audioEnabledVal != 0,
		ImageEnabled:  imageEnabledVal != 0,
		MemoryEnabled: memoryEnabledVal != 0,
	}

	if b.Model == "" {
		b.Model = "gemini-2.0-flash-exp" // Default
	}

	// Resolve Credentials for heritage bots too
	s.resolveCredentials(ctx, &b)

	return b, nil
}

func (s *botService) Update(ctx context.Context, id string, req domainBot.UpdateBotRequest) (domainBot.Bot, error) {
	if err := s.ensureDB(); err != nil {
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
	if provider != string(domainBot.ProviderGemini) {
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
	updated.MemoryEnabled = req.MemoryEnabled
	updated.CredentialID = strings.TrimSpace(req.CredentialID)
	updated.ChatwootCredentialID = strings.TrimSpace(req.ChatwootCredentialID)
	updated.ChatwootBotToken = strings.TrimSpace(req.ChatwootBotToken)
	updated.Whitelist = req.Whitelist

	query := `
		UPDATE bots
		SET name = ?, description = ?, provider = ?,
			api_key = ?, model = ?, system_prompt = ?, knowledge_base = ?, timezone = ?,
			audio_enabled = ?, image_enabled = ?, memory_enabled = ?, credential_id = ?, chatwoot_credential_id = ?, chatwoot_bot_token = ?,
			whitelist = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?;
	`

	if _, err := s.db.ExecContext(ctx, query,
		updated.Name, updated.Description, updated.Provider,
		updated.APIKey, updated.Model, updated.SystemPrompt, updated.KnowledgeBase, updated.Timezone,
		boolToInt(updated.AudioEnabled), boolToInt(updated.ImageEnabled), boolToInt(updated.MemoryEnabled), strings.TrimSpace(updated.CredentialID), strings.TrimSpace(updated.ChatwootCredentialID), strings.TrimSpace(updated.ChatwootBotToken),
		strings.Join(updated.Whitelist, ","),
		updated.ID,
	); err != nil {
		return domainBot.Bot{}, err
	}

	return updated, nil
}

func (s *botService) Delete(ctx context.Context, id string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return pkgError.ValidationError("id: cannot be blank.")
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM bots WHERE id = ?;`, trimmed)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

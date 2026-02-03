package repository

import (
	"context"
	"database/sql"
	"strings"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/config"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// BotSQLiteRepository implementa IBotRepository usando SQLite.
type BotSQLiteRepository struct {
	db *sql.DB
}

// NewBotSQLiteRepository crea una nueva instancia del repositorio SQLite.
func NewBotSQLiteRepository() (*BotSQLiteRepository, error) {
	db, err := config.GetAppDB()
	if err != nil {
		return nil, err
	}
	repo := &BotSQLiteRepository{db: db}
	if err := repo.Init(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

// NewBotSQLiteRepositoryWithDB crea una instancia usando una DB proporcionada (útil para tests).
func NewBotSQLiteRepositoryWithDB(db *sql.DB) (*BotSQLiteRepository, error) {
	repo := &BotSQLiteRepository{db: db}
	if err := repo.Init(context.Background()); err != nil {
		return nil, err
	}
	return repo, nil
}

// Init inicializa el esquema de la base de datos (tablas, migraciones).
func (r *BotSQLiteRepository) Init(ctx context.Context) error {
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

			audio_enabled INTEGER NOT NULL DEFAULT 0,
			image_enabled INTEGER NOT NULL DEFAULT 0,
			video_enabled INTEGER NOT NULL DEFAULT 0,
			document_enabled INTEGER NOT NULL DEFAULT 0,
			memory_enabled INTEGER NOT NULL DEFAULT 0,
			mindset_model TEXT,
			multimodal_model TEXT,
			credential_id TEXT,
			chatwoot_credential_id TEXT,
			chatwoot_bot_token TEXT,
			whitelist TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	if _, err := r.db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	return r.runMigrations(ctx)
}

func (r *BotSQLiteRepository) runMigrations(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `PRAGMA table_info(bots)`)
	if err != nil {
		return err
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
			return err
		}
		columns[name] = true
	}

	migrations := []struct {
		column string
		ddl    string
	}{
		{"credential_id", "ALTER TABLE bots ADD COLUMN credential_id TEXT"},
		{"chatwoot_credential_id", "ALTER TABLE bots ADD COLUMN chatwoot_credential_id TEXT"},
		{"chatwoot_bot_token", "ALTER TABLE bots ADD COLUMN chatwoot_bot_token TEXT"},
		{"whitelist", "ALTER TABLE bots ADD COLUMN whitelist TEXT"},
		{"video_enabled", "ALTER TABLE bots ADD COLUMN video_enabled INTEGER NOT NULL DEFAULT 0"},
		{"mindset_model", "ALTER TABLE bots ADD COLUMN mindset_model TEXT"},
		{"multimodal_model", "ALTER TABLE bots ADD COLUMN multimodal_model TEXT"},
		{"document_enabled", "ALTER TABLE bots ADD COLUMN document_enabled INTEGER NOT NULL DEFAULT 0"},
	}

	for _, m := range migrations {
		if !columns[m.column] {
			if _, err := r.db.ExecContext(ctx, m.ddl); err != nil {
				logrus.WithError(err).Warnf("[BotRepo] Failed to add column %s", m.column)
			}
		}
	}

	// Drop timezone column if it exists (Cleanup)
	if columns["timezone"] {
		if _, err := r.db.ExecContext(ctx, "ALTER TABLE bots DROP COLUMN timezone"); err != nil {
			logrus.WithError(err).Warn("[BotRepo] Failed to drop column timezone")
		} else {
			logrus.Info("[BotRepo] Dropped column timezone")
		}
	}

	return nil
}

// Create inserta un nuevo bot en la base de datos. (Garantizado: coincide con original)
func (r *BotSQLiteRepository) Create(ctx context.Context, bot domainBot.Bot) error {
	query := `
		INSERT INTO bots (
			id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base,
			audio_enabled, image_enabled, video_enabled, document_enabled, memory_enabled,
			mindset_model, multimodal_model, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	whitelistStr := strings.Join(bot.Whitelist, ",")

	_, err := r.db.ExecContext(ctx, query,
		bot.ID, bot.Name, bot.Description, bot.Provider, boolToInt(bot.Enabled),
		bot.APIKey, bot.Model, bot.SystemPrompt, bot.KnowledgeBase,
		boolToInt(bot.AudioEnabled), boolToInt(bot.ImageEnabled), boolToInt(bot.VideoEnabled),
		boolToInt(bot.DocumentEnabled), boolToInt(bot.MemoryEnabled),
		bot.MindsetModel, bot.MultimodalModel, bot.CredentialID, bot.ChatwootCredentialID, bot.ChatwootBotToken,
		whitelistStr,
	)
	return err
}

// GetByID obtiene un bot por su ID. (Garantizado: coincide con lógica original de mapeo)
func (r *BotSQLiteRepository) GetByID(ctx context.Context, id string) (domainBot.Bot, error) {
	var (
		b                  domainBot.Bot
		enabledVal         int
		audioEnabledVal    int
		imageEnabledVal    int
		videoEnabledVal    int
		documentEnabledVal int
		memoryEnabledVal   int
		mindsetModel       sql.NullString
		multimodalModel    sql.NullString
		credID             sql.NullString
		chatwootCredID     sql.NullString
		botTkn             sql.NullString
		whitelistStr       sql.NullString
	)

	query := `
		SELECT id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base,
			audio_enabled, image_enabled, video_enabled, document_enabled, memory_enabled,
			mindset_model, multimodal_model, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		FROM bots
		WHERE id = ?;
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.Name, &b.Description, &b.Provider, &enabledVal,
		&b.APIKey, &b.Model, &b.SystemPrompt, &b.KnowledgeBase,
		&audioEnabledVal, &imageEnabledVal, &videoEnabledVal, &documentEnabledVal, &memoryEnabledVal,
		&mindsetModel, &multimodalModel, &credID, &chatwootCredID, &botTkn,
		&whitelistStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domainBot.Bot{}, pkgError.NotFoundError("bot not found")
		}
		return domainBot.Bot{}, err
	}

	b.Enabled = enabledVal != 0
	b.AudioEnabled = audioEnabledVal != 0
	b.ImageEnabled = imageEnabledVal != 0
	b.VideoEnabled = videoEnabledVal != 0
	b.DocumentEnabled = documentEnabledVal != 0
	b.MemoryEnabled = memoryEnabledVal != 0

	// Mapeo EXACTO al original de bot copy.txt
	b.MindsetModel = mindsetModel.String
	b.MultimodalModel = multimodalModel.String

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
	} else {
		b.Whitelist = nil
	}

	return b, nil
}

// List retorna todos los bots ordenados por nombre.
func (r *BotSQLiteRepository) List(ctx context.Context) ([]domainBot.Bot, error) {
	query := `
		SELECT id, name, description, provider, enabled,
			api_key, model, system_prompt, knowledge_base,
			audio_enabled, image_enabled, video_enabled, document_enabled, memory_enabled,
			mindset_model, multimodal_model, credential_id, chatwoot_credential_id, chatwoot_bot_token,
			whitelist
		FROM bots
		ORDER BY name ASC;
	`

	rows, err := r.db.QueryContext(ctx, query)
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
			videoEnabledVal        int
			documentEnabledVal     int
			memoryEnabledVal       int
			mindsetModel           sql.NullString
			multimodalModel        sql.NullString
			credID                 sql.NullString
			chatwootCredID, botTkn sql.NullString
			whitelistStr           sql.NullString
		)
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Description, &b.Provider, &enabledVal,
			&b.APIKey, &b.Model, &b.SystemPrompt, &b.KnowledgeBase,
			&audioEnabledVal, &imageEnabledVal, &videoEnabledVal, &documentEnabledVal, &memoryEnabledVal,
			&mindsetModel, &multimodalModel, &credID, &chatwootCredID, &botTkn,
			&whitelistStr,
		); err != nil {
			return nil, err
		}

		b.Enabled = enabledVal != 0
		b.AudioEnabled = audioEnabledVal != 0
		b.ImageEnabled = imageEnabledVal != 0
		b.VideoEnabled = videoEnabledVal != 0
		b.DocumentEnabled = documentEnabledVal != 0
		b.MemoryEnabled = memoryEnabledVal != 0

		// Mapeo EXACTO al original de bot copy.txt
		b.MindsetModel = mindsetModel.String
		b.MultimodalModel = multimodalModel.String

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
		} else {
			b.Whitelist = nil
		}

		result = append(result, b)
	}

	return result, rows.Err()
}

// Update actualiza un bot existente.
func (r *BotSQLiteRepository) Update(ctx context.Context, bot domainBot.Bot) error {
	query := `
		UPDATE bots
		SET name = ?, description = ?, provider = ?,
			api_key = ?, model = ?, system_prompt = ?, knowledge_base = ?,
			audio_enabled = ?, image_enabled = ?, video_enabled = ?, document_enabled = ?, memory_enabled = ?,
			mindset_model = ?, multimodal_model = ?, credential_id = ?, chatwoot_credential_id = ?, chatwoot_bot_token = ?,
			whitelist = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?;
	`

	_, err := r.db.ExecContext(ctx, query,
		bot.Name, bot.Description, bot.Provider,
		bot.APIKey, bot.Model, bot.SystemPrompt, bot.KnowledgeBase,
		boolToInt(bot.AudioEnabled), boolToInt(bot.ImageEnabled), boolToInt(bot.VideoEnabled),
		boolToInt(bot.DocumentEnabled), boolToInt(bot.MemoryEnabled),
		bot.MindsetModel, bot.MultimodalModel, bot.CredentialID, bot.ChatwootCredentialID, bot.ChatwootBotToken,
		strings.Join(bot.Whitelist, ","),
		bot.ID,
	)
	return err
}

// Delete elimina un bot por su ID.
func (r *BotSQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bots WHERE id = ?;`, id)
	return err
}

// Helper functions (Garantizado: coincide con original)
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/AzielCF/az-wap/config"
	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	infraChatStorage "github.com/AzielCF/az-wap/infrastructure/chatstorage"
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type instanceService struct {
	mu               sync.RWMutex
	instancesByToken map[string]domainInstance.Instance
	db               *sql.DB
}

// ensureInstanceWebhookColumns agrega las columnas de webhook si no existen (migración ligera para bases antiguas).
func ensureInstanceWebhookColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(instances)`)
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

	if !columns["webhook_urls"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN webhook_urls TEXT`); err != nil {
			return err
		}
	}
	if !columns["webhook_secret"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN webhook_secret TEXT`); err != nil {
			return err
		}
	}
	if !columns["webhook_insecure_skip_verify"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN webhook_insecure_skip_verify INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_base_url"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_base_url TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_account_token"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_account_token TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_bot_token"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_bot_token TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_account_id"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_account_id TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_inbox_id"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_inbox_id TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_inbox_identifier"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_inbox_identifier TEXT`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_enabled"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_enabled INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}
	if !columns["chatwoot_credential_id"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN chatwoot_credential_id TEXT`); err != nil {
			return err
		}
	}
	if !columns["gemini_enabled"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["gemini_api_key"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_api_key TEXT`); err != nil {
			return err
		}
	}
	if !columns["gemini_model"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_model TEXT`); err != nil {
			return err
		}
	}
	if !columns["gemini_system_prompt"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_system_prompt TEXT`); err != nil {
			return err
		}
	}
	if !columns["gemini_memory_enabled"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_memory_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["gemini_knowledge_base"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_knowledge_base TEXT`); err != nil {
			return err
		}
	}
	if !columns["gemini_audio_enabled"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_audio_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["gemini_image_enabled"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_image_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if !columns["gemini_timezone"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN gemini_timezone TEXT`); err != nil {
			return err
		}
	}
	if !columns["bot_id"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN bot_id TEXT`); err != nil {
			return err
		}
	}
	if !columns["auto_reconnect"] {
		if _, err := db.Exec(`ALTER TABLE instances ADD COLUMN auto_reconnect INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}

	return nil
}

// UpdateWebhookConfig actualiza la configuración de webhooks para una instancia específica.
func (service *instanceService) UpdateWebhookConfig(_ context.Context, id string, urls []string, secret string, insecure bool) (domainInstance.Instance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: cannot be blank.")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	var (
		instToken string
		inst      domainInstance.Instance
	)
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmedID {
			instToken = token
			inst = candidate
			break
		}
	}

	if instToken == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
	}

	// Normalizar URLs: trim y descartar vacías
	cleanURLs := make([]string, 0, len(urls))
	for _, u := range urls {
		if v := strings.TrimSpace(u); v != "" {
			cleanURLs = append(cleanURLs, v)
		}
	}

	inst.WebhookURLs = cleanURLs
	inst.WebhookSecret = secret
	inst.WebhookInsecureSkipVerify = insecure

	service.instancesByToken[instToken] = inst
	service.persistInstance(inst)

	// Actualizar también la configuración de webhooks a nivel de WhatsApp para esta instancia.
	whatsapp.SetInstanceWebhookConfig(
		inst.ID,
		inst.WebhookURLs,
		inst.WebhookSecret,
		inst.WebhookInsecureSkipVerify,
	)

	return inst, nil
}

// UpdateChatwootConfig actualiza la configuración de Chatwoot para una instancia específica.
// Si credentialID no está vacío e existe una credencial kind='chatwoot',
// baseURL y accountToken se resolverán desde dicha credencial.
func (service *instanceService) UpdateChatwootConfig(ctx context.Context, id string, baseURL, accountID, inboxID, inboxIdentifier, accountToken, botToken, credentialID string, enabled bool) (domainInstance.Instance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: cannot be blank.")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	var (
		instToken string
		inst      domainInstance.Instance
	)
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmedID {
			instToken = token
			inst = candidate
			break
		}
	}

	if instToken == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
	}

	// Si se pasa credentialID e existe base en tabla credentials, la usamos para baseURL y accountToken.
	credTrim := strings.TrimSpace(credentialID)
	resolvedBaseURL := strings.TrimSpace(baseURL)
	resolvedAccountToken := strings.TrimSpace(accountToken)
	if credTrim != "" && service.db != nil {
		var credBaseURL, credAccountToken sql.NullString
		query := `SELECT chatwoot_base_url, chatwoot_account_token FROM credentials WHERE id = ? AND kind = 'chatwoot'`
		if err := service.db.QueryRowContext(ctx, query, credTrim).Scan(&credBaseURL, &credAccountToken); err != nil {
			if err != sql.ErrNoRows {
				logrus.WithError(err).WithField("instance_id", trimmedID).Error("[INSTANCE] failed to resolve Chatwoot credential")
			}
		} else {
			if credBaseURL.Valid && strings.TrimSpace(credBaseURL.String) != "" {
				resolvedBaseURL = strings.TrimSpace(credBaseURL.String)
			}
			if credAccountToken.Valid && strings.TrimSpace(credAccountToken.String) != "" {
				resolvedAccountToken = strings.TrimSpace(credAccountToken.String)
			}
		}
	}

	inst.ChatwootBaseURL = resolvedBaseURL
	inst.ChatwootAccountID = strings.TrimSpace(accountID)
	inst.ChatwootInboxID = strings.TrimSpace(inboxID)
	inst.ChatwootInboxIdentifier = strings.TrimSpace(inboxIdentifier)
	inst.ChatwootAccountToken = resolvedAccountToken
	inst.ChatwootBotToken = strings.TrimSpace(botToken)
	inst.ChatwootEnabled = enabled
	inst.ChatwootCredentialID = credTrim

	service.instancesByToken[instToken] = inst
	service.persistInstance(inst)

	return inst, nil
}

func NewInstanceService() domainInstance.IInstanceUsecase {
	svc := &instanceService{
		instancesByToken: make(map[string]domainInstance.Instance),
	}

	// Inicializar almacenamiento persistente en SQLite
	if db, err := initInstanceStorageDB(); err != nil {
		logrus.WithError(err).Error("[INSTANCE] failed to initialize instance storage, falling back to in-memory only")
	} else {
		svc.db = db
		if err := svc.loadFromDB(); err != nil {
			logrus.WithError(err).Error("[INSTANCE] failed to load instances from storage")
		}
	}

	return svc
}

func (service *instanceService) Create(_ context.Context, request domainInstance.CreateInstanceRequest) (domainInstance.Instance, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return domainInstance.Instance{}, errors.New("name is required")
	}

	id := uuid.NewString()
	token := uuid.NewString()

	instance := domainInstance.Instance{
		ID:                        id,
		Name:                      name,
		Token:                     token,
		Status:                    domainInstance.StatusCreated,
		WebhookURLs:               nil,
		WebhookSecret:             "",
		WebhookInsecureSkipVerify: false,
		ChatwootBaseURL:           "",
		ChatwootAccountToken:      "",
		ChatwootBotToken:          "",
		ChatwootAccountID:         "",
		ChatwootInboxID:           "",
		ChatwootInboxIdentifier:   "",
		ChatwootCredentialID:      "",
		GeminiEnabled:             false,
		GeminiAPIKey:              "",
		GeminiModel:               "",
		GeminiSystemPrompt:        "",
		AutoReconnect:             true,
	}

	service.mu.Lock()
	service.instancesByToken[token] = instance
	service.mu.Unlock()

	service.persistInstance(instance)

	return instance, nil
}

func (service *instanceService) Delete(ctx context.Context, id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return pkgError.ValidationError("id: cannot be blank.")
	}

	// First, resolve the instance and attempt a proper WhatsApp logout BEFORE deleting it.
	// If logout fails, we block deletion so the caller can explicitly logout first.
	service.mu.RLock()
	var inst domainInstance.Instance
	for _, candidate := range service.instancesByToken {
		if candidate.ID == trimmed {
			inst = candidate
			break
		}
	}
	service.mu.RUnlock()

	if strings.TrimSpace(inst.ID) == "" {
		return pkgError.ValidationError("id: instance not found.")
	}

	cli, _, err := whatsapp.GetOrInitInstanceClient(ctx, inst.ID, nil)
	if err != nil {
		return pkgError.InternalServerError(fmt.Sprintf("failed to init instance client: %v", err))
	}

	if cli != nil && cli.IsLoggedIn() {
		if !cli.IsConnected() {
			if err := cli.Connect(); err != nil {
				return pkgError.ValidationError("instance is not connected; please logout before deleting.")
			}
		}
		if err := cli.Logout(ctx); err != nil {
			return pkgError.ValidationError("failed to logout instance; please logout before deleting.")
		}
	}

	if err := whatsapp.CleanupInstanceSession(ctx, inst.ID, nil); err != nil {
		return pkgError.InternalServerError(fmt.Sprintf("failed to cleanup instance session: %v", err))
	}
	if err := infraChatStorage.CleanupInstanceRepository(inst.ID); err != nil {
		logrus.WithError(err).Error("[INSTANCE] failed to cleanup chatstorage for instance")
	}

	// Now safely remove instance from memory and storage
	service.mu.Lock()
	var tokenToDelete string
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmed {
			tokenToDelete = token
			break
		}
	}
	if tokenToDelete != "" {
		delete(service.instancesByToken, tokenToDelete)
	}
	service.mu.Unlock()

	if service.db != nil {
		if _, err := service.db.Exec(`DELETE FROM instances WHERE id = ?`, trimmed); err != nil {
			logrus.WithError(err).Error("[INSTANCE] failed to delete instance from storage")
		}
	}

	return nil
}

func (service *instanceService) List(ctx context.Context) ([]domainInstance.Instance, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()

	_ = ctx
	result := make([]domainInstance.Instance, 0, len(service.instancesByToken))
	for _, instance := range service.instancesByToken {
		status := domainInstance.StatusOffline
		if strings.TrimSpace(instance.ID) != "" {
			connected, loggedIn := whatsapp.GetInstanceConnectionStatus(instance.ID)
			if connected && loggedIn {
				status = domainInstance.StatusOnline
			}
		}
		instance.Status = status
		result = append(result, instance)
	}

	return result, nil
}

func (service *instanceService) GetByID(_ context.Context, id string) (domainInstance.Instance, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()

	for _, inst := range service.instancesByToken {
		if inst.ID == id {
			return inst, nil
		}
	}
	return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
}

func (service *instanceService) GetByToken(_ context.Context, token string) (domainInstance.Instance, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("token: cannot be blank.")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()

	instance, ok := service.instancesByToken[trimmed]
	if !ok {
		return domainInstance.Instance{}, pkgError.ValidationError("token: invalid or not found.")
	}

	return instance, nil
}

// initInstanceStorageDB abre (o crea) la base de datos SQLite para instancias
// bajo storages/instances.db y asegura que el esquema exista.
func initInstanceStorageDB() (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	createTable := `
		CREATE TABLE IF NOT EXISTS instances (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			webhook_urls TEXT,
			webhook_secret TEXT,
			webhook_insecure_skip_verify INTEGER NOT NULL DEFAULT 0,
			chatwoot_base_url TEXT,
			chatwoot_account_token TEXT,
			chatwoot_bot_token TEXT,
			chatwoot_account_id TEXT,
			chatwoot_inbox_id TEXT,
			chatwoot_inbox_identifier TEXT,
			chatwoot_enabled INTEGER NOT NULL DEFAULT 1,
			chatwoot_credential_id TEXT,
			bot_id TEXT,
			gemini_enabled INTEGER NOT NULL DEFAULT 0,
			gemini_api_key TEXT,
			gemini_model TEXT,
			gemini_system_prompt TEXT,
			gemini_knowledge_base TEXT,
			gemini_timezone TEXT,
			gemini_audio_enabled INTEGER NOT NULL DEFAULT 0,
			gemini_image_enabled INTEGER NOT NULL DEFAULT 0,
			gemini_memory_enabled INTEGER NOT NULL DEFAULT 0,
			auto_reconnect INTEGER NOT NULL DEFAULT 1
		);
	`
	if _, err := db.Exec(createTable); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Asegurar columnas nuevas en bases existentes
	if err := ensureInstanceWebhookColumns(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// loadFromDB carga todas las instancias persistidas a la caché en memoria.
func (service *instanceService) loadFromDB() error {
	if service.db == nil {
		return nil
	}

	rows, err := service.db.Query(`SELECT id, name, token, status, webhook_urls, webhook_secret, webhook_insecure_skip_verify, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token, chatwoot_account_id, chatwoot_inbox_id, chatwoot_inbox_identifier, chatwoot_enabled, gemini_enabled, gemini_api_key, gemini_model, gemini_system_prompt, gemini_knowledge_base, gemini_timezone, gemini_audio_enabled, gemini_image_enabled, gemini_memory_enabled, bot_id, chatwoot_credential_id, auto_reconnect FROM instances`)
	if err != nil {
		return err
	}
	defer rows.Close()

	service.mu.Lock()
	defer service.mu.Unlock()

	for rows.Next() {
		var inst domainInstance.Instance
		var statusStr string
		var urlsStr, secretStr sql.NullString
		var insecureVal sql.NullInt64
		var baseURLStr, accountTokenStr, botTokenStr sql.NullString
		var accountIDStr, inboxIDStr, inboxIdentifierStr sql.NullString
		var chatwootEnabledVal, geminiEnabledVal, geminiAudioEnabledVal, geminiImageEnabledVal, geminiMemoryEnabledVal sql.NullInt64
		var geminiAPIKeyStr, geminiModelStr, geminiSystemPromptStr, geminiKnowledgeBaseStr, geminiTimezoneStr sql.NullString
		var botIDStr, chatwootCredIDStr sql.NullString
		var autoReconnectVal sql.NullInt64
		if err := rows.Scan(&inst.ID, &inst.Name, &inst.Token, &statusStr, &urlsStr, &secretStr, &insecureVal, &baseURLStr, &accountTokenStr, &botTokenStr, &accountIDStr, &inboxIDStr, &inboxIdentifierStr, &chatwootEnabledVal, &geminiEnabledVal, &geminiAPIKeyStr, &geminiModelStr, &geminiSystemPromptStr, &geminiKnowledgeBaseStr, &geminiTimezoneStr, &geminiAudioEnabledVal, &geminiImageEnabledVal, &geminiMemoryEnabledVal, &botIDStr, &chatwootCredIDStr, &autoReconnectVal); err != nil {
			return err
		}
		inst.Status = domainInstance.Status(statusStr)

		// URLs de webhook
		if urlsStr.Valid && strings.TrimSpace(urlsStr.String) != "" {
			parts := strings.Split(urlsStr.String, ",")
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					inst.WebhookURLs = append(inst.WebhookURLs, trimmed)
				}
			}
		}

		// Secret
		if secretStr.Valid && strings.TrimSpace(secretStr.String) != "" {
			inst.WebhookSecret = secretStr.String
		} else {
			inst.WebhookSecret = ""
		}

		// Insecure flag
		if insecureVal.Valid {
			inst.WebhookInsecureSkipVerify = insecureVal.Int64 != 0
		} else {
			inst.WebhookInsecureSkipVerify = false
		}

		// Chatwoot config
		if baseURLStr.Valid && strings.TrimSpace(baseURLStr.String) != "" {
			inst.ChatwootBaseURL = strings.TrimSpace(baseURLStr.String)
		} else {
			inst.ChatwootBaseURL = ""
		}
		if accountTokenStr.Valid && strings.TrimSpace(accountTokenStr.String) != "" {
			inst.ChatwootAccountToken = strings.TrimSpace(accountTokenStr.String)
		} else {
			inst.ChatwootAccountToken = ""
		}
		if botTokenStr.Valid && strings.TrimSpace(botTokenStr.String) != "" {
			inst.ChatwootBotToken = strings.TrimSpace(botTokenStr.String)
		} else {
			inst.ChatwootBotToken = ""
		}
		if accountIDStr.Valid && strings.TrimSpace(accountIDStr.String) != "" {
			inst.ChatwootAccountID = strings.TrimSpace(accountIDStr.String)
		} else {
			inst.ChatwootAccountID = ""
		}
		if inboxIDStr.Valid && strings.TrimSpace(inboxIDStr.String) != "" {
			inst.ChatwootInboxID = strings.TrimSpace(inboxIDStr.String)
		} else {
			inst.ChatwootInboxID = ""
		}
		if inboxIdentifierStr.Valid && strings.TrimSpace(inboxIdentifierStr.String) != "" {
			inst.ChatwootInboxIdentifier = strings.TrimSpace(inboxIdentifierStr.String)
		} else {
			inst.ChatwootInboxIdentifier = ""
		}
		if chatwootEnabledVal.Valid {
			inst.ChatwootEnabled = chatwootEnabledVal.Int64 != 0
		} else {
			inst.ChatwootEnabled = true
		}
		if geminiEnabledVal.Valid {
			inst.GeminiEnabled = geminiEnabledVal.Int64 != 0
		} else {
			inst.GeminiEnabled = false
		}
		if geminiAPIKeyStr.Valid && strings.TrimSpace(geminiAPIKeyStr.String) != "" {
			inst.GeminiAPIKey = strings.TrimSpace(geminiAPIKeyStr.String)
		} else {
			inst.GeminiAPIKey = ""
		}
		if geminiModelStr.Valid && strings.TrimSpace(geminiModelStr.String) != "" {
			inst.GeminiModel = strings.TrimSpace(geminiModelStr.String)
		} else {
			inst.GeminiModel = ""
		}
		if geminiSystemPromptStr.Valid && strings.TrimSpace(geminiSystemPromptStr.String) != "" {
			inst.GeminiSystemPrompt = strings.TrimSpace(geminiSystemPromptStr.String)
		} else {
			inst.GeminiSystemPrompt = ""
		}
		if geminiKnowledgeBaseStr.Valid && strings.TrimSpace(geminiKnowledgeBaseStr.String) != "" {
			inst.GeminiKnowledgeBase = strings.TrimSpace(geminiKnowledgeBaseStr.String)
		} else {
			inst.GeminiKnowledgeBase = ""
		}
		if geminiTimezoneStr.Valid && strings.TrimSpace(geminiTimezoneStr.String) != "" {
			inst.GeminiTimezone = strings.TrimSpace(geminiTimezoneStr.String)
		} else {
			inst.GeminiTimezone = ""
		}
		if geminiAudioEnabledVal.Valid {
			inst.GeminiAudioEnabled = geminiAudioEnabledVal.Int64 != 0
		} else {
			inst.GeminiAudioEnabled = false
		}
		if geminiImageEnabledVal.Valid {
			inst.GeminiImageEnabled = geminiImageEnabledVal.Int64 != 0
		} else {
			inst.GeminiImageEnabled = false
		}
		if geminiMemoryEnabledVal.Valid {
			inst.GeminiMemoryEnabled = geminiMemoryEnabledVal.Int64 != 0
		} else {
			inst.GeminiMemoryEnabled = false
		}
		if botIDStr.Valid && strings.TrimSpace(botIDStr.String) != "" {
			inst.BotID = strings.TrimSpace(botIDStr.String)
		} else {
			inst.BotID = ""
		}
		if chatwootCredIDStr.Valid && strings.TrimSpace(chatwootCredIDStr.String) != "" {
			inst.ChatwootCredentialID = strings.TrimSpace(chatwootCredIDStr.String)
		} else {
			inst.ChatwootCredentialID = ""
		}
		if autoReconnectVal.Valid {
			inst.AutoReconnect = autoReconnectVal.Int64 != 0
		} else {
			inst.AutoReconnect = true
		}

		service.instancesByToken[inst.Token] = inst

		// Registrar configuración de webhooks de esta instancia en WhatsApp al arrancar.
		whatsapp.SetInstanceWebhookConfig(
			inst.ID,
			inst.WebhookURLs,
			inst.WebhookSecret,
			inst.WebhookInsecureSkipVerify,
		)
	}

	return rows.Err()
}

// persistInstance guarda/actualiza una instancia en la base de datos.
func (service *instanceService) persistInstance(inst domainInstance.Instance) {
	if service.db == nil {
		return
	}

	urlsStr := ""
	if len(inst.WebhookURLs) > 0 {
		urlsStr = strings.Join(inst.WebhookURLs, ",")
	}
	insecureInt := 0
	if inst.WebhookInsecureSkipVerify {
		insecureInt = 1
	}
	geminiEnabledInt := 0
	if inst.GeminiEnabled {
		geminiEnabledInt = 1
	}
	geminiAudioEnabledInt := 0
	if inst.GeminiAudioEnabled {
		geminiAudioEnabledInt = 1
	}
	geminiImageEnabledInt := 0
	if inst.GeminiImageEnabled {
		geminiImageEnabledInt = 1
	}
	geminiMemoryEnabledInt := 0
	if inst.GeminiMemoryEnabled {
		geminiMemoryEnabledInt = 1
	}
	chatwootEnabledInt := 0
	if inst.ChatwootEnabled {
		chatwootEnabledInt = 1
	}
	autoReconnectInt := 0
	if inst.AutoReconnect {
		autoReconnectInt = 1
	}

	query := `
		INSERT INTO instances (id, name, token, status, webhook_urls, webhook_secret, webhook_insecure_skip_verify, chatwoot_base_url, chatwoot_account_token, chatwoot_bot_token, chatwoot_account_id, chatwoot_inbox_id, chatwoot_inbox_identifier, chatwoot_enabled, gemini_enabled, gemini_api_key, gemini_model, gemini_system_prompt, gemini_knowledge_base, gemini_timezone, gemini_audio_enabled, gemini_image_enabled, gemini_memory_enabled, bot_id, chatwoot_credential_id, auto_reconnect)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			token = excluded.token,
			status = excluded.status,
			webhook_urls = excluded.webhook_urls,
			webhook_secret = excluded.webhook_secret,
			webhook_insecure_skip_verify = excluded.webhook_insecure_skip_verify,
			chatwoot_base_url = excluded.chatwoot_base_url,
			chatwoot_account_token = excluded.chatwoot_account_token,
			chatwoot_bot_token = excluded.chatwoot_bot_token,
			chatwoot_account_id = excluded.chatwoot_account_id,
			chatwoot_inbox_id = excluded.chatwoot_inbox_id,
			chatwoot_inbox_identifier = excluded.chatwoot_inbox_identifier,
			chatwoot_enabled = excluded.chatwoot_enabled,
			gemini_enabled = excluded.gemini_enabled,
			gemini_api_key = excluded.gemini_api_key,
			gemini_model = excluded.gemini_model,
			gemini_system_prompt = excluded.gemini_system_prompt,
			gemini_knowledge_base = excluded.gemini_knowledge_base,
			gemini_timezone = excluded.gemini_timezone,
			gemini_audio_enabled = excluded.gemini_audio_enabled,
			gemini_image_enabled = excluded.gemini_image_enabled,
			gemini_memory_enabled = excluded.gemini_memory_enabled,
			bot_id = excluded.bot_id,
			chatwoot_credential_id = excluded.chatwoot_credential_id,
			auto_reconnect = excluded.auto_reconnect,
			updated_at = CURRENT_TIMESTAMP;
	`

	if _, err := service.db.Exec(query, inst.ID, inst.Name, inst.Token, string(inst.Status), urlsStr, inst.WebhookSecret, insecureInt, inst.ChatwootBaseURL, inst.ChatwootAccountToken, inst.ChatwootBotToken, inst.ChatwootAccountID, inst.ChatwootInboxID, inst.ChatwootInboxIdentifier, chatwootEnabledInt, geminiEnabledInt, inst.GeminiAPIKey, inst.GeminiModel, inst.GeminiSystemPrompt, inst.GeminiKnowledgeBase, inst.GeminiTimezone, geminiAudioEnabledInt, geminiImageEnabledInt, geminiMemoryEnabledInt, strings.TrimSpace(inst.BotID), strings.TrimSpace(inst.ChatwootCredentialID), autoReconnectInt); err != nil {
		logrus.WithError(err).Error("[INSTANCE] failed to persist instance")
	}
}

func (service *instanceService) UpdateBotConfig(_ context.Context, id string, botID string) (domainInstance.Instance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: cannot be blank.")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	var (
		instToken string
		inst      domainInstance.Instance
	)
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmedID {
			instToken = token
			inst = candidate
			break
		}
	}

	if instToken == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
	}

	inst.BotID = strings.TrimSpace(botID)
	service.instancesByToken[instToken] = inst
	service.persistInstance(inst)

	return inst, nil
}

func (service *instanceService) UpdateGeminiConfig(_ context.Context, id string, enabled bool, apiKey, model, systemPrompt, knowledgeBase, timezone string, audioEnabled, imageEnabled, memoryEnabled bool) (domainInstance.Instance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: cannot be blank.")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	var (
		instToken string
		inst      domainInstance.Instance
	)
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmedID {
			instToken = token
			inst = candidate
			break
		}
	}

	if instToken == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
	}

	inst.GeminiEnabled = enabled
	inst.GeminiAPIKey = strings.TrimSpace(apiKey)
	inst.GeminiModel = strings.TrimSpace(model)
	inst.GeminiSystemPrompt = strings.TrimSpace(systemPrompt)
	inst.GeminiKnowledgeBase = strings.TrimSpace(knowledgeBase)
	inst.GeminiTimezone = strings.TrimSpace(timezone)
	inst.GeminiAudioEnabled = audioEnabled
	inst.GeminiImageEnabled = imageEnabled
	inst.GeminiMemoryEnabled = memoryEnabled

	service.instancesByToken[instToken] = inst
	service.persistInstance(inst)

	return inst, nil
}

func (service *instanceService) UpdateAutoReconnectConfig(_ context.Context, id string, enabled bool) (domainInstance.Instance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: cannot be blank.")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	var (
		instToken string
		inst      domainInstance.Instance
	)
	for token, candidate := range service.instancesByToken {
		if candidate.ID == trimmedID {
			instToken = token
			inst = candidate
			break
		}
	}

	if instToken == "" {
		return domainInstance.Instance{}, pkgError.ValidationError("id: instance not found.")
	}

	inst.AutoReconnect = enabled
	service.instancesByToken[instToken] = inst
	service.persistInstance(inst)

	return inst, nil
}

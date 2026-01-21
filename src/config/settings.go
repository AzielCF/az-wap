package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	AppVersion             = "v2.0.0-beta"
	AppPort                = "3000"
	AppDebug               = false
	AppOs                  = "AzielCf"
	AppPlatform            = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasicAuthCredential []string
	AppBasePath            = ""
	AppTrustedProxies      []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)
	AppBaseUrl             = "http://localhost:3000"

	McpPort = "8080"
	McpHost = "localhost"

	PathStatics   = "statics"
	PathQrCode    = "statics/qrcode"
	PathSendItems = "statics/senditems"
	PathMedia     = "statics/media"
	PathStorages  = "storages"

	DBURI     = "file:storages/whatsapp.db?_foreign_keys=on"
	DBKeysURI = ""

	WhatsappAutoReplyMessage          string
	WhatsappAutoMarkRead              = false // Auto-mark incoming messages as read
	WhatsappAutoDownloadMedia         = true  // Auto-download media from incoming messages
	WhatsappWebhook                   []string
	WhatsappWebhookSecret                   = "secret"
	WhatsappWebhookInsecureSkipVerify       = false // Skip TLS certificate verification for webhooks (insecure)
	WhatsappLogLevel                        = "ERROR"
	WhatsappSettingMaxImageSize       int64 = 20000000 // 20MB
	WhatsappSettingMaxFileSize        int64 = 50000000 // 50MB
	WhatsappSettingMaxVideoSize       int64 = 50000000 // 50MB
	WhatsappSettingMaxDownloadSize    int64 = 50000000 // 50MB
	WhatsappTypeUser                        = "@s.whatsapp.net"
	WhatsappTypeGroup                       = "@g.us"
	WhatsappAccountValidation               = true

	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

	AIGlobalSystemPrompt string
	AITimezone           string
	AIDebounceMs         int   = 3500
	AIWaitContactIdleMs  int   = 10000
	AITypingEnabled      bool  = true
	AIMaxAudioBytes      int64 = 4 * 1024 * 1024
	AIMaxImageBytes      int64 = 4 * 1024 * 1024

	// Message Worker Pool settings
	MessageWorkerPoolSize  int = 20
	MessageWorkerQueueSize int = 1000

	// Security
	AppSecretKey string = "changeme_please_change_me_in_prod_12345"
)

func init() {
	if v := strings.TrimSpace(os.Getenv("AI_GLOBAL_SYSTEM_PROMPT")); v != "" {
		AIGlobalSystemPrompt = v
	}
	loadAIGlobalSystemPromptFromDB()
	if v := strings.TrimSpace(os.Getenv("AI_TIMEZONE")); v != "" {
		AITimezone = v
	}
	loadAITimezoneFromDB()
	if v := strings.TrimSpace(os.Getenv("AI_DEBOUNCE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			AIDebounceMs = n
		}
	}
	loadAIDebounceFromDB()
	if v := strings.TrimSpace(os.Getenv("AI_WAIT_CONTACT_IDLE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			AIWaitContactIdleMs = n
		}
	}
	loadAIWaitContactIdleFromDB()
	if v := strings.TrimSpace(os.Getenv("AI_TYPING_ENABLED")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			AITypingEnabled = true
		case "0", "false", "no", "n", "off":
			AITypingEnabled = false
		}
	}
	loadAITypingEnabledFromDB()
	loadWhatsappMaxDownloadSizeFromDB()

	if v := strings.TrimSpace(os.Getenv("AI_MAX_AUDIO_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			AIMaxAudioBytes = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("AI_MAX_IMAGE_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			AIMaxImageBytes = n
		}
	}
	// Message Worker Pool env vars (support both old BOT_* and new MESSAGE_* names)
	if val := os.Getenv("MESSAGE_WORKER_POOL_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			MessageWorkerPoolSize = parsed
		}
	} else if val := os.Getenv("BOT_WORKER_POOL_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			MessageWorkerPoolSize = parsed
		}
	}

	if val := os.Getenv("MESSAGE_WORKER_QUEUE_SIZE"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			MessageWorkerQueueSize = parsed
		}
	}

	if val := os.Getenv("APP_SECRET_KEY"); val != "" {
		AppSecretKey = val
	}
}

var (
	appDB     *sql.DB
	appDBErr  error
	appDBOnce sync.Once
)

func GetAppDB() (*sql.DB, error) {
	appDBOnce.Do(func() {
		connStr := fmt.Sprintf("file:%s/app.db?_journal_mode=WAL&_foreign_keys=on", PathStorages)
		db, err := sql.Open("sqlite3", connStr)
		if err != nil {
			appDBErr = err
			return
		}
		// Configure connection pool for better concurrency in WAL mode
		db.SetMaxOpenConns(50)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(time.Hour)
		appDB = db
	})

	return appDB, appDBErr
}

func SetAIGlobalSystemPrompt(v string) {
	AIGlobalSystemPrompt = strings.TrimSpace(v)
}

func SaveAIGlobalSystemPrompt(v string) error {
	SetAIGlobalSystemPrompt(v)
	return saveAIGlobalSystemPromptToDB()
}

func SetAITimezone(v string) {
	AITimezone = strings.TrimSpace(v)
}

func SaveAITimezone(v string) error {
	SetAITimezone(v)
	return saveAITimezoneToDB()
}

func SetAIDebounceMs(v int) {
	if v < 0 {
		v = 0
	}
	AIDebounceMs = v
}

func SaveAIDebounceMs(v int) error {
	SetAIDebounceMs(v)
	return saveAIDebounceToDB()
}

func SetAIWaitContactIdleMs(v int) {
	if v < 0 {
		v = 0
	}
	AIWaitContactIdleMs = v
}

func SaveAIWaitContactIdleMs(v int) error {
	SetAIWaitContactIdleMs(v)
	return saveAIWaitContactIdleToDB()
}

func SetAITypingEnabled(v bool) {
	AITypingEnabled = v
}

func SaveAITypingEnabled(v bool) error {
	SetAITypingEnabled(v)
	return saveAITypingEnabledToDB()
}

func loadAIGlobalSystemPromptFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'ai_global_system_prompt'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		AIGlobalSystemPrompt = strings.TrimSpace(v.String)
	}
}

func saveAIGlobalSystemPromptToDB() error {
	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('ai_global_system_prompt', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, AIGlobalSystemPrompt)
	return err
}

func loadAITimezoneFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'ai_timezone'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		AITimezone = strings.TrimSpace(v.String)
	}
}

func saveAITimezoneToDB() error {
	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('ai_timezone', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, AITimezone)
	return err
}

func loadAIDebounceFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'ai_debounce_ms'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		if n, err := strconv.Atoi(strings.TrimSpace(v.String)); err == nil && n >= 0 {
			AIDebounceMs = n
		}
	}
}

func saveAIDebounceToDB() error {
	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('ai_debounce_ms', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", AIDebounceMs))
	return err
}

func loadAIWaitContactIdleFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'ai_wait_contact_idle_ms'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		if n, err := strconv.Atoi(strings.TrimSpace(v.String)); err == nil && n >= 0 {
			AIWaitContactIdleMs = n
		}
	}
}

func saveAIWaitContactIdleToDB() error {
	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('ai_wait_contact_idle_ms', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", AIWaitContactIdleMs))
	return err
}

func loadAITypingEnabledFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'ai_typing_enabled'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		switch strings.ToLower(strings.TrimSpace(v.String)) {
		case "1", "true", "yes", "y", "on":
			AITypingEnabled = true
		case "0", "false", "no", "n", "off":
			AITypingEnabled = false
		}
	}
}

func saveAITypingEnabledToDB() error {
	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	val := "0"
	if AITypingEnabled {
		val = "1"
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('ai_typing_enabled', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, val)
	return err
}

func loadWhatsappMaxDownloadSizeFromDB() {
	db, err := GetAppDB()
	if err != nil {
		return
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'whatsapp_max_download_size'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		if n, err := strconv.ParseInt(strings.TrimSpace(v.String), 10, 64); err == nil && n >= 0 {
			WhatsappSettingMaxDownloadSize = n
		}
	}
}

func SaveWhatsappMaxDownloadSize(v int64) error {
	if v < 0 {
		v = 0
	}
	WhatsappSettingMaxDownloadSize = v

	db, err := GetAppDB()
	if err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('whatsapp_max_download_size', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", v))
	return err
}

func GetAllSettings() map[string]any {
	return map[string]any{
		"ai_global_system_prompt":    AIGlobalSystemPrompt,
		"ai_timezone":                AITimezone,
		"ai_debounce_ms":             AIDebounceMs,
		"ai_wait_contact_idle_ms":    AIWaitContactIdleMs,
		"ai_typing_enabled":          AITypingEnabled,
		"whatsapp_max_download_size": WhatsappSettingMaxDownloadSize,
	}
}

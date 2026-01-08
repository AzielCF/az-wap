package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

var (
	AppVersion             = "v2.0.0"
	AppPort                = "3000"
	AppDebug               = false
	AppOs                  = "AzielCf"
	AppPlatform            = waCompanionReg.DeviceProps_PlatformType(1)
	AppBasicAuthCredential []string
	AppBasePath            = ""
	AppTrustedProxies      []string // Trusted proxy IP ranges (e.g., "0.0.0.0/0" for all, or specific CIDRs)

	McpPort = "8080"
	McpHost = "localhost"

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
	WhatsappSettingMaxImageSize       int64 = 20000000  // 20MB
	WhatsappSettingMaxFileSize        int64 = 50000000  // 50MB
	WhatsappSettingMaxVideoSize       int64 = 100000000 // 100MB
	WhatsappSettingMaxDownloadSize    int64 = 500000000 // 500MB
	WhatsappTypeUser                        = "@s.whatsapp.net"
	WhatsappTypeGroup                       = "@g.us"
	WhatsappAccountValidation               = true

	ChatStorageURI               = "file:storages/chatstorage.db"
	ChatStorageEnableForeignKeys = true
	ChatStorageEnableWAL         = true

	GeminiGlobalSystemPrompt string
	GeminiTimezone           string
	GeminiDebounceMs         int   = 3500
	GeminiWaitContactIdleMs  int   = 10000
	GeminiTypingEnabled      bool  = true
	GeminiMaxAudioBytes      int64 = 4 * 1024 * 1024
	GeminiMaxImageBytes      int64 = 4 * 1024 * 1024

	// Message Worker Pool settings
	MessageWorkerPoolSize  int = 20
	MessageWorkerQueueSize int = 1000

	// Security
	AppSecretKey string = "changeme_please_change_me_in_prod_12345"
)

func init() {
	if v := strings.TrimSpace(os.Getenv("GEMINI_GLOBAL_SYSTEM_PROMPT")); v != "" {
		GeminiGlobalSystemPrompt = v
	}
	loadGeminiGlobalSystemPromptFromDB()
	if v := strings.TrimSpace(os.Getenv("GEMINI_TIMEZONE")); v != "" {
		GeminiTimezone = v
	}
	loadGeminiTimezoneFromDB()
	if v := strings.TrimSpace(os.Getenv("GEMINI_DEBOUNCE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			GeminiDebounceMs = n
		}
	}
	loadGeminiDebounceFromDB()
	if v := strings.TrimSpace(os.Getenv("GEMINI_WAIT_CONTACT_IDLE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			GeminiWaitContactIdleMs = n
		}
	}
	loadGeminiWaitContactIdleFromDB()
	if v := strings.TrimSpace(os.Getenv("GEMINI_TYPING_ENABLED")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			GeminiTypingEnabled = true
		case "0", "false", "no", "n", "off":
			GeminiTypingEnabled = false
		}
	}
	loadGeminiTypingEnabledFromDB()
	if v := strings.TrimSpace(os.Getenv("GEMINI_MAX_AUDIO_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			GeminiMaxAudioBytes = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("GEMINI_MAX_IMAGE_BYTES")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			GeminiMaxImageBytes = n
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
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			MessageWorkerQueueSize = parsed
		}
	}

	if val := os.Getenv("APP_SECRET_KEY"); val != "" {
		AppSecretKey = val
	}
}

func SetGeminiGlobalSystemPrompt(v string) {
	GeminiGlobalSystemPrompt = strings.TrimSpace(v)
}

func SaveGeminiGlobalSystemPrompt(v string) error {
	SetGeminiGlobalSystemPrompt(v)
	return saveGeminiGlobalSystemPromptToDB()
}

func SetGeminiTimezone(v string) {
	GeminiTimezone = strings.TrimSpace(v)
}

func SaveGeminiTimezone(v string) error {
	SetGeminiTimezone(v)
	return saveGeminiTimezoneToDB()
}

func SetGeminiDebounceMs(v int) {
	if v < 0 {
		v = 0
	}
	GeminiDebounceMs = v
}

func SaveGeminiDebounceMs(v int) error {
	SetGeminiDebounceMs(v)
	return saveGeminiDebounceToDB()
}

func SetGeminiWaitContactIdleMs(v int) {
	if v < 0 {
		v = 0
	}
	GeminiWaitContactIdleMs = v
}

func SaveGeminiWaitContactIdleMs(v int) error {
	SetGeminiWaitContactIdleMs(v)
	return saveGeminiWaitContactIdleToDB()
}

func SetGeminiTypingEnabled(v bool) {
	GeminiTypingEnabled = v
}

func SaveGeminiTypingEnabled(v bool) error {
	SetGeminiTypingEnabled(v)
	return saveGeminiTypingEnabledToDB()
}

func loadGeminiGlobalSystemPromptFromDB() {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'gemini_global_system_prompt'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		GeminiGlobalSystemPrompt = strings.TrimSpace(v.String)
	}
}

func saveGeminiGlobalSystemPromptToDB() error {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('gemini_global_system_prompt', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, GeminiGlobalSystemPrompt)
	return err
}

func loadGeminiTimezoneFromDB() {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'gemini_timezone'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		GeminiTimezone = strings.TrimSpace(v.String)
	}
}

func saveGeminiTimezoneToDB() error {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('gemini_timezone', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, GeminiTimezone)
	return err
}

func loadGeminiDebounceFromDB() {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'gemini_debounce_ms'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		if n, err := strconv.Atoi(strings.TrimSpace(v.String)); err == nil && n >= 0 {
			GeminiDebounceMs = n
		}
	}
}

func saveGeminiDebounceToDB() error {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('gemini_debounce_ms', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", GeminiDebounceMs))
	return err
}

func loadGeminiWaitContactIdleFromDB() {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'gemini_wait_contact_idle_ms'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		if n, err := strconv.Atoi(strings.TrimSpace(v.String)); err == nil && n >= 0 {
			GeminiWaitContactIdleMs = n
		}
	}
}

func saveGeminiWaitContactIdleToDB() error {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('gemini_wait_contact_idle_ms', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, fmt.Sprintf("%d", GeminiWaitContactIdleMs))
	return err
}

func loadGeminiTypingEnabledFromDB() {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return
	}
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM global_settings WHERE key = 'gemini_typing_enabled'`).Scan(&v); err != nil {
		return
	}
	if v.Valid {
		switch strings.ToLower(strings.TrimSpace(v.String)) {
		case "1", "true", "yes", "y", "on":
			GeminiTypingEnabled = true
		case "0", "false", "no", "n", "off":
			GeminiTypingEnabled = false
		}
	}
}

func saveGeminiTypingEnabledToDB() error {
	dbPath := fmt.Sprintf("%s/instances.db", PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return err
	}
	val := "0"
	if GeminiTypingEnabled {
		val = "1"
	}
	_, err = db.Exec(`INSERT INTO global_settings (key, value) VALUES ('gemini_typing_enabled', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, val)
	return err
}

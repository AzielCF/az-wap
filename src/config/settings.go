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
	AppVersion             = "v1.0.5"
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
	GeminiMaxAudioBytes      int64 = 4 * 1024 * 1024
	GeminiMaxImageBytes      int64 = 4 * 1024 * 1024
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

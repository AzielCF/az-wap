package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
)

// Config holds all application configuration in a structured way.
type Config struct {
	App        AppConfig
	MCP        MCPConfig
	Paths      PathsConfig
	Database   DatabaseConfig
	Whatsapp   WhatsappConfig
	AI         AIConfig
	WorkerPool WorkerPoolConfig
	Security   SecurityConfig
	APIKeys    APIKeysConfig
}

type AppConfig struct {
	Version            string
	Port               string
	Debug              bool
	Environment        string
	OS                 string
	Platform           waCompanionReg.DeviceProps_PlatformType
	BasicAuth          []string
	BasePath           string
	TrustedProxies     []string
	BaseUrl            string
	CorsAllowedOrigins []string
	ServerID           string
}

type MCPConfig struct {
	Port string
	Host string
}

type PathsConfig struct {
	BaseDir   string
	Statics   string
	SendItems string
	Storages  string
}

type DatabaseConfig struct {
	Driver          string
	Host            string
	Port            int
	User            string
	Password        string
	Name            string // File path for SQLite, DB Name for Postgres
	ValkeyEnabled   bool
	ValkeyAddress   string
	ValkeyPassword  string
	ValkeyDB        int
	ValkeyKeyPrefix string
	// Legacy URIs support
	URI     string
	KeysURI string
}

type WhatsappConfig struct {
	LogLevel          string
	MaxImageSize      int64
	MaxFileSize       int64
	MaxVideoSize      int64
	MaxDownloadSize   int64
	TypeUser          string
	TypeGroup         string
	AccountValidation bool
}

type AIConfig struct {
	GlobalSystemPrompt string
	Timezone           string
	DebounceMs         int
	WaitContactIdleMs  int
	TypingEnabled      bool
	MaxAudioBytes      int64
	MaxImageBytes      int64
}

type WorkerPoolConfig struct {
	Size      int
	QueueSize int
}

type SecurityConfig struct {
	SecretKey string
}

type APIKeysConfig struct {
	Gemini string
	OpenAI string
	Claude string
	AI     string // Generic/Fallback
}

// Global provides access to the loaded configuration globally (Migration Helper)
var Global *Config

// LoadConfig loads configuration from Environment Variables or defaults.
func LoadConfig() (*Config, error) {
	// Base Directory Setup
	baseDir := getEnv("APP_BASE_DIR", "storages") // Default legacy path
	if baseDir == "storages" && !fileExists("storages") {
		// If default relative path doesn't exist, maybe we are inside src?
		// Logic to handle paths can be improved here.
	}

	// App Defaults
	debug := false
	if v := os.Getenv("APP_DEBUG"); v == "true" || v == "1" || v == "on" {
		debug = true
	} else if v := os.Getenv("DEBUG"); v == "true" || v == "1" {
		debug = true
	}

	// Basic Auth
	var basicAuth []string
	if v := os.Getenv("APP_BASIC_AUTH"); v != "" {
		basicAuth = strings.Split(v, ",")
	}

	// Cors
	cors_origins := []string{"http://localhost:3000", "http://localhost:5173"}
	if v := os.Getenv("APP_CORS_ALLOWED_ORIGINS"); v != "" {
		cors_origins = strings.Split(v, ",")
	}

	appCfg := AppConfig{
		Version:            "v2.0.0-beta.16", // To be synced or injected
		Port:               getEnv("APP_PORT", "3000"),
		Debug:              debug,
		Environment:        getEnv("APP_ENV", "development"),
		OS:                 getEnv("APP_OS", "AzielCf"),
		Platform:           waCompanionReg.DeviceProps_PlatformType(1), // Chrome
		BasicAuth:          basicAuth,
		BasePath:           getEnv("APP_BASE_PATH", ""),
		BaseUrl:            getEnv("APP_BASE_URL", "http://localhost:3000"),
		CorsAllowedOrigins: cors_origins,
		ServerID:           getEnv("SERVER_ID", ""),
	}
	if v := os.Getenv("APP_TRUSTED_PROXIES"); v != "" {
		appCfg.TrustedProxies = strings.Split(v, ",")
	}

	// Paths
	pathsCfg := PathsConfig{
		BaseDir:   baseDir,
		Statics:   getEnv("PATH_STATICS", "statics"),
		SendItems: getEnv("PATH_SEND_ITEMS", filepath.Join("statics", "senditems")),
		Storages:  baseDir,
	}

	// Database
	dbDriver := getEnv("DB_DRIVER", "sqlite")
	dbCfg := DatabaseConfig{
		Driver:          dbDriver,
		Name:            filepath.Join(pathsCfg.Storages, "app.db"), // Default SQLite
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", ""),
		ValkeyEnabled:   getEnvBool("VALKEY_ENABLED", false),
		ValkeyAddress:   getEnv("VALKEY_ADDRESS", "localhost:6379"),
		ValkeyPassword:  getEnv("VALKEY_PASSWORD", ""),
		ValkeyDB:        getEnvInt("VALKEY_DB", 0),
		ValkeyKeyPrefix: getEnv("VALKEY_KEY_PREFIX", "azwap:"),
		URI:             getEnv("DB_URI", fmt.Sprintf("file:%s?_foreign_keys=on", filepath.Join(pathsCfg.Storages, "whatsapp.db"))),
		KeysURI:         getEnv("DB_KEYS_URI", ""),
	}

	// WhatsApp
	waCfg := WhatsappConfig{
		LogLevel:          getEnv("WHATSAPP_LOG_LEVEL", "ERROR"),
		MaxImageSize:      getEnvInt64("WHATSAPP_MAX_IMAGE_SIZE", 20000000),
		MaxFileSize:       getEnvInt64("WHATSAPP_MAX_FILE_SIZE", 50000000),
		MaxVideoSize:      getEnvInt64("WHATSAPP_MAX_VIDEO_SIZE", 50000000),
		MaxDownloadSize:   getEnvInt64("WHATSAPP_MAX_DOWNLOAD_SIZE", 50000000),
		TypeUser:          "@s.whatsapp.net",
		TypeGroup:         "@g.us",
		AccountValidation: getEnvBool("WHATSAPP_ACCOUNT_VALIDATION", true),
	}

	// AI
	aiCfg := AIConfig{
		GlobalSystemPrompt: getEnv("AI_GLOBAL_SYSTEM_PROMPT", ""),
		Timezone:           getEnv("AI_TIMEZONE", ""),
		DebounceMs:         getEnvInt("AI_DEBOUNCE_MS", 3500),
		WaitContactIdleMs:  getEnvInt("AI_WAIT_CONTACT_IDLE_MS", 10000),
		TypingEnabled:      getEnvBool("AI_TYPING_ENABLED", true),
		MaxAudioBytes:      getEnvInt64("AI_MAX_AUDIO_BYTES", 4*1024*1024),
		MaxImageBytes:      getEnvInt64("AI_MAX_IMAGE_BYTES", 4*1024*1024),
	}

	// Worker Pool & Security & API Keys
	// Support legacy BOT_* env vars
	poolSize := getEnvInt("MESSAGE_WORKER_POOL_SIZE", 20)
	if v := os.Getenv("BOT_WORKER_POOL_SIZE"); v != "" {
		poolSize = getEnvInt("BOT_WORKER_POOL_SIZE", 20)
	}

	cfg := &Config{
		App:        appCfg,
		MCP:        MCPConfig{Port: getEnv("MCP_PORT", "8080"), Host: getEnv("MCP_HOST", "localhost")},
		Paths:      pathsCfg,
		Database:   dbCfg,
		Whatsapp:   waCfg,
		AI:         aiCfg,
		WorkerPool: WorkerPoolConfig{Size: poolSize, QueueSize: getEnvInt("MESSAGE_WORKER_QUEUE_SIZE", 1000)},
		Security:   SecurityConfig{SecretKey: getEnv("APP_SECRET_KEY", "changeme_please_change_me_in_prod_12345")},
		APIKeys: APIKeysConfig{
			Gemini: getEnv("GEMINI_API_KEY", ""),
			OpenAI: getEnv("OPENAI_API_KEY", ""),
			Claude: getEnv("CLAUDE_API_KEY", ""),
			AI:     getEnv("AI_API_KEY", ""),
		},
	}

	Global = cfg
	return cfg, nil
}

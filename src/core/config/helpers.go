package config

import (
	"os"
	"strconv"
	"strings"
)

// GetAllSettings returns a map of all dynamic settings currently loaded in memory.
// This replaces the legacy config.GetAllSettings function.
func GetAllSettings() map[string]any {
	if Global == nil {
		return map[string]any{}
	}
	return map[string]any{
		"whatsapp_setting_max_file_size":     Global.Whatsapp.MaxFileSize,
		"whatsapp_setting_max_video_size":    Global.Whatsapp.MaxVideoSize,
		"whatsapp_setting_max_download_size": Global.Whatsapp.MaxDownloadSize,
		"whatsapp_account_validation":        Global.Whatsapp.AccountValidation,
		"ai_global_system_prompt":            Global.AI.GlobalSystemPrompt,
		"ai_timezone":                        Global.AI.Timezone,
		"ai_debounce_ms":                     Global.AI.DebounceMs,
		"ai_wait_contact_idle_ms":            Global.AI.WaitContactIdleMs,
		"ai_typing_enabled":                  Global.AI.TypingEnabled,
		"app_debug":                          Global.App.Debug,
		"app_version":                        Global.App.Version,
	}
}

// Helpers
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		vLower := strings.ToLower(v)
		return vLower == "1" || vLower == "true" || vLower == "yes" || vLower == "on"
	}
	return fallback
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

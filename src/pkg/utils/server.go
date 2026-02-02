package utils

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// GetPersistentServerID returns a stable ID for the current server.
// Logic:
// 1. Return provided override if not empty.
// 2. Try to read from storages/.server_id
// 3. Try OS Hostname.
// 4. Generate and save a new one as fallback.
func GetPersistentServerID(override, storagePath string) string {
	// 1. Override (e.g. from environment variable)
	if override != "" {
		return override
	}

	// 2. Try to read from file
	idFile := filepath.Join(storagePath, ".server_id")
	if data, err := os.ReadFile(idFile); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// 3. Try Hostname
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		// Cleanup hostname to be safe for keys
		cleanHost := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return -1
		}, hostname)
		if cleanHost != "" {
			return "azwap-" + cleanHost
		}
	}

	// 4. Generate random and save
	randomPart := make([]byte, 4)
	rand.Read(randomPart)
	newID := "azwap-" + hex.EncodeToString(randomPart)

	_ = os.MkdirAll(storagePath, 0755)
	_ = os.WriteFile(idFile, []byte(newID), 0644)

	return newID
}

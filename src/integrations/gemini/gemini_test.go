package gemini

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/AzielCF/az-wap/config"
)

func TestLoadBotConfig_BasicBotConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Usamos una base de datos puramente en memoria, aislada por conexión.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	create := `
		CREATE TABLE bots (
			id TEXT PRIMARY KEY,
			enabled INTEGER,
			api_key TEXT,
			model TEXT,
			system_prompt TEXT,
			knowledge_base TEXT,
			timezone TEXT,
			audio_enabled INTEGER,
			image_enabled INTEGER,
			memory_enabled INTEGER,
			credential_id TEXT
		);
	`
	if _, err := db.Exec(create); err != nil {
		t.Fatalf("failed to create bots table: %v", err)
	}

	insert := `INSERT INTO bots (id, enabled, api_key, model, system_prompt, knowledge_base, timezone, audio_enabled, image_enabled, memory_enabled, credential_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if _, err := db.Exec(insert,
		"bot-1", 1, "API_KEY_1", "", "sys", "kb", "America/Lima", 1, 0, 1, "",
	); err != nil {
		t.Fatalf("failed to insert bot row: %v", err)
	}

	cfg, err := loadBotConfig(ctx, db, "bot-1")
	if err != nil {
		t.Fatalf("loadBotConfig() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("loadBotConfig() returned nil config")
	}
	if !cfg.Enabled {
		t.Fatalf("expected Enabled=true")
	}
	if cfg.APIKey != "API_KEY_1" {
		t.Fatalf("expected APIKey=API_KEY_1, got %q", cfg.APIKey)
	}
	if cfg.Model == "" {
		t.Fatalf("expected Model to be defaulted, got empty")
	}
}

func TestLoadBotConfig_UsesCredentialAPIKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Usamos una base de datos puramente en memoria, aislada por conexión.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE bots (
			id TEXT PRIMARY KEY,
			enabled INTEGER,
			api_key TEXT,
			model TEXT,
			system_prompt TEXT,
			knowledge_base TEXT,
			timezone TEXT,
			audio_enabled INTEGER,
			image_enabled INTEGER,
			memory_enabled INTEGER,
			credential_id TEXT
		);
	`); err != nil {
		t.Fatalf("failed to create bots table: %v", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE credentials (
			id TEXT PRIMARY KEY,
			gemini_api_key TEXT
		);
	`); err != nil {
		t.Fatalf("failed to create credentials table: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO bots (id, enabled, api_key, model, system_prompt, knowledge_base, timezone, audio_enabled, image_enabled, memory_enabled, credential_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		"bot-cred", 1, "BOT_KEY", "model-x", "sys", "kb", "UTC", 0, 0, 0, "cred-1",
	); err != nil {
		t.Fatalf("failed to insert bot row: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO credentials (id, gemini_api_key) VALUES (?, ?);`, "cred-1", "KEY_FROM_CREDENTIAL"); err != nil {
		t.Fatalf("failed to insert credential row: %v", err)
	}

	cfg, err := loadBotConfig(ctx, db, "bot-cred")
	if err != nil {
		t.Fatalf("loadBotConfig() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("loadBotConfig() returned nil config")
	}
	if cfg.APIKey != "KEY_FROM_CREDENTIAL" {
		t.Fatalf("expected APIKey=KEY_FROM_CREDENTIAL, got %q", cfg.APIKey)
	}
}

func TestGenerateBotTextReply_BlankBotID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := GenerateBotTextReply(ctx, "  ", "mem-1", "hola")
	if err == nil {
		t.Fatalf("expected error for blank botID, got nil")
	}
	if !strings.Contains(err.Error(), "botID: cannot be blank") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateBotTextReply_DisabledOrMisconfiguredBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	origPath := config.PathStorages
	t.Cleanup(func() { config.PathStorages = origPath })

	config.PathStorages = t.TempDir()

	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE bots (
			id TEXT PRIMARY KEY,
			enabled INTEGER,
			api_key TEXT,
			model TEXT,
			system_prompt TEXT,
			knowledge_base TEXT,
			timezone TEXT,
			audio_enabled INTEGER,
			image_enabled INTEGER,
			memory_enabled INTEGER,
			credential_id TEXT
		);
	`); err != nil {
		t.Fatalf("failed to create bots table: %v", err)
	}

	// Bot deshabilitado (enabled = 0) y sin api_key
	if _, err := db.Exec(`INSERT INTO bots (id, enabled, api_key, model, system_prompt, knowledge_base, timezone, audio_enabled, image_enabled, memory_enabled, credential_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		"bot-disabled", 0, "", "", "", "", "UTC", 0, 0, 0, "",
	); err != nil {
		t.Fatalf("failed to insert bot row: %v", err)
	}

	reply, err := GenerateBotTextReply(ctx, "bot-disabled", "mem-1", "hola")
	if err == nil {
		t.Fatalf("expected error for disabled/misconfigured bot, got nil")
	}
	if !strings.Contains(err.Error(), "bot AI is disabled or misconfigured") {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "" {
		t.Fatalf("expected empty reply on error, got %q", reply)
	}
}

func TestClearBotMemory_RemovesMatchingKeys(t *testing.T) {
	chatMemoryMu.Lock()
	chatMemory = map[string][]chatTurn{
		"bot|bot-1|m1": {{Role: "user", Text: "hi"}},
		"bot|bot-1|m2": {{Role: "assistant", Text: "ok"}},
		"bot|bot-2|m1": {{Role: "user", Text: "hi2"}},
		"inst1|jid1":   {{Role: "user", Text: "x"}},
	}
	chatMemoryMu.Unlock()

	ClearBotMemory("bot-1")

	chatMemoryMu.Lock()
	defer chatMemoryMu.Unlock()

	if _, ok := chatMemory["bot|bot-1|m1"]; ok {
		t.Fatalf("expected key bot|bot-1|m1 to be removed")
	}
	if _, ok := chatMemory["bot|bot-1|m2"]; ok {
		t.Fatalf("expected key bot|bot-1|m2 to be removed")
	}
	if _, ok := chatMemory["bot|bot-2|m1"]; !ok {
		t.Fatalf("expected key bot|bot-2|m1 to remain")
	}
	if _, ok := chatMemory["inst1|jid1"]; !ok {
		t.Fatalf("expected key inst1|jid1 to remain")
	}
}

func TestClearInstanceMemory_RemovesInstanceKeys(t *testing.T) {
	chatMemoryMu.Lock()
	chatMemory = map[string][]chatTurn{
		"inst1|jid1": {{Role: "user", Text: "a"}},
		"inst1|jid2": {{Role: "assistant", Text: "b"}},
		"inst2|jid1": {{Role: "user", Text: "c"}},
	}
	chatMemoryMu.Unlock()

	ClearInstanceMemory("inst1")

	chatMemoryMu.Lock()
	defer chatMemoryMu.Unlock()

	if _, ok := chatMemory["inst1|jid1"]; ok {
		t.Fatalf("expected inst1|jid1 to be removed")
	}
	if _, ok := chatMemory["inst1|jid2"]; ok {
		t.Fatalf("expected inst1|jid2 to be removed")
	}
	if _, ok := chatMemory["inst2|jid1"]; !ok {
		t.Fatalf("expected inst2|jid1 to remain")
	}
}

func TestCloseChat_RemovesSpecificChatMemory(t *testing.T) {
	chatMemoryMu.Lock()
	chatMemory = map[string][]chatTurn{
		"inst1|jid1": {{Role: "user", Text: "hi"}},
		"inst1|jid2": {{Role: "assistant", Text: "ok"}},
	}
	chatMemoryMu.Unlock()

	CloseChat("inst1", "jid1")

	chatMemoryMu.Lock()
	defer chatMemoryMu.Unlock()

	if _, ok := chatMemory["inst1|jid1"]; ok {
		t.Fatalf("expected inst1|jid1 to be removed")
	}
	if _, ok := chatMemory["inst1|jid2"]; !ok {
		t.Fatalf("expected inst1|jid2 to remain")
	}
}

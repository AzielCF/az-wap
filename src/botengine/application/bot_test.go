package application

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/botengine/repository"
	_ "github.com/mattn/go-sqlite3"
)

// helper to create a fresh BotService using a temporary database.
// No usamos config.GetAppDB() porque internamente tiene un sync.Once que bloquea
// la redirección de PathStorages si ya fue inicializado.
func newTestBotService(t *testing.T) *botService {
	t.Helper()

	// Crear una base de datos SQLite temporal aislada
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_app.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	// Crear el repositorio con la DB de prueba
	repo, err := repository.NewBotSQLiteRepositoryWithDB(db)
	if err != nil {
		t.Fatalf("Failed to create bot repository: %v", err)
	}

	// Crear el servicio inyectando el repositorio
	svc := NewBotServiceWithDeps(repo, nil)

	bs, ok := svc.(*botService)
	if !ok {
		t.Fatalf("NewBotServiceWithDeps() did not return *botService, got %T", svc)
	}

	return bs
}

func TestBotService_CreateAndList(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, domainBot.CreateBotRequest{
		Name:        "Test Bot",
		Description: "desc",
		Provider:    "", // se normaliza a ProviderAI
		APIKey:      "key-123",
		Model:       "model-1",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("Create() returned empty ID")
	}
	if created.Provider != domainBot.ProviderAI {
		t.Fatalf("Create() expected provider %q, got %q", domainBot.ProviderAI, created.Provider)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() expected 1 bot, got %d", len(list))
	}
	if list[0].ID != created.ID {
		t.Fatalf("List()[0].ID = %q, want %q", list[0].ID, created.ID)
	}
}

func TestBotService_Create_Validation(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, domainBot.CreateBotRequest{Name: ""}); err == nil {
		t.Fatalf("Create() expected error for empty name, got nil")
	}

	if _, err := svc.Create(ctx, domainBot.CreateBotRequest{
		Name:     "Bot",
		Provider: domainBot.Provider("other"),
	}); err == nil {
		t.Fatalf("Create() expected error for unsupported provider, got nil")
	}
}

func TestBotService_GetByID_ValidationAndNotFound(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	if _, err := svc.GetByID(ctx, " "); err == nil {
		t.Fatalf("GetByID() expected error for blank id, got nil")
	}

	if _, err := svc.GetByID(ctx, "non-existent"); err == nil {
		t.Fatalf("GetByID() expected error for not found id, got nil")
	}
}

func TestBotService_Update_ChangesFieldsAndValidatesProvider(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, domainBot.CreateBotRequest{
		Name:        "Original",
		Description: "desc",
		Provider:    domainBot.ProviderAI,
		APIKey:      "key-1",
		Model:       "model-1",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if _, err := svc.Update(ctx, created.ID, domainBot.UpdateBotRequest{
		Provider: domainBot.Provider("other"),
	}); err == nil {
		t.Fatalf("Update() expected error for unsupported provider, got nil")
	}

	updated, err := svc.Update(ctx, created.ID, domainBot.UpdateBotRequest{
		Name:   "Updated",
		Model:  "model-2",
		APIKey: "key-2",
	})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("Update() expected name 'Updated', got %q", updated.Name)
	}
	if updated.Model != "model-2" {
		t.Fatalf("Update() expected model 'model-2', got %q", updated.Model)
	}
	if updated.APIKey != "key-2" {
		t.Fatalf("Update() expected api_key 'key-2', got %q", updated.APIKey)
	}
}

func TestBotService_Delete_Validation(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	if err := svc.Delete(ctx, " "); err == nil {
		t.Fatalf("Delete() expected error for blank id, got nil")
	}

	if err := svc.Delete(ctx, "non-existent"); err != nil {
		t.Fatalf("Delete() expected no error for non-existent id, got %v", err)
	}
}

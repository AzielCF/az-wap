package usecase

import (
	"context"
	"testing"

	"github.com/AzielCF/az-wap/config"
	domainBot "github.com/AzielCF/az-wap/domains/bot"
)

// helper to create a fresh BotService using a temporary storages directory
func newTestBotService(t *testing.T) *botService {
	t.Helper()

	// Usamos un directorio temporal para no tocar `storages/instances.db` real.
	origPathStorages := config.PathStorages
	t.Cleanup(func() {
		config.PathStorages = origPathStorages
	})

	config.PathStorages = t.TempDir()

	svc := NewBotService()
	// NewBotService siempre devuelve domainBot.IBotUsecase; para los tests sabemos
	// que es *botService, salvo que falle la inicialización de la DB.
	bs, ok := svc.(*botService)
	if !ok {
		t.Fatalf("NewBotService() did not return *botService, got %T", svc)
	}
	if bs.db == nil {
		t.Fatalf("NewBotService() returned botService with nil db")
	}
	t.Cleanup(func() {
		if bs.db != nil {
			_ = bs.db.Close()
		}
	})
	return bs
}

func TestBotService_CreateAndList(t *testing.T) {
	svc := newTestBotService(t)
	ctx := context.Background()

	// Creamos un bot básico con provider vacío (debería asumir gemini).
	created, err := svc.Create(ctx, domainBot.CreateBotRequest{
		Name:        "Test Bot",
		Description: "desc",
		Provider:    "", // se normaliza a ProviderGemini
		APIKey:      "key-123",
		Model:       "model-1",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("Create() returned empty ID")
	}
	if created.Provider != domainBot.ProviderGemini {
		t.Fatalf("Create() expected provider %q, got %q", domainBot.ProviderGemini, created.Provider)
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

	// Nombre vacío
	if _, err := svc.Create(ctx, domainBot.CreateBotRequest{Name: ""}); err == nil {
		t.Fatalf("Create() expected error for empty name, got nil")
	}

	// Provider no soportado
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

	// id en blanco
	if _, err := svc.GetByID(ctx, " "); err == nil {
		t.Fatalf("GetByID() expected error for blank id, got nil")
	}

	// id inexistente
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
		Provider:    domainBot.ProviderGemini,
		APIKey:      "key-1",
		Model:       "model-1",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	// Provider inválido en Update debe fallar.
	if _, err := svc.Update(ctx, created.ID, domainBot.UpdateBotRequest{
		Provider: domainBot.Provider("other"),
	}); err == nil {
		t.Fatalf("Update() expected error for unsupported provider, got nil")
	}

	// Actualización válida: cambiamos nombre y modelo.
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

	// id en blanco
	if err := svc.Delete(ctx, " "); err == nil {
		t.Fatalf("Delete() expected error for blank id, got nil")
	}

	// id inexistente: debería no fallar (DELETE sobre fila que no existe).
	if err := svc.Delete(ctx, "non-existent"); err != nil {
		t.Fatalf("Delete() expected no error for non-existent id, got %v", err)
	}
}

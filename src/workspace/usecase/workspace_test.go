package usecase_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/AzielCF/az-wap/workspace/usecase"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *repository.SQLiteRepository {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	repo := repository.NewSQLiteRepository(db)
	if err := repo.Init(context.Background()); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	return repo
}

func TestCreateWorkspace(t *testing.T) {
	repo := setupTestDB(t)
	uc := usecase.NewWorkspaceUsecase(repo, nil)

	ws, err := uc.CreateWorkspace(context.Background(), "Test Workspace", "Description", "owner123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ws.Name != "Test Workspace" {
		t.Errorf("expected name 'Test Workspace', got %s", ws.Name)
	}

	if ws.Limits.MaxChannels != wsDomain.DefaultLimits.MaxChannels {
		t.Errorf("expected default limits")
	}

	// Verify persistence
	stored, err := repo.GetByID(context.Background(), ws.ID)
	if err != nil {
		t.Fatalf("failed to get workspace: %v", err)
	}
	if stored.OwnerID != "owner123" {
		t.Errorf("expected owner 'owner123', got %s", stored.OwnerID)
	}
}

func TestCreateChannel(t *testing.T) {
	repo := setupTestDB(t)
	uc := usecase.NewWorkspaceUsecase(repo, nil)

	ws, _ := uc.CreateWorkspace(context.Background(), "WS1", "Desc", "owner")

	ch, err := uc.CreateChannel(context.Background(), ws.ID, channel.ChannelTypeWhatsApp, "My WhatsApp")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ch.Type != channel.ChannelTypeWhatsApp {
		t.Errorf("expected type whatsapp, got %s", ch.Type)
	}

	if ch.Status != channel.ChannelStatusPending {
		t.Errorf("expected status pending, got %s", ch.Status)
	}

	// Verify persistence
	stored, err := repo.GetChannel(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("failed to get channel: %v", err)
	}
	if stored.Name != "My WhatsApp" {
		t.Errorf("expected name 'My WhatsApp', got %s", stored.Name)
	}
}

func TestGetWorkspaceNotFound(t *testing.T) {
	repo := setupTestDB(t)
	uc := usecase.NewWorkspaceUsecase(repo, nil)

	_, err := uc.GetWorkspace(context.Background(), "non-existent")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

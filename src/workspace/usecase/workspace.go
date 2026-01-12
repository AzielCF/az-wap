package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/google/uuid"
)

type WorkspaceUsecase struct {
	repo repository.IWorkspaceRepository
}

func NewWorkspaceUsecase(repo repository.IWorkspaceRepository) *WorkspaceUsecase {
	return &WorkspaceUsecase{repo: repo}
}

func (u *WorkspaceUsecase) CreateWorkspace(ctx context.Context, name, description, ownerID string) (domain.Workspace, error) {
	ws := domain.Workspace{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Config: domain.WorkspaceConfig{
			Timezone:        "UTC",
			DefaultLanguage: "en",
			Metadata:        make(map[string]string),
		},
		Limits:    domain.DefaultLimits,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := u.repo.Create(ctx, ws); err != nil {
		return domain.Workspace{}, fmt.Errorf("failed to create workspace: %w", err)
	}

	return ws, nil
}

func (u *WorkspaceUsecase) GetWorkspace(ctx context.Context, id string) (domain.Workspace, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *WorkspaceUsecase) ListWorkspaces(ctx context.Context) ([]domain.Workspace, error) {
	return u.repo.List(ctx)
}

func (u *WorkspaceUsecase) UpdateWorkspace(ctx context.Context, id, name, description string) (domain.Workspace, error) {
	ws, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("workspace not found: %w", err)
	}

	ws.Name = name
	ws.Description = description
	ws.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, ws); err != nil {
		return domain.Workspace{}, fmt.Errorf("failed to update workspace: %w", err)
	}

	return ws, nil
}

func (u *WorkspaceUsecase) DeleteWorkspace(ctx context.Context, id string) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}
	return nil
}

func (u *WorkspaceUsecase) CreateChannel(ctx context.Context, workspaceID string, chType domain.ChannelType, name string) (domain.Channel, error) {
	// Verify workspace exists
	if _, err := u.repo.GetByID(ctx, workspaceID); err != nil {
		return domain.Channel{}, fmt.Errorf("workspace not found: %w", err)
	}

	ch := domain.Channel{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Type:        chType,
		Name:        name,
		Enabled:     false, // Disabled until connected
		Config:      domain.ChannelConfig{Settings: make(map[string]interface{})},
		Status:      domain.ChannelStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := u.repo.CreateChannel(ctx, ch); err != nil {
		return domain.Channel{}, fmt.Errorf("failed to create channel: %w", err)
	}

	return ch, nil
}

func (u *WorkspaceUsecase) ListChannels(ctx context.Context, workspaceID string) ([]domain.Channel, error) {
	return u.repo.ListChannels(ctx, workspaceID)
}

func (u *WorkspaceUsecase) EnableChannel(ctx context.Context, channelID string) error {
	ch, err := u.repo.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("channel not found: %w", err)
	}

	ch.Enabled = true
	ch.UpdatedAt = time.Now().UTC()
	return u.repo.UpdateChannel(ctx, ch)
}

func (u *WorkspaceUsecase) DisableChannel(ctx context.Context, channelID string) error {
	ch, err := u.repo.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("channel not found: %w", err)
	}

	ch.Enabled = false
	ch.UpdatedAt = time.Now().UTC()
	return u.repo.UpdateChannel(ctx, ch)
}

func (u *WorkspaceUsecase) DeleteChannel(ctx context.Context, channelID string) error {
	return u.repo.DeleteChannel(ctx, channelID)
}

func (u *WorkspaceUsecase) UpdateChannel(ctx context.Context, ch domain.Channel) error {
	ch.UpdatedAt = time.Now().UTC()
	return u.repo.UpdateChannel(ctx, ch)
}

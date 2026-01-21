package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/google/uuid"
)

type WorkspaceUsecase struct {
	repo    repository.IWorkspaceRepository
	manager *workspace.Manager
}

func NewWorkspaceUsecase(repo repository.IWorkspaceRepository, manager *workspace.Manager) *WorkspaceUsecase {
	return &WorkspaceUsecase{repo: repo, manager: manager}
}

func (u *WorkspaceUsecase) CreateWorkspace(ctx context.Context, name, description, ownerID string) (wsDomain.Workspace, error) {
	ws := wsDomain.Workspace{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Config: wsDomain.WorkspaceConfig{
			Timezone: "UTC",
			Metadata: make(map[string]string),
		},
		Limits:    wsDomain.DefaultLimits,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := u.repo.Create(ctx, ws); err != nil {
		return wsDomain.Workspace{}, fmt.Errorf("failed to create workspace: %w", err)
	}

	return ws, nil
}

func (u *WorkspaceUsecase) GetWorkspace(ctx context.Context, id string) (wsDomain.Workspace, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *WorkspaceUsecase) ListWorkspaces(ctx context.Context) ([]wsDomain.Workspace, error) {
	return u.repo.List(ctx)
}

func (u *WorkspaceUsecase) UpdateWorkspace(ctx context.Context, id, name, description string) (wsDomain.Workspace, error) {
	ws, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return wsDomain.Workspace{}, fmt.Errorf("workspace not found: %w", err)
	}

	ws.Name = name
	ws.Description = description
	ws.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, ws); err != nil {
		return wsDomain.Workspace{}, fmt.Errorf("failed to update workspace: %w", err)
	}

	return ws, nil
}

func (u *WorkspaceUsecase) DeleteWorkspace(ctx context.Context, id string) error {
	if err := u.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}
	return nil
}

func (u *WorkspaceUsecase) CreateChannel(ctx context.Context, workspaceID string, chType channel.ChannelType, name string) (channel.Channel, error) {
	// Verify workspace exists
	if _, err := u.repo.GetByID(ctx, workspaceID); err != nil {
		return channel.Channel{}, fmt.Errorf("workspace not found: %w", err)
	}

	ch := channel.Channel{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Type:        chType,
		Name:        name,
		Enabled:     false, // Disabled until connected
		Config:      channel.ChannelConfig{Settings: make(map[string]interface{})},
		Status:      channel.ChannelStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := u.repo.CreateChannel(ctx, ch); err != nil {
		return channel.Channel{}, fmt.Errorf("failed to create channel: %w", err)
	}

	return ch, nil
}

func (u *WorkspaceUsecase) ListChannels(ctx context.Context, workspaceID string) ([]channel.Channel, error) {
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
	if u.manager == nil {
		return u.repo.DeleteChannel(ctx, channelID)
	}

	// 1. Try to logout from WhatsApp if there's an active session
	adapter, ok := u.manager.GetAdapter(channelID)
	if ok {
		// Adapter is running, do proper logout + cleanup
		_ = adapter.Logout(ctx)
		_ = adapter.Cleanup(ctx)
		u.manager.UnregisterAdapter(channelID)
	} else {
		// Adapter not running, try to start it for proper logout
		if err := u.manager.StartChannel(ctx, channelID); err == nil {
			if adapter, ok := u.manager.GetAdapter(channelID); ok {
				_ = adapter.Logout(ctx)
				_ = adapter.Cleanup(ctx)
				u.manager.UnregisterAdapter(channelID)
			}
		} else {
			// Could not start, just cleanup files
			u.manager.UnregisterAndCleanup(channelID)
		}
	}

	// 2. Delete channel from database
	return u.repo.DeleteChannel(ctx, channelID)
}

func (u *WorkspaceUsecase) UpdateChannel(ctx context.Context, ch channel.Channel) error {
	ch.UpdatedAt = time.Now().UTC()

	// Ensure ExternalRef is synced for WhatsApp channels to support infrastructure bypass
	if ch.Type == channel.ChannelTypeWhatsApp {
		if instID, ok := ch.Config.Settings["instance_id"].(string); ok && instID != "" {
			ch.ExternalRef = instID
		}
	}

	err := u.repo.UpdateChannel(ctx, ch)
	if err == nil && u.manager != nil {
		u.manager.UpdateChannelConfig(ch.ID, ch.Config)
	}
	return err
}

func (u *WorkspaceUsecase) GetChannel(ctx context.Context, id string) (channel.Channel, error) {
	return u.repo.GetChannel(ctx, id)
}

func (u *WorkspaceUsecase) GetChannelByExternalRef(ctx context.Context, externalRef string) (channel.Channel, error) {
	return u.repo.GetChannelByExternalRef(ctx, externalRef)
}

func (u *WorkspaceUsecase) StartEnabledChannels(ctx context.Context, manager interface {
	StartChannel(ctx context.Context, channelID string) error
}) error {
	workspaces, err := u.repo.List(ctx)
	if err != nil {
		return err
	}

	for _, ws := range workspaces {
		if !ws.Enabled {
			continue
		}
		channels, err := u.repo.ListChannels(ctx, ws.ID)
		if err != nil {
			continue
		}
		for _, ch := range channels {
			// Only auto-start if enabled AND previously connected.
			// This prevents creating empty .db files for channels that were never scanned.
			if ch.Enabled && ch.Status == channel.ChannelStatusConnected {
				_ = manager.StartChannel(ctx, ch.ID)
			}
		}
	}
	return nil
}

// Access Rules

func (u *WorkspaceUsecase) GetAccessRules(ctx context.Context, channelID string) ([]common.AccessRule, error) {
	return u.repo.GetAccessRules(ctx, channelID)
}

func (u *WorkspaceUsecase) AddAccessRule(ctx context.Context, channelID string, identity string, action common.AccessAction, label string) error {
	// check if rule already exists
	rules, err := u.repo.GetAccessRules(ctx, channelID)
	if err == nil {
		for _, r := range rules {
			if r.Identity == identity {
				return common.ErrDuplicateRule
			}
		}
	}

	rule := common.AccessRule{
		ID:        uuid.NewString(),
		ChannelID: channelID,
		Identity:  identity,
		Action:    action,
		Label:     label,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return u.repo.AddAccessRule(ctx, rule)
}

func (u *WorkspaceUsecase) DeleteAccessRule(ctx context.Context, id string) error {
	return u.repo.DeleteAccessRule(ctx, id)
}

func (u *WorkspaceUsecase) DeleteAllAccessRules(ctx context.Context, channelID string) error {
	return u.repo.DeleteAllAccessRules(ctx, channelID)
}

package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/core/pkg/utils"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/google/uuid"
)

// --- Guest Usecases ---

func (u *WorkspaceUsecase) CreateGuest(ctx context.Context, guest wsDomain.ClientWorkspaceGuest) (wsDomain.ClientWorkspaceGuest, error) {
	if guest.ID == "" {
		guest.ID = uuid.NewString()
	}
	guest.CreatedAt = time.Now().UTC()
	guest.UpdatedAt = time.Now().UTC()

	// Validate channels and identifiers
	channels, err := u.repo.ListChannelsInClientWorkspace(ctx, guest.ClientWorkspaceID)
	if err != nil {
		return wsDomain.ClientWorkspaceGuest{}, fmt.Errorf("failed to fetch workspace channels: %w", err)
	}

	if len(channels) == 0 {
		return wsDomain.ClientWorkspaceGuest{}, fmt.Errorf("no channels associated with this workspace")
	}

	// Format platform identifiers appropriately
	if waID, ok := guest.PlatformIdentifiers["whatsapp"]; ok && waID != "" {
		utils.SanitizePhone(&waID) // appends @s.whatsapp.net if missing
		guest.PlatformIdentifiers["whatsapp"] = waID
	}

	if err := u.repo.CreateGuest(ctx, guest); err != nil {
		return wsDomain.ClientWorkspaceGuest{}, err
	}

	// Propagate
	_ = u.propagateGuestToAllChannels(ctx, guest)

	return guest, nil
}

func (u *WorkspaceUsecase) UpdateGuest(ctx context.Context, updated wsDomain.ClientWorkspaceGuest) error {
	oldGuest, err := u.repo.GetGuest(ctx, updated.ID)
	if err != nil {
		return err
	}

	updated.UpdatedAt = time.Now().UTC()

	channels, err := u.repo.ListChannelsInClientWorkspace(ctx, updated.ClientWorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to fetch workspace channels: %w", err)
	}

	if len(channels) == 0 {
		return fmt.Errorf("no channels associated with this workspace")
	}

	// Format platform identifiers appropriately
	if waID, ok := updated.PlatformIdentifiers["whatsapp"]; ok && waID != "" {
		utils.SanitizePhone(&waID) // appends @s.whatsapp.net if missing
		updated.PlatformIdentifiers["whatsapp"] = waID
	}

	if err := u.repo.UpdateGuest(ctx, updated); err != nil {
		return err
	}

	// 1. Clean up old identifiers if they changed
	u.cleanupGuestOldIdentifiers(ctx, oldGuest, updated)

	// 2. Propagate new ones
	_ = u.propagateGuestToAllChannels(ctx, updated)

	return nil
}

func (u *WorkspaceUsecase) DeleteGuest(ctx context.Context, id string) error {
	guest, err := u.repo.GetGuest(ctx, id)
	if err != nil {
		return err
	}

	// 1. Clean accesses in channels
	u.revokeGuestAccess(ctx, guest)

	return u.repo.DeleteGuest(ctx, id)
}

// --- Propagator Internals ---

func (u *WorkspaceUsecase) propagateGuestToAllChannels(ctx context.Context, guest wsDomain.ClientWorkspaceGuest) error {
	channels, err := u.repo.ListChannelsInClientWorkspace(ctx, guest.ClientWorkspaceID)
	if err != nil {
		return err
	}

	for _, ch := range channels {
		if ch.Config.GuestAccess == nil {
			ch.Config.GuestAccess = make(map[string]channel.GuestConfigCache)
		}

		for _, ident := range guest.PlatformIdentifiers {
			ch.Config.GuestAccess[ident] = channel.GuestConfigCache{
				GuestID:    guest.ID,
				BotID:      guest.BotID,
				TemplateID: guest.BotTemplateID,
				Name:       guest.Name,
			}
		}

		_ = u.UpdateChannel(ctx, ch)
	}
	return nil
}

func (u *WorkspaceUsecase) propagateWorkspaceToChannel(ctx context.Context, workspaceID, channelID string) error {
	guests, err := u.repo.ListGuestsInClientWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	ch, err := u.repo.GetChannel(ctx, channelID)
	if err != nil {
		return err
	}

	if ch.Config.GuestAccess == nil {
		ch.Config.GuestAccess = make(map[string]channel.GuestConfigCache)
	}

	for _, g := range guests {
		for _, ident := range g.PlatformIdentifiers {
			ch.Config.GuestAccess[ident] = channel.GuestConfigCache{
				GuestID:    g.ID,
				BotID:      g.BotID,
				TemplateID: g.BotTemplateID,
				Name:       g.Name,
			}
		}
	}

	return u.UpdateChannel(ctx, ch)
}

func (u *WorkspaceUsecase) cleanupGuestOldIdentifiers(ctx context.Context, old, new wsDomain.ClientWorkspaceGuest) {
	// Find which identifiers were removed or changed
	toRemove := make([]string, 0)
	for platform, oldIdent := range old.PlatformIdentifiers {
		newIdent, exists := new.PlatformIdentifiers[platform]
		if !exists || newIdent != oldIdent {
			toRemove = append(toRemove, oldIdent)
		}
	}

	if len(toRemove) == 0 {
		return
	}

	channels, _ := u.repo.ListChannelsInClientWorkspace(ctx, old.ClientWorkspaceID)
	for _, ch := range channels {
		if ch.Config.GuestAccess == nil {
			continue
		}

		changed := false
		for _, ident := range toRemove {
			if _, ok := ch.Config.GuestAccess[ident]; ok {
				delete(ch.Config.GuestAccess, ident)
				changed = true
			}
		}

		if changed {
			_ = u.UpdateChannel(ctx, ch)
		}
	}
}

func (u *WorkspaceUsecase) revokeGuestAccess(ctx context.Context, guest wsDomain.ClientWorkspaceGuest) {
	channels, _ := u.repo.ListChannelsInClientWorkspace(ctx, guest.ClientWorkspaceID)
	for _, ch := range channels {
		if ch.Config.GuestAccess == nil {
			continue
		}

		changed := false
		for _, ident := range guest.PlatformIdentifiers {
			if _, ok := ch.Config.GuestAccess[ident]; ok {
				delete(ch.Config.GuestAccess, ident)
				changed = true
			}
		}

		if changed {
			_ = u.UpdateChannel(ctx, ch)
		}
	}
}

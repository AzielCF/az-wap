package cmd

import (
	"context"
	"fmt"

	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	"github.com/AzielCF/az-wap/workspace/domain"
	"github.com/AzielCF/az-wap/workspace/usecase"
	"github.com/sirupsen/logrus"
)

// AutoMigrateLegacyInstances ensures that all legacy instances are mapped to a workspace channel.
func AutoMigrateLegacyInstances(ctx context.Context, instUC domainInstance.IInstanceUsecase, wsUC *usecase.WorkspaceUsecase) error {
	logrus.Info("[MIGRATION] Checking for legacy instances to migrate...")

	// 1. Check/Create Default Workspace
	workspaces, err := wsUC.ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %w", err)
	}

	var defaultWS domain.Workspace
	if len(workspaces) == 0 {
		logrus.Info("[MIGRATION] No workspaces found. Creating 'Default Workspace'...")
		defaultWS, err = wsUC.CreateWorkspace(ctx, "Default Workspace", "Auto-generated for legacy instances", "system_migration")
		if err != nil {
			return fmt.Errorf("failed to create default workspace: %w", err)
		}
	} else {
		defaultWS = workspaces[0]
	}

	// 2. Get Legacy Instances
	instances, err := instUC.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// 3. Sync
	channels, err := wsUC.ListChannels(ctx, defaultWS.ID)
	if err != nil {
		return fmt.Errorf("failed to list channels: %w", err)
	}

	// Create a map of existing instance IDs in channels for O(1) lookup
	existingMap := make(map[string]bool)
	for _, ch := range channels {
		if instID, ok := ch.Config.Settings["instance_id"].(string); ok {
			existingMap[instID] = true
		}
	}

	migratedCount := 0
	for _, inst := range instances {
		if _, exists := existingMap[inst.ID]; exists {
			continue // Already mapped
		}

		logrus.Infof("[MIGRATION] Migrating instance %s to workspace %s...", inst.ID, defaultWS.ID)

		// Create Channel
		// We use the internal repo/usecase logic. using CreateChannel from usecase.
		// Note: CreateChannel in usecase creates a basic channel. We might need to inject the config.
		// Since u.CreateChannel doesn't take config, we might need a richer method or update it after creation.
		// For simplicity, we assume we can update it or passing it in specific way.
		// Wait, CreateChannel in usecase doesn't accept config. It creates with empty config.
		// Implementation detail: We should create it then Update it.

		chName := fmt.Sprintf("WhatsApp %s", inst.ID)
		if inst.ID == "" {
			chName = "WhatsApp Default"
		}

		newCh, err := wsUC.CreateChannel(ctx, defaultWS.ID, domain.ChannelTypeWhatsApp, chName)
		if err != nil {
			logrus.Errorf("[MIGRATION] Failed to create channel for instance %s: %v", inst.ID, err)
			continue
		}

		// Update config to link instance_id
		newCh.Config.Settings["instance_id"] = inst.ID
		newCh.Config.Settings["workspace_id"] = defaultWS.ID
		newCh.Config.Settings["channel_id"] = newCh.ID
		// Assume legacy instances are enabled if they exist
		newCh.Enabled = true

		if err := wsUC.UpdateChannel(ctx, newCh); err != nil {
			logrus.Errorf("[MIGRATION] Failed to update channel config for instance %s: %v", inst.ID, err)
			continue
		}

		migratedCount++
	}

	if migratedCount > 0 {
		logrus.Infof("[MIGRATION] Successfully migrated %d instances.", migratedCount)
	} else {
		logrus.Info("[MIGRATION] No new instances to migrate.")
	}

	return nil
}

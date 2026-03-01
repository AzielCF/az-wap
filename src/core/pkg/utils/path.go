package utils

import (
	"fmt"
	"os"
	"path/filepath"

	coreconfig "github.com/AzielCF/az-wap/core/config"
	"github.com/sirupsen/logrus"
)

// GetWorkspaceStoragePath returns the path for a specific workspace and subfolder.
// DEPRECATED: Use GetChannelStoragePath instead to maintain hierarchical organization.
func GetWorkspaceStoragePath(workspaceID, subfolder string) string {
	logrus.Warnf("[DEPRECATED] GetWorkspaceStoragePath called for workspace %s, subfolder %s. Consider migrating to GetChannelStoragePath.", workspaceID, subfolder)
	path := filepath.Join(coreconfig.Global.Paths.Statics, "workspaces", workspaceID, subfolder)
	_ = os.MkdirAll(path, 0755)
	return path
}

// GetChannelStoragePath returns the path for a specific channel within a workspace
func GetChannelStoragePath(workspaceID, channelID, subfolder string) string {
	path := filepath.Join(coreconfig.Global.Paths.Statics, "workspaces", workspaceID, channelID, subfolder)
	_ = os.MkdirAll(path, 0755)
	return path
}

// GetWorkspaceCachePath returns the cache path for a specific workspace
func GetWorkspaceCachePath(workspaceID string) string {
	path := filepath.Join(coreconfig.Global.Paths.Statics, "cache", "workspaces", workspaceID)
	_ = os.MkdirAll(path, 0755)
	return path
}

// EnsureWorkspaceDirectories creates the basic directory structure for a workspace
func EnsureWorkspaceDirectories(workspaceID string) error {
	dirs := []string{
		filepath.Join(coreconfig.Global.Paths.Statics, "cache", "workspaces", workspaceID),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

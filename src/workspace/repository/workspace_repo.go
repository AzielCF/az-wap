package repository

import (
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
)

// IWorkspaceRepository is an alias for the domain repository interface to maintain backward compatibility in the repository package.
type IWorkspaceRepository interface {
	workspace.IWorkspaceRepository
}

package repository

import (
	"context"

	"github.com/AzielCF/az-wap/workspace/domain"
)

type IWorkspaceRepository interface {
	Init(ctx context.Context) error

	// Workspace CRUD
	Create(ctx context.Context, ws domain.Workspace) error
	GetByID(ctx context.Context, id string) (domain.Workspace, error)
	List(ctx context.Context) ([]domain.Workspace, error)
	Update(ctx context.Context, ws domain.Workspace) error
	Delete(ctx context.Context, id string) error

	// Channel CRUD
	CreateChannel(ctx context.Context, ch domain.Channel) error
	GetChannel(ctx context.Context, channelID string) (domain.Channel, error)
	ListChannels(ctx context.Context, workspaceID string) ([]domain.Channel, error)
	UpdateChannel(ctx context.Context, ch domain.Channel) error
	DeleteChannel(ctx context.Context, channelID string) error

	// Queries
	GetChannelByExternalRef(ctx context.Context, externalRef string) (domain.Channel, error)
}

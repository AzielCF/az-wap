package repository

import (
	"context"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
)

type IWorkspaceRepository interface {
	Init(ctx context.Context) error

	// Workspace CRUD
	Create(ctx context.Context, ws workspace.Workspace) error
	GetByID(ctx context.Context, id string) (workspace.Workspace, error)
	List(ctx context.Context) ([]workspace.Workspace, error)
	Update(ctx context.Context, ws workspace.Workspace) error
	Delete(ctx context.Context, id string) error

	// Channel CRUD
	CreateChannel(ctx context.Context, ch channel.Channel) error
	GetChannel(ctx context.Context, channelID string) (channel.Channel, error)
	ListChannels(ctx context.Context, workspaceID string) ([]channel.Channel, error)
	UpdateChannel(ctx context.Context, ch channel.Channel) error
	DeleteChannel(ctx context.Context, channelID string) error
	AddChannelCost(ctx context.Context, channelID string, cost float64) error
	AddChannelComplexCost(ctx context.Context, channelID string, total float64, details map[string]float64) error

	// Queries
	GetChannelByExternalRef(ctx context.Context, externalRef string) (channel.Channel, error)

	// Access Rules
	GetAccessRules(ctx context.Context, channelID string) ([]common.AccessRule, error)
	AddAccessRule(ctx context.Context, rule common.AccessRule) error
	DeleteAccessRule(ctx context.Context, id string) error
	DeleteAllAccessRules(ctx context.Context, channelID string) error

	// Scheduled Posts
	CreateScheduledPost(ctx context.Context, post common.ScheduledPost) error
	GetScheduledPost(ctx context.Context, id string) (common.ScheduledPost, error)
	ListScheduledPosts(ctx context.Context, channelID string) ([]common.ScheduledPost, error)
	ListPendingScheduledPosts(ctx context.Context) ([]common.ScheduledPost, error)
	ListUpcomingScheduledPosts(ctx context.Context, limitTime time.Time) ([]common.ScheduledPost, error)
	UpdateScheduledPost(ctx context.Context, post common.ScheduledPost) error
	DeleteScheduledPost(ctx context.Context, id string) error
}

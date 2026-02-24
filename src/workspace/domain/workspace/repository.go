package workspace

import (
	"context"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
)

type IWorkspaceRepository interface {
	Init(ctx context.Context) error

	// Workspace CRUD
	Create(ctx context.Context, ws Workspace) error
	GetByID(ctx context.Context, id string) (Workspace, error)
	List(ctx context.Context) ([]Workspace, error)
	Update(ctx context.Context, ws Workspace) error
	Delete(ctx context.Context, id string) error

	// Channel CRUD
	CreateChannel(ctx context.Context, ch channel.Channel) error
	GetChannel(ctx context.Context, channelID string) (channel.Channel, error)
	ListChannels(ctx context.Context, workspaceID string) ([]channel.Channel, error)
	ListChannelsByOwnerID(ctx context.Context, ownerID string) ([]channel.Channel, error)
	UpdateChannel(ctx context.Context, ch channel.Channel) error
	DeleteChannel(ctx context.Context, channelID string) error
	GetChannelByExternalRef(ctx context.Context, externalRef string) (channel.Channel, error)
	AddChannelCost(ctx context.Context, channelID string, cost float64) error
	AddChannelComplexCost(ctx context.Context, channelID string, total float64, details map[string]float64) error

	// Client Workspace CRUD
	CreateClientWorkspace(ctx context.Context, ws ClientWorkspace) error
	GetClientWorkspace(ctx context.Context, id string) (ClientWorkspace, error)
	ListClientWorkspaces(ctx context.Context, ownerID string) ([]ClientWorkspace, error)
	UpdateClientWorkspace(ctx context.Context, ws ClientWorkspace) error
	DeleteClientWorkspace(ctx context.Context, id string) error

	// Client Workspace Channels
	LinkChannelToClientWorkspace(ctx context.Context, workspaceID, channelID string) error
	UnlinkChannelFromClientWorkspace(ctx context.Context, workspaceID, channelID string) error
	ListChannelsInClientWorkspace(ctx context.Context, workspaceID string) ([]channel.Channel, error)

	// Client Workspace Guests
	CreateGuest(ctx context.Context, guest ClientWorkspaceGuest) error
	GetGuest(ctx context.Context, id string) (ClientWorkspaceGuest, error)
	ListGuestsInClientWorkspace(ctx context.Context, workspaceID string) ([]ClientWorkspaceGuest, error)
	UpdateGuest(ctx context.Context, guest ClientWorkspaceGuest) error
	DeleteGuest(ctx context.Context, id string) error
	ListGuestsByOwnerID(ctx context.Context, ownerID string) ([]ClientWorkspaceGuest, error)

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
	CountPendingScheduledPosts(ctx context.Context) (int64, error)
}

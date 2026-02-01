package workspace

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace/application"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MOCKS ---

type MockRepo struct {
	workspaceDomain.IWorkspaceRepository
	GetScheduledPostFunc    func(id string) (wsCommonDomain.ScheduledPost, error)
	DeleteScheduledPostFunc func(id string) error
	UpdateScheduledPostFunc func(post wsCommonDomain.ScheduledPost) error
}

func (m *MockRepo) GetScheduledPost(ctx context.Context, id string) (wsCommonDomain.ScheduledPost, error) {
	if m.GetScheduledPostFunc != nil {
		return m.GetScheduledPostFunc(id)
	}
	return wsCommonDomain.ScheduledPost{}, fmt.Errorf("not implemented")
}
func (m *MockRepo) DeleteScheduledPost(ctx context.Context, id string) error {
	if m.DeleteScheduledPostFunc != nil {
		return m.DeleteScheduledPostFunc(id)
	}
	return nil
}
func (m *MockRepo) UpdateScheduledPost(ctx context.Context, post wsCommonDomain.ScheduledPost) error {
	if m.UpdateScheduledPostFunc != nil {
		return m.UpdateScheduledPostFunc(post)
	}
	return nil
}

// --- TESTS ---

func TestManager_Deduplication(t *testing.T) {
	t.Run("LocalFallback", func(t *testing.T) {
		m := &Manager{messageDedup: sync.Map{}}
		msgID := "test-msg-local"
		assert.True(t, m.tryLockMessage("chan1", msgID, "u1", "h"))
		assert.False(t, m.tryLockMessage("chan1", msgID, "u1", "h"))
	})
}

func TestManager_Scheduler_Integration(t *testing.T) {
	cfg := valkey.Config{Address: "localhost:6379", KeyPrefix: "test"}
	vk, _ := valkey.NewClient(cfg)
	if vk == nil {
		t.Skip("Valkey not available at localhost:6379")
	}
	defer vk.Close()

	ctx := context.Background()
	taskID := uuid.NewString()[:8]
	futureTime := time.Now().Add(1 * time.Hour).UTC().Truncate(time.Second)

	// 1. Mock SQLite Response
	repo := &MockRepo{
		GetScheduledPostFunc: func(id string) (wsCommonDomain.ScheduledPost, error) {
			if id == taskID {
				return wsCommonDomain.ScheduledPost{
					ID:          taskID,
					ScheduledAt: futureTime,
					Status:      wsCommonDomain.ScheduledPostStatusPending,
					Text:        "Test Message",
					ChannelID:   "chan1",
				}, nil
			}
			return wsCommonDomain.ScheduledPost{}, fmt.Errorf("not found")
		},
	}

	m := &Manager{
		valkeyClient: vk,
		repo:         repo,
		channels:     application.NewChannelService(repo, nil, nil),
	}

	// Initialize Scheduler
	// Note: We access the private field 'scheduler' because we are in the same package 'workspace'
	m.scheduler = application.NewTaskScheduler(repo, vk, m.channels, m.acquireLock)

	t.Run("ScoreRetrieval", func(t *testing.T) {
		taskKey := vk.Key("scheduler:tasks")
		_ = vk.Inner().Do(ctx, vk.Inner().B().Del().Key(taskKey).Build())

		// WRITE TO VALKEY
		err := vk.Inner().Do(ctx, vk.Inner().B().Zadd().Key(taskKey).ScoreMember().ScoreMember(float64(futureTime.Unix()), taskID).Build()).Error()
		require.NoError(t, err)

		// ACT
		nextTime := m.scheduler.ExecTasks(ctx)

		// ASSERT
		assert.Equal(t, futureTime.Unix(), nextTime.Unix(), "Manager should read the correct time from Valkey")
	})

	t.Run("FullExecutionLifecycle", func(t *testing.T) {
		taskKey := vk.Key("scheduler:tasks")
		// Clean start
		_ = vk.Inner().Do(ctx, vk.Inner().B().Del().Key(taskKey).Build())

		pastTime := time.Now().Add(-1 * time.Minute).UTC().Truncate(time.Second)

		// Setup deletion tracker
		deleted := false
		repo.DeleteScheduledPostFunc = func(id string) error {
			if id == taskID {
				deleted = true
			}
			return nil
		}

		// Push a MATURED task to Valkey
		_ = vk.Inner().Do(ctx, vk.Inner().B().Zadd().Key(taskKey).ScoreMember().ScoreMember(float64(pastTime.Unix()), taskID).Build())

		// We need an adapter or it will fail
		// But for this test, we skip actual send logic by checking if it at least tried to get the adapter
		nextTime := m.scheduler.ExecTasks(ctx)

		assert.True(t, nextTime.IsZero(), "Should have no more tasks")
		// Verification: Ensure deleted is false because no adapter was found to execute SendMessage
		assert.False(t, deleted, "Should not be deleted as no adapter exists in test")
	})
}

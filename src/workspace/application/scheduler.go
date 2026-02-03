package application

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/sirupsen/logrus"
	valkeylib "github.com/valkey-io/valkey-go"
)

// TaskScheduler manages the lifecycle of scheduled messages using
// a combination of SQLite persistence and Valkey's reactive capabilities.
type TaskScheduler struct {
	repo         workspaceDomain.IWorkspaceRepository
	valkeyClient *valkey.Client
	channels     *ChannelService
	acquireLock  func(key string, expiration time.Duration) bool
}

// NewTaskScheduler creates a new instance of the scheduler.
func NewTaskScheduler(
	repo workspaceDomain.IWorkspaceRepository,
	vk *valkey.Client,
	channels *ChannelService,
	lockFunc func(key string, expiration time.Duration) bool,
) *TaskScheduler {
	return &TaskScheduler{
		repo:         repo,
		valkeyClient: vk,
		channels:     channels,
		acquireLock:  lockFunc,
	}
}

// StartLoop initiates the reactive background worker.
func (s *TaskScheduler) StartLoop(ctx context.Context) {
	if s.valkeyClient == nil {
		logrus.Warn("[SCHEDULER] Valkey disabled. Background scheduler will not run.")
		return
	}

	signalChan := s.valkeyClient.Key("scheduler:signal")
	logrus.Infof("[SCHEDULER] Reactive worker started. Watching channel %s", signalChan)

	go func() {
		err := s.valkeyClient.Inner().Receive(ctx, s.valkeyClient.Inner().B().Subscribe().Channel(signalChan).Build(), func(msg valkeylib.PubSubMessage) {
			logrus.Debug("[SCHEDULER] Wake-up signal received from Valkey")
		})
		if err != nil && ctx.Err() == nil {
			logrus.WithError(err).Error("[SCHEDULER] Pub/Sub listener failed")
		}
	}()

	go s.runWorker(ctx)
}

func (s *TaskScheduler) runWorker(ctx context.Context) {
	// Initial Hydration
	if err := s.PromoteTasks(ctx); err != nil {
		logrus.WithError(err).Error("[SCHEDULER] Initial task promotion failed")
	}

	safetyTicker := time.NewTicker(5 * time.Minute)
	defer safetyTicker.Stop()

	for {
		nextTaskAt := s.ExecTasks(ctx)

		sleepDuration := 1 * time.Hour
		if !nextTaskAt.IsZero() {
			sleepDuration = time.Until(nextTaskAt)
			if sleepDuration < 0 {
				sleepDuration = 1 * time.Second
			}
			if sleepDuration > 1*time.Hour {
				sleepDuration = 1 * time.Hour
			}
		}

		adaptiveTimer := time.NewTimer(sleepDuration)
		select {
		case <-ctx.Done():
			adaptiveTimer.Stop()
			return
		case <-safetyTicker.C:
			adaptiveTimer.Stop()
			s.PromoteTasks(ctx)
			s.ExecTasks(ctx)
		case <-adaptiveTimer.C:
			s.ExecTasks(ctx)
		}
	}
}

// PromoteTasks looks 24h ahead in SQLite and populates Valkey ZSET.
func (s *TaskScheduler) PromoteTasks(ctx context.Context) error {
	if s.valkeyClient == nil {
		return nil
	}

	lockKey := s.valkeyClient.Key("lock:scheduler:promo")
	res := s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Set().Key(lockKey).Value("1").Nx().Ex(55*time.Second).Build())
	if err := res.Error(); err != nil {
		if valkey.IsNil(err) {
			return nil
		}
		return err
	}

	lookAhead := time.Now().Add(24 * time.Hour).UTC()
	posts, err := s.repo.ListUpcomingScheduledPosts(ctx, lookAhead)
	if err != nil {
		return err
	}

	key := s.valkeyClient.Key("scheduler:tasks")
	for _, post := range posts {
		if post.Status == wsCommonDomain.ScheduledPostStatusPending {
			post.Status = wsCommonDomain.ScheduledPostStatusEnqueued
			post.UpdatedAt = time.Now().UTC()
			if err := s.repo.UpdateScheduledPost(ctx, post); err != nil {
				continue
			}

			// Atomic Adjustment: One less in DB, one more in Memory
			statsKey := s.valkeyClient.Key("monitoring") + ":stats"
			_ = s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Hincrby().Key(statsKey).Field("tasks_db").Increment(-1).Build())
		}

		if post.Status == wsCommonDomain.ScheduledPostStatusEnqueued {
			score := float64(post.ScheduledAt.Unix())
			_ = s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zadd().Key(key).ScoreMember().ScoreMember(score, post.ID).Build())
		}
	}
	return nil
}

// ExecTasks executes matured tasks and returns the time for the NEXT task.
func (s *TaskScheduler) ExecTasks(ctx context.Context) time.Time {
	key := s.valkeyClient.Key("scheduler:tasks")
	now := float64(time.Now().Unix())

	res := s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zrangebyscore().Key(key).Min("-inf").Max(fmt.Sprintf("%f", now)).Build())
	taskIDs, err := res.AsStrSlice()

	if err == nil && len(taskIDs) > 0 {
		for _, id := range taskIDs {
			if !s.acquireLock("lock:exec:"+id, 30*time.Second) {
				continue
			}

			post, err := s.repo.GetScheduledPost(ctx, id)
			if err != nil {
				_ = s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zrem().Key(key).Member(id).Build())
				continue
			}

			adapter, ok := s.channels.GetAdapter(post.ChannelID)
			if !ok {
				logrus.Errorf("[SCHEDULER] Adapter %s not found for task %s. Removing from memory to prevent loop.", post.ChannelID, id)
				_ = s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zrem().Key(key).Member(id).Build())
				continue
			}

			logrus.Infof("[SCHEDULER] Executing task %s -> %s", id, post.TargetID)

			_, err = adapter.SendMessage(ctx, post.TargetID, post.Text, "")
			if err != nil {
				logrus.WithError(err).Errorf("[SCHEDULER] Task %s failed", id)
				post.Status = wsCommonDomain.ScheduledPostStatusFailed
				post.Error = err.Error()
				_ = s.repo.UpdateScheduledPost(ctx, post)
			} else {
				logrus.Infof("[SCHEDULER] Success! Cleaning up task %s.", id)
				_ = s.repo.DeleteScheduledPost(ctx, id)
				_ = s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zrem().Key(key).Member(id).Build())
			}
		}
	}

	// Find the score of the NEXT task
	cmdPeek := s.valkeyClient.Inner().B().Zrangebyscore().Key(key).Min("-inf").Max("+inf").Limit(0, 1).Build()
	peekRes, _ := s.valkeyClient.Inner().Do(ctx, cmdPeek).AsStrSlice()

	if len(peekRes) > 0 && peekRes[0] != "" {
		memberID := peekRes[0]
		score, err := s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zscore().Key(key).Member(memberID).Build()).AsFloat64()
		if err == nil {
			return time.Unix(int64(score), 0)
		}
	}

	return time.Time{}
}

// CountActiveTasks returns the number of tasks currently in the memory queue (Valkey).
func (s *TaskScheduler) CountActiveTasks(ctx context.Context) int64 {
	if s.valkeyClient == nil {
		return 0
	}
	key := s.valkeyClient.Key("scheduler:tasks")
	res, err := s.valkeyClient.Inner().Do(ctx, s.valkeyClient.Inner().B().Zcard().Key(key).Build()).AsInt64()
	if err != nil {
		return 0
	}
	return res
}

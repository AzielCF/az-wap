package usecase

import (
	"context"
	"fmt"
	"time"

	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
	wsRepo "github.com/AzielCF/az-wap/workspace/repository"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type serviceNewsletter struct {
	workspaceMgr *workspace.Manager
	repo         wsRepo.IWorkspaceRepository
	monitor      monitoring.MonitoringStore
	vk           *valkey.Client
}

func NewNewsletterService(workspaceMgr *workspace.Manager, repo wsRepo.IWorkspaceRepository, monitor monitoring.MonitoringStore, vk *valkey.Client) domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{
		workspaceMgr: workspaceMgr,
		repo:         repo,
		monitor:      monitor,
		vk:           vk,
	}
}

func (service serviceNewsletter) getAdapterForToken(ctx context.Context, token string) (wsChannelDomain.ChannelAdapter, error) {
	if token == "" || service.workspaceMgr == nil {
		return nil, fmt.Errorf("workspace manager or token missing")
	}

	adapter, ok := service.workspaceMgr.GetAdapter(token)
	if !ok {
		return nil, fmt.Errorf("channel adapter %s not found or not active. Ensure the channel is enabled and running", token)
	}

	return adapter, nil
}

func (service serviceNewsletter) Unfollow(ctx context.Context, request domainNewsletter.UnfollowRequest) (err error) {
	if err = validations.ValidateUnfollowNewsletter(ctx, request); err != nil {
		return err
	}

	adapter, err := service.getAdapterForToken(ctx, request.Token)
	if err != nil {
		return err
	}

	return adapter.UnfollowNewsletter(ctx, request.NewsletterID)
}

func (service serviceNewsletter) List(ctx context.Context, channelID string) ([]wsCommonDomain.NewsletterInfo, error) {
	adapter, err := service.getAdapterForToken(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return adapter.FetchNewsletters(ctx)
}

func (service serviceNewsletter) SchedulePost(ctx context.Context, request domainNewsletter.SchedulePostRequest) (wsCommonDomain.ScheduledPost, error) {
	if request.ChannelID == "" || request.TargetID == "" {
		return wsCommonDomain.ScheduledPost{}, fmt.Errorf("channel_id and target_id are required")
	}

	post := wsCommonDomain.ScheduledPost{
		ID:          uuid.NewString(),
		ChannelID:   request.ChannelID,
		TargetID:    request.TargetID,
		SenderID:    request.SenderID,
		Text:        request.Text,
		MediaPath:   request.MediaPath,
		ScheduledAt: request.ScheduledAt.UTC(), // Always UTC
		Status:      wsCommonDomain.ScheduledPostStatusPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// 1. Persist to DB
	if err := service.repo.CreateScheduledPost(ctx, post); err != nil {
		return wsCommonDomain.ScheduledPost{}, err
	}

	// 2. If within 24h window, push to Valkey
	if service.vk != nil && post.ScheduledAt.Before(time.Now().Add(24*time.Hour)) {
		key := service.vk.Key("scheduler:tasks")
		err := service.vk.Inner().Do(ctx, service.vk.Inner().B().Zadd().Key(key).ScoreMember().ScoreMember(float64(post.ScheduledAt.Unix()), post.ID).Build()).Error()
		if err == nil {
			post.Status = wsCommonDomain.ScheduledPostStatusEnqueued
			_ = service.repo.UpdateScheduledPost(ctx, post)
		} else {
			logrus.WithError(err).Warnf("[SCHEDULER] Failed to enqueue post %s in Valkey", post.ID)
		}
	}

	return post, nil
}

func (service serviceNewsletter) ListScheduled(ctx context.Context, channelID string) ([]wsCommonDomain.ScheduledPost, error) {
	return service.repo.ListScheduledPosts(ctx, channelID)
}

func (service serviceNewsletter) ListScheduledBySender(ctx context.Context, channelID, senderID string) ([]wsCommonDomain.ScheduledPost, error) {
	// First get all posts for channel
	// Note: Ideally we add a repo method ListScheduledPostsBySender to optimize db hit
	// For now, let's filter in memory or add repo method if performance needed
	// User requested "index the target_id with the channel id" -> implies we should query efficiently.
	// But SenderID is separate column now.
	// Let's implement filtering in UseCase for now to avoid altering repo interface again deeply if not needed immediately
	// BUT repo is best place.

	// Let's filter in memory from ListScheduledPosts since typically not MILLIONS of scheduled posts per channel?
	// Actually, wait, "ListScheduledPosts" gets everything.
	// Let's use ListScheduledPosts and filter.

	posts, err := service.repo.ListScheduledPosts(ctx, channelID)
	if err != nil {
		return nil, err
	}

	var filtered []wsCommonDomain.ScheduledPost
	for _, p := range posts {
		if p.SenderID == senderID {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func (service serviceNewsletter) CancelScheduled(ctx context.Context, postID string) error {
	post, err := service.repo.GetScheduledPost(ctx, postID)
	if err != nil {
		return err
	}

	if post.Status != wsCommonDomain.ScheduledPostStatusPending && post.Status != wsCommonDomain.ScheduledPostStatusEnqueued {
		return fmt.Errorf("cannot cancel post in status %s", post.Status)
	}

	// 1. Remove from Valkey if enqueued
	if service.vk != nil && post.Status == wsCommonDomain.ScheduledPostStatusEnqueued {
		key := service.vk.Key("scheduler:tasks")
		_ = service.vk.Inner().Do(ctx, service.vk.Inner().B().Zrem().Key(key).Member(post.ID).Build()).Error()
	}

	post.Status = wsCommonDomain.ScheduledPostStatusCancelled
	post.UpdatedAt = time.Now().UTC()

	return service.repo.UpdateScheduledPost(ctx, post)
}

// ProcessScheduledPosts (The Promoter)
// It moves tasks from SQLite to Valkey for the next 24h window.
func (service serviceNewsletter) ProcessScheduledPosts(ctx context.Context) error {
	if service.vk == nil {
		return nil
	}

	// 0. Update global counter for monitoring
	if count, err := service.repo.CountPendingScheduledPosts(ctx); err == nil {
		_ = service.monitor.UpdateStat(ctx, "pending", count)
	}

	// 1. Acceder al lock para evitar ejecutor múltiple
	lockKey := service.vk.Key("lock:scheduler:promo")
	err := service.vk.Inner().Do(ctx, service.vk.Inner().B().Set().Key(lockKey).Value("1").Nx().Ex(55*time.Second).Build()).Error()
	if err != nil {
		if valkey.IsNil(err) {
			// Already locked by another node
			return nil
		}
		return err
	}

	// Horizon: 24h from now
	lookAhead := time.Now().Add(24 * time.Hour).UTC()

	posts, err := service.repo.ListUpcomingScheduledPosts(ctx, lookAhead)
	if err != nil {
		return err
	}

	key := service.vk.Key("scheduler:tasks")
	for _, post := range posts {
		if post.Status == wsCommonDomain.ScheduledPostStatusPending {
			// Atomic update in DB to avoid race conditions
			post.Status = wsCommonDomain.ScheduledPostStatusEnqueued
			post.UpdatedAt = time.Now().UTC()
			if err := service.repo.UpdateScheduledPost(ctx, post); err != nil {
				continue
			}
		} else if post.Status != wsCommonDomain.ScheduledPostStatusEnqueued {
			// Skip other statuses (sent, failed, cancelled)
			continue
		}

		// Push (or re-push) to Valkey
		score := float64(post.ScheduledAt.Unix())
		err := service.vk.Inner().Do(ctx, service.vk.Inner().B().Zadd().Key(key).ScoreMember().ScoreMember(score, post.ID).Build()).Error()
		if err != nil {
			logrus.WithError(err).Errorf("[SCHEDULER] Failed to move post %s to Valkey", post.ID)
		}
	}

	return nil
}

// RunTaskWorker (The Worker)
// Pollea Valkey y ejecuta las tareas cuyo timestamp ya pasó.
func (service serviceNewsletter) RunTaskWorker(ctx context.Context) error {
	if service.vk == nil {
		logrus.Warn("[SCHEDULER] Valkey not available, Worker disabled")
		return nil
	}

	key := service.vk.Key("scheduler:tasks")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			now := float64(time.Now().Unix())
			// Get IDs of tasks that should be executed
			resp, err := service.vk.Inner().Do(ctx, service.vk.Inner().B().Zrangebyscore().Key(key).Min("-inf").Max(fmt.Sprintf("%f", now)).Limit(0, 10).Build()).AsStrSlice()
			if err != nil {
				if !valkey.IsNil(err) {
					logrus.WithError(err).Error("[SCHEDULER] Failed to poll Valkey ZSET")
				}
				continue
			}

			for _, id := range resp {
				// Try to claim the task atómically
				remErr := service.vk.Inner().Do(ctx, service.vk.Inner().B().Zrem().Key(key).Member(id).Build()).Error()
				if remErr == nil {
					// We claimed it!
					go func(taskID string) {
						taskCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
						defer cancel()
						service.executeTaskByID(taskCtx, taskID)
					}(id)
				}
			}
		}
	}
}

func (service serviceNewsletter) executeTaskByID(ctx context.Context, id string) {
	post, err := service.repo.GetScheduledPost(ctx, id)
	if err != nil {
		logrus.Errorf("[SCHEDULER] Failed to fetch task %s from DB: %v", id, err)
		return
	}

	// Check eligibility
	if post.Status != wsCommonDomain.ScheduledPostStatusEnqueued {
		// Already cancelled or processed
		return
	}

	service.executePost(ctx, post)
}

func (service serviceNewsletter) executePost(ctx context.Context, post wsCommonDomain.ScheduledPost) {
	logrus.Infof("[SCHEDULER] Queuing post %s for execution", post.ID)

	// Use Worker Pool for execution to ensure monitoring visibility and concurrency control
	msgworker.GetGlobalPool().Dispatch(msgworker.MessageJob{
		InstanceID: post.ChannelID,
		ChatJID:    post.TargetID,
		Handler: func(workerCtx context.Context) error {
			logrus.Infof("[SCHEDULER] Worker executing post %s", post.ID)

			// Add a timeout to prevent the worker from being blocked indefinitely
			sendCtx, cancel := context.WithTimeout(workerCtx, 30*time.Second)
			defer cancel()

			adapter, err := service.getAdapterForToken(sendCtx, post.ChannelID)
			if err != nil {
				service.handlePostError(sendCtx, post, err)
				return nil
			}

			var errSend error

			// Determine logical target type
			isNewsletter := false
			if len(post.TargetID) > 11 && post.TargetID[len(post.TargetID)-11:] == "@newsletter" {
				isNewsletter = true
			}

			if isNewsletter {
				_, errSend = adapter.SendNewsletterMessage(sendCtx, post.TargetID, post.Text, post.MediaPath)
			} else {
				// Standard Group or Chat
				if post.MediaPath != "" {
					errSend = fmt.Errorf("media scheduling for groups not fully implemented yet in auto-scheduler, only text supported")
				} else {
					if post.Text != "" {
						_, errSend = adapter.SendMessage(sendCtx, post.TargetID, post.Text, "")
					}
				}
			}

			if errSend != nil {
				service.handlePostError(sendCtx, post, errSend)
			} else {
				logrus.Infof("[SCHEDULER] Post %s sent successfully", post.ID)
				post.Status = wsCommonDomain.ScheduledPostStatusSent
				post.Error = ""
				post.UpdatedAt = time.Now()
				if err := service.repo.UpdateScheduledPost(sendCtx, post); err != nil {
					logrus.Errorf("Failed to update post status after execution %s: %v", post.ID, err)
				}
			}
			return nil
		},
	})
}

func (service serviceNewsletter) handlePostError(ctx context.Context, post wsCommonDomain.ScheduledPost, err error) {
	logrus.Errorf("Failed to send scheduled post %s: %v", post.ID, err)
	post.Status = wsCommonDomain.ScheduledPostStatusFailed
	post.Error = err.Error()
	post.UpdatedAt = time.Now()
	if resultErr := service.repo.UpdateScheduledPost(ctx, post); resultErr != nil {
		logrus.Errorf("Failed to update post status %s: %v", post.ID, resultErr)
	}
}

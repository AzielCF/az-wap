package usecase

import (
	"context"
	"fmt"
	"time"

	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
	wsRepo "github.com/AzielCF/az-wap/workspace/repository"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type serviceNewsletter struct {
	workspaceMgr *workspace.Manager
	repo         wsRepo.IWorkspaceRepository
}

func NewNewsletterService(workspaceMgr *workspace.Manager, repo wsRepo.IWorkspaceRepository) domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{
		workspaceMgr: workspaceMgr,
		repo:         repo,
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
		ScheduledAt: request.ScheduledAt,
		Status:      wsCommonDomain.ScheduledPostStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := service.repo.CreateScheduledPost(ctx, post); err != nil {
		return wsCommonDomain.ScheduledPost{}, err
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

	if post.Status != wsCommonDomain.ScheduledPostStatusPending {
		return fmt.Errorf("cannot cancel post in status %s", post.Status)
	}

	post.Status = wsCommonDomain.ScheduledPostStatusCancelled
	post.UpdatedAt = time.Now()

	return service.repo.UpdateScheduledPost(ctx, post)
}

// ProcessScheduledPosts checks for pending posts and schedules them for execution.
// It uses a look-ahead window to load upcoming posts into memory for precise execution.
func (service serviceNewsletter) ProcessScheduledPosts(ctx context.Context) error {
	// Look ahead 2 minutes to load upcoming posts "hot"
	lookAhead := time.Now().Add(2 * time.Minute)

	posts, err := service.repo.ListUpcomingScheduledPosts(ctx, lookAhead)
	if err != nil {
		return err
	}

	for _, post := range posts {
		// 1. Mark as processing IMMEDIATELY to prevent re-fetching by next ticker
		post.Status = wsCommonDomain.ScheduledPostStatusProcessing
		post.UpdatedAt = time.Now()
		if err := service.repo.UpdateScheduledPost(ctx, post); err != nil {
			logrus.Errorf("Failed to mark post %s as processing: %v", post.ID, err)
			continue
		}

		// 2. Schedule execution
		waitDuration := time.Until(post.ScheduledAt)
		if waitDuration < 0 {
			waitDuration = 0
		}

		logrus.Infof("[SCHEDULER] Hot-loading post %s. Scheduled in %v", post.ID, waitDuration)

		// Launch in goroutine
		go func(p wsCommonDomain.ScheduledPost, wait time.Duration) {
			if wait > 0 {
				time.Sleep(wait)
			}
			service.executePost(context.Background(), p)
		}(post, waitDuration)
	}

	return nil
}

func (service serviceNewsletter) executePost(ctx context.Context, post wsCommonDomain.ScheduledPost) {
	logrus.Infof("[SCHEDULER] Executing post %s now", post.ID)

	adapter, err := service.getAdapterForToken(ctx, post.ChannelID)
	if err != nil {
		logrus.Errorf("Failed to get adapter for scheduled post %s: %v", post.ID, err)
		post.Status = wsCommonDomain.ScheduledPostStatusFailed
		post.Error = err.Error()
		resultErr := service.repo.UpdateScheduledPost(ctx, post)
		if resultErr != nil {
			logrus.Errorf("Failed to update post status %s: %v", post.ID, resultErr)
		}
		return
	}

	var errSend error

	// Determine logical target type by ID analysis (naive but effective for now)
	isNewsletter := false
	if len(post.TargetID) > 11 && post.TargetID[len(post.TargetID)-11:] == "@newsletter" {
		isNewsletter = true
	}

	if isNewsletter {
		_, errSend = adapter.SendNewsletterMessage(ctx, post.TargetID, post.Text, post.MediaPath)
	} else {
		// Standard Group or Chat
		if post.MediaPath != "" {
			errSend = fmt.Errorf("media scheduling for groups not fully implemented yet in auto-scheduler, only text supported")
		} else {
			if post.Text != "" {
				_, errSend = adapter.SendMessage(ctx, post.TargetID, post.Text, "")
			}
		}
	}

	if errSend != nil {
		logrus.Errorf("Failed to send scheduled post %s: %v", post.ID, errSend)
		post.Status = wsCommonDomain.ScheduledPostStatusFailed
		post.Error = errSend.Error()
	} else {
		logrus.Infof("[SCHEDULER] Post %s sent successfully", post.ID)
		post.Status = wsCommonDomain.ScheduledPostStatusSent
		post.Error = ""
	}

	post.UpdatedAt = time.Now()
	if err := service.repo.UpdateScheduledPost(ctx, post); err != nil {
		logrus.Errorf("Failed to update post status after execution %s: %v", post.ID, err)
	}
}

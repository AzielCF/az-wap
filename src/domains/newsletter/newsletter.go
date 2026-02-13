package newsletter

import (
	"context"
	"time"

	wsDomainCommon "github.com/AzielCF/az-wap/workspace/domain/common"
)

type INewsletterUsecase interface {
	Unfollow(ctx context.Context, request UnfollowRequest) (err error)
	List(ctx context.Context, channelID string) ([]wsDomainCommon.NewsletterInfo, error)
	SchedulePost(ctx context.Context, request SchedulePostRequest) (wsDomainCommon.ScheduledPost, error)
	ListScheduled(ctx context.Context, channelID string) ([]wsDomainCommon.ScheduledPost, error)
	ListScheduledBySender(ctx context.Context, channelID, senderID string) ([]wsDomainCommon.ScheduledPost, error)
	CancelScheduled(ctx context.Context, postID string) error
	ProcessScheduledPosts(ctx context.Context) error
	RunTaskWorker(ctx context.Context) error
}

type UnfollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
	Token        string `json:"token,omitempty" form:"token"`
}

type SchedulePostRequest struct {
	ChannelID      string    `json:"channel_id"`
	TargetID       string    `json:"target_id"`
	SenderID       string    `json:"sender_id"` // Optional: who is scheduling
	Text           string    `json:"text"`
	MediaPath      string    `json:"media_path,omitempty"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	RecurrenceDays string    `json:"recurrence_days"` // "0,1,2" (Sun, Mon, Tue)
}

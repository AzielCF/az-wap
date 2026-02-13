package common

import "time"

type ScheduledPostStatus string

const (
	ScheduledPostStatusPending    ScheduledPostStatus = "pending"
	ScheduledPostStatusEnqueued   ScheduledPostStatus = "enqueued"
	ScheduledPostStatusSent       ScheduledPostStatus = "sent"
	ScheduledPostStatusFailed     ScheduledPostStatus = "failed"
	ScheduledPostStatusCancelled  ScheduledPostStatus = "cancelled"
	ScheduledPostStatusProcessing ScheduledPostStatus = "processing"
)

type ScheduledPost struct {
	ID             string              `json:"id"`
	ChannelID      string              `json:"channel_id"` // Which WhatsApp channel accounts sends it
	TargetID       string              `json:"target_id"`  // The target JID (Newsletter, Group, or User)
	SenderID       string              `json:"sender_id"`  // Who scheduled this (User JID/LID)
	Text           string              `json:"text"`
	MediaPath      string              `json:"media_path,omitempty"` // Path to media file if any
	MediaType      MediaType           `json:"media_type,omitempty"`
	ScheduledAt    time.Time           `json:"scheduled_at"`
	Status         ScheduledPostStatus `json:"status"`
	Error          string              `json:"error,omitempty"`
	RecurrenceDays string              `json:"recurrence_days,omitempty"` // Days of week "1,3,5"
	OriginalTime   string              `json:"original_time,omitempty"`   // HH:MM
	ExecutionCount int                 `json:"execution_count"`           // Number of times executed
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
)

type ReminderTools struct {
	service domainNewsletter.INewsletterUsecase
}

func NewReminderTools(service domainNewsletter.INewsletterUsecase) *ReminderTools {
	return &ReminderTools{service: service}
}

func (t *ReminderTools) ScheduleReminderTool() *domain.NativeTool {
	return &domain.NativeTool{
		Tool: domainMCP.Tool{
			Name:        "schedule_reminder",
			Description: "Schedules a reminder (message) for the user themselves at a future time. Use this for tasks, appointments, or personal reminders.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The message to send. Write it in your own words, strictly adhering to your SYSTEM PROMPT persona and language. Be natural, human-like, and avoid robotic prefixes like 'REMINDER:'. Speak directly to the user (e.g., 'Hey [Name], remember to...' or as appropriate for your character).",
					},
					"scheduled_at": map[string]interface{}{
						"type":        "string",
						"description": "ISO 8601 formatted date string for when to remind (must be in future)",
					},
				},
				"required": []string{"text", "scheduled_at"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, ok := ctxData["instance_id"].(string)
			if !ok || instanceID == "" {
				return nil, fmt.Errorf("instance_id not available in context")
			}

			// The sender is the target for a reminder
			senderID, ok := ctxData["sender_id"].(string)
			if !ok || senderID == "" {
				return nil, fmt.Errorf("sender_id not available in context. Cannot schedule reminder for unknown user.")
			}

			text, _ := args["text"].(string)
			scheduledAtStr, _ := args["scheduled_at"].(string)

			scheduledAt, err := time.Parse(time.RFC3339, scheduledAtStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date format, use ISO 8601 (RFC3339): %v", err)
			}

			req := domainNewsletter.SchedulePostRequest{
				ChannelID:   instanceID,
				TargetID:    senderID, // Target is the user themselves
				SenderID:    senderID, // Track who created it
				Text:        text,
				ScheduledAt: scheduledAt,
			}

			post, err := t.service.SchedulePost(ctx, req)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"status":       "scheduled",
				"post_id":      post.ID,
				"target_id":    post.TargetID,
				"scheduled_at": post.ScheduledAt.String(),
				"message":      "Reminder scheduled successfully",
			}, nil
		},
	}
}

func (t *ReminderTools) ListRemindersTool() *domain.NativeTool {
	return &domain.NativeTool{
		Tool: domainMCP.Tool{
			Name:        "list_my_reminders",
			Description: "Lists all scheduled reminders/tasks for the user.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, ok := ctxData["instance_id"].(string)
			if !ok || instanceID == "" {
				return nil, fmt.Errorf("instance_id not available in context")
			}

			senderID, ok := ctxData["sender_id"].(string)
			if !ok || senderID == "" {
				return nil, fmt.Errorf("sender_id not available in context")
			}

			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			// Format for display
			var result []map[string]interface{}
			for _, p := range posts {
				result = append(result, map[string]interface{}{
					"id":           p.ID,
					"target_id":    p.TargetID,
					"text":         p.Text,
					"scheduled_at": p.ScheduledAt.String(),
					"status":       p.Status,
				})
			}

			return map[string]interface{}{
				"reminders": result,
				"count":     len(result),
			}, nil
		},
	}
}

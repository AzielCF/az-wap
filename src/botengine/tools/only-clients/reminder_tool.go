package onlyclients

import (
	"context"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	globalConfig "github.com/AzielCF/az-wap/config"
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
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "schedule_reminder",
			Description: "Schedules a reminder (message) for the user themselves at a future time. Use this for tasks, appointments, or personal reminders.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The CONTENT of the reminder message. IMPORTANT: This message will be sent IN THE FUTURE when the reminder triggers. Do NOT include relative time phrases like 'in 10 minutes'. Instead of 'Meeting in 10 minutes', write 'It is time for your meeting!' or 'You have a meeting now'. Write it as if it's happening at that moment.",
					},
					"date": map[string]interface{}{
						"type":        "string",
						"description": "Date in 'YYYY-MM-DD' format. YOU MUST CALCULATE THIS based on 'TODAY' in your system prompt + the user's relative request (e.g., 'tomorrow'). DO NOT ASK THE USER FOR THE DATE.",
					},
					"time": map[string]interface{}{
						"type":        "string",
						"description": "Time to remind. YOU MUST CALCULATE THIS based on 'TIME_NOW' + duration (e.g., 'in 5 mins'). DO NOT ASK THE USER.",
					},
				},
				"required": []string{"text", "date", "time"},
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

			text, _ := args["text"].(string)
			dateStr, _ := args["date"].(string)
			timeStr, _ := args["time"].(string)

			fullStr := fmt.Sprintf("%s %s", dateStr, timeStr)

			// FIX: Load Location from Context (Client/Channel) > Config > Default UTC
			locName := "UTC"

			// Try to get metadata map first
			if meta, ok := ctxData["metadata"].(map[string]interface{}); ok {
				if tz, ok := meta["bot_timezone"].(string); ok && tz != "" {
					locName = tz
				}
			}

			// If not found in metadata, check root (backwards compatibility) or config
			if locName == "UTC" {
				if tz, ok := ctxData["bot_timezone"].(string); ok && tz != "" {
					locName = tz
				} else if globalConfig.AITimezone != "" {
					locName = globalConfig.AITimezone
				}
			}

			loc, err := time.LoadLocation(locName)
			if err != nil {
				loc = time.UTC // Fallback if invalid
			}

			// Try multiple formats including AM/PM
			var scheduledAt time.Time

			formats := []string{
				"2006-01-02 15:04",    // 24h standard
				"2006-01-02 15:04:05", // 24h with seconds
				"2006-01-02 03:04 PM", // 12h with space and uppercase
				"2006-01-02 03:04 pm", // 12h with space and lowercase
				"2006-01-02 03:04PM",  // 12h no space
				"2006-01-02 3:04 PM",  // 12h single digit hour
			}

			parsed := false
			for _, f := range formats {
				// Use ParseInLocation
				scheduledAt, err = time.ParseInLocation(f, fullStr, loc)
				if err == nil {
					parsed = true
					break
				}
			}

			if !parsed {
				return nil, fmt.Errorf("invalid time format. Received: %s. Please use 'HH:MM' (24h) or 'HH:MM PM' (12h)", fullStr)
			}

			req := domainNewsletter.SchedulePostRequest{
				ChannelID:   instanceID,
				TargetID:    senderID,
				SenderID:    senderID,
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
		IsVisible: IsClientRegistered,
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

package onlyclients

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	coreconfig "github.com/AzielCF/az-wap/core/config"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	wsCommonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
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
			Description: "Schedules a reminder for the user. YOU MUST REWRITE THE CONTENT from your perspective as an assistant.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The FINAL NOTIFICATION MESSAGE to be sent. \nRULES:\n1. **SWITCH PRONOUNS**: If user says 'my girlfriend', you write 'YOUR girlfriend'. If user says 'I have to go', write 'You have to go'.\n2. **BE CREATIVE**: Use emojis and bold text. (e.g., '⚠️ **Priority**: Time to call **your** mom!').\n3. **LONG-TERM CONTEXT**: If the reminder is for >2 days in the future, Start with context: '📅 **Reminder from [Today's Day/Date]**: You asked me to remind you...'.\n4. **NO VERBATIM**: Never just copy what the user said.\n5. **CURRENT TIME**: Write it as if the event is happening NOW.",
					},
					"date": map[string]interface{}{
						"type":        "string",
						"description": "Date YYYY-MM-DD. Calculate based on user's relative request (today, tomorrow, next friday).",
					},
					"time": map[string]interface{}{
						"type":        "string",
						"description": "Time HH:MM. Calculate based on context.",
					},
				},
				"required": []string{"text", "date", "time"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			text, _ := args["text"].(string)
			dateStr, _ := args["date"].(string)
			timeStr, _ := args["time"].(string)
			recurrenceDays, _ := args["recurrence_days"].(string)

			// Resolve Location
			loc := t.resolveLocation(ctxData)

			// Parse time using helper (Reference is Now for schedule)
			// But for Schedule, both date and time are REQUIRED by schema, so they will be present.
			// Passing a dummy reference since we expect full date/time.
			scheduledAt, err := t.parseVariableTime(dateStr, timeStr, loc, time.Now().In(loc))
			if err != nil {
				return nil, err
			}

			req := domainNewsletter.SchedulePostRequest{
				ChannelID:      instanceID,
				TargetID:       senderID,
				SenderID:       senderID,
				Text:           text,
				ScheduledAt:    scheduledAt,
				RecurrenceDays: recurrenceDays,
			}

			post, err := t.service.SchedulePost(ctx, req)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"status":       "scheduled",
				"scheduled_at": post.ScheduledAt.In(loc).Format("Mon 02 Jan 15:04"),
				"text_created": post.Text,
				"message":      fmt.Sprintf("Reminder scheduled: '%s' for %s", post.Text, post.ScheduledAt.In(loc).Format("Mon, 02 Jan 15:04")),
			}, nil
		},
	}
}

func (t *ReminderTools) ListPendingRemindersTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "list_pending_reminders",
			Description: "Lists only ACTIVE (pending) reminders/tasks for the user. Use this when the user asks 'what do I have to do?' or 'what is pending?'. Active means status 'pending' or 'enqueued'. Can optionally filter by date.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"date": map[string]interface{}{
						"type":        "string",
						"description": "Optional. Date in 'YYYY-MM-DD' format. If provided, lists only active reminders scheduled for this specific day (e.g. today). If omitted, lists ALL active pending reminders.",
					},
				},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			targetDateStr, _ := args["date"].(string)

			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			// Resolve timezone for display and filtering
			loc := t.resolveLocation(ctxData)

			var targetStart, targetEnd time.Time
			filterByDate := false
			if targetDateStr != "" {
				parsed, err := time.ParseInLocation("2006-01-02", targetDateStr, loc)
				if err == nil {
					filterByDate = true
					targetStart = parsed
					targetEnd = parsed.Add(24 * time.Hour)
				}
			}

			var activePosts []wsCommonDomain.ScheduledPost
			for _, p := range posts {
				// Filter ACTIVE only
				if p.Status == "pending" || p.Status == "enqueued" || p.Status == "processing" {
					// Optional Date Filter
					if filterByDate {
						localTime := p.ScheduledAt.In(loc)
						if localTime.Before(targetStart) || localTime.After(targetEnd) || localTime.Equal(targetEnd) {
							continue
						}
					}
					activePosts = append(activePosts, p)
				}
			}

			// Return optimized TEXT
			return map[string]interface{}{
				"internal_reminders_data": t.formatRemindersAsText(activePosts, loc),
				"total_count":             len(activePosts),
			}, nil
		},
	}
}

func (t *ReminderTools) SearchRemindersHistoryTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "search_reminders_history",
			Description: "Searches for past or future reminders within a specific DATE RANGE. Use this when user asks 'what did I do last month?' or 'what reminders did I have yesterday?'.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date in 'YYYY-MM-DD' format (inclusive).",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date in 'YYYY-MM-DD' format (inclusive).",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Optional status to filter: 'sent', 'failed', 'cancelled', 'pending'. Leave empty for all.",
						"enum":        []string{"sent", "failed", "cancelled", "pending"},
					},
				},
				"required": []string{"start_date", "end_date"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			startStr, _ := args["start_date"].(string)
			endStr, _ := args["end_date"].(string)
			statusFilter, _ := args["status"].(string)

			loc := t.resolveLocation(ctxData)

			// Parse dates in User's Timezone
			startDate, err := time.ParseInLocation("2006-01-02", startStr, loc)
			if err != nil {
				return nil, fmt.Errorf("invalid start_date format, use YYYY-MM-DD")
			}
			endDate, err := time.ParseInLocation("2006-01-02", endStr, loc)
			if err != nil {
				return nil, fmt.Errorf("invalid end_date format, use YYYY-MM-DD")
			}
			// End of day for endDate
			endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			var filteredPosts []wsCommonDomain.ScheduledPost
			for _, p := range posts {
				// 1. Date Filter
				itemTime := p.ScheduledAt.In(loc)
				if itemTime.After(startDate) && itemTime.Before(endDate) {
					// 2. Status Filter
					if statusFilter != "" && string(p.Status) != statusFilter {
						continue
					}
					filteredPosts = append(filteredPosts, p)
				}
			}

			return map[string]interface{}{
				"internal_reminders_data": t.formatRemindersAsText(filteredPosts, loc),
				"total_count":             len(filteredPosts),
				"period":                  fmt.Sprintf("%s to %s", startStr, endStr),
			}, nil
		},
	}
}

func (t *ReminderTools) CancelReminderTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "cancel_reminder",
			Description: "Cancels a reminder by describing it. The system will find the best match based on your description (e.g., 'cancel my dentist appointment' or 'delete the reminder at 5pm').",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Text description of the reminder to cancel (e.g., 'dentist', 'meeting').",
					},
					"date": map[string]interface{}{
						"type":        "string",
						"description": "Optional date (YYYY-MM-DD) to narrow down the search.",
					},
					"time": map[string]interface{}{
						"type":        "string",
						"description": "Optional time (HH:MM) to narrow down the search.",
					},
				},
				"required": []string{"query"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			query, _ := args["query"].(string)
			dateStr, _ := args["date"].(string)
			timeStr, _ := args["time"].(string)

			if query == "" && dateStr == "" && timeStr == "" {
				return nil, fmt.Errorf("please provide a description, date, or time to identify the reminder")
			}

			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			loc := t.resolveLocation(ctxData)
			targetID := t.findBestMatch(posts, query, dateStr, timeStr, loc)

			if targetID == "" {
				return nil, fmt.Errorf("could not find any pending reminder matching your description")
			}

			err = t.service.CancelScheduled(ctx, targetID)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"status":  "cancelled",
				"message": "Reminder cancelled successfully",
			}, nil
		},
	}
}

func (t *ReminderTools) UpdateReminderTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "update_reminder",
			Description: "Updates an existing reminder by describing which one to change. The system finds the best match.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Text description to identify the reminder (e.g., 'dentist', 'meeting').",
					},
					"new_text": map[string]interface{}{
						"type":        "string",
						"description": "New content. Optional.",
					},
					"new_date": map[string]interface{}{
						"type":        "string",
						"description": "New date YYYY-MM-DD. Optional.",
					},
					"new_time": map[string]interface{}{
						"type":        "string",
						"description": "New time. Optional.",
					},
				},
				"required": []string{"query"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			query, _ := args["query"].(string)
			newText, _ := args["new_text"].(string)
			newDate, _ := args["new_date"].(string)
			newTime, _ := args["new_time"].(string)

			if query == "" {
				return nil, fmt.Errorf("query description is required to find the reminder")
			}

			// 1. Find Existing
			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			loc := t.resolveLocation(ctxData)
			targetID := t.findBestMatch(posts, query, "", "", loc)

			if targetID == "" {
				return nil, fmt.Errorf("could not find any reminder matching '%s'", query)
			}

			var original *wsCommonDomain.ScheduledPost
			for _, p := range posts {
				if p.ID == targetID {
					original = &p
					break
				}
			}

			// 2. Prepare New Values
			finalText := original.Text
			if newText != "" {
				finalText = newText
			}

			originalInLoc := original.ScheduledAt.In(loc)

			// Use Helper to parse new values merge with original
			finalTime, err := t.parseVariableTime(newDate, newTime, loc, originalInLoc)
			if err != nil {
				return nil, err
			}

			// 3. Delete Old
			if err := t.service.CancelScheduled(ctx, targetID); err != nil {
				return nil, fmt.Errorf("failed to delete old reminder: %v", err)
			}

			// 4. Create New
			req := domainNewsletter.SchedulePostRequest{
				ChannelID:      instanceID,
				TargetID:       senderID,
				SenderID:       senderID,
				Text:           finalText,
				ScheduledAt:    finalTime,
				RecurrenceDays: original.RecurrenceDays,
			}

			post, err := t.service.SchedulePost(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("deleted old reminder but failed to create new one: %v", err)
			}

			return map[string]interface{}{
				"status":       "updated",
				"scheduled_at": post.ScheduledAt.In(loc).Format("Mon 02 Jan 15:04"),
				"text":         post.Text,
				"message":      "Reminder updated successfully",
			}, nil
		},
	}
}

func (t *ReminderTools) CountRemindersTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "count_reminders",
			Description: "PROACTIVE CHECK: Use this when user mentions future unavailability (e.g. 'I'm traveling next week'). \nIMPORTANT: Calculate dates correctly. \n- 'Next week' means the NEXT MONDAY to the FOLLOWING SUNDAY (not starting today).\n- 'This weekend' means this coming Saturday and Sunday.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date YYYY-MM-DD. If 'next week', find the date of the next Monday.",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date YYYY-MM-DD. If 'next week', find the date of the Sunday after the next Monday.",
					},
				},
				"required": []string{"start_date", "end_date"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, senderID, err := t.extractIDs(ctxData)
			if err != nil {
				return nil, err
			}

			startStr, _ := args["start_date"].(string)
			endStr, _ := args["end_date"].(string)

			loc := t.resolveLocation(ctxData)

			// Parse Query Range
			startDate, err := time.ParseInLocation("2006-01-02", startStr, loc)
			if err != nil {
				return nil, fmt.Errorf("invalid start_date")
			}
			endDate, err := time.ParseInLocation("2006-01-02", endStr, loc)
			if err != nil {
				return nil, fmt.Errorf("invalid end_date")
			}
			// End of day
			endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

			posts, err := t.service.ListScheduledBySender(ctx, instanceID, senderID)
			if err != nil {
				return nil, err
			}

			count := 0
			priorityCount := 0
			daysWithLoad := make(map[string]bool)

			// Simple keywords for "urgency" to flag priority
			urgentKeywords := []string{"urgent", "urgente", "priority", "prioridad", "important", "importante", "pago", "pay", "bill", "cita", "doctor", "medico", "🚨", "⚠️", "❗"}

			for _, p := range posts {
				if p.Status != "pending" && p.Status != "enqueued" && p.Status != "processing" {
					continue
				}

				pTime := p.ScheduledAt.In(loc)
				if pTime.After(startDate) && pTime.Before(endDate) {
					count++
					dayStr := pTime.Format("2006-01-02")
					daysWithLoad[dayStr] = true

					// Check priority
					lowerText := strings.ToLower(p.Text)
					for _, k := range urgentKeywords {
						if strings.Contains(lowerText, k) {
							priorityCount++
							break
						}
					}
				}
			}

			var busyDays []string
			for d := range daysWithLoad {
				busyDays = append(busyDays, d)
			}

			return map[string]interface{}{
				"total_count":    count,
				"priority_count": priorityCount,
				"busy_days":      busyDays,
				"message":        fmt.Sprintf("Found %d reminders (%d seem priority) between %s and %s.", count, priorityCount, startStr, endStr),
			}, nil
		},
	}
}

// --- Helpers ---

func (t *ReminderTools) extractIDs(ctxData map[string]interface{}) (string, string, error) {
	instanceID, ok := ctxData["instance_id"].(string)
	if !ok || instanceID == "" {
		return "", "", fmt.Errorf("instance_id not available in context")
	}
	senderID, ok := ctxData["sender_id"].(string)
	if !ok || senderID == "" {
		return "", "", fmt.Errorf("sender_id not available in context")
	}
	return instanceID, senderID, nil
}

// Flexible time parser
func (t *ReminderTools) parseVariableTime(dateStr, timeStr string, loc *time.Location, referenceTime time.Time) (time.Time, error) {
	// If both are empty, return the reference time unchanged
	if dateStr == "" && timeStr == "" {
		return referenceTime, nil
	}

	// Strategy:
	// 1. If date provided, use it. If not, use reference date.
	// 2. If time provided, use it. If not, use reference time.

	refYear, refMonth, refDay := referenceTime.Date()
	refHour, refMin, refSec := referenceTime.Clock()

	// Parse Date
	y, m, d := refYear, refMonth, refDay // Default to reference
	if dateStr != "" {
		dVal, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date format %s", dateStr)
		}
		y, m, d = dVal.Date()
	}

	// Parse Time
	h, min, s := refHour, refMin, refSec // Default to reference
	if timeStr != "" {
		// Try formats
		formats := []string{
			"15:04", "15:04:05", "03:04 PM", "03:04 pm", "3:04 PM", "3:04 pm",
		}
		var parsedTime time.Time
		var err error
		found := false
		for _, f := range formats {
			parsedTime, err = time.ParseInLocation(f, timeStr, loc)
			if err == nil {
				h, min, s = parsedTime.Clock()
				found = true
				break
			}
		}
		if !found {
			return time.Time{}, fmt.Errorf("invalid time format %s", timeStr)
		}
	}

	return time.Date(y, m, d, h, min, s, 0, loc), nil
}

// Simple heuristic matching
func (t *ReminderTools) findBestMatch(posts []wsCommonDomain.ScheduledPost, query, dateStr, timeStr string, loc *time.Location) string {
	query = strings.ToLower(query)
	var bestID string
	var bestScore int

	for _, p := range posts {
		if p.Status != "pending" && p.Status != "enqueued" {
			continue
		}

		score := 0
		text := strings.ToLower(p.Text)

		// Text match
		if query != "" && strings.Contains(text, query) {
			score += 10
		}

		// Exact date/time match bonus
		pTime := p.ScheduledAt.In(loc)
		if dateStr != "" && pTime.Format("2006-01-02") == dateStr {
			score += 5
		}
		if timeStr != "" && (pTime.Format("15:04") == timeStr || pTime.Format("15:04:05") == timeStr) {
			score += 5
		}

		if score > bestScore {
			bestScore = score
			bestID = p.ID
		}
	}

	if bestScore > 0 {
		return bestID
	}
	return ""
}

func (t *ReminderTools) resolveLocation(ctxData map[string]interface{}) *time.Location {
	locName := "UTC"
	if meta, ok := ctxData["metadata"].(map[string]interface{}); ok {
		if tz, ok := meta["bot_timezone"].(string); ok && tz != "" {
			locName = tz
		}
	}
	if locName == "UTC" {
		if tz, ok := ctxData["bot_timezone"].(string); ok && tz != "" {
			locName = tz
		} else if coreconfig.Global.AI.Timezone != "" {
			locName = coreconfig.Global.AI.Timezone
		}
	}
	loc, err := time.LoadLocation(locName)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Token Saver Helper: Returns plain text list instead of JSON
func (t *ReminderTools) formatRemindersAsText(posts []wsCommonDomain.ScheduledPost, loc *time.Location) string {
	if len(posts) == 0 {
		return "No reminders found."
	}
	var sb strings.Builder
	for _, p := range posts {
		timeStr := p.ScheduledAt.In(loc).Format("Mon 02 Jan 15:04")
		// Label as internal hint to discourage verbatim copying
		sb.WriteString(fmt.Sprintf("- [%s] INTERNAL_SUBJECT_HINT: %s\n", timeStr, p.Text))
	}
	return sb.String()
}

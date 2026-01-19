package tools

import (
	"context"
	"fmt" // Keep fmt as it's used for error formatting
	"strings"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/workspace"
)

type NewsletterTools struct {
	service      domainNewsletter.INewsletterUsecase
	workspaceMgr *workspace.Manager
}

func NewNewsletterTools(service domainNewsletter.INewsletterUsecase, workspaceMgr *workspace.Manager) *NewsletterTools {
	return &NewsletterTools{
		service:      service,
		workspaceMgr: workspaceMgr,
	}
}

func (t *NewsletterTools) ListNewslettersTool() *domain.NativeTool {
	return &domain.NativeTool{
		Tool: domainMCP.Tool{
			Name:        "list_newsletters",
			Description: "Lists all WhatsApp newsletters (channels) the bot is subscribed to or admin of. Use this to check available alias names for scheduling.",
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

			// 1. Fetch Newsletters directly
			newsletters, err := t.service.List(ctx, instanceID)
			if err != nil {
				return nil, err
			}

			// Filter only Admin/Owner
			var filtered []map[string]interface{}
			for _, nl := range newsletters {
				role := strings.ToUpper(nl.Role)
				if role == "ADMIN" || role == "OWNER" {
					// Output ONLY Name and Role. NO IDs.
					clean := map[string]interface{}{
						"name": nl.Name,
						"role": nl.Role,
					}
					// Optional stats if available
					if nl.Subscribers > 0 {
						clean["subscribers"] = nl.Subscribers
					}
					filtered = append(filtered, clean)
				}
			}

			return map[string]interface{}{
				"newsletters": filtered,
				"count":       len(filtered),
			}, nil
		},
	}
}

func (t *NewsletterTools) SchedulePostTool() *domain.NativeTool {
	return &domain.NativeTool{
		Tool: domainMCP.Tool{
			Name:        "schedule_post",
			Description: "Schedules a message to be posted to a WhatsApp newsletter or group. Provide the EXACT NAME of the group or newsletter.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_name": map[string]interface{}{
						"type":        "string",
						"description": "The exact name of the group or newsletter to post to.",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The text content of the post",
					},
					"scheduled_at": map[string]interface{}{
						"type":        "string",
						"description": "ISO 8601 formatted date string for when to post (must be in future)",
					},
					"media_path": map[string]interface{}{
						"type":        "string",
						"description": "Optional absolute path to media file",
					},
				},
				"required": []string{"target_name", "text", "scheduled_at"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, ok := ctxData["instance_id"].(string)
			if !ok || instanceID == "" {
				return nil, fmt.Errorf("instance_id not available in context")
			}

			// Optional: sender_id to track who scheduled it
			senderID, _ := ctxData["sender_id"].(string)

			targetName, _ := args["target_name"].(string)
			text, _ := args["text"].(string)
			mediaPath, _ := args["media_path"].(string)
			scheduledAtStr, _ := args["scheduled_at"].(string)

			scheduledAt, err := time.Parse(time.RFC3339, scheduledAtStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date format, use ISO 8601 (RFC3339): %v", err)
			}

			// RESOLVE ID
			targetID, err := t.resolveTargetName(ctx, instanceID, targetName)
			if err != nil {
				return nil, fmt.Errorf("could not find target '%s': %v", targetName, err)
			}

			req := domainNewsletter.SchedulePostRequest{
				ChannelID:   instanceID,
				TargetID:    targetID,
				SenderID:    senderID,
				Text:        text,
				MediaPath:   mediaPath,
				ScheduledAt: scheduledAt,
			}

			post, err := t.service.SchedulePost(ctx, req)
			if err != nil {
				return nil, err
			}

			return map[string]interface{}{
				"status":       "scheduled",
				"post_id":      post.ID,
				"target_name":  targetName,
				"scheduled_at": post.ScheduledAt.String(),
			}, nil
		},
	}
}

// resolveTargetName searches for a group or newsletter matching the name
func (t *NewsletterTools) resolveTargetName(ctx context.Context, instanceID, name string) (string, error) {
	adapter, ok := t.workspaceMgr.GetAdapter(instanceID)
	if !ok {
		return "", fmt.Errorf("adapter not found")
	}

	searchName := strings.ToLower(strings.TrimSpace(name))

	// 1. Search Newsletters
	newsletters, err := t.service.List(ctx, instanceID)
	if err == nil {
		for _, nl := range newsletters {
			role := strings.ToUpper(nl.Role)
			if role != "ADMIN" && role != "OWNER" {
				continue
			}
			if strings.ToLower(strings.TrimSpace(nl.Name)) == searchName {
				return nl.ID, nil
			}
		}
	}

	// 2. Search Groups (where bot is Admin)
	groups, err := adapter.GetJoinedGroups(ctx)
	if err == nil {
		me, errMe := adapter.GetMe()
		myID := me.JID
		myLID := me.LID

		for _, g := range groups {
			gName := strings.ToLower(strings.TrimSpace(g.Name))
			if gName == searchName {
				// We need full info to verify admin status
				fullInfo, err := adapter.GetGroupInfo(ctx, g.JID)
				if err != nil {
					continue
				}

				// Verify Admin role logic
				isAdmin := false
				if errMe == nil {
					for _, p := range fullInfo.Participants {
						// Check against JID or LID
						isMe := (p.JID != "" && strings.Contains(p.JID, myID)) ||
							(myLID != "" && p.LID != "" && strings.Contains(p.LID, myLID)) ||
							(myLID != "" && strings.Contains(p.JID, myLID))

						if isMe {
							if p.IsAdmin || p.IsSuperAdmin {
								isAdmin = true
							}
							break
						}
					}
				}

				if isAdmin {
					return g.JID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no administrative channel (group or newsletter) found with name '%s'", name)
}

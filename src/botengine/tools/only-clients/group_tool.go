package onlyclients

import (
	"context"
	"fmt"
	"strings"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/sirupsen/logrus"
)

type GroupTools struct {
	workspaceMgr *workspace.Manager
}

func NewGroupTools(workspaceMgr *workspace.Manager) *GroupTools {
	return &GroupTools{workspaceMgr: workspaceMgr}
}

func (t *GroupTools) ListGroupsTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "list_my_groups_and_chats",
			Description: "Lists the available WhatsApp Groups where the bot is an admin. Use this to check available alias names for scheduling.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Limit the number of groups to return (default 20)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			instanceID, ok := ctxData["instance_id"].(string)
			if !ok || instanceID == "" {
				return nil, fmt.Errorf("instance_id not available in context")
			}

			adapter, ok := t.workspaceMgr.GetAdapter(instanceID)
			if !ok {
				return nil, fmt.Errorf("channel adapter not found")
			}

			// Use GetJoinedGroups as it's more specific for groups and usually reliable for current list
			groups, err := adapter.GetJoinedGroups(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list groups: %w", err)
			}

			// We need to filter for groups where we are admin.
			// GetJoinedGroups typically returns minimal info. We might need to fetch more info or check cache if available.
			// Ideally, we iterate and check our role.
			// NOTE: This can be slow if we have many groups.
			// Optimization: Start with basic list, maybe only fetch full info for first N or if requested.
			// BUT requirement is strict: "only list the chats where bot is administrator".

			var adminGroups []map[string]interface{}
			limit := 20
			if l, ok := args["limit"].(float64); ok && l > 0 {
				limit = int(l)
			}

			// We need our own JID (Identity) to check against participant list
			// The adapter identity usually matches the instanceID (phone number) or we can resolve it.
			// instanceID is usually the channel ID, which might be just digits. WhatsApp JID needs @s.whatsapp.net
			// Let's assume instanceID is the channel ID/Phone.
			// Best to use adapter.ResolveIdentity("me") but that might not exist.

			// Quick workaround: Usually format is userJID.
			// We'll iterate participants and check for a JID that contains instanceID

			logrus.Infof("[GroupTools] checking groups for instanceID: %s. Found %d total groups.", instanceID, len(groups))

			// Get our own identity (JID and LID)
			me, err := adapter.GetMe()
			if err != nil {
				logrus.Warnf("[GroupTools] Failed to get own identity: %v", err)
			} else {
				logrus.Infof("[GroupTools] Me: JID=%s, LID=%s", me.JID, me.LID)
			}

			if len(groups) > 0 {
				logrus.Infof("[GroupTools] Sample Group JID: %s", groups[0].JID)
			}

			for _, g := range groups {
				// We need full info to see participants and roles
				fullInfo, err := adapter.GetGroupInfo(ctx, g.JID)
				if err != nil {
					logrus.Warnf("[GroupTools] Failed to get info for group %s: %v", g.JID, err)
					continue // skip if can't get info
				}

				isAdmin := false
				for _, p := range fullInfo.Participants {
					// Check if this participant is ME (compare both JID and LID)
					isMe := false
					if me.JID != "" && strings.Contains(p.JID, me.JID) {
						isMe = true
					}
					// If strict LID check is required or available
					if !isMe && me.LID != "" && strings.Contains(p.JID, me.LID) {
						isMe = true
					}

					if isMe {
						logrus.Infof("[GroupTools] MATCH FOUND in %s! IsAdmin: %v", g.Name, p.IsAdmin || p.IsSuperAdmin)
						if p.IsAdmin || p.IsSuperAdmin {
							isAdmin = true
						} else {
							logrus.Debugf("[GroupTools] Bot found in group %s but NOT admin", g.Name)
						}
						break
					}
				}

				if isAdmin {
					adminGroups = append(adminGroups, map[string]interface{}{
						"name": g.Name,
						"type": "group",
					})
				}

				if len(adminGroups) >= limit {
					break
				}
			}

			logrus.Infof("[GroupTools] Found %d admin groups", len(adminGroups))

			return map[string]interface{}{
				"groups": adminGroups,
				"count":  len(adminGroups),
			}, nil
		},
	}
}

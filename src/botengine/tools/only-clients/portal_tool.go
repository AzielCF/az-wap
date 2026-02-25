package onlyclients

import (
	"context"
	"fmt"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	coreconfig "github.com/AzielCF/az-wap/core/config"
)

// GetPortalLinkTool allows a registered client to get their portal access link
func (t *ClientTools) GetPortalLinkTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "get_portal_access_link",
			Description: "Generates a secure, temporary magic link for the user to access their client portal. Use this ONLY when the user asks for 'access', 'portal', 'link to my account', or similar. If the user is not a registered client, it will fail.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			// 1. Get client ID from context (Already verified by IsVisible)
			clientID := ""
			phone := ""

			if cc, ok := ctxData["client_context"].(*domain.ClientContext); ok && cc != nil {
				clientID = cc.ClientID
				phone = cc.Phone
			}

			// Fallback if context is somehow missing (safety)
			if clientID == "" {
				if metadata, ok := ctxData["metadata"].(map[string]any); ok {
					if cID, ok := metadata["client_id"].(string); ok {
						clientID = cID
					}
					if ph, ok := metadata["phone"].(string); ok {
						phone = ph
					}
				}
			}

			if clientID == "" {
				return map[string]interface{}{
					"success": false,
					"message": "I couldn't identify your client profile. Access denied.",
				}, nil
			}

			// 2. Generate Magic Link via AuthService
			token, err := t.authService.GenerateMagicLink(ctx, clientID, phone)
			if err != nil {
				return nil, fmt.Errorf("failed to generate portal link: %w", err)
			}

			// 3. Build the final URL
			portalURL := coreconfig.Global.App.PortalURL
			if portalURL == "" {
				// Fallback to local server if no dedicated portal URL is set
				portalURL = fmt.Sprintf("%s/portal", coreconfig.Global.App.BaseUrl)
			}

			accessLink := fmt.Sprintf("%s/auth/redeem?token=%s", portalURL, token)

			return map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Access link generated: %s (Expires in 15m). Please send this link to the user.", accessLink),
				"link":    accessLink,
			}, nil
		},
	}
}

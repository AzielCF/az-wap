package onlyclients

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	clientsDomain "github.com/AzielCF/az-wap/clients/domain"
)

const (
	// MaxMetadataFields is the maximum number of custom fields in metadata
	MaxMetadataFields = 10
	// MaxMetadataBytes is the maximum total size of serialized metadata (~2KB)
	MaxMetadataBytes = 2048
)

// AllowedClientFields defines the fields that the client can update via AI
var AllowedClientFields = []string{
	"name",           // Client Name
	"email",          // Contact Email
	"preferences",    // General Preferences
	"interests",      // Interests/Hobbies
	"notes",          // Additional Personal Notes
	"contact_method", // Preferred Contact Method
	"birthday",       // Birthday (Optional)
	"company",        // Company/Organization (Optional)
	"role",           // Role/Job Title (Optional)
	"custom",         // Generic Custom Field
}

// ClientTools provides tools for the client to manage their information
type ClientTools struct {
	clientRepo clientsDomain.ClientRepository
}

// NewClientTools creates a new instance of ClientTools
func NewClientTools(clientRepo clientsDomain.ClientRepository) *ClientTools {
	return &ClientTools{clientRepo: clientRepo}
}

// UpdateMyInfoTool allows the client to update their personal information
func (t *ClientTools) UpdateMyInfoTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "update_my_info",
			Description: "Updates the user's personal information. Use this when the user wants to save or update their personal details like name, email, preferences, interests, or other notes. The user can add or modify multiple fields at once. Fields are stored securely in their profile.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The user's preferred name or display name. If the user tells you their name, use this field.",
					},
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Contact email address.",
					},
					"preferences": map[string]interface{}{
						"type":        "string",
						"description": "General preferences (e.g., communication style, topics of interest).",
					},
					"interests": map[string]interface{}{
						"type":        "string",
						"description": "User's hobbies, interests, or favorite things.",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "Any additional notes or information the user wants to save.",
					},
					"contact_method": map[string]interface{}{
						"type":        "string",
						"description": "Preferred contact method (e.g., 'whatsapp', 'email', 'phone').",
					},
					"birthday": map[string]interface{}{
						"type":        "string",
						"description": "User's birthday (format: YYYY-MM-DD or any format the user provides).",
					},
					"company": map[string]interface{}{
						"type":        "string",
						"description": "Company or organization name.",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "User's role or job title.",
					},
					"custom": map[string]interface{}{
						"type":        "string",
						"description": "Any other custom information the user wants to store.",
					},
				},
				"required": []string{},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			// Get client_id from context
			// Get client_id from context
			clientID := ""

			// Try explicit ClientContext first (Most reliable)
			if cc, ok := ctxData["client_context"].(*domain.ClientContext); ok && cc != nil {
				clientID = cc.ClientID
			}

			// Fallback to metadata
			if clientID == "" {
				if metadata, ok := ctxData["metadata"].(map[string]any); ok {
					if cID, ok := metadata["client_id"].(string); ok {
						clientID = cID
					}
				}
			}

			if clientID == "" {
				return map[string]interface{}{
					"success": false,
					"message": "No client profile found. You might be chatting as a guest.",
				}, nil
			}

			// Get current client
			client, err := t.clientRepo.GetByID(ctx, clientID)
			if err != nil {
				return nil, fmt.Errorf("failed to get client: %w", err)
			}

			// Initialize metadata if nil
			if client.Metadata == nil {
				client.Metadata = make(map[string]any)
			}

			// Update allowed fields
			updatedFields := []string{}
			for _, field := range AllowedClientFields {
				if value, ok := args[field]; ok && value != nil && value != "" {
					client.Metadata[field] = value
					updatedFields = append(updatedFields, field)
				}
			}

			if len(updatedFields) == 0 {
				return map[string]interface{}{
					"success": false,
					"message": "No valid fields provided to update.",
				}, nil
			}

			// Validate limits
			if len(client.Metadata) > MaxMetadataFields {
				return map[string]interface{}{
					"success": false,
					"message": fmt.Sprintf("Maximum of %d fields allowed. Please remove some before adding more.", MaxMetadataFields),
				}, nil
			}

			// Validate size
			metadataJSON, _ := json.Marshal(client.Metadata)
			if len(metadataJSON) > MaxMetadataBytes {
				return map[string]interface{}{
					"success": false,
					"message": fmt.Sprintf("Profile data exceeds maximum size (%d bytes). Please shorten some entries.", MaxMetadataBytes),
				}, nil
			}

			// Save changes
			if err := t.clientRepo.Update(ctx, client); err != nil {
				return nil, fmt.Errorf("failed to save profile: %w", err)
			}

			return map[string]interface{}{
				"success":        true,
				"message":        "Profile updated successfully.",
				"updated_fields": updatedFields,
			}, nil
		},
	}
}

// GetMyInfoTool allows the client to query their stored information
func (t *ClientTools) GetMyInfoTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "get_my_info",
			Description: "Retrieves the user's stored personal information. Use this when the user asks about their saved profile, wants to know what information you have about them, or needs to review their details.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			// Get client_id from context
			// Get client_id from context
			clientID := ""

			// Try explicit ClientContext first (Most reliable)
			if cc, ok := ctxData["client_context"].(*domain.ClientContext); ok && cc != nil {
				clientID = cc.ClientID
			}

			// Fallback to metadata
			if clientID == "" {
				if metadata, ok := ctxData["metadata"].(map[string]any); ok {
					if cID, ok := metadata["client_id"].(string); ok {
						clientID = cID
					}
				}
			}

			if clientID == "" {
				return map[string]interface{}{
					"success":     false,
					"message":     "No client profile found. You appear to be chatting as a guest.",
					"has_profile": false,
				}, nil
			}

			// Get client
			client, err := t.clientRepo.GetByID(ctx, clientID)
			if err != nil {
				return nil, fmt.Errorf("failed to get client: %w", err)
			}

			// Build response with client data
			info := map[string]interface{}{
				"success":     true,
				"has_profile": true,
			}

			// Basic profile data (immutable system info)
			if client.Language != "" {
				info["language"] = client.Language
			}

			// Client Tier
			info["tier"] = string(client.Tier)

			// Registration Date
			info["member_since"] = client.CreatedAt.Format("2006-01-02")

			// Last Interaction
			if client.LastInteraction != nil {
				info["last_interaction"] = client.LastInteraction.Format("2006-01-02 15:04")
			}

			// Custom Metadata (mutable personal info like name, email, etc)
			// Merge metadata into info for cleaner access or keep separate?
			// Request said "lo mutable solo se define y se consulta en metadata"
			// Let's copy metadata fields to root info to make it easier for AI, or keep as sub-object.
			// Current implementation puts it in "personal_data".
			// But for "name" to be visible, it must be in metadata.
			if len(client.Metadata) > 0 {
				info["personal_data"] = client.Metadata
			}

			// Tags if exist
			if len(client.Tags) > 0 {
				info["tags"] = client.Tags
			}

			return info, nil
		},
	}
}

// DeleteMyFieldTool allows the client to delete specific fields from their metadata
func (t *ClientTools) DeleteMyFieldTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "delete_my_field",
			Description: "Deletes a specific field from the user's personal profile. Use this when the user wants to remove specific information from their stored data.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type":        "string",
						"description": "The field name to delete (e.g., 'interests', 'notes', 'custom').",
						"enum":        AllowedClientFields,
					},
				},
				"required": []string{"field"},
			},
		},
		Handler: func(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			// Get client_id from context
			clientID := ""

			// Try explicit ClientContext first (Most reliable)
			if cc, ok := ctxData["client_context"].(*domain.ClientContext); ok && cc != nil {
				clientID = cc.ClientID
			}

			// Fallback to metadata
			if clientID == "" {
				if metadata, ok := ctxData["metadata"].(map[string]any); ok {
					if cID, ok := metadata["client_id"].(string); ok {
						clientID = cID
					}
				}
			}

			if clientID == "" {
				return map[string]interface{}{
					"success": false,
					"message": "No client profile found.",
				}, nil
			}

			fieldName, _ := args["field"].(string)
			if fieldName == "" {
				return map[string]interface{}{
					"success": false,
					"message": "Please specify which field to delete.",
				}, nil
			}

			// Verify that it is an allowed field
			allowed := false
			for _, f := range AllowedClientFields {
				if f == fieldName {
					allowed = true
					break
				}
			}
			if !allowed {
				return map[string]interface{}{
					"success": false,
					"message": fmt.Sprintf("Field '%s' is not a valid profile field.", fieldName),
				}, nil
			}

			client, err := t.clientRepo.GetByID(ctx, clientID)
			if err != nil {
				return nil, fmt.Errorf("failed to get client: %w", err)
			}

			if client.Metadata == nil {
				return map[string]interface{}{
					"success": false,
					"message": "No personal data stored yet.",
				}, nil
			}

			if _, exists := client.Metadata[fieldName]; !exists {
				return map[string]interface{}{
					"success": false,
					"message": fmt.Sprintf("Field '%s' is not stored in your profile.", fieldName),
				}, nil
			}

			// Update (delete) the field
			delete(client.Metadata, fieldName)

			if err := t.clientRepo.Update(ctx, client); err != nil {
				return nil, fmt.Errorf("failed to save profile: %w", err)
			}

			return map[string]interface{}{
				"success":       true,
				"message":       fmt.Sprintf("Field '%s' has been removed from your profile.", fieldName),
				"deleted_field": fieldName,
			}, nil
		},
	}
}

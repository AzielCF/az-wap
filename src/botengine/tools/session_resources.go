package tools

import (
	"context"
	"fmt"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// NativeTool definition duplicated here if needed or use a generic one?
// To avoid circular dependency, we just return the components to build the NativeTool in BotEngine
// or we use a generic structure.
// Let's use a struct that doesn't depend on botengine.

type ToolDefinition struct {
	domainMCP.Tool
	Handler func(ctx context.Context, context map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error)
}

// NewSessionResourcesTool crea la herramienta para listar archivos de la sesión
func NewSessionResourcesTool() ToolDefinition {
	return ToolDefinition{
		Tool: domainMCP.Tool{
			Name:        "get_session_resources",
			Description: "Retrieves the list of files (images, audio, documents, videos) available in the current chat session. If you already know the file name (friendly_name), you can provide it to get its specific details directly.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Optional file name to search for (e.g., 'invoice.pdf').",
					},
				},
			},
		},
		Handler: func(ctx context.Context, contextData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			metadata, ok := contextData["metadata"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("metadata missing in context")
			}

			resources, ok := metadata["session_resources"]
			if !ok {
				return map[string]interface{}{
					"resources": []interface{}{},
					"count":     0,
					"message":   "No resources were found in the current session.",
				}, nil
			}

			var resList []map[string]string
			switch v := resources.(type) {
			case []map[string]string:
				resList = v
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]string); ok {
						resList = append(resList, m)
					} else if m, ok := item.(map[string]interface{}); ok {
						nm := make(map[string]string)
						for k, val := range m {
							nm[k] = fmt.Sprintf("%v", val)
						}
						resList = append(resList, nm)
					}
				}
			default:
				return nil, fmt.Errorf("invalid resources format in metadata: %T", resources)
			}

			// Filter by name if provided
			searchName, _ := args["name"].(string)
			if searchName != "" {
				for _, r := range resList {
					if r["name"] == searchName {
						return map[string]interface{}{
							"resource": r,
							"found":    true,
						}, nil
					}
				}
				return map[string]interface{}{
					"message": fmt.Sprintf("No file found with name '%s'", searchName),
					"found":   false,
				}, nil
			}

			return map[string]interface{}{
				"resources": resList,
				"count":     len(resList),
			}, nil
		},
	}
}

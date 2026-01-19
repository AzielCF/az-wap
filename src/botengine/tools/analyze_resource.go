package tools

import (
	"context"
	"fmt"
	"os"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// NewAnalyzeSessionResourceTool crea la herramienta para analizar un archivo de la sesión
func NewAnalyzeSessionResourceTool() ToolDefinition {
	return ToolDefinition{
		Tool: domainMCP.Tool{
			Name:        "analyze_session_resource",
			Description: "Analyzes the content of a resource (image, audio, document, video) from the current session using multimodal capabilities. NOTE: You only need the file's 'name' (friendly_name).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Friendly name of the file to analyze (e.g., 'invoice.pdf').",
					},
					"intent": map[string]interface{}{
						"type":        "string",
						"description": "What you want to know about the file (e.g., 'summary', 'transcription', 'description').",
					},
				},
				"required": []string{"name"},
			},
		},
		Handler: func(ctx context.Context, contextData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			metadata, ok := contextData["metadata"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("metadata missing in context")
			}

			resources, ok := metadata["session_resources"]
			if !ok {
				return map[string]interface{}{"error": "No resources available in this session"}, nil
			}

			searchName, _ := args["name"].(string)
			if searchName == "" {
				return map[string]interface{}{"error": "File name is required"}, nil
			}

			// Try to find the resource by name
			var targetResource map[string]string
			switch v := resources.(type) {
			case []map[string]string:
				for _, r := range v {
					if r["name"] == searchName {
						targetResource = r
						break
					}
				}
			case []interface{}:
				for _, item := range v {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					if fmt.Sprintf("%v", m["name"]) == searchName {
						targetResource = make(map[string]string)
						for k, val := range m {
							targetResource[k] = fmt.Sprintf("%v", val)
						}
						break
					}
				}
			}

			if targetResource == nil {
				return map[string]interface{}{"error": fmt.Sprintf("Resource '%s' not found", searchName)}, nil
			}

			path := targetResource["path"]
			if path == "" {
				return map[string]interface{}{"error": "File path not available for this resource"}, nil
			}

			// Check if file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return map[string]interface{}{"error": "The physical file is no longer available on the server (it likely expired)"}, nil
			}

			// SIGNAL FOR THE PROVIDER:
			return map[string]interface{}{
				"action":    "trigger_multimodal_analysis",
				"path":      path,
				"mime_type": targetResource["mime"],
				"filename":  targetResource["name"],
				"intent":    args["intent"],
			}, nil
		},
	}
}

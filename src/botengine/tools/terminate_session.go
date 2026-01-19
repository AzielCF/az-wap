package tools

import (
	"context"

	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// NewTerminateSessionTool crea la herramienta para terminar la sesión actual y limpiar la memoria
func NewTerminateSessionTool() ToolDefinition {
	return ToolDefinition{
		Tool: domainMCP.Tool{
			Name:        "terminate_session",
			Description: "Ends the current chat session immediately. IMPORTANT: Once called, the session is deleted. You MUST provide a natural, human-like goodbye message in the 'farewell_message' parameter so the user receives a proper response before the session ends.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"farewell_message": map[string]interface{}{
						"type":        "string",
						"description": "Your final natural language response to the user. Strictly follow your SYSTEM PROMPT persona.",
					},
				},
			},
		},
		Handler: func(ctx context.Context, contextData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
			farewell, _ := args["farewell_message"].(string)
			return map[string]interface{}{
				"action":           "terminate_session",
				"farewell_message": farewell,
				"status":           "ok",
			}, nil
		},
	}
}

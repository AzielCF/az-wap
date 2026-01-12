package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/botengine"
	"github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"google.golang.org/genai"
)

type GeminiProvider struct {
	mcpUsecase  domainMCP.IMCPUsecase
	memoryStore *botengine.MemoryStore
}

func NewGeminiProvider(mcpService domainMCP.IMCPUsecase, memory *botengine.MemoryStore) *GeminiProvider {
	return &GeminiProvider{
		mcpUsecase:  mcpService,
		memoryStore: memory,
	}
}

type audioResponse struct {
	Transcription string `json:"transcription"`
}

type imageResponse struct {
	Description string `json:"description"`
}

func (p *GeminiProvider) GenerateReply(ctx context.Context, b bot.Bot, input botengine.BotInput, tools []domainMCP.Tool) (botengine.BotOutput, error) {
	if b.APIKey == "" {
		return botengine.BotOutput{}, fmt.Errorf("bot %s has no API key", b.ID)
	}
	if b.Model == "" {
		b.Model = "gemini-flash-latest"
	}

	memoryKey := ""
	if b.MemoryEnabled {
		if input.WorkspaceID != "" {
			memoryKey = fmt.Sprintf("ws|%s|bot|%s|%s", input.WorkspaceID, b.ID, input.SenderID)
		} else {
			// Fallback for legacy/global memory
			memoryKey = fmt.Sprintf("bot|%s|%s", b.ID, input.SenderID)
		}
	}

	traceID := input.TraceID
	if traceID == "" {
		traceID = fmt.Sprintf("engine:%s:%d", b.ID, time.Now().UnixNano())
	}

	if input.Media != nil {
		if strings.Contains(input.Media.MimeType, "audio") {
			reply, err := p.generateReplyFromAudio(ctx, b, memoryKey, input.Media.Data, input.Media.MimeType, input.Text, traceID, input.InstanceID, input.ChatID)
			return botengine.BotOutput{Text: reply}, err
		}
		if strings.Contains(input.Media.MimeType, "image") {
			reply, err := p.generateReplyFromImage(ctx, b, memoryKey, input.Media.Data, input.Media.MimeType, input.Text, traceID, input.InstanceID, input.ChatID)
			return botengine.BotOutput{Text: reply}, err
		}
	}

	reply, err := p.generateReply(ctx, b, memoryKey, input.Text, traceID, input.InstanceID, input.ChatID, tools)
	return botengine.BotOutput{Text: reply}, err
}

func (p *GeminiProvider) generateReply(ctx context.Context, b bot.Bot, memoryKey string, input string, traceID, instanceID, chatJID string, mcpTools []domainMCP.Tool) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  b.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	var genConfig *genai.GenerateContentConfig
	systemText := p.buildSystemInstructions(b)

	serverMap := make(map[string]string)
	var mcpInstructions strings.Builder
	if b.ID != "" && p.mcpUsecase != nil {
		if servers, err := p.mcpUsecase.ListServersForBot(ctx, b.ID); err == nil {
			for _, srv := range servers {
				if srv.Enabled {
					if srv.Instructions != "" || srv.BotInstructions != "" {
						mcpInstructions.WriteString(fmt.Sprintf("\n\n### TOOLSET: %s", srv.Name))
						if srv.Instructions != "" {
							mcpInstructions.WriteString(fmt.Sprintf("\nGeneral Purpose: %s", srv.Instructions))
						}
						if srv.BotInstructions != "" {
							mcpInstructions.WriteString(fmt.Sprintf("\nGuidelines: %s", srv.BotInstructions))
						}
					}
					for _, t := range srv.Tools {
						serverMap[t.Name] = srv.ID
					}
				}
			}
		}
	}

	if mcpInstructions.Len() > 0 {
		systemText += "\n\n## MCP TOOL GUIDELINES" + mcpInstructions.String()
	}

	if systemText != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemText, genai.RoleUser),
		}
	}

	var functionDecls []*genai.FunctionDeclaration
	if b.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		functionDecls = append(functionDecls, &genai.FunctionDeclaration{
			Name:        "close_chat",
			Description: "Ends the conversation.",
			Parameters: &genai.Schema{
				Type: "object",
				Properties: map[string]*genai.Schema{
					"farewell_message": {Type: "string"},
				},
				Required: []string{"farewell_message"},
			},
		})
	}

	for _, t := range mcpTools {
		functionDecls = append(functionDecls, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  p.convertMCPSchemaToGemini(t.InputSchema),
		})
	}

	if len(functionDecls) > 0 {
		if genConfig == nil {
			genConfig = &genai.GenerateContentConfig{}
		}
		genConfig.Tools = []*genai.Tool{{FunctionDeclarations: functionDecls}}
	}

	var contents []*genai.Content
	if b.MemoryEnabled && strings.TrimSpace(memoryKey) != "" && p.memoryStore != nil {
		history := p.memoryStore.Get(memoryKey)
		// Añadir input actual al historial temporalmente (ya que se guardará al final)
		history = append(history, botengine.ChatTurn{Role: "user", Text: input})

		// Guardar el input del usuario en el store real
		p.memoryStore.Save(memoryKey, botengine.ChatTurn{Role: "user", Text: input}, 10)

		for _, t := range history {
			role := genai.RoleUser
			if t.Role == "assistant" {
				role = genai.RoleModel
			}
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: []*genai.Part{{Text: t.Text}},
			})
		}
	} else {
		contents = []*genai.Content{{Role: genai.RoleUser, Parts: []*genai.Part{{Text: input}}}}
	}

	closed := false
	var farewellMsg string
	var finalResponse string

	for i := 0; i < 10; i++ {
		botmonitor.Record(botmonitor.Event{
			TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
			Provider: string(b.Provider), Stage: "ai_request", Status: "ok",
			Metadata: map[string]string{
				"trace_id":            traceID,
				"system_instructions": systemText,
				"input":               input,
			},
		})

		start := time.Now()
		result, err := p.generateContentWithRetry(ctx, client, b.Model, contents, genConfig)
		duration := time.Since(start).Milliseconds()

		if err != nil {
			botmonitor.Record(botmonitor.Event{
				TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
				Provider: string(b.Provider), Stage: "ai_response", Status: "error", Error: err.Error(),
				DurationMs: duration,
			})
			return "", err
		}

		if result != nil && len(result.Candidates) > 0 {
			botmonitor.Record(botmonitor.Event{
				TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
				Provider: string(b.Provider), Stage: "ai_response", Status: "ok",
				DurationMs: duration,
				Metadata: map[string]string{
					"trace_id": traceID,
					"response": result.Text(),
				},
			})
		}
		if result == nil || len(result.Candidates) == 0 {
			break
		}

		candidate := result.Candidates[0]
		contents = append(contents, candidate.Content)

		hasToolCall := false
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				hasToolCall = true
				toolName := part.FunctionCall.Name
				args := part.FunctionCall.Args

				var toolResult map[string]any
				if toolName == "close_chat" {
					if fw, ok := args["farewell_message"].(string); ok {
						farewellMsg = strings.TrimSpace(fw)
					}
					if p.memoryStore != nil {
						p.memoryStore.Clear(memoryKey)
					}
					closed = true
					toolResult = map[string]any{"status": "ok"}
				} else if serverID, ok := serverMap[toolName]; ok {
					startCall := time.Now()
					mcpRes, mErr := p.mcpUsecase.CallTool(ctx, b.ID, domainMCP.CallToolRequest{
						ServerID:  serverID,
						ToolName:  toolName,
						Arguments: args,
					})
					duration := time.Since(startCall).Milliseconds()
					if mErr != nil {
						toolResult = map[string]any{"error": mErr.Error()}
					} else {
						toolResult = map[string]any{"content": mcpRes.Content, "is_error": mcpRes.IsError}
					}
					argsJS, _ := json.Marshal(args)
					resJS, _ := json.Marshal(toolResult)

					botmonitor.Record(botmonitor.Event{
						TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
						Provider: "mcp", Stage: "mcp_call", Kind: toolName, Status: "ok", DurationMs: duration,
						Metadata: map[string]string{
							"trace_id": traceID,
							"request":  string(argsJS),
							"response": string(resJS),
						},
					})
				} else {
					toolResult = map[string]any{"error": "tool not found"}
				}

				contents = append(contents, &genai.Content{
					Role: "user",
					Parts: []*genai.Part{{
						FunctionResponse: &genai.FunctionResponse{
							Name:     toolName,
							Response: toolResult,
						},
					}},
				})
			}
		}

		if !hasToolCall {
			finalResponse = result.Text()
			break
		}
	}

	if closed && farewellMsg != "" {
		finalResponse = farewellMsg
	}

	if finalResponse != "" && b.MemoryEnabled && strings.TrimSpace(memoryKey) != "" && !closed && p.memoryStore != nil {
		p.memoryStore.Save(memoryKey, botengine.ChatTurn{Role: "assistant", Text: finalResponse}, 10)
	}

	return finalResponse, nil
}

func (p *GeminiProvider) buildSystemInstructions(b bot.Bot) string {
	var sb strings.Builder
	if config.GeminiGlobalSystemPrompt != "" {
		sb.WriteString(config.GeminiGlobalSystemPrompt)
		sb.WriteString("\n\n")
	}
	if b.SystemPrompt != "" {
		sb.WriteString(b.SystemPrompt)
		sb.WriteString("\n\n")
	}
	if b.KnowledgeBase != "" {
		sb.WriteString(b.KnowledgeBase)
		sb.WriteString("\n\n")
	}
	tz := b.Timezone
	if tz == "" {
		tz = config.GeminiTimezone
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	sb.WriteString(fmt.Sprintf("IMPORTANT - Current date and time (%s): %s", tz, now.Format(time.RFC3339)))
	return sb.String()
}

func (p *GeminiProvider) generateReplyFromAudio(ctx context.Context, b bot.Bot, memoryKey string, audioBytes []byte, mimeType string, caption string, traceID, instanceID, chatJID string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  b.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	prompt := `Listen to this voice message and transcribe literally. Return JSON with "transcription" field.`
	if caption != "" {
		prompt += fmt.Sprintf("\n\nContext: %s", caption)
	}

	contents := []*genai.Content{{
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			{Text: prompt},
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: audioBytes}},
		},
	}}

	genCfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"transcription": {Type: "string"},
			},
			Required: []string{"transcription"},
		},
	}

	result, err := p.generateContentWithRetry(ctx, client, b.Model, contents, genCfg)
	if err != nil {
		return "", err
	}

	var audioResp audioResponse
	json.Unmarshal([]byte(result.Text()), &audioResp)

	fullInput := audioResp.Transcription
	if caption != "" {
		fullInput += "\n\n[Context: " + caption + "]"
	}

	return p.generateReply(ctx, b, memoryKey, fullInput, traceID, instanceID, chatJID, nil)
}

func (p *GeminiProvider) generateReplyFromImage(ctx context.Context, b bot.Bot, memoryKey string, imageBytes []byte, mimeType string, caption string, traceID, instanceID, chatJID string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  b.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	prompt := `Look at this image and describe exactly what you see. Return JSON with "description" field.`
	if caption != "" {
		prompt += fmt.Sprintf("\n\nContext: %s", caption)
	}

	contents := []*genai.Content{{
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			{Text: prompt},
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageBytes}},
		},
	}}

	genCfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"description": {Type: "string"},
			},
			Required: []string{"description"},
		},
	}

	result, err := p.generateContentWithRetry(ctx, client, b.Model, contents, genCfg)
	if err != nil {
		return "", err
	}

	var imgResp imageResponse
	json.Unmarshal([]byte(result.Text()), &imgResp)

	fullInput := "Image context: " + imgResp.Description
	if caption != "" {
		fullInput += "\n\n[Text: " + caption + "]"
	}

	return p.generateReply(ctx, b, memoryKey, fullInput, traceID, instanceID, chatJID, nil)
}

func (p *GeminiProvider) convertMCPSchemaToGemini(input interface{}) *genai.Schema {
	data, _ := json.Marshal(input)
	var schema genai.Schema
	json.Unmarshal(data, &schema)
	if schema.Type == "" {
		schema.Type = "object"
	}
	return &schema
}

func (p *GeminiProvider) generateContentWithRetry(ctx context.Context, client *genai.Client, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	for i := 0; i < 3; i++ {
		result, err := client.Models.GenerateContent(ctx, model, contents, cfg)
		if err == nil {
			return result, nil
		}
		if strings.Contains(err.Error(), "503") {
			time.Sleep(time.Duration(1<<uint(i)) * time.Second)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("max retries exceeded")
}

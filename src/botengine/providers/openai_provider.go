package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/sirupsen/logrus"
)

// OpenAIProvider is the adapter for the OpenAI API
type OpenAIProvider struct {
	mcpUsecase domainMCP.IMCPUsecase
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(mcpService domainMCP.IMCPUsecase) *OpenAIProvider {
	return &OpenAIProvider{
		mcpUsecase: mcpService,
	}
}

// Chat implements the AIProvider interface for OpenAI
func (p *OpenAIProvider) Chat(ctx context.Context, b domainBot.Bot, req domain.ChatRequest) (domain.ChatResponse, error) {
	if b.APIKey == "" {
		return domain.ChatResponse{}, fmt.Errorf("bot %s has no API key", b.ID)
	}

	client := openai.NewClient(
		option.WithAPIKey(b.APIKey),
	)

	model := req.Model
	if model == "" {
		model = domainBot.DefaultOpenAIModel
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
	}

	// System Prompt
	var messages []openai.ChatCompletionMessageParamUnion
	if req.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(req.SystemPrompt))
	}

	// History
	for _, t := range req.History {
		// Parity check: If we have RawContent, use it if it matches our expected type
		if t.RawContent != nil {
			if msg, ok := t.RawContent.(openai.ChatCompletionMessageParamUnion); ok {
				messages = append(messages, msg)
				continue
			}
		}

		// Tool Calls from Assistant
		if len(t.ToolCalls) > 0 {
			var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
			for _, tc := range t.ToolCalls {
				argsData, _ := json.Marshal(tc.Args)
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(argsData),
						},
						Type: "function",
					},
				})
			}
			msg := openai.ChatCompletionAssistantMessageParam{
				ToolCalls: toolCalls,
			}
			if t.Text != "" {
				msg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(t.Text),
				}
			}
			messages = append(messages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &msg,
			})
			continue
		}

		// Tool Responses from User/System (Tool role)
		if len(t.ToolResponses) > 0 {
			for _, tr := range t.ToolResponses {
				data, _ := json.Marshal(tr.Data)
				messages = append(messages, openai.ToolMessage(string(data), tr.ID))
			}
			continue
		}

		// Normal Messages
		if t.Role == "assistant" {
			messages = append(messages, openai.AssistantMessage(t.Text))
		} else {
			messages = append(messages, openai.UserMessage(t.Text))
		}
	}

	// Current User Text
	if req.UserText != "" {
		messages = append(messages, openai.UserMessage(req.UserText))
	}

	params.Messages = messages

	// Tools
	var tools []openai.ChatCompletionToolUnionParam
	for _, t := range req.Tools {
		tools = append(tools, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        t.Name,
					Description: openai.String(t.Description),
					Parameters:  openai.FunctionParameters(t.InputSchema.(map[string]any)),
				},
			},
		})
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	// Execute call
	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return domain.ChatResponse{}, err
	}

	if len(completion.Choices) == 0 {
		return domain.ChatResponse{}, fmt.Errorf("no response from openai")
	}

	choice := completion.Choices[0]
	resp := domain.ChatResponse{
		Text:       choice.Message.Content,
		RawContent: choice.Message.ToParam(),
	}

	// Extract Tool Calls
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		resp.ToolCalls = append(resp.ToolCalls, domain.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	// Usage stats
	resp.Usage = p.extractUsage(model, completion.Usage)

	logrus.WithFields(logrus.Fields{
		"chat_key":       req.ChatKey,
		"model":          model,
		"input_tokens":   resp.Usage.InputTokens,
		"output_tokens":  resp.Usage.OutputTokens,
		"cost_usd":       fmt.Sprintf("$%.6f", resp.Usage.CostUSD),
		"has_tool_calls": len(resp.ToolCalls) > 0,
	}).Debug("[OPENAI] Chat completed")

	return resp, nil
}

// Interpret implements the MultimodalInterpreter interface for OpenAI
func (p *OpenAIProvider) Interpret(ctx context.Context, apiKey string, model string, userText string, language string, medias []*domain.BotMedia) (*domain.MultimodalResult, *domain.UsageStats, error) {
	if apiKey == "" {
		return nil, nil, fmt.Errorf("multimodal interpretation requires an API key")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	if model == "" {
		model = domainBot.DefaultOpenAIModel
	}

	var contentParts []openai.ChatCompletionContentPartUnionParam
	contentParts = append(contentParts, openai.TextContentPart(fmt.Sprintf(`Analyze the following media files. User message: "%s"
Primary language for descriptions: %s.
Return result in the specified JSON format.`, userText, language)))

	for _, m := range medias {
		if strings.HasPrefix(m.MimeType, "image/") {
			dataURL := fmt.Sprintf("data:%s;base64,%s", m.MimeType, base64.StdEncoding.EncodeToString(m.Data))
			contentParts = append(contentParts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
				URL: dataURL,
			}))
		}
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"transcriptions":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"descriptions":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"summaries":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"video_summaries": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required":             []string{"transcriptions", "descriptions", "summaries", "video_summaries"},
		"additionalProperties": false,
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(contentParts),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "interpretation_result",
					Schema: any(schema),
					Strict: openai.Bool(true),
				},
			},
		},
	}

	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	var result struct {
		Transcriptions []string `json:"transcriptions"`
		Descriptions   []string `json:"descriptions"`
		Summaries      []string `json:"summaries"`
		VideoSummaries []string `json:"video_summaries"`
	}

	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &result); err != nil {
		return nil, nil, err
	}

	usage := p.extractUsage(model, completion.Usage)

	return &domain.MultimodalResult{
		Transcriptions: result.Transcriptions,
		Descriptions:   result.Descriptions,
		Summaries:      result.Summaries,
		VideoSummaries: result.VideoSummaries,
	}, usage, nil
}

// PreAnalyzeMindset analyzes the sentiment and effort required
func (p *OpenAIProvider) PreAnalyzeMindset(ctx context.Context, b domainBot.Bot, input domain.BotInput, history []domain.ChatTurn) (*domain.Mindset, *domain.UsageStats, error) {
	if b.APIKey == "" {
		return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
	}

	client := openai.NewClient(option.WithAPIKey(b.APIKey))

	model := b.MindsetModel
	if model == "" {
		model = domainBot.DefaultOpenAIMiniModel
	}

	var histStr strings.Builder
	for _, h := range history {
		histStr.WriteString(fmt.Sprintf("%s: %s\n", h.Role, h.Text))
	}

	var agendaStr strings.Builder
	if len(input.PendingTasks) > 0 {
		agendaStr.WriteString("CURRENT BOT AGENDA:\n")
		for _, t := range input.PendingTasks {
			agendaStr.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}

	langCtx := ""
	if input.Language != "" {
		langCtx = fmt.Sprintf("\n- PRIMARY LANGUAGE: %s. Use ONLY this language for the acknowledgement.", input.Language)
	}

	prompt := fmt.Sprintf(`Analyze the user message and emotional context.
User message: "%s"

CONTEXT:
- Recent History:
%s
- Bot Agenda: %s
%s

Return JSON with mindset details.`, input.Text, histStr.String(), agendaStr.String(), langCtx)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pace":            map[string]any{"type": "string", "enum": []string{"fast", "steady", "deep"}},
			"focus":           map[string]any{"type": "boolean"},
			"work":            map[string]any{"type": "boolean"},
			"acknowledgement": map[string]any{"type": "string"},
			"should_respond":  map[string]any{"type": "boolean"},
			"enqueue_task":    map[string]any{"type": "string"},
			"clear_tasks":     map[string]any{"type": "boolean"},
		},
		"required":             []string{"pace", "focus", "work", "acknowledgement", "should_respond", "enqueue_task", "clear_tasks"},
		"additionalProperties": false,
	}

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "mindset_analysis",
					Schema: any(schema),
					Strict: openai.Bool(true),
				},
			},
		},
	}

	completion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		logrus.WithError(err).Warn("[OPENAI] Intuition phase failed, using fallback")
		return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
	}

	var mindset domain.Mindset
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &mindset); err != nil {
		return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
	}

	usage := p.extractUsage(model, completion.Usage)
	return &mindset, usage, nil
}

func (p *OpenAIProvider) extractUsage(model string, usage openai.CompletionUsage) *domain.UsageStats {
	inputTokens := int(usage.PromptTokens)
	outputTokens := int(usage.CompletionTokens)
	cachedTokens := int(usage.PromptTokensDetails.CachedTokens)

	// Calculate cost
	costUSD := p.calculateCost(model, inputTokens, outputTokens)

	return &domain.UsageStats{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
		CostUSD:      costUSD,
	}
}

func (p *OpenAIProvider) calculateCost(model string, input, output int) float64 {
	pricing, ok := domainBot.OpenAIModelPrices[model]
	if !ok {
		pricing = domainBot.OpenAIModelPrices[domainBot.DefaultOpenAIModel]
	}

	inputCost := float64(input) * pricing.InputPerMToken / 1_000_000
	outputCost := float64(output) * pricing.OutputPerMToken / 1_000_000

	return inputCost + outputCost
}

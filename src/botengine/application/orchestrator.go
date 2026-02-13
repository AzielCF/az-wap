package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/sirupsen/logrus"
)

// Orchestrator maneja el ciclo de vida de una conversación con herramientas
type Orchestrator struct {
	mcpUsecase       domainMCP.IMCPUsecase
	nativeToolCaller func(ctx context.Context, name string, input domain.BotInput, args map[string]interface{}) (map[string]interface{}, error)
}

func NewOrchestrator(mcp domainMCP.IMCPUsecase, nativeCaller func(context.Context, string, domain.BotInput, map[string]interface{}) (map[string]interface{}, error)) *Orchestrator {
	return &Orchestrator{
		mcpUsecase:       mcp,
		nativeToolCaller: nativeCaller,
	}
}

// Execute realiza el bucle de razonamiento de l IA hasta obtener una respuesta de texto
func (o *Orchestrator) Execute(ctx context.Context, p domain.AIProvider, b domainBot.Bot, input domain.BotInput, req domain.ChatRequest, serverMap map[string]string) (domain.BotOutput, error) {
	traceID := input.TraceID
	instanceID := input.InstanceID
	chatJID := input.ChatID
	originalUserText := input.Text

	var finalAction string
	var farewellMsg string
	var finalResponse string
	var lastAIText string
	var totalCost float64 // Acumulador de costos de todas las iteraciones
	var costDetails []domain.ExecutionCost

	addCost := func(botID, model string, cost float64) {
		if cost <= 0 {
			return
		}
		totalCost += cost
		for i, d := range costDetails {
			if d.BotID == botID && d.Model == model {
				costDetails[i].Cost += cost
				return
			}
		}
		costDetails = append(costDetails, domain.ExecutionCost{BotID: botID, Model: model, Cost: cost})
	}

	// REDACTION LOGIC: Check if client is a tester
	isTester := false
	if input.ClientContext != nil {
		isTester = input.ClientContext.IsTester
	}

	// Helper for redaction
	redactIfNeeded := func(text string) string {
		if isTester {
			return text
		}
		return "[REDACTED]"
	}

	// Helper for tool args redaction
	redactArgsIfNeeded := func(toolName string, args map[string]interface{}, isNative bool) string {
		if isTester {
			js, _ := json.Marshal(args)
			return string(js)
		}
		// Operational args allowed ONLY for native tools
		if isNative {
			allowedProps := []string{"time", "date", "duration", "quantity", "status"}
			cleanArgs := make(map[string]interface{})
			for k, v := range args {
				for _, allowed := range allowedProps {
					if strings.Contains(strings.ToLower(k), allowed) {
						cleanArgs[k] = v
						break
					}
				}
			}
			if len(cleanArgs) > 0 {
				cleanArgs["_redacted"] = "Sensitive content hidden"
				js, _ := json.Marshal(cleanArgs)
				return string(js)
			}
		}
		return `{"_redacted": "Content hidden for privacy"}`
	}

	// Preparar historial para evitar repetición de UserText en el bucle (Identidad Paridad)
	if req.UserText != "" {
		req.History = append(req.History, domain.ChatTurn{
			Role: "user",
			Text: req.UserText,
		})
		req.UserText = ""
	}

	// Bucle de herramientas (máximo 10 iteraciones)
	for i := 0; i < 10; i++ {
		botmonitor.Record(botmonitor.Event{
			TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
			Provider: string(b.Provider), Stage: "ai_request", Status: "ok",
			Kind: fmt.Sprintf("step_%d", i+1),
			Metadata: map[string]string{
				"trace_id":            traceID,
				"iteration":           fmt.Sprintf("%d", i+1),
				"system_instructions": redactIfNeeded(req.SystemPrompt),
				"input":               redactIfNeeded(originalUserText),
			},
		})

		start := time.Now()
		res, err := p.Chat(ctx, b, req)
		duration := time.Since(start).Milliseconds()

		if err == nil {
			lastAIText = res.Text
		}

		if err != nil {
			botmonitor.Record(botmonitor.Event{
				TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
				Provider: string(b.Provider), Stage: "ai_reply", Status: "error", Error: err.Error(),
				DurationMs: duration,
			})
			return domain.BotOutput{}, err
		}

		// Identidad Original: Extraer multimodal_content para el log si existe
		md := map[string]string{
			"trace_id": traceID,
			"response": redactIfNeeded(res.Text),
		}
		if idx := strings.Index(res.Text, "]: "); idx != -1 && idx < 20 {
			md["multimodal_content"] = res.Text
		}

		// Acumular costo de esta iteración y registrar en monitor si existe usage
		if res.Usage != nil {
			addCost(b.ID, res.Usage.Model, res.Usage.CostUSD)
			md["model"] = res.Usage.Model
			md["usage_cost"] = fmt.Sprintf("$%.6f", res.Usage.CostUSD)
			md["usage_input_tokens"] = fmt.Sprintf("%d", res.Usage.InputTokens)
			md["usage_output_tokens"] = fmt.Sprintf("%d", res.Usage.OutputTokens)
			if res.Usage.SystemTokens > 0 {
				md["usage_system_tokens"] = fmt.Sprintf("%d", res.Usage.SystemTokens)
			}
			if res.Usage.UserTokens > 0 {
				md["usage_user_tokens"] = fmt.Sprintf("%d", res.Usage.UserTokens)
			}
			if res.Usage.HistoryTokens > 0 {
				md["usage_history_tokens"] = fmt.Sprintf("%d", res.Usage.HistoryTokens)
			}
			if res.Usage.CachedTokens > 0 {
				md["usage_cached_tokens"] = fmt.Sprintf("%d", res.Usage.CachedTokens)
			}
		}

		botmonitor.Record(botmonitor.Event{
			TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
			Provider: string(b.Provider), Stage: "ai_reply", Status: "ok",
			DurationMs: duration,
			Metadata:   md,
		})

		// Si no hay llamadas a herramientas, la respuesta de texto es la final
		if len(res.ToolCalls) == 0 {
			finalResponse = res.Text
			break
		}

		// Turno asistente que contiene las llamadas a herramientas
		// PARIDAD ORIGINAL: Preservar RawContent para re-inyección directa
		req.History = append(req.History, domain.ChatTurn{
			Role:       "assistant",
			Text:       res.Text,
			ToolCalls:  res.ToolCalls,
			RawContent: res.RawContent,
		})

		// PARIDAD ULTRA: Agrupar TODAS las respuestas de herramientas en un solo turno (Identidad Gemini)
		var responses []domain.ToolResponse
		shouldBreak := false
		for _, tc := range res.ToolCalls {
			var toolResult map[string]any

			// 1. Intentar MCP
			if serverID, ok := serverMap[tc.Name]; ok && o.mcpUsecase != nil {
				startCall := time.Now()
				mcpRes, mErr := o.mcpUsecase.CallTool(ctx, b.ID, domainMCP.CallToolRequest{
					ServerID:  serverID,
					ToolName:  tc.Name,
					Arguments: tc.Args,
				})
				duration := time.Since(startCall).Milliseconds()
				if mErr != nil {
					toolResult = map[string]any{"error": mErr.Error()}
				} else {
					toolResult = map[string]any{"content": mcpRes.Content, "is_error": mcpRes.IsError}
				}

				botmonitor.Record(botmonitor.Event{
					TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
					Provider: "mcp", Stage: "mcp_call", Kind: tc.Name, Status: "ok", DurationMs: duration,
					Metadata: map[string]string{
						"trace_id": traceID,
						"request":  redactArgsIfNeeded(tc.Name, tc.Args, false),    // MCP: Not Native
						"response": redactArgsIfNeeded(tc.Name, toolResult, false), // Redact response too
					},
				})
			} else if o.nativeToolCaller != nil {
				// 2. Try Native Tool
				logrus.Infof("[GEMINI] Executing native tool: %s", tc.Name)

				// Inject bot (Channel) time context for tools
				toolInput := input
				if toolInput.Metadata == nil {
					toolInput.Metadata = make(map[string]interface{})
				}
				// Clone metadata to ensure thread safety and avoid side effects
				toolMetadata := make(map[string]interface{})
				for k, v := range toolInput.Metadata {
					toolMetadata[k] = v
				}

				// Timezone resolution: Client (if registered) > Channel > UTC
				tz := ""
				if input.ClientContext != nil && input.ClientContext.IsRegistered && input.ClientContext.Timezone != "" {
					tz = input.ClientContext.Timezone
				} else if channelTZ, ok := toolMetadata["channel_timezone"].(string); ok && channelTZ != "" {
					tz = channelTZ
				}
				if tz == "" {
					tz = "UTC"
				}
				toolMetadata["bot_timezone"] = tz

				// Inject client country for regional context (currency, time format, etc.)
				if input.ClientContext != nil && input.ClientContext.Country != "" {
					toolMetadata["client_country"] = input.ClientContext.Country
				}

				toolInput.Metadata = toolMetadata

				startCall := time.Now()
				nRes, nErr := o.nativeToolCaller(ctx, tc.Name, toolInput, tc.Args)
				duration := time.Since(startCall).Milliseconds()

				if nErr != nil {
					logrus.Errorf("[GEMINI] Native tool %s error: %v", tc.Name, nErr)
					toolResult = map[string]any{"error": nErr.Error()}
				} else {
					logrus.Infof("[GEMINI] Native tool %s success", tc.Name)

					// LOGICA ESPECIAL: Si la tool nativa pide un análisis multimodal dinámico
					if action, ok := nRes["action"].(string); ok && action == "trigger_multimodal_analysis" {
						logrus.Infof("[GEMINI] Triggering dynamic multimodal analysis for %s", tc.Name)
						path, _ := nRes["path"].(string)
						mime, _ := nRes["mime_type"].(string)
						fname, _ := nRes["filename"].(string)
						intent, _ := nRes["intent"].(string)

						data, err := os.ReadFile(path)
						if err != nil {
							// Paridad exacta en el mensaje de error de lectura
							toolResult = map[string]any{"error": fmt.Sprintf("could not read file at %s: %v", path, err)}
						} else if multimodal, ok := p.(domain.MultimodalInterpreter); ok {
							media := &domain.BotMedia{Data: data, MimeType: mime, FileName: fname}
							interp, usageInt, err := multimodal.Interpret(ctx, b.APIKey, b.Model, intent, input.Language, []*domain.BotMedia{media})
							if err == nil && usageInt != nil {
								addCost(b.ID, usageInt.Model, usageInt.CostUSD)
							}
							if err != nil {
								toolResult = map[string]any{"error": fmt.Sprintf("multimodal analysis error: %v", err)}
							} else {
								toolResult = map[string]any{
									"analysis": interp,
									"message":  "Analysis completed successfully",
								}
								if usageInt != nil {
									toolResult["usage"] = map[string]any{
										"cost":          usageInt.CostUSD,
										"input_tokens":  usageInt.InputTokens,
										"output_tokens": usageInt.OutputTokens,
									}
								}
							}
						}
					} else {
						toolResult = nRes
						// Check for termination action
						if action, ok := nRes["action"].(string); ok && action == "terminate_session" {
							finalAction = "terminate_session"
							shouldBreak = true
							if fw, ok := nRes["farewell_message"].(string); ok && fw != "" {
								farewellMsg = fw
							}
						}
					}
				}

				md := map[string]string{
					"trace_id": traceID,
					"request":  redactArgsIfNeeded(tc.Name, tc.Args, true), // Native: Is Native
					"response": redactArgsIfNeeded(tc.Name, toolResult, true),
				}
				// Si hubo un análisis multimodal en esta tool, extraer su costo para el monitor
				if usageRaw, ok := toolResult["usage"]; ok {
					if usage, ok := usageRaw.(map[string]any); ok {
						if m, ok := usage["model"].(string); ok {
							md["model"] = m
						}
						if c, ok := usage["cost"].(float64); ok {
							md["usage_cost"] = fmt.Sprintf("$%.6f", c)
						}
					}
				}
				botmonitor.Record(botmonitor.Event{
					TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID,
					Provider: "native", Stage: "native_call", Kind: tc.Name, Status: "ok", DurationMs: duration,
					Metadata: md,
				})
			} else {
				// Warn exacto del original
				logrus.Warnf("[GEMINI] Tool caller not found for: %s (NativeCallerIsNil: %v)", tc.Name, o.nativeToolCaller == nil)
				toolResult = map[string]any{"error": "tool not found"}
			}

			// Añadir resultado a la lista de respuestas de este turno
			responses = append(responses, domain.ToolResponse{
				ID:   tc.ID,
				Name: tc.Name,
				Data: toolResult,
			})
		}

		// Registrar un ÚNICO turno de usuario con TODAS las respuestas (Importante para Gemini)
		if len(responses) > 0 {
			req.History = append(req.History, domain.ChatTurn{
				Role:          "user",
				ToolResponses: responses,
			})
		}

		if shouldBreak {
			break
		}
	}

	if finalAction == "terminate_session" {
		if farewellMsg != "" {
			finalResponse = farewellMsg
		} else if finalResponse == "" {
			finalResponse = lastAIText
		}
	}

	return domain.BotOutput{
		Text:        finalResponse,
		Action:      finalAction,
		TotalCost:   totalCost,
		CostDetails: costDetails,
	}, nil
}

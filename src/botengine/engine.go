package botengine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/botengine/application"
	"github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/botengine/infrastructure"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type PostReplyHook func(ctx context.Context, b bot.Bot, input domain.BotInput, output domain.BotOutput)

// PresenceConfig centraliza los tiempos y umbrales de la humanización situacional
type PresenceConfig struct {
	ImmediateReadWindow  time.Duration // Tiempo tras responder donde el visto es instantáneo
	HighFocusThreshold   int           // Score para entrar en modo enfoque alto
	MediumFocusThreshold int           // Score para enfoque moderado
	NoticeDelayBase      time.Duration // Tiempo base para "abrir" el chat
}

var DefaultPresenceConfig = PresenceConfig{
	ImmediateReadWindow:  5 * time.Second,
	HighFocusThreshold:   70,
	MediumFocusThreshold: 40,
	NoticeDelayBase:      1000 * time.Millisecond,
}

type Engine struct {
	botUsecase  bot.IBotUsecase
	mcpUsecase  domainMCP.IMCPUsecase
	providers   map[string]domain.AIProvider
	mu          sync.RWMutex
	transports  map[string]domain.Transport
	humanizer   *infrastructure.Humanizer
	onPostReply []PostReplyHook
	nativeTools map[string]*domain.NativeTool

	// Nuevos servicios desacoplados
	prompter     *application.Prompter
	orchestrator *application.Orchestrator
}

func NewEngine(botService bot.IBotUsecase, mcpService domainMCP.IMCPUsecase) *Engine {
	e := &Engine{
		botUsecase:  botService,
		mcpUsecase:  mcpService,
		providers:   make(map[string]domain.AIProvider),
		transports:  make(map[string]domain.Transport),
		humanizer:   infrastructure.NewHumanizer(true),
		nativeTools: make(map[string]*domain.NativeTool),
	}

	// Default tools are now registered in cmd/root.go to avoid import cycles

	// Inicializar servicios
	e.prompter = application.NewPrompter()
	e.orchestrator = application.NewOrchestrator(mcpService, e.CallNativeTool)

	return e
}

func (e *Engine) RegisterNativeTool(t *domain.NativeTool) {
	e.nativeTools[t.Name] = t
}

func (e *Engine) GetNativeTools(input domain.BotInput) []domainMCP.Tool {
	var tools []domainMCP.Tool
	for _, t := range e.nativeTools {
		// Check visibility condition if present
		if t.IsVisible != nil && !t.IsVisible(input) {
			continue
		}
		tools = append(tools, t.Tool)
	}
	return tools
}

func (e *Engine) CallNativeTool(ctx context.Context, name string, input domain.BotInput, args map[string]interface{}) (map[string]interface{}, error) {
	t, ok := e.nativeTools[name]
	if !ok {
		return nil, fmt.Errorf("native tool %s not found", name)
	}

	// Prepare generic context
	ctxData := map[string]interface{}{
		"metadata":       input.Metadata,
		"text":           input.Text,
		"sender_id":      input.SenderID,
		"chat_id":        input.ChatID,
		"instance_id":    input.InstanceID,
		"workspace_id":   input.WorkspaceID,
		"client_context": input.ClientContext,
	}

	return t.Handler(ctx, ctxData, args)
}

func (e *Engine) RegisterPostReplyHook(h PostReplyHook) {
	e.onPostReply = append(e.onPostReply, h)
}

func (e *Engine) RegisterProvider(name string, p domain.AIProvider) {
	e.providers[name] = p
}

func (e *Engine) RegisterTransport(t domain.Transport) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.transports[t.ID()] = t
}

func (e *Engine) UnregisterTransport(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.transports, id)
}

func (e *Engine) Humanizer() *infrastructure.Humanizer {
	return e.humanizer
}

func (e *Engine) GetBotUsecase() bot.IBotUsecase {
	return e.botUsecase
}

// Process maneja el ciclo de vida completo de un mensaje
func (e *Engine) Process(ctx context.Context, input domain.BotInput) (domain.BotOutput, error) {
	// 0. Ensure TraceID
	if input.TraceID == "" {
		input.TraceID = uuid.NewString()
	}

	logrus.Infof("[ENGINE] Processing message from %s for bot %s (Instance/Channel: %s, Trace: %s)", input.SenderID, input.BotID, input.InstanceID, input.TraceID)

	meta := map[string]string{"trace_id": input.TraceID}

	// 0. Record Inbound
	botmonitor.Record(botmonitor.Event{
		TraceID:    input.TraceID,
		InstanceID: input.InstanceID,
		ChatJID:    input.ChatID,
		Provider:   string(input.Platform), // Initial provider is the platform
		Stage:      "inbound",
		Kind:       "text",
		Status:     "ok",
		Metadata: map[string]string{
			"trace_id": input.TraceID,
			"input":    input.Text,
			"platform": string(input.Platform),
		},
	})

	// 1. Cargar configuración del bot
	b, err := e.botUsecase.GetByID(ctx, input.BotID)
	if err != nil {
		meta["error_detail"] = err.Error()
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Stage: "bot_load", Status: "error", Error: err.Error(),
			Metadata: meta,
		})
		return domain.BotOutput{}, fmt.Errorf("failed to load bot %s: %w", input.BotID, err)
	}

	// 2. Whitelist logic
	if len(b.Whitelist) > 0 {
		allowed := false
		for _, jid := range b.Whitelist {
			if strings.TrimSpace(jid) == strings.TrimSpace(input.SenderID) ||
				strings.Contains(input.SenderID, strings.TrimSpace(jid)) {
				allowed = true
				break
			}
		}
		if !allowed {
			logrus.Infof("[ENGINE] Message from %s ignored (not in whitelist for bot %s)", input.SenderID, b.ID)
			botmonitor.Record(botmonitor.Event{
				TraceID:    input.TraceID,
				InstanceID: input.InstanceID, ChatJID: input.ChatID,
				Stage: "ai_response", Status: "skipped",
				Metadata: meta,
			})
			return domain.BotOutput{}, nil
		}
	}

	// 2.5 Media Intent Detection (Conserje de Recursos)
	e.detectMediaIntents(b, &input)

	// NOTE: Resource tracking is now handled by Workspace.SessionEntry
	// Files are tracked in SessionEntry.DownloadedFiles and cleaned up
	// when the session expires via cleanupSessionFiles callback

	// 3. Seleccionar Proveedor
	providerName := string(b.Provider)
	if providerName == "" {
		providerName = "ai"
	}

	p, ok := e.providers[providerName]
	if !ok {
		return domain.BotOutput{}, fmt.Errorf("provider %s not registered", providerName)
	}

	// 4. Cargar Herramientas
	var tools []domainMCP.Tool
	if e.mcpUsecase != nil {
		tools, _ = e.mcpUsecase.GetBotTools(ctx, b.ID)
	}
	// Agregar herramientas nativas (filtered by visibility)
	tools = append(tools, e.GetNativeTools(input)...)

	// 5. INTUITION PHASE: Pre-analyze mindset before presence
	// Prepare history for intuition
	var intuitionHistory []domain.ChatTurn
	if b.MemoryEnabled && len(input.History) > 0 {
		intuitionHistory = input.History
	}

	var totalExecutionCost float64
	var costDetails []domain.ExecutionCost

	addExecutionCost := func(botID, model string, cost float64) {
		if cost <= 0 {
			return
		}
		totalExecutionCost += cost
		for i, d := range costDetails {
			if d.BotID == botID && d.Model == model {
				costDetails[i].Cost += cost
				return
			}
		}
		costDetails = append(costDetails, domain.ExecutionCost{BotID: botID, Model: model, Cost: cost})
	}

	mindset, usageInt, err := p.PreAnalyzeMindset(ctx, b, input, intuitionHistory)
	if err == nil && usageInt != nil {
		modelName := usageInt.Model
		if modelName == "" {
			modelName = b.MindsetModel
			if modelName == "" {
				modelName = b.Model
			}
		}
		addExecutionCost(b.ID, modelName, usageInt.CostUSD)
	}
	if err == nil && mindset != nil {
		md := map[string]string{
			"pace":           mindset.Pace,
			"focus":          fmt.Sprintf("%v", mindset.Focus),
			"work":           fmt.Sprintf("%v", mindset.Work),
			"ack":            mindset.Acknowledgement,
			"should_respond": fmt.Sprintf("%v", mindset.ShouldRespond),
		}
		if usageInt != nil {
			md["model"] = usageInt.Model
			md["cost"] = fmt.Sprintf("$%.6f", usageInt.CostUSD)
			md["input_tokens"] = fmt.Sprintf("%d", usageInt.InputTokens)
			md["output_tokens"] = fmt.Sprintf("%d", usageInt.OutputTokens)
		}
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Provider: string(b.Provider), Stage: "intuition", Status: "ok",
			Metadata: md,
		})
	}

	// inteligente Gateway: Si la IA decide que no es necesario responder
	if mindset != nil && !mindset.ShouldRespond {
		logrus.Infof("[ENGINE] AI decided NOT to respond to this message (ShouldRespond=false). Trace: %s", input.TraceID)
		e.mu.RLock()
		transport, hasTransport := e.transports[input.InstanceID]
		e.mu.RUnlock()
		if hasTransport {
			_ = transport.MarkRead(ctx, input.ChatID, []string{input.TraceID})
		}
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Stage: "ai_response", Status: "skipped",
			Metadata: map[string]string{"reason": "intuitive_gatekeeper"},
		})
		return domain.BotOutput{Mindset: mindset, Metadata: map[string]any{"skipped": true}}, nil
	}

	// 5.5 HUMAN PRESENCE SIMULATOR (Parallel to AI)
	isAlreadyActive, _ := input.Metadata["is_delayed"].(bool)

	e.mu.RLock()
	transport, hasTransport := e.transports[input.InstanceID]
	e.mu.RUnlock()

	// Acknowledgement (Outbound ACK) - Humanized
	if hasTransport && mindset != nil && mindset.Acknowledgement != "" {
		// Only send ACK if we haven't replied in the last 10 seconds
		// (Avoid "Give me a second" loops if we are already talking)
		isRecentlyReplied := !input.LastReplyTime.IsZero() && time.Since(input.LastReplyTime) < 10*time.Second

		if !isRecentlyReplied {
			go func() {
				// Simulate thinking for ACK
				time.Sleep(time.Duration(300+e.humanizer.Rng.Intn(500)) * time.Millisecond)

				// Fast typing for ACK
				if ok := e.humanizer.SimulateTypingWithProfile(ctx, transport, input.ChatID, mindset.Acknowledgement, infrastructure.FastTyperProfile); ok {
					_ = transport.SendMessage(ctx, input.ChatID, mindset.Acknowledgement, "")

					botmonitor.Record(botmonitor.Event{
						TraceID:    input.TraceID,
						InstanceID: input.InstanceID, ChatJID: input.ChatID,
						Stage: "outbound_ack", Status: "ok",
						Metadata: map[string]string{"text": mindset.Acknowledgement},
					})
				}
			}()
		}
	}

	// presencerDone notifies when the "reading/thinking" phase is over and we should start typing
	presencerStartedTyping := make(chan bool, 1)

	go func() {
		if hasTransport {
			// 1. Notice Delay (Time to open the chat)
			if !isAlreadyActive {
				// Base notice delay de la config global
				noticeBase := int(domain.DefaultPresenceConfig.NoticeDelayBase / time.Millisecond)
				if input.FocusScore > domain.DefaultPresenceConfig.HighFocusThreshold {
					noticeBase = 0 // Instant notice if focused
				} else if mindset != nil && mindset.Pace == "fast" {
					noticeBase = 200 // Faster if trivial
				}

				noticeDelay := time.Duration(noticeBase+e.humanizer.Rng.Intn(1500)) * time.Millisecond
				if noticeBase > 0 {
					time.Sleep(noticeDelay)
				}

				// Mark as Read
				if ids, ok := input.Metadata["message_ids"].([]string); ok && len(ids) > 0 {
					_ = transport.MarkRead(ctx, input.ChatID, ids)
				} else if msgID, ok := input.Metadata["message_id"].(string); ok && msgID != "" {
					_ = transport.MarkRead(ctx, input.ChatID, []string{msgID})
				}

				// NOTIFY MANAGER: Chat is now open!
				if input.OnChatOpen != nil {
					input.OnChatOpen()
				}
			} else {
				// If already active, Ensure Manager knows it's still open
				if input.OnChatOpen != nil {
					input.OnChatOpen()
				}
			}

			// 2. Reading/Processing Delay (Time to digest the text)
			readingBase := 1200
			if isAlreadyActive {
				readingBase = 400
			}
			if mindset != nil {
				if mindset.Pace == "fast" {
					readingBase = 300
				} else if mindset.Pace == "deep" || mindset.Work {
					readingBase = 2500 // Take more time to "read" complex things
				}
			}

			readDelay := time.Duration(readingBase+e.humanizer.Rng.Intn(1500)) * time.Millisecond
			time.Sleep(readDelay)
		}
		presencerStartedTyping <- true
	}()

	// 6. Generar Respuesta (Orquestación Escalable)

	// Pre-requisito: Mapeo de herramientas MCP e instrucciones
	serverMap := make(map[string]string)
	var mcpInstructions strings.Builder
	if b.ID != "" && e.mcpUsecase != nil {
		if servers, err := e.mcpUsecase.ListServersForBot(ctx, b.ID); err == nil {
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

	// A. Construir instrucciones del sistema
	systemPrompt := e.prompter.BuildSystemInstructions(b, input, mcpInstructions.String())

	// B. Interpretar medios y enriquecer input
	var interpreter *application.Interpreter
	if multimodal, ok := p.(domain.MultimodalInterpreter); ok {
		interpreter = application.NewInterpreter(multimodal, b.APIKey)
	}
	multimodalModel := b.MultimodalModel
	if multimodalModel == "" {
		multimodalModel = b.Model
	}
	enrichedText, usageMult, err := interpreter.EnrichInput(ctx, multimodalModel, input)
	if err == nil && usageMult != nil {
		modelName := usageMult.Model
		if modelName == "" {
			modelName = multimodalModel
		}
		addExecutionCost(b.ID, modelName, usageMult.CostUSD)
	}
	if err != nil {
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Provider: string(b.Provider), Stage: "multimodal_interpretation", Status: "error",
			Error: err.Error(),
		})
	} else if len(input.Medias) > 0 {
		md := map[string]string{}
		if usageMult != nil {
			md["model"] = usageMult.Model
			md["cost"] = fmt.Sprintf("$%.6f", usageMult.CostUSD)
			md["input_tokens"] = fmt.Sprintf("%d", usageMult.InputTokens)
			md["output_tokens"] = fmt.Sprintf("%d", usageMult.OutputTokens)
		}
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Provider: string(b.Provider), Stage: "multimodal_interpretation", Status: "ok",
			Metadata: md,
		})
	}

	// C. Chat Request agnóstico
	var chatHistory []domain.ChatTurn
	if b.MemoryEnabled {
		chatHistory = input.History
	}

	req := domain.ChatRequest{
		SystemPrompt: systemPrompt,
		History:      chatHistory,
		Tools:        tools,
		UserText:     enrichedText,
		Model:        b.Model,
		ChatKey:      input.InstanceID + "|" + input.ChatID,
	}

	// D. Ejecutar Orquestador (Ciclo de herramientas)
	output, err := e.orchestrator.Execute(ctx, p, b, input, req, serverMap)
	if err != nil {
		meta["error_detail"] = err.Error()
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Stage: "ai_execution", Status: "error", Error: err.Error(),
			Metadata: meta,
		})
		return domain.BotOutput{}, fmt.Errorf("orchestrator failed: %w", err)
	}

	// Consolidate costs details
	for _, d := range costDetails {
		output.CostDetails = append(output.CostDetails, d)
	}
	// Re-calculate total just in case or trust the sum
	output.TotalCost += totalExecutionCost
	output.Mindset = mindset // Preservar mindset para el hook si es necesario

	if output.Text == "" {
		return output, nil
	}

	// Extract Mindset if IA provided it in text (overrides intuition)
	newMindset := e.parseMindset(output.Text)
	if newMindset != nil && newMindset.Pace != "steady" { // If IA really put tags
		output.Mindset = newMindset
	} else {
		output.Mindset = mindset // Use intuition
	}
	output.Text = e.cleanMindsetTags(output.Text)

	// 7. Post-Proceso: Simular escritura y Enviar
	if hasTransport {
		// IMPORTANT: Ensure the presencer has at least finished the "Reading" phase
		// so we don't start sending messages before marking as read or waiting
		select {
		case <-presencerStartedTyping:
		case <-ctx.Done():
			return output, nil
		}

		// 8. Work Simulation (If IA says it was hard work)
		if output.Mindset != nil && output.Mindset.Work {
			// Additional delay to simulate "processing/working"
			workDelay := time.Duration(1500+e.humanizer.Rng.Intn(2500)) * time.Millisecond
			select {
			case <-time.After(workDelay):
			case <-ctx.Done():
				return output, nil
			}
		}

		bubbles := e.humanizer.SplitIntoBubbles(output.Text)
		if output.Metadata == nil {
			output.Metadata = make(map[string]any)
		}
		output.Metadata["bubbles"] = fmt.Sprintf("%d", len(bubbles))

		for i, bubble := range bubbles {
			// 2. Select Typing Profile based on Mindset Pace
			profile := infrastructure.DefaultProfile
			if output.Mindset != nil {
				switch output.Mindset.Pace {
				case "fast":
					profile = infrastructure.FastTyperProfile
				case "deep":
					profile = infrastructure.CasualTyperProfile // More pauses for deep thoughts
				}
			}

			// 3. Simulate Typing for this bubble
			if ok := e.humanizer.SimulateTypingWithProfile(ctx, transport, input.ChatID, bubble, profile); !ok {
				break
			}

			// HUMAN ESSENCE: Decide if we should REPLY (quote) the message
			quoteID := ""
			if i == 0 {
				chance := e.humanizer.BaseQuoteChance

				// 1. If previous response was multi-bubble, we MUST quote to maintain context (100% chance)
				if lastCount, ok := input.Metadata["last_bubble_count"].(int); ok && lastCount > 1 {
					chance = e.humanizer.MultiBubbleQuoteChance
				}

				// 2. If message is delayed, high chance of quoting
				if delayed, ok := input.Metadata["is_delayed"].(bool); ok && delayed {
					if chance < e.humanizer.DelayedQuoteChance {
						chance = e.humanizer.DelayedQuoteChance
					}
				}

				if e.humanizer.Rng.Intn(100) < chance {
					if id, ok := input.Metadata["message_id"].(string); ok {
						quoteID = id
					}
				}
			}

			// 3. Send message
			_ = transport.SendMessage(ctx, input.ChatID, bubble, quoteID)

			// 4. Record event
			if i == 0 {
				botmonitor.Record(botmonitor.Event{
					TraceID:    input.TraceID,
					InstanceID: input.InstanceID, ChatJID: input.ChatID,
					Provider: fmt.Sprintf("%s / %s", input.Platform, providerName),
					Stage:    "outbound", Status: "ok",
					Metadata: map[string]string{
						"trace_id":             input.TraceID,
						"model":                b.Model,
						"output":               bubble,
						"bubbles":              fmt.Sprintf("%d", len(bubbles)),
						"total_execution_cost": fmt.Sprintf("$%.6f", output.TotalCost),
					},
				})
			}

			// 5. Small gap between bubbles
			if i < len(bubbles)-1 {
				gap := time.Duration(500+e.humanizer.Rng.Intn(700)) * time.Millisecond
				select {
				case <-time.After(gap):
				case <-ctx.Done():
					return output, nil
				}
			}
		}

		// 6. Execute Hooks (ej: Chatwoot)
		for _, h := range e.onPostReply {
			h(ctx, b, input, output)
		}
	} else {
		// No transport (REST API), hooks only.
		for _, h := range e.onPostReply {
			h(ctx, b, input, output)
		}
	}

	return output, nil
}

func (e *Engine) parseMindset(text string) *domain.Mindset {
	m := &domain.Mindset{Pace: "steady", Focus: false, Work: false}
	if !strings.Contains(text, "<mindset") {
		return m
	}

	if strings.Contains(text, `pace="fast"`) {
		m.Pace = "fast"
	} else if strings.Contains(text, `pace="deep"`) {
		m.Pace = "deep"
	}

	if strings.Contains(text, `focus="true"`) {
		m.Focus = true
	}
	if strings.Contains(text, `work="true"`) {
		m.Work = true
	}
	// Note: Strings like acknowledgement and enqueue_task are usually parsed
	// from JSON in PreAnalyzeMindset, but here we clean tags for the full response.
	// We'll add simple regex/strings detection for them if they appear in tags too.

	return m
}

func (e *Engine) cleanMindsetTags(text string) string {
	// Simple cleanup of the tag
	start := strings.Index(text, "<mindset")
	if start == -1 {
		return text
	}
	end := strings.Index(text[start:], "/>")
	if end == -1 {
		return text
	}
	return strings.TrimSpace(text[:start] + text[start+end+2:])
}

// detectMediaIntents clasifica los medios en inmediatos o diferidos según la intención y los switches del bot
func (e *Engine) detectMediaIntents(b bot.Bot, input *domain.BotInput) {
	if len(input.Medias) == 0 {
		return
	}

	lowerText := strings.ToLower(input.Text)
	hasIntent := strings.Contains(lowerText, "analiza") ||
		strings.Contains(lowerText, "resume") ||
		strings.Contains(lowerText, "lee") ||
		strings.Contains(lowerText, "checa") ||
		strings.Contains(lowerText, "mira") ||
		strings.Contains(lowerText, "procesa") ||
		strings.Contains(lowerText, "explica")

	for _, m := range input.Medias {
		if m.State == domain.MediaStateBlocked {
			continue
		}

		// Categoría A: Visibilidad/Escucha Inmediata
		isVisual := strings.HasPrefix(m.MimeType, "image/") || m.MimeType == "image/webp"
		isAudio := strings.HasPrefix(m.MimeType, "audio/")
		isSticker := strings.Contains(m.MimeType, "sticker") || (isVisual && strings.Contains(strings.ToLower(m.FileName), "sticker"))

		// Categoría B: Acceso Diferido (Soportados por IA pero pesados)
		isVideo := strings.HasPrefix(m.MimeType, "video/")
		isAnalyzableDoc := strings.Contains(m.MimeType, "/pdf") ||
			strings.Contains(m.MimeType, "/plain") ||
			strings.Contains(m.MimeType, "/msword") ||
			strings.Contains(m.MimeType, "vnd.openxmlformats-officedocument")

		if isVisual || isSticker {
			if b.ImageEnabled {
				m.State = domain.MediaStateAnalyzed
			} else {
				m.State = domain.MediaStateAvailable
			}
		} else if isAudio {
			if b.AudioEnabled {
				m.State = domain.MediaStateAnalyzed
			} else {
				m.State = domain.MediaStateAvailable
			}
		} else if isVideo {
			if b.VideoEnabled && hasIntent {
				m.State = domain.MediaStateAnalyzed
			} else {
				m.State = domain.MediaStateAvailable
			}
		} else if isAnalyzableDoc {
			if b.DocumentEnabled && hasIntent {
				m.State = domain.MediaStateAnalyzed
			} else {
				m.State = domain.MediaStateAvailable
			}
		} else {
			// Categoría C: Recursos de Propósito General (Solo metadatos)
			m.State = domain.MediaStateAvailable
		}
	}
}

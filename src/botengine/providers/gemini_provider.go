package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

// GeminiProvider is the adapter for the Google Gemini API
type GeminiProvider struct {
	mcpUsecase   domainMCP.IMCPUsecase
	contextCache domain.ContextCacheStore
}

// NewGeminiProvider creates a new Gemini provider with the given MCP service and context cache store.
func NewGeminiProvider(mcpService domainMCP.IMCPUsecase, cacheStore domain.ContextCacheStore) *GeminiProvider {
	return &GeminiProvider{
		mcpUsecase:   mcpService,
		contextCache: cacheStore,
	}
}

// intuitionSystemPrompt is the fixed template for the Intuition (PreAnalyzeMindset) phase.
// This is cached globally to reduce token costs, as it's identical across all requests.
const intuitionSystemPrompt = `You are an emotional context and intent analyzer for a conversational AI system.
Your job is to quickly categorize user messages to help the main AI respond appropriately.

ANALYSIS RULES:
- pace: Determine the appropriate response speed.
  - 'fast': Simple greetings, acknowledgements, trivial messages.
  - 'steady': Normal conversation, questions, requests.
  - 'deep': Complex analysis, multi-step tasks, file processing.

- focus: Set to true if the topic seems high priority or urgent.

- work: Set to true ONLY if the message requires NEW tools, file processing, or deep analysis.
  Do NOT set to true for simple questions or continuation of previous topics.

- acknowledgement: A short, natural, human phrase to acknowledge the user while processing.
  RULES:
  1. Use the PRIMARY LANGUAGE specified in the context.
  2. ONLY provide if WORK is true. Empty string otherwise.
  3. NEVER provide if the message is a direct answer or relates to pending tasks.
  Examples: "Un momento...", "Déjame revisar...", "One moment...", "Let me check..."

- enqueue_task: If a NEW task is requested while the bot is BUSY, describe it briefly. Empty otherwise.

- clear_tasks: Set to true if this message RESOLVES current pending tasks.

- should_respond: Decision logic:
  1. If message contains info for a task in the BOT AGENDA: true
  2. If message contains a NEW COMMAND, UPDATE, or CORRECTION: true
  3. If WORK is true: true
  4. Set false ONLY if (IS_BUSY is true) AND (message is trivial/acknowledgement) AND (User is NOT answering a question) AND (User is NOT greeting).
  5. ALWAYS CHECK HISTORY: If the last Bot message was a QUESTION or PROPOSAL, then "si", "ok", "thumbs up", "no" are ANSWERS. Set should_respond: true.
  6. GREETINGS ARE PRIORITARY: "Hola", "Hello", "Hi", "Buenas" must ALWAYS be answered (should_respond: true), even if busy.
  7. If IS_BUSY is false: ALWAYS true

Return ONLY a JSON object with these fields: pace, focus, work, acknowledgement, should_respond, enqueue_task, clear_tasks.`

// Chat implementa la interfaz AIProvider enviando una petición a la API de Gemini
func (p *GeminiProvider) Chat(ctx context.Context, b domainBot.Bot, req domain.ChatRequest) (domain.ChatResponse, error) {
	if b.APIKey == "" {
		return domain.ChatResponse{}, fmt.Errorf("bot %s has no API key", b.ID)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  b.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return domain.ChatResponse{}, err
	}

	var genConfig *genai.GenerateContentConfig

	// Combine System Prompt + Dynamic Context (Time, Tasks, etc.)
	fullSystemPrompt := req.SystemPrompt

	if fullSystemPrompt != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(fullSystemPrompt, ""),
		}
	}

	// Herramientas
	var functionDecls []*genai.FunctionDeclaration
	for _, t := range req.Tools {
		functionDecls = append(functionDecls, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  p.convertMCPSchemaToAI(t.InputSchema),
		})
	}

	if len(functionDecls) > 0 {
		if genConfig == nil {
			genConfig = &genai.GenerateContentConfig{}
		}
		genConfig.Tools = []*genai.Tool{{FunctionDeclarations: functionDecls}}
	}

	// Historial y Conversión de Turnos (PARIDAD EXACTA)
	contents := make([]*genai.Content, 0)

	// Inject DynamicContext at the start of dialogue to keep it outside the static cache.
	// This ensures the cache remains stable while the model stays updated with time/date.
	if req.DynamicContext != "" {
		contents = append(contents, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: "[SYSTEM_CONTEXT/TODAY]\n" + req.DynamicContext}},
		})
	}

	for _, t := range req.History {
		// Si hay RawContent (de una iteración previa), usarlo directamente (PARIDAD ORIGINAL)
		if t.RawContent != nil {
			if raw, ok := t.RawContent.(*genai.Content); ok {
				contents = append(contents, raw)
				continue
			}
		}

		role := genai.RoleUser
		if t.Role == "assistant" {
			role = genai.RoleModel
		}

		// Si el turno tiene ToolCalls, es una respuesta del modelo con llamadas a funciones
		if len(t.ToolCalls) > 0 {
			parts := []*genai.Part{}
			if t.Text != "" {
				parts = append(parts, &genai.Part{Text: t.Text})
			}
			for _, tc := range t.ToolCalls {
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: tc.Name,
						Args: tc.Args,
					},
				})
			}
			contents = append(contents, &genai.Content{Role: genai.RoleModel, Parts: parts})
			continue
		}

		// Si el turno tiene ToolResponses, es una respuesta de herramienta (rol user)
		// IMPORTANTE: Todas las respuestas de un mismo turno deben ir en el mismo Content
		if len(t.ToolResponses) > 0 {
			parts := []*genai.Part{}
			for _, tr := range t.ToolResponses {
				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:       tr.ID,
						Name:     tr.Name,
						Response: tr.Data.(map[string]any),
					},
				})
			}
			contents = append(contents, &genai.Content{
				Role:  "function", // CORRECT ROLE for function responses (was "user")
				Parts: parts,
			})
			continue
		}

		// Turno de texto simple
		if t.Text != "" {
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: []*genai.Part{{Text: t.Text}},
			})
		}
	}

	// Último mensaje del usuario (UserText)
	if req.UserText != "" {
		text := req.UserText
		contents = append(contents, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: text}},
		})
	}

	model := req.Model
	if model == "" {
		model = domainBot.DefaultGeminiModel
	}

	// --- CONTEXT CACHING INTELLIGENCE ---
	// Google (Gemini) requires a minimum token count for caching (usually 4096).
	// We estimate tokens (Prompt + Tools) to decide if we should cache.
	promptTokens := p.estimateTokens(req.SystemPrompt)
	toolTokens := 0
	if len(functionDecls) > 0 {
		// Estimated 150 tokens per tool definition on average
		toolTokens = len(functionDecls) * 150
	}
	systemTokens := promptTokens + toolTokens

	logrus.WithFields(logrus.Fields{
		"model":         model,
		"prompt_tokens": promptTokens,
		"tool_tokens":   toolTokens,
		"total_est":     systemTokens,
	}).Debug("[GEMINI] Checking CORE cache eligibility...")

	var cachedContentName string
	if p.contextCache != nil {
		// Calculate stable fingerprint for System + Tools (Structural Cache)
		// We DON'T include contents here because history tokens are handled by implicit caching usually.
		// For explicit caching, we focus on the heaviest part: Instructions + Tools.
		fingerprint := p.calculateFingerprint(req.ChatKey, req.SystemPrompt, nil, functionDecls)

		// 1. Check if we already have a REAL cache name in Valkey
		entry, err := p.contextCache.Get(ctx, fingerprint)
		if err == nil && entry != nil && entry.Name != "" && !strings.HasPrefix(entry.Name, "SIM_") {
			// Check if cache is still alive (locally estimated expiry)
			if time.Now().Before(entry.ExpiresAt) {
				cachedContentName = entry.Name
				logrus.WithFields(logrus.Fields{
					"cache_name":  cachedContentName,
					"model":       model,
					"fingerprint": fingerprint,
					"ttl_left":    time.Until(entry.ExpiresAt).Round(time.Second),
				}).Info("[CACHE_HIT] Reusing existing Gemini Context Cache")

				// SLIDING TTL: If less than 2 minutes remain, extend the cache
				if time.Until(entry.ExpiresAt) < 2*time.Minute {
					logrus.WithField("cache", cachedContentName).Info("[CACHE_EXTEND] Extending TTL (Small Window)...")
					// Extension increment: 5 minutes (Enough for session termination gap)
					newTTL := 5 * time.Minute
					_, err := client.Caches.Update(ctx, cachedContentName, &genai.UpdateCachedContentConfig{
						TTL: newTTL,
					})
					if err == nil {
						// Update Valkey
						entry.ExpiresAt = time.Now().Add(newTTL)
						_ = p.contextCache.Save(ctx, fingerprint, entry, newTTL)
					} else {
						logrus.WithError(err).Warn("[CACHE_EXTEND] Failed to extend cache TTL")
					}
				}
			}
		}

		// 2. If no cache found, create one if worth it, or track maturation
		if cachedContentName == "" {
			// Minimum for most models is 4096 as seen in Intuition error logs.
			// Flash 1.5 officially says 1024, but 2.0/2.5 seem to require more.
			minTokenThreshold := 4000

			if systemTokens >= minTokenThreshold {
				logrus.WithFields(logrus.Fields{
					"system_tokens": systemTokens,
					"threshold":     minTokenThreshold,
					"model":         model,
				}).Info("[CACHE_CREATE] Creating new Explicit Cache...")

				cacheName, err := p.createExplicitCache(ctx, client, model, "sys-"+req.ChatKey, req.SystemPrompt, functionDecls)
				if err != nil {
					logrus.WithError(err).Warn("[CACHE_FAIL] Failed to create cache, proceeding without it")
				} else {
					cachedContentName = cacheName
					// Initial TTL: Set to 15 minutes (Initial gap for session start)
					// This will be extended/shrunk dynamically by sliding window
					initialTTL := 15 * time.Minute
					_ = p.contextCache.Save(ctx, fingerprint, &domain.ContextCacheEntry{
						Name:        cacheName,
						ExpiresAt:   time.Now().Add(initialTTL),
						Model:       model,
						Provider:    "gemini",
						Type:        domain.CacheTypeBot,
						Scope:       req.ChatKey,
						Fingerprint: fingerprint,
						Content:     req.SystemPrompt,
					}, initialTTL)
					logrus.WithFields(logrus.Fields{
						"cache":       cacheName,
						"fingerprint": fingerprint,
						"ttl":         initialTTL,
					}).Info("[CACHE_READY] New cache created and saved to Valkey")
				}
			} else {
				// MATURATION LOGIC: Track progress for smaller bots
				maturationKey := "maturation:" + req.ChatKey
				_ = p.contextCache.Save(ctx, maturationKey, &domain.ContextCacheEntry{
					Name:        fmt.Sprintf("SIM_MATURE_%d_%d", systemTokens, minTokenThreshold),
					ExpiresAt:   time.Now().Add(30 * time.Minute),
					Model:       model,
					Provider:    "gemini",
					Type:        "maturing",
					Scope:       req.ChatKey,
					Fingerprint: maturationKey,
				}, 30*time.Minute)

				logrus.WithFields(logrus.Fields{
					"chat":    req.ChatKey,
					"current": systemTokens,
					"target":  minTokenThreshold,
				}).Debug("[CACHE_MATURATION] Prompt still maturing")
			}
		}
	}

	if genConfig == nil {
		genConfig = &genai.GenerateContentConfig{}
	}
	// ONLY clear tools and instructions if we have a VALID external cache name
	if cachedContentName != "" && !strings.HasPrefix(cachedContentName, "SIM_") {
		genConfig.CachedContent = cachedContentName
		genConfig.SystemInstruction = nil
		genConfig.Tools = nil
	}

	p.applyThinking(genConfig, model, "dynamic")

	// Intelligent User Token Estimation
	textToEstimate := req.UserText
	if textToEstimate == "" && len(req.History) > 0 {
		lastTurn := req.History[len(req.History)-1]
		if lastTurn.Role == "user" {
			textToEstimate = lastTurn.Text
		}
	}
	userTokens := p.estimateTokens(textToEstimate)

	result, err := p.generateContentWithRetry(ctx, client, model, contents, genConfig)
	if err != nil {
		return domain.ChatResponse{}, err
	}

	var usage *domain.UsageStats
	if result.UsageMetadata != nil {
		usage = p.extractUsage(model, result.UsageMetadata)
		if usage != nil {
			// TOKEN INTELLIGENCE: Direct estimation
			// 1. Use User Tokens already estimated

			// 2. Estimate History Tokens (Directly from history contents)
			historyTokens := 0
			for _, c := range req.History {
				for _, pPart := range c.ToolResponses {
					js, _ := json.Marshal(pPart.Data)
					historyTokens += p.estimateTokens(string(js))
				}
				historyTokens += p.estimateTokens(c.Text)
				// Small overhead for role/metadata
				historyTokens += 2
			}

			// 3. Assign
			usage.UserTokens = userTokens
			usage.HistoryTokens = historyTokens

			// 4. System + Tools is the remainder (Total - History - User)
			usage.SystemTokens = usage.InputTokens - historyTokens - userTokens
			if usage.SystemTokens < 0 {
				usage.SystemTokens = 0
			}

			logrus.WithFields(logrus.Fields{
				"total_input": usage.InputTokens,
				"cached":      usage.CachedTokens,
				"system_est":  usage.SystemTokens,
				"user_est":    userTokens,
				"history_est": historyTokens,
			}).Debug("[GEMINI] Token Intelligence collected")
		}
	}

	if result == nil || len(result.Candidates) == 0 {
		return domain.ChatResponse{}, fmt.Errorf("no response from gemini")
	}

	candidate := result.Candidates[0]

	// SAFETY CHECK: If the model refused to respond due to safety or other reasons
	if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonMaxTokens {
		// Log safety ratings for debugging
		for _, rating := range candidate.SafetyRatings {
			logrus.Warnf("[GEMINI] Safety Rating: %s - %s (Blocked: %v)", rating.Category, rating.Probability, rating.Blocked)
		}

		// Map FinishReason to a human-readable error
		reasonStr := "Unknown"
		switch candidate.FinishReason {
		case genai.FinishReasonSafety:
			reasonStr = "Safety Filter Triggered"
		case genai.FinishReasonRecitation:
			reasonStr = "Recitation (Copyright)"
		case genai.FinishReasonOther:
			reasonStr = "Other/Unknown"
		}

		// Return error so the Engine/Orchestrator knows it failed
		return domain.ChatResponse{}, fmt.Errorf("gemini blocked response. reason: %s", reasonStr)
	}

	// Extraer texto manualmente de las partes (más robusto que result.Text())
	var fullText string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			fullText += part.Text
		}
	}

	resp := domain.ChatResponse{
		Text:       fullText,
		RawContent: candidate.Content, // PARIDAD ORIGINAL: Preservar el contenido completo
		Usage:      usage,
	}

	// Extraer llamadas a herramientas
	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			resp.ToolCalls = append(resp.ToolCalls, domain.ToolCall{
				ID:   part.FunctionCall.ID,
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}

	// Extraer UsageMetadata y calcular costo
	// Usage Log Final
	if resp.Usage != nil {
		logrus.WithFields(logrus.Fields{
			"chat_key":      req.ChatKey,
			"model":         model,
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
			"cached_tokens": resp.Usage.CachedTokens,
			"cost_usd":      fmt.Sprintf("$%.6f", resp.Usage.CostUSD),
		}).Debug("[USAGE] Token usage recorded")
	}

	return resp, nil
}

// Interpret implementa la interfaz MultimodalInterpreter para Gemini
func (p *GeminiProvider) Interpret(ctx context.Context, apiKey string, model string, userText string, language string, medias []*domain.BotMedia) (*domain.MultimodalResult, *domain.UsageStats, error) {
	if apiKey == "" {
		return nil, nil, fmt.Errorf("multimodal interpretation requires an API key")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, nil, err
	}

	if model == "" {
		model = domainBot.DefaultGeminiLiteModel
	}

	parts := []*genai.Part{{Text: fmt.Sprintf(`Analyze the following media files sent by the user. Their text message was: "%s"

For each media:
- If it's AUDIO: Transcribe it literally.
- If it's a STICKER (usually image/webp): Interpret it as an expression, emotion, or meme vibe. Don't be overly literal; describe the "feeling" or "intent" the user is conveying (e.g., 'Sending a funny crying cat to express mock sadness').
- If it's an IMAGE: Describe what you see in detail.
- If it's a DOCUMENT: Summarize its content.
- If it's a VIDEO: Describe what happens in the video and any relevant speech.

Return the results in JSON format.
Your PRIMARY language for descriptions and summaries is: %s.`, userText, language)}}

	for _, m := range medias {
		if len(m.Data) > 0 {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{MIMEType: m.MimeType, Data: m.Data},
			})
		}
	}

	contents := []*genai.Content{{Role: genai.RoleUser, Parts: parts}}

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"transcriptions": {
					Type:        "array",
					Items:       &genai.Schema{Type: "string"},
					Description: "Literal transcriptions of audio files, in order.",
				},
				"descriptions": {
					Type:        "array",
					Items:       &genai.Schema{Type: "string"},
					Description: "Visual descriptions of image files, in order.",
				},
				"summaries": {
					Type:        "array",
					Items:       &genai.Schema{Type: "string"},
					Description: "Summaries of document files, in order.",
				},
				"video_summaries": {
					Type:        "array",
					Items:       &genai.Schema{Type: "string"},
					Description: "Detailed analysis and speech transcription of video files, in order.",
				},
			},
			Required: []string{"transcriptions", "descriptions", "summaries", "video_summaries"},
		},
	}

	p.applyThinking(cfg, model, "off")

	result, err := p.generateContentWithRetry(ctx, client, model, contents, cfg)
	if err != nil {
		return nil, nil, err
	}

	var usage *domain.UsageStats
	if result.UsageMetadata != nil {
		usage = p.extractUsage(model, result.UsageMetadata)
		logrus.WithFields(logrus.Fields{
			"stage": "multimodal_interpretation",
			"cost":  usage.CostUSD,
		}).Debug("[USAGE] Multimodal cost recorded")
	}

	var interpretation struct {
		Transcriptions []string `json:"transcriptions"`
		Descriptions   []string `json:"descriptions"`
		Summaries      []string `json:"summaries"`
		VideoSummaries []string `json:"video_summaries"`
	}

	if err := json.Unmarshal([]byte(result.Text()), &interpretation); err != nil {
		return nil, usage, fmt.Errorf("failed to parse interpretation JSON: %w", err)
	}

	return &domain.MultimodalResult{
		Transcriptions: interpretation.Transcriptions,
		Descriptions:   interpretation.Descriptions,
		Summaries:      interpretation.Summaries,
		VideoSummaries: interpretation.VideoSummaries,
	}, usage, nil
}

func (p *GeminiProvider) PreAnalyzeMindset(ctx context.Context, b domainBot.Bot, input domain.BotInput, history []domain.ChatTurn) (*domain.Mindset, *domain.UsageStats, error) {
	if b.APIKey == "" {
		return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  b.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, nil, err
	}

	model := b.MindsetModel
	if model == "" {
		model = b.Model
	}
	if model == "" {
		model = domainBot.DefaultGeminiLiteModel
	}

	// Build dynamic context for user prompt
	isBusy := false
	if input.LastMindset != nil && input.LastMindset.Work {
		isBusy = true
	}

	var histStr strings.Builder
	for _, h := range history {
		histStr.WriteString(fmt.Sprintf("%s: %s\n", h.Role, h.Text))
	}

	var agendaStr strings.Builder
	if len(input.PendingTasks) > 0 {
		agendaStr.WriteString("CURRENT BOT AGENDA (Tasks the bot is planning to do):\n")
		for _, t := range input.PendingTasks {
			agendaStr.WriteString(fmt.Sprintf("- %s\n", t))
		}
	} else {
		agendaStr.WriteString("BOT AGENDA: Empty (No pending tasks).")
	}

	langCtx := ""
	if input.Language != "" {
		langCtx = fmt.Sprintf("\n- PRIMARY LANGUAGE: %s. Use ONLY this language for the acknowledgement.", input.Language)
	}

	// User prompt contains ONLY dynamic data (message, history, context)
	userPrompt := fmt.Sprintf(`Analyze this user message and provide your analysis.

USER MESSAGE:
"%s"

CONTEXT:
- Recent conversation history:
%s
- Is the bot currently busy with a previous task? %v
- %s%s

Now categorize this message according to the system rules.`, input.Text, histStr.String(), isBusy, agendaStr.String(), langCtx)

	// Try to get or create cached content for the system prompt
	cacheFingerprint := fmt.Sprintf("global:intuition:%s", model)
	var cacheName string
	systemCached := false

	if p.contextCache != nil {
		// global lock for intuition cache creation
		lockKey := "lock:" + cacheFingerprint
		locked, _ := p.contextCache.Lock(ctx, lockKey, 2*time.Minute)
		if locked {
			defer p.contextCache.Unlock(ctx, lockKey)
		}

		entry, _ := p.contextCache.Get(ctx, cacheFingerprint)
		if entry != nil && time.Now().Before(entry.ExpiresAt) {
			cacheName = entry.Name
			systemCached = true
			logrus.WithFields(logrus.Fields{
				"cache":    cacheName,
				"model":    model,
				"ttl_left": time.Until(entry.ExpiresAt).Round(time.Second),
			}).Info("[GEMINI] Using existing intuition cache")

			// SLIDING TTL: Extend intuition cache if < 2 mins remain
			if time.Until(entry.ExpiresAt) < 2*time.Minute {
				logrus.WithField("cache", cacheName).Debug("[CACHE_EXTEND] Extending Intuition TTL...")
				// Small increment to match session lifecycle
				newTTL := 5 * time.Minute
				if _, err := client.Caches.Update(ctx, cacheName, &genai.UpdateCachedContentConfig{TTL: newTTL}); err == nil {
					entry.ExpiresAt = time.Now().Add(newTTL)
					_ = p.contextCache.Save(ctx, cacheFingerprint, entry, newTTL)
				}
			}
		} else {
			// Create new cache in Gemini
			logrus.WithField("model", model).Info("[GEMINI] Checking intuition cache eligibility...")
			cacheName, err = p.createExplicitCache(ctx, client, model, "intuition-sys", intuitionSystemPrompt, nil)
			if err != nil {
				logrus.WithError(err).Warn("[GEMINI] Failed to create intuition cache, proceeding without cache")
			} else if cacheName != "" {
				// Initial Life: matching default session or slightly more
				ttl := 15 * time.Minute
				_ = p.contextCache.Save(ctx, cacheFingerprint, &domain.ContextCacheEntry{
					Name:        cacheName,
					ExpiresAt:   time.Now().Add(ttl),
					Fingerprint: cacheFingerprint,
					Model:       model,
					Provider:    "gemini",
					Type:        domain.CacheTypeGlobal,
					Scope:       "intuition",
					Content:     intuitionSystemPrompt,
				}, ttl)
				systemCached = true
				logrus.WithFields(logrus.Fields{
					"cache":       cacheName,
					"fingerprint": cacheFingerprint,
				}).Info("[GEMINI] Created and saved new intuition cache")
			}
		}
	}

	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"pace":            {Type: "string", Enum: []string{"fast", "steady", "deep"}},
				"focus":           {Type: "boolean"},
				"work":            {Type: "boolean"},
				"acknowledgement": {Type: "string"},
				"should_respond":  {Type: "boolean"},
				"enqueue_task":    {Type: "string"},
				"clear_tasks":     {Type: "boolean"},
			},
			Required: []string{"pace", "focus", "work", "acknowledgement", "should_respond", "enqueue_task", "clear_tasks"},
		},
	}

	// If we have a cache, use it; otherwise include system instruction directly
	if cacheName != "" {
		cfg.CachedContent = cacheName
	} else {
		cfg.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: intuitionSystemPrompt}},
		}
	}

	p.applyThinking(cfg, model, "off")

	contents := []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: userPrompt}}}}
	result, err := p.generateContentWithRetry(ctx, client, model, contents, cfg)
	if err != nil {
		// If it's an authentication or permission error, we MUST return it to let the user know.
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "api_key") || strings.Contains(errStr, "401") || strings.Contains(errStr, "403") || strings.Contains(errStr, "invalid") {
			return nil, nil, fmt.Errorf("intuition phase failed with critical error: %w", err)
		}

		// If cache-related error, invalidate and retry without cache
		if strings.Contains(errStr, "cached") || strings.Contains(errStr, "not found") {
			if p.contextCache != nil {
				_ = p.contextCache.Delete(ctx, cacheFingerprint)
			}
			logrus.Warn("[GEMINI] Cache error, retrying intuition without cache")
			cfg.CachedContent = ""
			cfg.SystemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: intuitionSystemPrompt}},
			}
			result, err = p.generateContentWithRetry(ctx, client, model, contents, cfg)
			if err != nil {
				logrus.WithError(err).Warn("[GEMINI] Intuition phase failed, using safe fallback")
				return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
			}
			systemCached = false
		} else {
			logrus.WithError(err).Warn("[GEMINI] Intuition phase failed, using safe fallback")
			return &domain.Mindset{Pace: "steady", ShouldRespond: true}, nil, nil
		}
	}

	var usage *domain.UsageStats
	if result.UsageMetadata != nil {
		usage = p.extractUsage(model, result.UsageMetadata)

		// Token Intelligence for Intuition
		estUser := p.estimateTokens(input.Text)
		estHistory := p.estimateTokens(histStr.String())

		usage.UserTokens = estUser
		usage.HistoryTokens = estHistory

		// If system was cached, CachedTokens should reflect the system prompt tokens
		if systemCached && usage.CachedTokens > 0 {
			usage.SystemTokens = usage.CachedTokens
		} else {
			usage.SystemTokens = usage.InputTokens - estUser - estHistory
			if usage.SystemTokens < 0 {
				usage.SystemTokens = 0
			}
		}

		// Mark if system prompt came from cache
		usage.SystemCached = systemCached

		logrus.WithFields(logrus.Fields{
			"stage":         "intuition",
			"cost":          usage.CostUSD,
			"system":        usage.SystemTokens,
			"user":          usage.UserTokens,
			"history":       usage.HistoryTokens,
			"system_cached": systemCached,
		}).Debug("[USAGE] Intuition cost recorded")
	}

	var mindset domain.Mindset
	if err := json.Unmarshal([]byte(result.Text()), &mindset); err != nil {
		return &domain.Mindset{Pace: "steady"}, usage, nil
	}

	return &mindset, usage, nil
}

// createExplicitCache creates a new cached content in Gemini for long-lived system prompts.
func (p *GeminiProvider) createExplicitCache(ctx context.Context, client *genai.Client, model, name, content string, tools []*genai.FunctionDeclaration) (string, error) {
	// Gemini requires explicit model version for caching
	isExplicit := strings.Contains(model, "-001") || strings.Contains(model, "-002") || strings.Contains(model, "-preview") || strings.Contains(model, "-02-05")
	if !isExplicit && !strings.Contains(model, "flash") && !strings.Contains(model, "pro") {
		return "", nil
	}

	ttl := 1 * time.Hour

	config := &genai.CreateCachedContentConfig{
		DisplayName: name[:min(len(name), 100)], // Limit display name length
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: content}},
		},
		TTL: ttl,
	}

	// Add tools to cache if present
	if len(tools) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: tools}}
	}

	cache, err := client.Caches.Create(ctx, model, config)
	if err != nil {
		return "", fmt.Errorf("failed to create cache: %w", err)
	}

	return cache.Name, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Privados utilitarios

func (p *GeminiProvider) estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

func (p *GeminiProvider) convertMCPSchemaToAI(input interface{}) *genai.Schema {
	data, _ := json.Marshal(input)
	var schema genai.Schema
	json.Unmarshal(data, &schema)
	if schema.Type == "" {
		schema.Type = "object"
	}
	return &schema
}

func (p *GeminiProvider) extractUsage(model string, usage *genai.GenerateContentResponseUsageMetadata) *domain.UsageStats {
	if usage == nil {
		return nil
	}
	inputTokens := int(usage.PromptTokenCount)
	outputTokens := int(usage.CandidatesTokenCount)
	cachedTokens := int(usage.CachedContentTokenCount)

	// Calcular costo usando precios del modelo
	costUSD := p.calculateCost(model, inputTokens, outputTokens, cachedTokens)

	return &domain.UsageStats{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CachedTokens: cachedTokens,
		CostUSD:      costUSD,
	}
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

func (p *GeminiProvider) applyThinking(cfg *genai.GenerateContentConfig, model string, mode string) {
	if cfg == nil || model == "" {
		return
	}

	isG3 := strings.Contains(model, "gemini-3")
	isG25 := strings.Contains(model, "gemini-2.5")

	if !isG3 && !isG25 {
		return
	}

	if cfg.ThinkingConfig == nil {
		cfg.ThinkingConfig = &genai.ThinkingConfig{}
	}

	if mode == "off" {
		if isG3 {
			// Para Gemini 3 Flash, minimal es lo más cercano a apagado
			// Para Gemini 3 Pro no se puede apagar, pero low ahorra recursos
			lvl := "minimal"
			if strings.Contains(model, "pro") {
				lvl = "low"
			}
			cfg.ThinkingConfig.ThinkingLevel = genai.ThinkingLevel(lvl)
		} else if isG25 {
			// Para Gemini 2.5 Flash/Lite, 0 apaga el pensamiento
			// Pro no permite apagarlo, delegamos a dinámico (-1)
			if strings.Contains(model, "pro") {
				budget := int32(-1)
				cfg.ThinkingConfig.ThinkingBudget = &budget
			} else {
				budget := int32(0)
				cfg.ThinkingConfig.ThinkingBudget = &budget
			}
		}
	} else if mode == "dynamic" {
		if isG3 {
			cfg.ThinkingConfig.ThinkingLevel = genai.ThinkingLevel("high")
		} else if isG25 {
			budget := int32(-1) // Default dinámico para G2.5
			cfg.ThinkingConfig.ThinkingBudget = &budget
		}
	}
}
func (p *GeminiProvider) calculateFingerprint(chatKey, systemPrompt string, contents []*genai.Content, tools []*genai.FunctionDeclaration) string {
	h := sha256.New()
	h.Write([]byte(chatKey)) // Usar ChatKey (InstanceID|ChatID) para cache por sesión
	h.Write([]byte(systemPrompt))

	// Serializar contenido estable para el hash
	data, _ := json.Marshal(contents)
	h.Write(data)

	// Serializar herramientas
	toolData, _ := json.Marshal(tools)
	h.Write(toolData)

	return hex.EncodeToString(h.Sum(nil))
}

// calculateCost calcula el costo USD basado en tokens y precios del modelo
func (p *GeminiProvider) calculateCost(model string, inputTokens, outputTokens, cachedTokens int) float64 {
	pricing, ok := domainBot.GeminiModelPrices[model]
	if !ok {
		// Fallback a precios de gemini-2.5-flash si no se encuentra el modelo
		pricing = domainBot.GeminiModelPrices[domainBot.DefaultGeminiModel]
	}

	// Tokens no cacheados (input - cached)
	regularInputTokens := inputTokens - cachedTokens
	if regularInputTokens < 0 {
		regularInputTokens = 0
	}

	// Costo = (regularInput * inputPrice + cachedInput * cachePrice + output * outputPrice) / 1,000,000
	inputCost := float64(regularInputTokens) * pricing.InputPerMToken / 1_000_000
	cachedCost := float64(cachedTokens) * pricing.CacheInputPerMT / 1_000_000
	outputCost := float64(outputTokens) * pricing.OutputPerMToken / 1_000_000

	return inputCost + cachedCost + outputCost
}

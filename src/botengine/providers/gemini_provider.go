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
  4. Set false ONLY if (IS_BUSY is true) AND (message is trivial: "ok", "vale", "gracias", "thanks", "cool")
  5. If IS_BUSY is false: ALWAYS true

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
	if req.SystemPrompt != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(req.SystemPrompt, ""),
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
	var contents []*genai.Content
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
				Role:  "user",
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
		// Inyectar contexto dinámico como DATOS (no instrucciones) para evitar leaks
		if req.DynamicContext != "" {
			text = fmt.Sprintf("[SESS_DATA]\n%s\n[END_DATA]\n\nINPUT: %s", req.DynamicContext, req.UserText)
		}
		contents = append(contents, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: text}},
		})
	} else if req.DynamicContext != "" {
		contents = append(contents, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: "[SESS_REFRESH]\n" + req.DynamicContext}},
		})
	}

	model := req.Model
	if model == "" {
		model = domainBot.DefaultGeminiModel
	}

	// --- CONTEXT CACHING INTELLIGENCE (Checkpoint Logic) ---
	// Google (Gemini) requires a minimum of 1024 tokens for caching in Flash models.
	// We'll use Valkey to track "maturation" before actually hitting the Google API.

	// 1. Calculate tokens for the STABLE prefix (Instruction + Past History)
	// We exclude the last turn to keep the cache stable while the conversation flows.
	stablePrefixCount := len(contents) - 1
	if stablePrefixCount < 0 {
		stablePrefixCount = 0
	}
	stablePrefix := contents[:stablePrefixCount]

	// Estimate tokens (1 token ≈ 4 characters for Gemini)
	estimatedHistoryChars := 0
	for _, c := range stablePrefix {
		for _, p := range c.Parts {
			estimatedHistoryChars += len(p.Text)
		}
	}
	systemTokens := p.estimateTokens(req.SystemPrompt)
	historyTokens := estimatedHistoryChars / 4
	totalStableTokens := systemTokens + historyTokens

	var cachedContentName string
	if p.contextCache != nil {
		maturationKey := fmt.Sprintf("maturation:%s", req.ChatKey)
		// distributed lock to prevent races in maturation logic
		locked, _ := p.contextCache.Lock(ctx, maturationKey, 10*time.Second)
		if locked {
			defer p.contextCache.Unlock(ctx, maturationKey)
		}

		fingerprint := p.calculateFingerprint(req.ChatKey, req.SystemPrompt, stablePrefix, functionDecls)

		// 1. Check if we already have a REAL cache name in Valkey (via Precise Fingerprint)
		entry, err := p.contextCache.Get(ctx, fingerprint)
		if err == nil && entry != nil && entry.Name != "" && !strings.HasPrefix(entry.Name, "SIMULATED_") {
			if time.Now().Before(entry.ExpiresAt) {
				cachedContentName = entry.Name
				contents = contents[stablePrefixCount:]
				logrus.WithFields(logrus.Fields{
					"chat_key":   req.ChatKey,
					"cache_name": cachedContentName,
					"tokens":     totalStableTokens,
				}).Info("[CACHE_HIT] Reusing existing Google context cache")
			}
		}

		// 2. If no real cache, update/overwrite the Maturation Status for this chat
		if cachedContentName == "" {
			minTokenThreshold := 1024 // Gemini 1.5/2.5 Flash minimum
			if totalStableTokens < minTokenThreshold {
				logrus.WithFields(logrus.Fields{
					"chat_key": req.ChatKey,
					"current":  totalStableTokens,
					"required": minTokenThreshold,
				}).Debug("[CACHE_MATURING] Context too small for Google Cache")

				// Save "maturation" state in Valkey using the STABLE ChatKey to OVERWRITE
				_ = p.contextCache.Save(ctx, maturationKey, &domain.ContextCacheEntry{
					Name:        fmt.Sprintf("SIM_P_%d", totalStableTokens),
					ExpiresAt:   time.Now().Add(30 * time.Minute),
					Fingerprint: fingerprint,
				}, 30*time.Minute)
			} else {
				// REACHED THRESHOLD!
				logrus.WithFields(logrus.Fields{
					"chat_key":       req.ChatKey,
					"total_tokens":   totalStableTokens,
					"system_tokens":  systemTokens,
					"history_tokens": historyTokens,
				}).Warn("[CACHE_PROPOSED] Stable context reached threshold! READY for Google sync.")

				// Update "PROPOSED" state in Valkey (Overwrites current maturation)
				_ = p.contextCache.Save(ctx, maturationKey, &domain.ContextCacheEntry{
					Name:        fmt.Sprintf("SIM_R_%d", totalStableTokens),
					ExpiresAt:   time.Now().Add(30 * time.Minute),
					Fingerprint: fingerprint,
				}, 30*time.Minute)
			}
		}
	}

	if genConfig == nil {
		genConfig = &genai.GenerateContentConfig{}
	}
	if cachedContentName != "" {
		genConfig.CachedContent = cachedContentName
		genConfig.SystemInstruction = nil
		genConfig.Tools = nil
	}

	p.applyThinking(genConfig, model, "dynamic")

	// --- TOKEN INTELLIGENCE (Estimation) ---
	// Already calculated above in caching logic for some cases, but ensuring it's available here
	if systemTokens == 0 {
		systemTokens = p.estimateTokens(req.SystemPrompt)
	}

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
			usage.SystemTokens = systemTokens
			usage.UserTokens = userTokens

			// History is accurately inferred: Official Total - Local Estimates
			usage.HistoryTokens = usage.InputTokens - systemTokens - userTokens
			if usage.HistoryTokens < 0 {
				usage.HistoryTokens = 0
			}

			logrus.WithFields(logrus.Fields{
				"total_input": usage.InputTokens,
				"cached":      usage.CachedTokens,
				"system_est":  systemTokens,
				"user_est":    userTokens,
				"history":     usage.HistoryTokens,
			}).Debug("[GEMINI] Token Intelligence collected")
		}
	}

	if result == nil || len(result.Candidates) == 0 {
		return domain.ChatResponse{}, fmt.Errorf("no response from gemini")
	}

	candidate := result.Candidates[0]

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
				"cache": cacheName,
				"model": model,
			}).Info("[GEMINI] Using existing intuition cache")
		} else {
			// Create new cache in Gemini
			logrus.WithField("model", model).Info("[GEMINI] Checking intuition cache eligibility...")
			cacheName, err = p.createExplicitCache(ctx, client, model, "intuition-system-prompt", intuitionSystemPrompt)
			if err != nil {
				logrus.WithError(err).Warn("[GEMINI] Failed to create intuition cache, proceeding without cache")
			} else if cacheName != "" {
				// Save to local store
				ttl := 55 * time.Minute // Slightly less than Gemini's 1 hour default
				_ = p.contextCache.Save(ctx, cacheFingerprint, &domain.ContextCacheEntry{
					Name:        cacheName,
					ExpiresAt:   time.Now().Add(ttl),
					Fingerprint: cacheFingerprint,
				}, ttl)
				systemCached = true
				logrus.WithField("cache", cacheName).Info("[GEMINI] Created new intuition cache")
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
func (p *GeminiProvider) createExplicitCache(ctx context.Context, client *genai.Client, model, name, content string) (string, error) {
	// Gemini requires explicit model version for caching
	isExplicit := strings.Contains(model, "-001") || strings.Contains(model, "-002") || strings.Contains(model, "-preview") || strings.Contains(model, "-02-05")
	if !isExplicit && !strings.Contains(model, "flash") && !strings.Contains(model, "pro") {
		return "", nil
	}

	// Validate minimum tokens (Google requires at least 2048-32768 depending on the model, usually 4096 for flash)
	// We use our local estimator to avoid an extra API call.
	estimated := p.estimateTokens(content)
	if estimated < 4000 { // Use 4000 as safety threshold
		logrus.WithFields(logrus.Fields{
			"tokens": estimated,
			"name":   name,
		}).Debug("[GEMINI] Prompt too small for explicit caching, skipping API call")
		return "", nil
	}

	ttl := 1 * time.Hour
	cache, err := client.Caches.Create(ctx, model, &genai.CreateCachedContentConfig{
		DisplayName: name,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: content}},
		},
		TTL: ttl,
	})
	if err != nil {
		// If it's still a "too small" error from Google, just ignore it and return empty
		if strings.Contains(err.Error(), "too small") {
			logrus.WithField("tokens", estimated).Debug("[GEMINI] Google reported content too small for cache")
			return "", nil
		}
		return "", fmt.Errorf("failed to create cache: %w", err)
	}

	return cache.Name, nil
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

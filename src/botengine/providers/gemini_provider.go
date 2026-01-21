package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

type contextCacheEntry struct {
	Name      string
	ExpiresAt time.Time
}

// GeminiProvider es el adaptador para la API de Google Gemini
type GeminiProvider struct {
	mcpUsecase domainMCP.IMCPUsecase
	caches     sync.Map // key: fingerprint string, value: contextCacheEntry
}

func NewGeminiProvider(mcpService domainMCP.IMCPUsecase) *GeminiProvider {
	return &GeminiProvider{
		mcpUsecase: mcpService,
	}
}

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
		contents = append(contents, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: req.UserText}},
		})
	}

	model := req.Model
	if model == "" {
		model = domainBot.DefaultGeminiModel
	}

	// CONTEXT CACHING LOGIC
	// Threshold: 80,000 characters (~26k-32k tokens)
	totalChars := 0
	for _, c := range contents {
		for _, p := range c.Parts {
			totalChars += len(p.Text)
		}
	}
	totalChars += len(req.SystemPrompt)

	var cachedContentName string
	if totalChars > 80000 && len(contents) > 4 {
		// Cachear el prefijo estable (todo menos los últimos 2 turnos)
		stablePrefixCount := len(contents) - 2
		stablePrefix := contents[:stablePrefixCount]

		fingerprint := p.calculateFingerprint(req.ChatKey, req.SystemPrompt, stablePrefix, functionDecls)

		if entry, ok := p.caches.Load(fingerprint); ok {
			e := entry.(contextCacheEntry)
			if time.Now().Before(e.ExpiresAt) {
				cachedContentName = e.Name
				contents = contents[stablePrefixCount:] // Solo enviar lo nuevo
				logrus.WithFields(logrus.Fields{
					"chat_key":      req.ChatKey,
					"cache_name":    cachedContentName,
					"total_chars":   totalChars,
					"stable_prefix": stablePrefixCount,
				}).Info("[CACHE_HIT] Context cache reutilizado")
			}
		}

		if cachedContentName == "" {
			// Crear nuevo cache
			ttl := 1 * time.Hour
			cReq := &genai.CreateCachedContentConfig{
				Contents: stablePrefix,
				TTL:      ttl,
			}
			if req.SystemPrompt != "" {
				cReq.SystemInstruction = genai.NewContentFromText(req.SystemPrompt, genai.RoleUser)
			}
			if len(functionDecls) > 0 {
				cReq.Tools = []*genai.Tool{{FunctionDeclarations: functionDecls}}
			}

			cache, cErr := client.Caches.Create(ctx, model, cReq)
			if cErr == nil {
				cachedContentName = cache.Name
				p.caches.Store(fingerprint, contextCacheEntry{
					Name:      cache.Name,
					ExpiresAt: time.Now().Add(55 * time.Minute), // Guardar con margen
				})
				contents = contents[stablePrefixCount:] // Solo enviar lo nuevo
				logrus.WithFields(logrus.Fields{
					"chat_key":        req.ChatKey,
					"cache_name":      cachedContentName,
					"total_chars":     totalChars,
					"cached_turns":    stablePrefixCount,
					"remaining_turns": len(contents),
				}).Info("[CACHE_CREATED] Nuevo context cache creado")
			} else {
				logrus.WithField("error", cErr.Error()).Warn("[CACHE_ERROR] No se pudo crear context cache")
			}
		}
	}

	if genConfig == nil {
		genConfig = &genai.GenerateContentConfig{}
	}
	if cachedContentName != "" {
		genConfig.CachedContent = cachedContentName
		// Si usamos cache, el SystemInstruction y Tools ya están en el cache
		genConfig.SystemInstruction = nil
		genConfig.Tools = nil
	}

	p.applyThinking(genConfig, model, "dynamic")

	result, err := p.generateContentWithRetry(ctx, client, model, contents, genConfig)
	if err != nil {
		return domain.ChatResponse{}, err
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
	if result.UsageMetadata != nil {
		resp.Usage = p.extractUsage(model, result.UsageMetadata)

		logrus.WithFields(logrus.Fields{
			"chat_key":      req.ChatKey,
			"model":         model,
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
			"cached_tokens": resp.Usage.CachedTokens,
			"cost_usd":      fmt.Sprintf("$%.6f", resp.Usage.CostUSD),
		}).Info("[USAGE] Token usage recorded")
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
		}).Info("[USAGE] Multimodal cost recorded")
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

	prompt := fmt.Sprintf(`Analyze the following user message and determine the emotional context.
User message: "%s"

CONTEXT:
- Recent History:
%s
- Is bot busy with a previous task? %v
- %s%s

Categorize into:
- pace: 'fast', 'steady', 'deep'.
- focus: true if topic is high priority.
- work: true ONLY if message requires NEW tools or deep analysis.
- acknowledgement: A short, natural, human phrase. 
  RULES FOR ACK:
  1. If PRIMARY LANGUAGE is provided, YOU MUST strictly use it for the acknowledgement.
     - Example (ES): "Un momento, estoy analizando los archivos..."
     - Example (EN): "One moment, I'm analyzing the files..."
     - Example (FR): "Un instant, j'analyse les fichiers..."
  2. ONLY provide if WORK is true. Empty otherwise.
  3. NEVER provide if this message is a direct answer to your previous question or relates to a task in the Agenda.
- enqueue_task: If NEW task is requested while BUSY, describe it. Empty otherwise.
- clear_tasks: true if this message RESOLVES current pending tasks.
- should_respond: Decisions:
  1. If message contains info for a task in the BOT AGENDA, set should_respond=true.
  2. If message contains a NEW COMMAND, UPDATE, or CORRECTION, set should_respond=true.
  3. If WORK is true, set should_respond=true.
  4. Set should_respond=false ONLY if (IS_BUSY is true) AND (message is Trivial: "ok", "vale", "gracias", "merci", "cool", "thanks", "ça marche").
  5. If IS_BUSY is false, ALWAYS set should_respond=true.

Return ONLY a JSON with these fields.`, input.Text, histStr.String(), isBusy, agendaStr.String(), langCtx)

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

	p.applyThinking(cfg, model, "off")

	contents := []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: prompt}}}}
	result, err := p.generateContentWithRetry(ctx, client, model, contents, cfg)
	if err != nil {
		return &domain.Mindset{Pace: "steady"}, nil, nil // Safe fallback
	}

	var usage *domain.UsageStats
	if result.UsageMetadata != nil {
		usage = p.extractUsage(model, result.UsageMetadata)
		logrus.WithFields(logrus.Fields{
			"stage": "intuition",
			"cost":  usage.CostUSD,
		}).Info("[USAGE] Intuition cost recorded")
	}

	var mindset domain.Mindset
	if err := json.Unmarshal([]byte(result.Text()), &mindset); err != nil {
		return &domain.Mindset{Pace: "steady"}, usage, nil
	}

	return &mindset, usage, nil
}

// Privados utilitarios

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

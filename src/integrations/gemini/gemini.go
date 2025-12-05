package gemini

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/genai"
	"google.golang.org/protobuf/proto"
)

type instanceGeminiConfig struct {
	Enabled       bool
	APIKey        string
	Model         string
	SystemPrompt  string
	KnowledgeBase string
	Timezone      string
	AudioEnabled  bool
	ImageEnabled  bool
	MemoryEnabled bool
}

type chatTurn struct {
	Role string
	Text string
}

var (
	chatMemoryMu sync.Mutex
	chatMemory   = make(map[string][]chatTurn)
)

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents,omitempty"`
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func ClearInstanceMemory(instanceID string) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	prefix := instanceID + "|"
	chatMemoryMu.Lock()
	for k := range chatMemory {
		if strings.HasPrefix(k, prefix) {
			delete(chatMemory, k)
		}
	}
	chatMemoryMu.Unlock()
}

func ClearBotMemory(botID string) {
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return
	}
	prefix := fmt.Sprintf("bot|%s|", botID)
	chatMemoryMu.Lock()
	for k := range chatMemory {
		if strings.HasPrefix(k, prefix) {
			delete(chatMemory, k)
		}
	}
	chatMemoryMu.Unlock()
}

func CloseChat(instanceID, chatJID string) {
	instanceID = strings.TrimSpace(instanceID)
	chatJID = strings.TrimSpace(chatJID)
	if instanceID == "" || chatJID == "" {
		return
	}
	key := fmt.Sprintf("%s|%s", instanceID, chatJID)
	chatMemoryMu.Lock()
	delete(chatMemory, key)
	chatMemoryMu.Unlock()
}

func HandleIncomingMessage(ctx context.Context, client *whatsmeow.Client, instanceID string, phone string, evt *events.Message) {
	if evt == nil || client == nil {
		return
	}
	if evt.Info.IsFromMe || evt.Info.IsIncomingBroadcast() || utils.IsGroupJID(evt.Info.Chat.String()) {
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	// Usamos siempre el JID del chat (conversación) como destinatario para evitar device parts.
	recipientJID := utils.FormatJID(evt.Info.Chat.String())
	if recipientJID.String() == "" {
		return
	}
	cfg, err := loadInstanceConfig(ctx, instanceID)
	if err != nil {
		logrus.WithError(err).Error("[GEMINI] failed to load config")
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIKey == "" {
		return
	}
	if img := evt.Message.GetImageMessage(); img != nil && cfg.ImageEnabled {
		media, err := utils.ExtractMedia(ctx, client, config.PathMedia, img)
		if err != nil || strings.TrimSpace(media.MediaPath) == "" {
			return
		}
		info, err := os.Stat(media.MediaPath)
		if err != nil {
			return
		}
		maxImage := config.GeminiMaxImageBytes
		if maxImage > 0 && info.Size() > maxImage {
			return
		}
		imageBytes, err := os.ReadFile(media.MediaPath)
		if err != nil || len(imageBytes) == 0 {
			return
		}
		reply, err := generateReplyFromImage(ctx, cfg, imageBytes, media.MimeType)
		if err != nil {
			logrus.WithError(err).Error("[GEMINI] failed to generate reply from image")
			return
		}
		reply = strings.TrimSpace(reply)
		if reply == "" {
			return
		}
		msg := &waE2E.Message{Conversation: proto.String(reply)}
		if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
			logrus.WithError(err).Error("[GEMINI] failed to send reply")
		}
		return
	}
	if audio := evt.Message.GetAudioMessage(); audio != nil && audio.GetPTT() && cfg.AudioEnabled {
		media, err := utils.ExtractMedia(ctx, client, config.PathMedia, audio)
		if err != nil || strings.TrimSpace(media.MediaPath) == "" {
			return
		}
		info, err := os.Stat(media.MediaPath)
		if err != nil {
			return
		}
		maxAudio := config.GeminiMaxAudioBytes
		if maxAudio > 0 && info.Size() > maxAudio {
			msg := &waE2E.Message{Conversation: proto.String("El audio es muy largo. Por favor envía un mensaje de voz más corto.")}
			if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
				logrus.WithError(err).Error("[GEMINI] failed to send too-long-audio warning")
			}
			return
		}
		audioBytes, err := os.ReadFile(media.MediaPath)
		if err != nil || len(audioBytes) == 0 {
			return
		}
		reply, err := generateReplyFromAudio(ctx, cfg, audioBytes, media.MimeType)
		if err != nil {
			logrus.WithError(err).Error("[GEMINI] failed to generate reply from audio")
			return
		}
		reply = strings.TrimSpace(reply)
		if reply == "" {
			return
		}
		msg := &waE2E.Message{Conversation: proto.String(reply)}
		if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
			logrus.WithError(err).Error("[GEMINI] failed to send reply")
		}
		return
	}
	text := strings.TrimSpace(utils.ExtractMessageTextFromProto(evt.Message))
	if text == "" {
		return
	}
	key := fmt.Sprintf("%s|%s", instanceID, recipientJID.String())
	reply, err := generateReply(ctx, cfg, key, text)
	if err != nil {
		logrus.WithError(err).Error("[GEMINI] failed to generate reply")
		return
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}
	// Si Chatwoot está habilitado para esta instancia, dejamos que Chatwoot sea quien
	// reciba también el mensaje para sincronización, pero seguimos respondiendo
	// directamente en WhatsApp para la conversación activa.
	if chatwoot.IsInstanceEnabled(ctx, instanceID) {
		if strings.TrimSpace(phone) != "" {
			go chatwoot.ForwardBotReplyFromEvent(ctx, instanceID, phone, reply)
		}
	}

	// Sin Chatwoot habilitado: respondemos directamente en WhatsApp como antes.
	msg := &waE2E.Message{Conversation: proto.String(reply)}
	if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
		logrus.WithError(err).Error("[GEMINI] failed to send reply")
		return
	}
}

func loadInstanceConfig(ctx context.Context, instanceID string) (*instanceGeminiConfig, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var enabled, audioEnabled, imageEnabled, memoryEnabled sql.NullInt64
	var apiKey, model, systemPrompt, knowledgeBase, timezone, botID sql.NullString
	query := `SELECT gemini_enabled, gemini_api_key, gemini_model, gemini_system_prompt, gemini_knowledge_base, gemini_timezone, gemini_audio_enabled, gemini_image_enabled, gemini_memory_enabled, bot_id FROM instances WHERE id = ?`
	if err := db.QueryRowContext(ctx, query, instanceID).Scan(&enabled, &apiKey, &model, &systemPrompt, &knowledgeBase, &timezone, &audioEnabled, &imageEnabled, &memoryEnabled, &botID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	instanceEnabled := enabled.Valid && enabled.Int64 != 0
	if !instanceEnabled {
		return nil, nil
	}

	// Si la instancia tiene un Bot AI asignado, usamos la configuración del bot.
	if botID.Valid && strings.TrimSpace(botID.String) != "" {
		botCfg, err := loadBotConfig(ctx, db, strings.TrimSpace(botID.String))
		if err != nil {
			logrus.WithError(err).WithField("instance_id", instanceID).Error("[GEMINI] failed to load bot config, disabling Bot AI for this instance")
			return nil, nil
		}
		if botCfg == nil || !botCfg.Enabled || botCfg.APIKey == "" {
			// Bot deshabilitado o mal configurado: tratamos la IA como deshabilitada para esta instancia.
			return nil, nil
		}
		return botCfg, nil
	}

	// Sin bot_id: usamos la configuración Gemini embebida en la instancia (comportamiento legacy).
	cfg := &instanceGeminiConfig{
		Enabled:       instanceEnabled,
		APIKey:        strings.TrimSpace(apiKey.String),
		Model:         strings.TrimSpace(model.String),
		SystemPrompt:  strings.TrimSpace(systemPrompt.String),
		KnowledgeBase: strings.TrimSpace(knowledgeBase.String),
		Timezone:      strings.TrimSpace(timezone.String),
		AudioEnabled:  audioEnabled.Valid && audioEnabled.Int64 != 0,
		ImageEnabled:  imageEnabled.Valid && imageEnabled.Int64 != 0,
		MemoryEnabled: memoryEnabled.Valid && memoryEnabled.Int64 != 0,
	}
	if !cfg.Enabled || cfg.APIKey == "" {
		return nil, nil
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	return cfg, nil
}

func loadBotConfig(ctx context.Context, db *sql.DB, botID string) (*instanceGeminiConfig, error) {
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return nil, nil
	}
	var (
		enabled, audioEnabled, imageEnabled, memoryEnabled           sql.NullInt64
		apiKey, model, systemPrompt, knowledgeBase, timezone, credID sql.NullString
	)

	query := `SELECT enabled, api_key, model, system_prompt, knowledge_base, timezone, audio_enabled, image_enabled, memory_enabled, credential_id FROM bots WHERE id = ?`
	if err := db.QueryRowContext(ctx, query, botID).Scan(&enabled, &apiKey, &model, &systemPrompt, &knowledgeBase, &timezone, &audioEnabled, &imageEnabled, &memoryEnabled, &credID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Construimos la configuración base del bot.
	cfg := &instanceGeminiConfig{
		Enabled:       enabled.Valid && enabled.Int64 != 0,
		APIKey:        strings.TrimSpace(apiKey.String),
		Model:         strings.TrimSpace(model.String),
		SystemPrompt:  strings.TrimSpace(systemPrompt.String),
		KnowledgeBase: strings.TrimSpace(knowledgeBase.String),
		Timezone:      strings.TrimSpace(timezone.String),
		AudioEnabled:  audioEnabled.Valid && audioEnabled.Int64 != 0,
		ImageEnabled:  imageEnabled.Valid && imageEnabled.Int64 != 0,
		MemoryEnabled: memoryEnabled.Valid && memoryEnabled.Int64 != 0,
	}

	// Si el bot tiene credential_id, intentamos obtener la API key desde la tabla credentials.
	if credID.Valid && strings.TrimSpace(credID.String) != "" {
		credRef := strings.TrimSpace(credID.String)
		// Verificamos de forma segura si existe la tabla credentials antes de consultar.
		var tblName string
		if err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name='credentials'",
		).Scan(&tblName); err == nil && tblName == "credentials" {
			var credAPIKey sql.NullString
			if err := db.QueryRowContext(ctx,
				"SELECT gemini_api_key FROM credentials WHERE id = ?",
				credRef,
			).Scan(&credAPIKey); err == nil {
				if credAPIKey.Valid && strings.TrimSpace(credAPIKey.String) != "" {
					cfg.APIKey = strings.TrimSpace(credAPIKey.String)
				}
			}
		}
	}
	if !cfg.Enabled || cfg.APIKey == "" {
		return nil, nil
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	return cfg, nil
}

func GenerateBotTextReply(ctx context.Context, botID string, memoryID string, input string) (string, error) {
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return "", fmt.Errorf("botID: cannot be blank")
	}

	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath))
	if err != nil {
		return "", err
	}
	defer db.Close()

	cfg, err := loadBotConfig(ctx, db, botID)
	if err != nil {
		return "", err
	}
	if cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.APIKey) == "" {
		return "", fmt.Errorf("bot AI is disabled or misconfigured")
	}

	memoryID = strings.TrimSpace(memoryID)
	memoryKey := ""
	if cfg.MemoryEnabled && memoryID != "" {
		memoryKey = fmt.Sprintf("bot|%s|%s", botID, memoryID)
	}

	return generateReply(ctx, cfg, memoryKey, input)
}

func generateReply(ctx context.Context, cfg *instanceGeminiConfig, memoryKey string, input string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	var genConfig *genai.GenerateContentConfig
	systemText := strings.TrimSpace(config.GeminiGlobalSystemPrompt)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.SystemPrompt
		} else {
			systemText = cfg.SystemPrompt
		}
	}
	if strings.TrimSpace(cfg.KnowledgeBase) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.KnowledgeBase
		} else {
			systemText = cfg.KnowledgeBase
		}
	}
	tz := strings.TrimSpace(cfg.Timezone)
	if tz == "" {
		tz = strings.TrimSpace(config.GeminiTimezone)
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	currentTimeText := fmt.Sprintf("Hora actual del sistema (%s): %s", tz, now.Format("2006-01-02 15:04"))
	if systemText != "" {
		systemText = currentTimeText + "\n\n" + systemText
	} else {
		systemText = currentTimeText
	}
	if systemText != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemText, genai.RoleUser),
		}
	}

	var contents []*genai.Content
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		history = append(history, chatTurn{Role: "user", Text: input})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		chatMemory[memoryKey] = history
		chatMemoryMu.Unlock()

		for _, t := range history {
			role := genai.RoleUser
			if t.Role == "assistant" {
				role = genai.RoleModel
			}
			contents = append(contents, &genai.Content{
				Role: role,
				Parts: []*genai.Part{
					{Text: t.Text},
				},
			})
		}
	} else {
		contents = []*genai.Content{
			{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					{Text: input},
				},
			},
		}
	}

	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		closeChatFunc := &genai.FunctionDeclaration{
			Name:        "close_chat",
			Description: "Signals that the current WhatsApp conversation is finished and memory for this chat should be cleared.",
			Parameters: &genai.Schema{
				Type: "object",
				Properties: map[string]*genai.Schema{
					"reason": {Type: "string"},
				},
			},
		}
		if genConfig == nil {
			genConfig = &genai.GenerateContentConfig{}
		}
		genConfig.Tools = []*genai.Tool{
			{
				FunctionDeclarations: []*genai.FunctionDeclaration{closeChatFunc},
			},
		}
	}

	result, err := client.Models.GenerateContent(ctx, cfg.Model, contents, genConfig)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	closed := false
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			for _, p := range result.Candidates[0].Content.Parts {
				if p.FunctionCall != nil && p.FunctionCall.Name == "close_chat" {
					chatMemoryMu.Lock()
					delete(chatMemory, memoryKey)
					chatMemoryMu.Unlock()
					closed = true
					break
				}
			}
		}
	}
	text := strings.TrimSpace(result.Text())
	if text != "" && cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" && !closed {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		history = append(history, chatTurn{Role: "assistant", Text: text})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		chatMemory[memoryKey] = history
		chatMemoryMu.Unlock()
	}
	return text, nil
}

func generateReplyFromAudio(ctx context.Context, cfg *instanceGeminiConfig, audioBytes []byte, mimeType string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}
	var genConfig *genai.GenerateContentConfig
	systemText := strings.TrimSpace(config.GeminiGlobalSystemPrompt)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.SystemPrompt
		} else {
			systemText = cfg.SystemPrompt
		}
	}
	if strings.TrimSpace(cfg.KnowledgeBase) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.KnowledgeBase
		} else {
			systemText = cfg.KnowledgeBase
		}
	}
	tz := strings.TrimSpace(cfg.Timezone)
	if tz == "" {
		tz = strings.TrimSpace(config.GeminiTimezone)
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	currentTimeText := fmt.Sprintf("Hora actual del sistema (%s): %s", tz, now.Format("2006-01-02 15:04"))
	if systemText != "" {
		systemText = currentTimeText + "\n\n" + systemText
	} else {
		systemText = currentTimeText
	}
	if systemText != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemText, genai.RoleUser),
		}
	}
	prompt := "Transcribe brevemente este audio y responde al usuario en texto, en español, como si hubiera escrito ese mensaje."
	contents := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: prompt},
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: audioBytes}},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, cfg.Model, contents, genConfig)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return strings.TrimSpace(result.Text()), nil
}

func generateReplyFromImage(ctx context.Context, cfg *instanceGeminiConfig, imageBytes []byte, mimeType string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}
	var genConfig *genai.GenerateContentConfig
	systemText := strings.TrimSpace(config.GeminiGlobalSystemPrompt)
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.SystemPrompt
		} else {
			systemText = cfg.SystemPrompt
		}
	}
	if strings.TrimSpace(cfg.KnowledgeBase) != "" {
		if systemText != "" {
			systemText = systemText + "\n\n" + cfg.KnowledgeBase
		} else {
			systemText = cfg.KnowledgeBase
		}
	}
	tz := strings.TrimSpace(cfg.Timezone)
	if tz == "" {
		tz = strings.TrimSpace(config.GeminiTimezone)
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	currentTimeText := fmt.Sprintf("Hora actual del sistema (%s): %s", tz, now.Format("2006-01-02 15:04"))
	if systemText != "" {
		systemText = currentTimeText + "\n\n" + systemText
	} else {
		systemText = currentTimeText
	}
	if systemText != "" {
		genConfig = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(systemText, genai.RoleUser),
		}
	}
	prompt := "Analiza y describe brevemente esta imagen y responde al usuario en texto, en español. Si la imagen contiene texto relevante, transcríbelo de forma resumida."
	contents := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{Text: prompt},
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageBytes}},
			},
		},
	}
	result, err := client.Models.GenerateContent(ctx, cfg.Model, contents, genConfig)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return strings.TrimSpace(result.Text()), nil
}

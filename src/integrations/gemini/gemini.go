package gemini

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
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
	chatStr := evt.Info.Chat.String()
	if evt.Info.IsFromMe || evt.Info.IsIncomingBroadcast() || utils.IsGroupJID(chatStr) {
		return
	}
	src := strings.ToLower(strings.TrimSpace(evt.Info.SourceString()))
	if strings.HasPrefix(chatStr, "status@") ||
		strings.HasSuffix(chatStr, "@broadcast") ||
		strings.Contains(src, "status@broadcast") ||
		strings.EqualFold(strings.TrimSpace(evt.Info.Category), "status") {
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	// Usamos siempre el JID del chat (conversación) como destinatario para evitar device parts.
	recipientJID := utils.FormatJID(chatStr)
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

	provider := "gemini"
	chatJID := recipientJID.String()
	traceID := string(evt.Info.ID)

	if img := evt.Message.GetImageMessage(); img != nil && cfg.ImageEnabled {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "inbound", Kind: "image", Status: "ok"})
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_request", Kind: "image", Status: "ok"})
		start := time.Now()
		media, err := utils.ExtractMedia(ctx, client, config.PathMedia, img)
		if err != nil || strings.TrimSpace(media.MediaPath) == "" {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "image", Status: "error", Error: "extract_media_failed", DurationMs: time.Since(start).Milliseconds()})
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
		key := fmt.Sprintf("%s|%s", instanceID, recipientJID.String())
		reply, err := generateReplyFromImage(ctx, cfg, key, imageBytes, media.MimeType)
		if err != nil {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "image", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
			logrus.WithError(err).Error("[GEMINI] failed to generate reply from image")
			return
		}
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "image", Status: "ok", DurationMs: time.Since(start).Milliseconds()})
		reply = strings.TrimSpace(reply)
		if reply == "" {
			return
		}
		if ok := simulateHumanTyping(jobCtxOrCtx(ctx), client, recipientJID, reply); !ok {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if chatwoot.IsInstanceEnabled(ctx, instanceID) {
			if strings.TrimSpace(phone) != "" {
				botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "chatwoot", Status: "ok"})
				go chatwoot.ForwardBotReplyFromEvent(ctx, instanceID, phone, reply)
			}
		}
		msg := &waE2E.Message{Conversation: proto.String(reply)}
		sendStart := time.Now()
		if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "error", Error: err.Error(), DurationMs: time.Since(sendStart).Milliseconds()})
			logrus.WithError(err).Error("[GEMINI] failed to send reply")
		} else {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "ok", DurationMs: time.Since(sendStart).Milliseconds()})
		}
		return
	}
	if audio := evt.Message.GetAudioMessage(); audio != nil && audio.GetPTT() && cfg.AudioEnabled {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "inbound", Kind: "audio", Status: "ok"})
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_request", Kind: "audio", Status: "ok"})
		start := time.Now()
		media, err := utils.ExtractMedia(ctx, client, config.PathMedia, audio)
		if err != nil || strings.TrimSpace(media.MediaPath) == "" {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "audio", Status: "error", Error: "extract_media_failed", DurationMs: time.Since(start).Milliseconds()})
			return
		}
		info, err := os.Stat(media.MediaPath)
		if err != nil {
			return
		}
		maxAudio := config.GeminiMaxAudioBytes
		if maxAudio > 0 && info.Size() > maxAudio {
			msg := &waE2E.Message{Conversation: proto.String("The audio is too long. Please send a shorter voice message.")}
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_request", Kind: "audio", Status: "skipped"})
			sendStart := time.Now()
			if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
				botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "error", Error: err.Error(), DurationMs: time.Since(sendStart).Milliseconds()})
				logrus.WithError(err).Error("[GEMINI] failed to send too-long-audio warning")
			} else {
				botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "ok", DurationMs: time.Since(sendStart).Milliseconds()})
			}
			return
		}
		audioBytes, err := os.ReadFile(media.MediaPath)
		if err != nil || len(audioBytes) == 0 {
			return
		}
		key := fmt.Sprintf("%s|%s", instanceID, recipientJID.String())
		reply, err := generateReplyFromAudio(ctx, cfg, key, audioBytes, media.MimeType)
		if err != nil {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "audio", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
			logrus.WithError(err).Error("[GEMINI] failed to generate reply from audio")
			return
		}
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "audio", Status: "ok", DurationMs: time.Since(start).Milliseconds()})
		reply = strings.TrimSpace(reply)
		if reply == "" {
			return
		}
		if ok := simulateHumanTyping(jobCtxOrCtx(ctx), client, recipientJID, reply); !ok {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if chatwoot.IsInstanceEnabled(ctx, instanceID) {
			if strings.TrimSpace(phone) != "" {
				botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "chatwoot", Status: "ok"})
				go chatwoot.ForwardBotReplyFromEvent(ctx, instanceID, phone, reply)
			}
		}
		msg := &waE2E.Message{Conversation: proto.String(reply)}
		sendStart := time.Now()
		if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "error", Error: err.Error(), DurationMs: time.Since(sendStart).Milliseconds()})
			logrus.WithError(err).Error("[GEMINI] failed to send reply")
		} else {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "ok", DurationMs: time.Since(sendStart).Milliseconds()})
		}
		return
	}
	text := strings.TrimSpace(utils.ExtractMessageTextFromProto(evt.Message))
	if text == "" {
		return
	}
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "inbound", Kind: "text", Status: "ok"})
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_request", Kind: "text", Status: "ok"})
	start := time.Now()
	key := fmt.Sprintf("%s|%s", instanceID, recipientJID.String())
	reply, err := generateReply(ctx, cfg, key, text)
	if err != nil {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
		logrus.WithError(err).Error("[GEMINI] failed to generate reply")
		return
	}
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "ok", DurationMs: time.Since(start).Milliseconds()})
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}
	if ok := simulateHumanTyping(jobCtxOrCtx(ctx), client, recipientJID, reply); !ok {
		return
	}
	if ctx.Err() != nil {
		return
	}
	// Si Chatwoot está habilitado para esta instancia, dejamos que Chatwoot sea quien
	// reciba también el mensaje para sincronización, pero seguimos respondiendo
	// directamente en WhatsApp para la conversación activa.
	if chatwoot.IsInstanceEnabled(ctx, instanceID) {
		if strings.TrimSpace(phone) != "" {
			botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "chatwoot", Status: "ok"})
			go chatwoot.ForwardBotReplyFromEvent(ctx, instanceID, phone, reply)
		}
	}

	// Sin Chatwoot habilitado: respondemos directamente en WhatsApp como antes.
	msg := &waE2E.Message{Conversation: proto.String(reply)}
	sendStart := time.Now()
	if _, err := client.SendMessage(ctx, recipientJID, msg); err != nil {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "error", Error: err.Error(), DurationMs: time.Since(sendStart).Milliseconds()})
		logrus.WithError(err).Error("[GEMINI] failed to send reply")
		return
	}
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: instanceID, ChatJID: chatJID, Provider: provider, Stage: "outbound", Kind: "whatsapp", Status: "ok", DurationMs: time.Since(sendStart).Milliseconds()})
}

func jobCtxOrCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func simulateHumanTyping(ctx context.Context, client *whatsmeow.Client, jid types.JID, text string) bool {
	if client == nil {
		return true
	}
	if !config.GeminiTypingEnabled {
		return true
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	initialDelay := time.Duration(50+rng.Intn(80)) * time.Millisecond
	if !sleepWithContext(ctx, initialDelay) {
		stopTypingPresence(client, jid)
		return false
	}
	_ = client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	defer stopTypingPresence(client, jid)

	perCharBase := time.Duration(12+rng.Intn(10)) * time.Millisecond
	maxChars := utf8.RuneCountInString(text)
	if maxChars <= 0 {
		maxChars = len(text)
	}
	if maxChars < 20 {
		perCharBase = time.Duration(8+rng.Intn(8)) * time.Millisecond
	}

	segmentChars := 0
	segmentWords := 0
	lastWasSpace := true
	lastCharWasNewline := false

	nextBreakWords := 18 + rng.Intn(15)

	flushSegment := func(perChar time.Duration) bool {
		if segmentChars <= 0 {
			return true
		}
		d := time.Duration(segmentChars) * perChar
		segmentChars = 0
		segmentWords = 0
		if d > 3*time.Second {
			d = 3 * time.Second
		}
		return sleepWithContext(ctx, d)
	}

	pause := func(minMs, maxMs int) bool {
		ms := minMs
		if maxMs > minMs {
			ms = minMs + rng.Intn(maxMs-minMs+1)
		}
		if ms >= 200 {
			_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
		}
		if !sleepWithContext(ctx, time.Duration(ms)*time.Millisecond) {
			return false
		}
		if ms >= 200 {
			_ = client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		}
		return true
	}

	perChar := perCharBase
	for _, r := range text {
		segmentChars++
		isSpace := r == ' ' || r == '\t' || r == '\r' || r == '\n'
		if isSpace {
			if !lastWasSpace {
				segmentWords++
			}
			lastWasSpace = true
		} else {
			lastWasSpace = false
		}

		if segmentWords >= nextBreakWords {
			perChar = perCharBase + time.Duration(rng.Intn(8))*time.Millisecond
			if !flushSegment(perChar) {
				return false
			}
			if rng.Intn(100) < 30 {
				if !pause(120, 280) {
					return false
				}
			}
			segmentWords = 0
			nextBreakWords = 18 + rng.Intn(15)
			continue
		}

		switch r {
		case '.', '!', '?':
			if rng.Intn(100) < 40 {
				perChar = perCharBase + time.Duration(rng.Intn(10))*time.Millisecond
				if !flushSegment(perChar) {
					return false
				}
				if !pause(150, 320) {
					return false
				}
			}
		case '\n':
			perChar = perCharBase + time.Duration(rng.Intn(8))*time.Millisecond
			if !flushSegment(perChar) {
				return false
			}
			if lastCharWasNewline {
				if !pause(280, 600) {
					return false
				}
			} else {
				if !pause(180, 400) {
					return false
				}
			}
			lastCharWasNewline = true
			continue
		default:
			lastCharWasNewline = false
		}
	}

	perChar = perCharBase + time.Duration(rng.Intn(10))*time.Millisecond
	if !flushSegment(perChar) {
		return false
	}

	_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	finalPause := time.Duration(80+rng.Intn(150)) * time.Millisecond
	if !sleepWithContext(ctx, finalPause) {
		return false
	}
	return true
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func stopTypingPresence(client *whatsmeow.Client, jid types.JID) {
	if client == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = client.SendChatPresence(stopCtx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
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

	provider := "gemini"
	traceID := fmt.Sprintf("webhook:%s:%d", botID, time.Now().UnixNano())
	monitorInstanceID := "bot:" + botID
	monitorChatID := strings.TrimSpace(memoryID)
	if monitorChatID == "" {
		monitorChatID = "(no-memory-id)"
	}
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "inbound", Kind: "webhook", Status: "ok"})
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "ai_request", Kind: "text", Status: "ok"})
	start := time.Now()

	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath))
	if err != nil {
		return "", err
	}
	defer db.Close()

	cfg, err := loadBotConfig(ctx, db, botID)
	if err != nil {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "outbound", Kind: "webhook", Status: "error", Error: err.Error()})
		return "", err
	}
	if cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.APIKey) == "" {
		err := fmt.Errorf("bot AI is disabled or misconfigured")
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "outbound", Kind: "webhook", Status: "error", Error: err.Error()})
		return "", err
	}

	memoryID = strings.TrimSpace(memoryID)
	memoryKey := ""
	if cfg.MemoryEnabled && memoryID != "" {
		memoryKey = fmt.Sprintf("bot|%s|%s", botID, memoryID)
	}

	reply, err := generateReply(ctx, cfg, memoryKey, input)
	if err != nil {
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "error", Error: err.Error(), DurationMs: time.Since(start).Milliseconds()})
		botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "outbound", Kind: "webhook", Status: "error", Error: err.Error()})
		return "", err
	}
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "ai_response", Kind: "text", Status: "ok", DurationMs: time.Since(start).Milliseconds()})
	botmonitor.Record(botmonitor.Event{TraceID: traceID, InstanceID: monitorInstanceID, ChatJID: monitorChatID, Provider: provider, Stage: "outbound", Kind: "webhook", Status: "ok"})
	return reply, nil
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
	weekday := now.Format("Monday")
	currentTimeText := fmt.Sprintf("IMPORTANT - Current date and time (%s timezone): %s, %s %d, %d at %s (Day of week: %s)",
		tz,
		weekday,
		now.Format("January"),
		now.Day(),
		now.Year(),
		now.Format("15:04"),
		weekday)
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
			Description: "Call this function when the conversation is finished (user says goodbye, thanks, or explicitly ends the chat). You MUST provide a farewell_message in the same language the user is speaking.",
			Parameters: &genai.Schema{
				Type: "object",
				Properties: map[string]*genai.Schema{
					"farewell_message": {
						Type:        "string",
						Description: "A friendly farewell message to the user in their language. Do NOT mention memory, data deletion, or technical details. Just say goodbye naturally.",
					},
				},
				Required: []string{"farewell_message"},
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
	var farewellMsg string
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		if len(result.Candidates) > 0 && result.Candidates[0].Content != nil {
			for _, p := range result.Candidates[0].Content.Parts {
				if p.FunctionCall != nil && p.FunctionCall.Name == "close_chat" {
					if args := p.FunctionCall.Args; args != nil {
						if fw, ok := args["farewell_message"]; ok {
							if fwStr, ok := fw.(string); ok {
								farewellMsg = strings.TrimSpace(fwStr)
							}
						}
					}
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
	if closed && farewellMsg != "" {
		text = farewellMsg
	}
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

type audioResponse struct {
	Transcription string `json:"transcription"`
	Response      string `json:"response"`
}

func generateReplyFromAudio(ctx context.Context, cfg *instanceGeminiConfig, memoryKey string, audioBytes []byte, mimeType string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	// Build system prompt
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
	weekday := now.Format("Monday")
	currentTimeText := fmt.Sprintf("IMPORTANT - Current date and time (%s timezone): %s, %s %d, %d at %s (Day of week: %s)",
		tz,
		weekday,
		now.Format("January"),
		now.Day(),
		now.Year(),
		now.Format("15:04"),
		weekday)
	if systemText != "" {
		systemText = currentTimeText + "\n\n" + systemText
	} else {
		systemText = currentTimeText
	}

	// Build contents with history if memory is enabled
	var contents []*genai.Content
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		chatMemoryMu.Unlock()
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
	}

	// Add the audio message with structured output prompt
	prompt := `Listen to this voice message and do the following:
1. Transcribe literally what the user says in the audio.
2. Reply to the user as if they had typed that message.

Return a JSON object with fields "transcription" and "response".

Important: The "response" MUST be written in the same language the user is speaking.`

	contents = append(contents, &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			{Text: prompt},
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: audioBytes}},
		},
	})

	// Configure structured output
	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"transcription": {
					Type:        "string",
					Description: "A literal transcription of what the user says in the audio",
				},
				"response": {
					Type:        "string",
					Description: "A helpful reply to the user based on what they said in the audio (must be in the user's language)",
				},
			},
			Required:         []string{"transcription", "response"},
			PropertyOrdering: []string{"transcription", "response"},
		},
	}
	if systemText != "" {
		genConfig.SystemInstruction = genai.NewContentFromText(systemText, genai.RoleUser)
	}

	result, err := client.Models.GenerateContent(ctx, cfg.Model, contents, genConfig)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}

	// Parse structured response
	var audioResp audioResponse
	if err := json.Unmarshal([]byte(result.Text()), &audioResp); err != nil {
		logrus.WithError(err).Warn("[GEMINI] Failed to parse audio structured response, using raw text")
		return strings.TrimSpace(result.Text()), nil
	}

	transcription := strings.TrimSpace(audioResp.Transcription)
	response := strings.TrimSpace(audioResp.Response)

	logrus.Infof("[GEMINI] Audio transcription: %s", transcription)

	// Store both transcription and response in memory
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" && transcription != "" {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		history = append(history, chatTurn{Role: "user", Text: transcription})
		if response != "" {
			history = append(history, chatTurn{Role: "assistant", Text: response})
		}
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		chatMemory[memoryKey] = history
		chatMemoryMu.Unlock()
	}

	return response, nil
}

type imageResponse struct {
	Description string `json:"description"`
	Response    string `json:"response"`
}

func generateReplyFromImage(ctx context.Context, cfg *instanceGeminiConfig, memoryKey string, imageBytes []byte, mimeType string) (string, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", err
	}

	// Build system prompt
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
	weekday := now.Format("Monday")
	currentTimeText := fmt.Sprintf("IMPORTANT - Current date and time (%s timezone): %s, %s %d, %d at %s (Day of week: %s)",
		tz,
		weekday,
		now.Format("January"),
		now.Day(),
		now.Year(),
		now.Format("15:04"),
		weekday)
	if systemText != "" {
		systemText = currentTimeText + "\n\n" + systemText
	} else {
		systemText = currentTimeText
	}

	// Build contents with history if memory is enabled
	var contents []*genai.Content
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		chatMemoryMu.Unlock()
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
	}

	// Add the image message with structured output prompt
	prompt := `Look at this image and do the following:
1. Describe exactly what you see in the image (objects, text, people, context, etc.)
2. Reply to the user based on the image content.

Return a JSON object with fields "description" and "response".

Important: The "response" MUST be written in the same language the user is speaking.`

	contents = append(contents, &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			{Text: prompt},
			{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageBytes}},
		},
	})

	// Configure structured output
	genConfig := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseJsonSchema: &genai.Schema{
			Type: "object",
			Properties: map[string]*genai.Schema{
				"description": {
					Type:        "string",
					Description: "A detailed description of what is visible in the image (objects, text, people, context, etc.)",
				},
				"response": {
					Type:        "string",
					Description: "A helpful reply to the user based on the image content (must be in the user's language)",
				},
			},
			Required:         []string{"description", "response"},
			PropertyOrdering: []string{"description", "response"},
		},
	}
	if systemText != "" {
		genConfig.SystemInstruction = genai.NewContentFromText(systemText, genai.RoleUser)
	}

	result, err := client.Models.GenerateContent(ctx, cfg.Model, contents, genConfig)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}

	// Parse structured response
	var imgResp imageResponse
	if err := json.Unmarshal([]byte(result.Text()), &imgResp); err != nil {
		logrus.WithError(err).Warn("[GEMINI] Failed to parse image structured response, using raw text")
		return strings.TrimSpace(result.Text()), nil
	}

	description := strings.TrimSpace(imgResp.Description)
	response := strings.TrimSpace(imgResp.Response)

	logrus.Infof("[GEMINI] Image description: %s", description)

	// Store both description and response in memory
	if cfg.MemoryEnabled && strings.TrimSpace(memoryKey) != "" && description != "" {
		chatMemoryMu.Lock()
		history := chatMemory[memoryKey]
		history = append(history, chatTurn{Role: "user", Text: "[Image] " + description})
		if response != "" {
			history = append(history, chatTurn{Role: "assistant", Text: response})
		}
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
		chatMemory[memoryKey] = history
		chatMemoryMu.Unlock()
	}

	return response, nil
}

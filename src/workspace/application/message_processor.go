package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	channelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	commonDomain "github.com/AzielCF/az-wap/workspace/domain/common"
	messageDomain "github.com/AzielCF/az-wap/workspace/domain/message"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/sirupsen/logrus"
)

type MessageProcessor struct {
	repo         workspaceDomain.IWorkspaceRepository
	orchestrator *SessionOrchestrator
}

func NewMessageProcessor(repo workspaceDomain.IWorkspaceRepository, orch *SessionOrchestrator) *MessageProcessor {
	return &MessageProcessor{
		repo:         repo,
		orchestrator: orch,
	}
}

func (p *MessageProcessor) ProcessFinal(ctx context.Context, ch channelDomain.Channel, msg messageDomain.IncomingMessage, botID string, botProcess func(context.Context, botengineDomain.BotInput) (botengineDomain.BotOutput, error), markRead func(context.Context, string, []string), closeSession func(context.Context, string, string) error) (botengineDomain.BotOutput, error) {
	logrus.WithFields(logrus.Fields{
		"channel_id": ch.ID,
		"bot_id":     botID,
		"sender_id":  msg.SenderID,
		"mode":       ch.Config.AccessMode,
	}).Debug("[MessageProcessor] Entering ProcessFinal")

	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("[PANIC DEBUG] ProcessFinal panicked: %v\nStack trace:\n%s", r, string(debug.Stack()))
			panic(r) // Re-throw to let worker handle it
		}
	}()

	key := ch.ID + "|" + msg.ChatID + "|" + msg.SenderID

	// Chatwoot Forwarding
	if ch.Config.Chatwoot != nil && ch.Config.Chatwoot.Enabled {
		phone, _ := msg.Metadata["sender_id"].(string)
		if phone == "" {
			phone = msg.SenderID
		}
		cwCfg := &chatwoot.Config{
			InstanceID:         ch.ID,
			BaseURL:            ch.Config.Chatwoot.URL,
			AccountID:          int64(ch.Config.Chatwoot.AccountID),
			InboxID:            int64(ch.Config.Chatwoot.InboxID),
			AccountToken:       ch.Config.Chatwoot.Token,
			BotToken:           ch.Config.Chatwoot.BotToken,
			InboxIdentifier:    ch.Config.Chatwoot.InboxIdentifier,
			Enabled:            ch.Config.Chatwoot.Enabled,
			InsecureSkipVerify: ch.Config.SkipTLSVerification,
			CredentialID:       ch.Config.Chatwoot.CredentialID,
		}
		// NOTE: Simplifying config copy to keep focus on moving logic
		name, _ := msg.Metadata["sender_name"].(string)
		chatwoot.ForwardIncomingMessageWithConfig(ctx, cwCfg, phone, name, msg.Text, nil)
	}

	entry, hasSession := p.orchestrator.GetEntry(key)
	currentFocus := 0
	lastBubbleCount := 0
	if hasSession {
		currentFocus = entry.FocusScore
		lastBubbleCount = entry.LastBubbleCount
	}

	// Initialize metadata proactively to prevent nil map assignments
	safeMetadata := make(map[string]any)
	if msg.Metadata != nil {
		for k, v := range msg.Metadata {
			safeMetadata[k] = v
		}
	}

	input := botengineDomain.BotInput{
		BotID:       botID,
		WorkspaceID: ch.WorkspaceID,
		TraceID:     fmt.Sprintf("%v", safeMetadata["message_id"]),
		InstanceID:  ch.ID,
		ChatID:      msg.ChatID,
		SenderID:    msg.SenderID,
		Platform:    botengineDomain.PlatformWhatsApp,
		Text:        msg.Text,
		Metadata:    safeMetadata,
		FocusScore:  currentFocus,
		Language:    ch.Config.DefaultLanguage, // Default from channel
	}

	// Attach ClientContext if present in metadata
	if cc, ok := safeMetadata["client_context"].(*botengineDomain.ClientContext); ok {
		input.ClientContext = cc
		if cc.Language != "" {
			input.Language = cc.Language // Override with client preference
		}
		logrus.WithFields(logrus.Fields{
			"client_id":     cc.ClientID,
			"display_name":  cc.DisplayName,
			"is_registered": cc.IsRegistered,
			"has_sub":       cc.HasSubscription,
		}).Debugf("[MessageProcessor] Attaching ClientContext to BotInput")
	} else if safeMetadata["client_context"] != nil {
		logrus.Warnf("[MessageProcessor] client_context found in metadata but type is %T, expected *botengineDomain.ClientContext", safeMetadata["client_context"])
	}

	if input.Language == "" {
		input.Language = "en"
	}

	if input.Metadata == nil {
		input.Metadata = make(map[string]any)
	}
	input.Metadata["last_bubble_count"] = lastBubbleCount
	input.Metadata["trace_id"] = input.TraceID // Ensure trace is in metadata too

	input.OnChatOpen = func() {
		if entry, ok := p.orchestrator.GetEntry(key); ok {
			entry.ChatOpen = true
			if len(entry.MessageIDs) > 0 && markRead != nil {
				markRead(ctx, msg.ChatID, entry.MessageIDs)
			}
		}
	}

	if hasSession {
		input.LastMindset = entry.LastMindset
		input.PendingTasks = entry.PendingTasks
		input.LastReplyTime = entry.LastReplyTime
		input.History = entry.Memory.GetHistory()

		// Resources
		rawResources := entry.Memory.GetResources()
		var resList []map[string]string
		for _, r := range rawResources {
			resList = append(resList, map[string]string{
				"name": r.FriendlyName,
				"mime": r.MimeType,
				"hash": r.FileHash,
				"path": r.LocalPath,
			})
		}
		input.Metadata["session_resources"] = resList
	}

	if msg.Media != nil {
		if m := p.loadBotMedia(msg.Media); m != nil {
			input.Medias = append(input.Medias, m)
		}
	}
	for _, m := range msg.Medias {
		if bm := p.loadBotMedia(m); bm != nil {
			input.Medias = append(input.Medias, bm)
		}
	}

	output, err := botProcess(ctx, input)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"channel_id": ch.ID,
			"error":      err,
			"trace_id":   input.TraceID,
		}).Error("[MessageProcessor] Bot Process failed")
		return output, err
	}

	logrus.WithFields(logrus.Fields{
		"channel_id": ch.ID,
		"trace_id":   input.TraceID,
		"action":     output.Action,
	}).Debug("[MessageProcessor] Bot message processed successfully")

	// Handle Session Termination Action from AI
	if output.Action == "terminate_session" {
		logrus.Infof("[MessageProcessor] Terminating session %s per IA request", key)
		if closeSession != nil {
			_ = closeSession(ctx, ch.ID, msg.ChatID)
		}
		return output, nil
	}

	if entry, ok := p.orchestrator.GetEntry(key); ok {
		entry.LastReplyTime = time.Now()
		entry.LastMindset = output.Mindset
		if output.Mindset != nil {
			if output.Mindset.EnqueueTask != "" {
				entry.PendingTasks = append(entry.PendingTasks, output.Mindset.EnqueueTask)
			}
			if output.Mindset.ClearTasks {
				entry.PendingTasks = nil
			}
			if output.Mindset.Focus {
				entry.FocusScore += 25
			}
			if output.Mindset.Pace == "fast" {
				entry.FocusScore += 10
			}
			if entry.FocusScore > 100 {
				entry.FocusScore = 100
			}
		}

		// Memory
		if input.Text != "" {
			entry.Memory.AddTurn("user", input.Text, 10)
		}
		if output.Text != "" {
			entry.Memory.AddTurn("assistant", output.Text, 10)
		}
	}

	return output, nil
}

func (p *MessageProcessor) loadBotMedia(m *messageDomain.IncomingMedia) *botengineDomain.BotMedia {
	if m == nil {
		return nil
	}
	state := botengineDomain.MediaStateAvailable
	if m.Blocked {
		state = botengineDomain.MediaStateBlocked
	}
	var data []byte
	if !m.Blocked && m.Path != "" {
		var err error
		data, err = os.ReadFile(m.Path)
		if err != nil {
			logrus.Errorf("[MessageProcessor] Failed to read media file at %s: %v", m.Path, err)
			return nil
		}
	}
	return &botengineDomain.BotMedia{
		Data:      data,
		MimeType:  m.MimeType,
		FileName:  filepath.Base(m.Path),
		LocalPath: m.Path,
		State:     state,
	}
}

func (p *MessageProcessor) PrepareSessionFile(workspaceID, channelID, sessionKey string, fileName string, friendlyName string, mimeType string, fileHash string) (string, error) {
	entry, _ := p.orchestrator.GetOrCreateEntry(sessionKey, messageDomain.IncomingMessage{ChatID: "temp"})

	if entry.SessionPath == "" {
		hash := sha256.Sum256([]byte(sessionKey))
		sessionID := hex.EncodeToString(hash[:])[:8]
		entry.SessionPath = filepath.Join("statics", "workspaces", workspaceID, channelID, sessionID)
	}

	if err := os.MkdirAll(entry.SessionPath, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(entry.SessionPath, fileName)
	entry.DownloadedFiles = append(entry.DownloadedFiles, fullPath)

	if friendlyName != "" && fileHash != "" {
		entry.Memory.AddResource(friendlyName, fileHash, mimeType, fullPath)
	}

	return fullPath, nil
}

func (p *MessageProcessor) IsAccessAllowed(ctx context.Context, ch channelDomain.Channel, senderID string) bool {
	mode := ch.Config.AccessMode
	if mode == "" {
		mode = channelDomain.AccessModePrivate
	}

	rules, err := p.repo.GetAccessRules(ctx, ch.ID)
	if err != nil {
		logrus.WithError(err).Error("[MessageProcessor] Failed to get access rules")
		return mode == channelDomain.AccessModePublic
	}

	senderID = p.normalizeIdentity(senderID)
	senderPhone := pkgUtils.ExtractPhoneNumber(senderID)

	var match *commonDomain.AccessRule
	for _, r := range rules {
		identityList := strings.Split(r.Identity, "|")
		found := false
		for _, identity := range identityList {
			normRule := p.normalizeIdentity(identity)
			rulePhone := pkgUtils.ExtractPhoneNumber(identity)
			if normRule == senderID || (normRule != "" && strings.Contains(senderID, normRule)) || (senderID != "" && strings.Contains(normRule, senderID)) || (senderPhone != "" && rulePhone != "" && senderPhone == rulePhone) {
				found = true
				break
			}
		}
		if found {
			match = &r
			break
		}
	}

	logrus.WithFields(logrus.Fields{
		"channel_id": ch.ID,
		"sender_id":  senderID,
		"mode":       mode,
		"matched":    match != nil,
	}).Debug("[MessageProcessor] Evaluating access rule")

	if mode == channelDomain.AccessModePrivate {
		return match != nil && match.Action == commonDomain.AccessActionAllow
	}
	if mode == channelDomain.AccessModePublic {
		return match == nil || match.Action != commonDomain.AccessActionDeny
	}
	return false
}

func (p *MessageProcessor) normalizeIdentity(id string) string {
	if idx := strings.Index(id, ":"); idx != -1 {
		if atIdx := strings.Index(id, "@"); atIdx != -1 && idx < atIdx {
			return id[:idx] + id[atIdx:]
		} else if atIdx == -1 {
			return id[:idx]
		}
	}
	return id
}

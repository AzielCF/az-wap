package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coreconfig "github.com/AzielCF/az-wap/core/config"
	pkgUtils "github.com/AzielCF/az-wap/core/pkg/utils"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	waUtils "github.com/AzielCF/az-wap/workspace/infrastructure/whatsapp/adapter/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// OnMessage registers a handler for incoming messages
func (wa *WhatsAppAdapter) OnMessage(handler func(message.IncomingMessage)) {
	wa.eventHandler = handler
	if wa.handlerID != 0 {
		wa.client.RemoveEventHandler(wa.handlerID)
	}
	wa.handlerID = wa.client.AddEventHandler(wa.handleEvent)
}

// handleEvent converts whatsmeow events to workspace events
func (wa *WhatsAppAdapter) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.QR:
		var code string
		wa.qrMu.Lock()
		if len(v.Codes) > 0 {
			wa.currentQR = v.Codes[0]
			code = v.Codes[0]
		}
		wa.qrMu.Unlock()

		if code != "" {
			wa.subsMu.Lock()
			for _, ch := range wa.qrSubs {
				select {
				case ch <- code:
				default:
				}
			}
			wa.subsMu.Unlock()
		}

	case *events.Connected:
		wa.qrMu.Lock()
		wa.currentQR = ""
		wa.qrMu.Unlock()

		// RECOVERY: Resubscribe to all active presence events
		if wa.manager != nil {
			activeChats := wa.manager.GetActiveChats(wa.channelID)
			if len(activeChats) > 0 {
				logrus.Infof("[WHATSAPP] Reconnected channel %s. Recovering presence for %d active chats", wa.channelID, len(activeChats))
				for _, chatID := range activeChats {
					jid, err := wa.parseJID(chatID)
					if err == nil {
						_ = wa.client.SubscribePresence(context.Background(), jid)
					}
				}
			}
		}

	case *events.ChatPresence:
		if !v.Chat.IsEmpty() {
			// Resolve and unify ID
			unifiedID := wa.getUnifiedID(v.Chat)

			// If it's a LID we haven't linked yet, try a background resolution
			if strings.Contains(unifiedID, "@lid") {
				go func(lidJID types.JID) {
					wa.resolveAndCacheLID(lidJID)
				}(v.Chat)
			}

			// Try again after potential cache hit
			unifiedID = wa.getUnifiedID(v.Chat)

			logrus.Debugf("[WHATSAPP] Presence update from %s (unified: %s) in channel %s: %s (media: %v)", v.Chat.String(), unifiedID, wa.channelID, v.State, v.Media)

			if wa.manager != nil {
				media := channel.TypingMediaText
				if v.Media == types.ChatPresenceMediaAudio {
					media = channel.TypingMediaAudio
				}
				_ = wa.manager.UpdateTyping(context.Background(), wa.channelID, unifiedID, v.State == types.ChatPresenceComposing, media)
			}
		}

	case *events.Message:
		// Notify activity to presence manager to reset sleep timers
		if wa.manager != nil {
			wa.manager.PokeActivity(wa.channelID)
		}

		// Check for status/stories/broadcasts
		isStatus := v.Info.Chat.String() == "status@broadcast" || v.Info.Sender.String() == "status@broadcast" || v.Info.IsIncomingBroadcast()
		if isStatus {
			return
		}

		// Get copy of config for thread-safety during this event
		wa.configMu.RLock()
		conf := wa.config
		wa.configMu.RUnlock()

		// Send presence (typing)
		_ = wa.client.SubscribePresence(context.Background(), v.Info.Chat)

		// 1. Webhook Forwarding (Legacy support)
		go func() {
			ctx := context.Background()
			maxSize := wa.config.MaxDownloadSize
			if maxSize <= 0 {
				maxSize = coreconfig.Global.Whatsapp.MaxDownloadSize
			}
			payload, err := waUtils.CreateMessagePayload(ctx, v, wa.client, wa.workspaceID, wa.channelID, maxSize)
			if err == nil {
				// _ = wa.submitWebhook(ctx, payload, wa.config.URL)

				// But we also need to forward to all configured URLs
				if webhookCfg, ok := wa.config.Settings["webhook"].(map[string]any); ok {
					if urls, ok := webhookCfg["urls"].([]interface{}); ok {
						for _, u := range urls {
							if strURL, ok := u.(string); ok {
								_ = wa.submitWebhook(ctx, payload, strURL)
							}
						}
					}
				}
			}
		}()

		if wa.eventHandler == nil || v.Info.IsFromMe || pkgUtils.IsGroupJID(v.Info.Chat.String()) {
			return
		}

		// 0. LOCAL DEDUPLICATION: Prevent processing the same Message ID twice in this adapter
		if _, loaded := wa.eventDedup.LoadOrStore(v.Info.ID, time.Now()); loaded {
			logrus.Debugf("[WHATSAPP] Dropping duplicate event for message %s", v.Info.ID)
			return
		}
		// Cleanup dedup entry after 1 minute
		go func() {
			time.Sleep(1 * time.Minute)
			wa.eventDedup.Delete(v.Info.ID)
		}()

		text := pkgUtils.ExtractMessageTextFromEvent(v)

		// Extract Quoted Message for Contextual Responses (Helpful for URLs or targeted answers)
		evtMsg := pkgUtils.BuildEventMessage(v)
		if evtMsg.QuotedMessage != "" {
			text = fmt.Sprintf("[Replying to message: \"%s\"]\n%s", evtMsg.QuotedMessage, text)
		}

		msg := message.IncomingMessage{
			WorkspaceID: wa.workspaceID,
			ChannelID:   wa.channelID,
			ChatID:      wa.getUnifiedID(v.Info.Chat),
			SenderID:    wa.getUnifiedID(v.Info.Sender),
			IsStatus:    isStatus,
			Text:        text,
			Metadata: map[string]any{
				"platform":   "whatsapp",
				"message_id": v.Info.ID,
				"timestamp":  v.Info.Timestamp.Unix(),
				"push_name":  v.Info.PushName,
				"sender_jid": v.Info.Sender.String(),
				"chat_jid":   v.Info.Chat.String(),
				"sender_pn":  wa.getPNForLID(v.Info.Sender),
			},
		}

		// Parse Primary Media
		wa.configMu.RLock()
		conf = wa.config
		wa.configMu.RUnlock()

		if primaryMedia := wa.processWhatsAppMedia(&msg, v.Message, conf); primaryMedia != nil {
			msg.Media = primaryMedia
		}

		// Parse Quoted Media
		extendedMessage := v.Message.GetExtendedTextMessage()
		var quoted *waE2E.Message
		if extendedMessage != nil && extendedMessage.ContextInfo != nil {
			quoted = extendedMessage.ContextInfo.GetQuotedMessage()
		}

		var quotedMedia *message.IncomingMedia
		if quoted != nil {
			quotedMedia = wa.processWhatsAppMedia(&msg, quoted, conf)
		}

		if quotedMedia != nil {
			msg.Medias = append(msg.Medias, quotedMedia)
			if quotedMedia.Blocked {
				reason := "safety or configuration limits"
				if quotedMedia.BlockReason != "" {
					reason = quotedMedia.BlockReason
				}
				msg.Text += fmt.Sprintf("\n[SYSTEM NOTE: Access Denied. You do not have permission to read/listen to the quoted media (%s) because: %s]", quotedMedia.MimeType, reason)
			} else if quotedMedia.Path != "" {
				msg.Text += fmt.Sprintf("\n[SYSTEM NOTE: The quoted media is available as '%s']", filepath.Base(quotedMedia.Path))
			}
		}

		wa.eventHandler(msg)
	}
}

func (wa *WhatsAppAdapter) processWhatsAppMedia(msg *message.IncomingMessage, rawMsg *waE2E.Message, conf channel.ChannelConfig) *message.IncomingMedia {
	if rawMsg == nil {
		return nil
	}

	var downloadable whatsmeow.DownloadableMessage
	mediaType := ""
	caption := ""
	var fileSize int64 = 0
	var fileName string

	if img := rawMsg.GetImageMessage(); img != nil {
		downloadable = img
		mediaType = "image"
		caption = img.GetCaption()
		if img.FileLength != nil {
			fileSize = int64(*img.FileLength)
		}
	} else if audio := rawMsg.GetAudioMessage(); audio != nil {
		downloadable = audio
		mediaType = "audio"
		if audio.FileLength != nil {
			fileSize = int64(*audio.FileLength)
		}
		if conf.VoiceNotesOnly && (audio.PTT == nil || !*audio.PTT) {
			logrus.Debugf("[WHATSAPP] Blocking audio because it's not a voice note (PTT)")
			return nil
		}
	} else if video := rawMsg.GetVideoMessage(); video != nil {
		downloadable = video
		mediaType = "video"
		caption = video.GetCaption()
		if video.FileLength != nil {
			fileSize = int64(*video.FileLength)
		}
	} else if doc := rawMsg.GetDocumentMessage(); doc != nil {
		downloadable = doc
		mediaType = "document"
		caption = doc.GetCaption()
		fileName = doc.GetFileName()
		if doc.FileLength != nil {
			fileSize = int64(*doc.FileLength)
		}
	} else if sticker := rawMsg.GetStickerMessage(); sticker != nil {
		downloadable = sticker
		mediaType = "sticker"
		if sticker.FileLength != nil {
			fileSize = int64(*sticker.FileLength)
		}
	}

	if downloadable == nil || mediaType == "" {
		return nil
	}

	// Extension checks
	if len(conf.AllowedExtensions) > 0 && fileName != "" {
		ext := strings.ToLower(filepath.Ext(fileName))
		allowed := false
		for _, a := range conf.AllowedExtensions {
			if strings.ToLower(a) == ext || strings.ToLower(a) == strings.TrimPrefix(ext, ".") {
				allowed = true
				break
			}
		}
		if !allowed {
			logrus.Warnf("[WHATSAPP] Blocked file %s due to extension %s not in allowed list", fileName, ext)
			return nil
		}
	}

	// Permission checks
	isAllowed := true
	switch mediaType {
	case "image":
		isAllowed = conf.AllowImages
	case "audio":
		isAllowed = conf.AllowAudio
	case "video":
		isAllowed = conf.AllowVideo
	case "document":
		isAllowed = conf.AllowDocuments
	case "sticker":
		isAllowed = conf.AllowStickers
	}

	if !isAllowed {
		reason := fmt.Sprintf("Downloading %s is disabled in channel settings", mediaType)
		logrus.Warnf("[WHATSAPP] Blocked %s download for channel %s due to config", mediaType, wa.channelID)
		return &message.IncomingMedia{MimeType: mediaType, Blocked: true, BlockReason: reason}
	}

	maxSize := conf.MaxDownloadSize
	if maxSize <= 0 {
		maxSize = coreconfig.Global.Whatsapp.MaxDownloadSize
	}

	if fileSize > maxSize {
		sizeMB := float64(fileSize) / (1024 * 1024)
		limitMB := float64(maxSize) / (1024 * 1024)
		reason := fmt.Sprintf("File size (%.2fMB) exceeds the maximum allowed limit (%.2fMB)", sizeMB, limitMB)
		logrus.Warnf("[WHATSAPP] Blocked %s download for channel %s: size %.2fMB exceeds limit %.2fMB", mediaType, wa.channelID, sizeMB, limitMB)
		return &message.IncomingMedia{MimeType: mediaType, Blocked: true, BlockReason: reason}
	}

	// Gather Info
	info, errInfo := pkgUtils.GetMediaInfo(downloadable)
	if errInfo != nil {
		logrus.Warnf("[WHATSAPP] Failed to get media info: %v", errInfo)
		return nil
	}

	sessionKey := wa.channelID + "|" + msg.ChatID + "|" + msg.SenderID
	physicalFileName := info.FileHash + info.Extension
	friendlyName := physicalFileName
	if info.OriginalFilename != "" {
		friendlyName = pkgUtils.SanitizeFilename(info.OriginalFilename)
	}

	targetPath, errPrep := wa.manager.PrepareSessionFile(
		wa.workspaceID,
		wa.channelID,
		sessionKey,
		physicalFileName,
		friendlyName,
		info.MimeType,
		info.FileHash,
	)

	if errPrep != nil {
		logrus.Errorf("[WHATSAPP] Failed to prepare session file: %v", errPrep)
		return nil
	}

	if _, errExist := os.Stat(targetPath); errExist != nil {
		errDownload := pkgUtils.DownloadToFile(context.Background(), wa.client, downloadable, targetPath)
		if errDownload != nil {
			logrus.Errorf("[WHATSAPP] Failed to download to file: %v", errDownload)
			os.Remove(targetPath)
			return nil
		}
	} else {
		logrus.Debugf("[WHATSAPP] Media %s already exists in session, skipping download", physicalFileName)
	}

	// Attach text prefix for primary message only, if needed? No, let's keep it clean
	// We just return the media object
	return &message.IncomingMedia{
		Path:     targetPath,
		MimeType: info.MimeType,
		Caption:  caption,
	}
}

// getUnifiedID returns the best available JID for identity tracking, ALWAYS preferring LID if available
func (wa *WhatsAppAdapter) getUnifiedID(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	rawJID := jid.ToNonAD().String()

	// 1. Si ya es un LID, devolverlo directamente
	if strings.HasSuffix(rawJID, "@lid") {
		return rawJID
	}

	// 2. Si es un grupo o newsletter, devolver el JID original
	if strings.HasSuffix(rawJID, "@g.us") || strings.HasSuffix(rawJID, "@newsletter") {
		return rawJID
	}

	// 3. Revisar caché de identidad en memoria
	if linked, ok := wa.identityMap.Load(rawJID); ok {
		return linked.(string)
	}

	// 4. Intentar resolver desde el Store de whatsmeow síncronamente
	if wa.client != nil && wa.client.Store != nil && wa.client.Store.LIDs != nil {
		lid, err := wa.client.Store.LIDs.GetLIDForPN(context.Background(), jid.ToNonAD())
		if err == nil && !lid.IsEmpty() {
			lidStr := lid.ToNonAD().String()
			wa.identityMap.Store(rawJID, lidStr)
			return lidStr
		}
	}

	return rawJID
}

func (wa *WhatsAppAdapter) resolveAndCacheLID(jid types.JID) {
	if wa.client == nil || wa.client.Store == nil || wa.client.Store.LIDs == nil || !strings.Contains(jid.String(), "@lid") {
		return
	}

	rawLID := jid.ToNonAD().String()

	// Resolve PN (Phone Number) for this LID
	pn, err := wa.client.Store.LIDs.GetPNForLID(context.Background(), jid)
	if err == nil && !pn.IsEmpty() {
		pnStr := pn.ToNonAD().String()
		logrus.Infof("[WHATSAPP] Linked LID %s to PN %s. Forcing identity to LID.", rawLID, pnStr)

		// Map PN -> LID (for getUnifiedID to keep LID as primary)
		wa.identityMap.Store(pnStr, rawLID)

		// Map LID -> PN (for metadata and fallback resolution)
		wa.identityMap.Store("REV:"+rawLID, pnStr)
	}
}

// getPNForLID returns the Phone Number JID for a LID JID if cached or resolvable
func (wa *WhatsAppAdapter) getPNForLID(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	rawID := jid.ToNonAD().String()

	if strings.HasSuffix(rawID, "@s.whatsapp.net") {
		return rawID
	}

	if !strings.HasSuffix(rawID, "@lid") {
		return ""
	}

	// 1. Check cache
	if linked, ok := wa.identityMap.Load("REV:" + rawID); ok {
		return linked.(string)
	}

	// 2. Try sync resolve from store
	if wa.client != nil && wa.client.Store != nil && wa.client.Store.LIDs != nil {
		pn, err := wa.client.Store.LIDs.GetPNForLID(context.Background(), jid.ToNonAD())
		if err == nil && !pn.IsEmpty() {
			pnStr := pn.ToNonAD().String()
			wa.identityMap.Store("REV:"+rawID, pnStr)
			return pnStr
		}
	}

	return ""
}

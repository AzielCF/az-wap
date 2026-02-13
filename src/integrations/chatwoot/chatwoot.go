package chatwoot

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/pkg/chatmedia"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// --- CONFIG & CACHE ---

const (
	httpTimeout     = 15 * time.Second
	contactCacheTTL = 5 * time.Minute
	convCacheTTL    = 10 * time.Minute
)

var (
	httpClient         = &http.Client{Timeout: httpTimeout}
	insecureHttpClient = &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	contactCacheMu      sync.RWMutex
	contactCache        = make(map[string]cachedContact)
	conversationCacheMu sync.RWMutex
	conversationCache   = make(map[string]cachedConversation)

	wkRepo repository.IWorkspaceRepository
	mainDB *gorm.DB
)

func SetRepositories(wr repository.IWorkspaceRepository, db *gorm.DB) {
	wkRepo = wr
	mainDB = db
}

type Config struct {
	InstanceID         string
	BaseURL            string
	AccountID          int64
	InboxID            int64
	InboxIdentifier    string
	AccountToken       string
	BotToken           string
	Enabled            bool
	InsecureSkipVerify bool
	CredentialID       string
}

type cachedContact struct {
	ContactID int64
	SourceID  string
	ExpiresAt time.Time
}

type cachedConversation struct {
	ConversationID int64
	ExpiresAt      time.Time
}

// --- MAIN FUNCTION ---

// ForwardWhatsAppMessage envía mensajes de WhatsApp (del cliente) a Chatwoot.
func ForwardWhatsAppMessage(ctx context.Context, instanceID, phone string, evt *events.Message) {
	if evt == nil || evt.Info.IsFromMe || evt.Info.IsIncomingBroadcast() || utils.IsGroupJID(evt.Info.Chat.String()) {
		return
	}

	source := evt.Info.SourceString()
	chatStr := evt.Info.Chat.String()
	if strings.Contains(source, "broadcast") ||
		strings.HasSuffix(chatStr, "@broadcast") ||
		strings.HasPrefix(chatStr, "status@") ||
		strings.EqualFold(evt.Info.Category, "status") {
		return
	}

	instanceID, phone = strings.TrimSpace(instanceID), strings.TrimSpace(phone)
	if instanceID == "" || phone == "" {
		return
	}

	cfg, err := loadChannelConfig(ctx, instanceID)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to load config for %s", instanceID)
		return
	}
	if cfg == nil || !cfg.Enabled {
		return // Canal no configurado o Chatwoot deshabilitado
	}

	// Procesar Texto
	text := strings.TrimSpace(utils.ExtractMessageTextFromProto(evt.Message))
	if text == "" && (evt.Message.GetPollCreationMessageV3() != nil || evt.Message.GetPollCreationMessageV4() != nil || evt.Message.GetPollCreationMessageV5() != nil) {
		text = strings.TrimSpace(utils.ExtractMessageTextFromEvent(evt))
	}

	// Procesar Medios (Deduplicar por Kind)
	rawMedia := chatmedia.Get(evt.Info.ID)
	mediaItems := make([]chatmedia.Item, 0, len(rawMedia))
	seenKinds := make(map[string]struct{})
	for _, it := range rawMedia {
		k := strings.TrimSpace(it.Kind)
		if k == "" {
			mediaItems = append(mediaItems, it)
			continue
		}
		if _, exists := seenKinds[k]; !exists {
			seenKinds[k] = struct{}{}
			mediaItems = append(mediaItems, it)
		}
	}

	if text == "" && len(mediaItems) == 0 {
		return
	}

	// Gestionar Contacto y Conversación
	displayName := strings.TrimSpace(evt.Info.PushName)
	contactID, sourceID, err := ensureContact(ctx, cfg, phone, displayName)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to ensure contact for %s", phone)
		return
	}

	convID, err := createConversationWithMessage(ctx, cfg, contactID, sourceID, text, mediaItems, 0)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to create conversation for %s", phone)
		return
	}
	setCachedConversation(cfg.InstanceID, phone, convID)
}

// ForwardBotReplyFromEvent envía la respuesta de un Bot IA a Chatwoot usando el teléfono
// ya normalizado (por ejemplo, vía NormalizePhoneForChatwoot en el manejador de WhatsApp).
func ForwardBotReplyFromEvent(ctx context.Context, instanceID, phone, reply string) {
	reply = strings.TrimSpace(reply)
	instanceID = strings.TrimSpace(instanceID)
	phone = strings.TrimSpace(phone)
	if reply == "" || instanceID == "" || phone == "" {
		return
	}
	if err := forwardBotTextMessage(ctx, instanceID, phone, reply); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"instance_id": instanceID,
			"phone":       phone,
		}).Error("[CHATWOOT] failed to forward bot reply to Chatwoot")
	}
}

// ForwardBotReplyWithConfig allows forwarding a bot reply using an explicit configuration.
func ForwardBotReplyWithConfig(ctx context.Context, conf *Config, phone, reply string) {
	if !conf.Enabled || reply == "" || phone == "" {
		return
	}

	contactID, sourceID, err := ensureContact(ctx, conf, phone, "")
	if err != nil {
		logrus.WithError(err).Error("[CHATWOOT] failed to ensure contact")
		return
	}

	// Try cached conversation
	if convID, ok := getCachedConversation(conf.InstanceID, phone); ok && convID != 0 {
		if err := sendTextToConversation(ctx, conf, convID, reply, "outgoing", map[string]interface{}{"from_bot": true}); err == nil {
			return
		}
	}

	// Create/Find conversation
	convID, err := createConversationWithMessage(ctx, conf, contactID, sourceID, reply, nil, 1)
	if err != nil {
		logrus.WithError(err).Error("[CHATWOOT] failed to forward bot reply with config")
		return
	}
	setCachedConversation(conf.InstanceID, phone, convID)
}

// ForwardIncomingMessageWithConfig forwards an incoming message to Chatwoot using an explicit configuration.
func ForwardIncomingMessageWithConfig(ctx context.Context, cfg *Config, phone, name, text string, mediaItems []chatmedia.Item) {
	if cfg == nil || !cfg.Enabled {
		return
	}

	phone = strings.TrimSpace(phone)
	if phone == "" {
		return
	}

	// Gestionar Contacto y Conversación
	contactID, sourceID, err := ensureContact(ctx, cfg, phone, name)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to ensure contact for %s", phone)
		return
	}

	convID, err := createConversationWithMessage(ctx, cfg, contactID, sourceID, text, mediaItems, 0)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to create conversation for %s", phone)
		return
	}
	setCachedConversation(cfg.InstanceID, phone, convID)
}

// sendTextToConversation envía solo texto a una conversación ya conocida.
func sendTextToConversation(ctx context.Context, cfg *Config, convID int64, text string, messageType string, contentAttrs map[string]interface{}) error {
	if convID == 0 || text == "" {
		return fmt.Errorf("missing conversation or text")
	}
	mt := strings.TrimSpace(messageType)
	if mt == "" {
		mt = "incoming"
	}
	if contentAttrs == nil {
		contentAttrs = map[string]interface{}{}
	}

	// Elegimos qué token usar:
	// - Si el mensaje es de bot (from_bot=true) y hay BotToken, usamos BotToken.
	// - Si message_type=outgoing y hay BotToken, usamos BotToken y marcamos from_bot=true.
	// - En cualquier otro caso, AccountToken.
	token := cfg.AccountToken
	tokenKind := "account"
	botToken := strings.TrimSpace(cfg.BotToken)

	// Detectar flag from_bot si viene en content_attrs
	isBot := false
	if v, ok := contentAttrs["from_bot"]; ok {
		if vb, ok2 := v.(bool); ok2 && vb {
			isBot = true
		}
	}

	if isBot && botToken != "" {
		token = botToken
		tokenKind = "bot"
	} else if strings.EqualFold(mt, "outgoing") && botToken != "" {
		token = botToken
		tokenKind = "bot"
		if _, exists := contentAttrs["from_bot"]; !exists {
			contentAttrs["from_bot"] = true
		}
	}

	logrus.WithFields(logrus.Fields{
		"message_type": mt,
		"is_bot":       isBot,
		"token_kind":   tokenKind,
		"has_bot":      botToken != "",
		"conv_id":      convID,
	}).Info("[CHATWOOT] sending text to conversation")

	req := map[string]interface{}{
		"content":            text,
		"message_type":       mt,
		"private":            false,
		"content_type":       "text",
		"content_attributes": contentAttrs,
	}
	url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", cfg.BaseURL, cfg.AccountID, convID)
	flag := viper.GetString("capture_chatwoot_payloads")
	if flag == "" {
		flag = viper.GetString("CAPTURE_CHATWOOT_PAYLOADS")
	}
	if flag == "1" {
		if data, err := json.MarshalIndent(req, "", "  "); err == nil {
			logrus.WithFields(logrus.Fields{
				"url":        url,
				"token_kind": tokenKind,
			}).Info("[CHATWOOT_CAPTURE] payload")
			logrus.Info(string(data))
		}
	}
	if err := jsonRequest(ctx, http.MethodPost, url, token, req, nil); err != nil {
		return fmt.Errorf("send text failed: %w", err)
	}
	return nil
}

// --- LOGIC: CONFIG LOADING ---

func loadChannelConfig(ctx context.Context, externalRef string) (*Config, error) {
	if wkRepo == nil {
		return nil, fmt.Errorf("workspace repository not initialized")
	}

	ch, err := wkRepo.GetChannelByExternalRef(ctx, externalRef)
	if err != nil {
		return nil, nil // Not found or error
	}

	if !ch.Enabled || ch.Config.Chatwoot == nil || !ch.Config.Chatwoot.Enabled {
		return nil, nil
	}

	cw := ch.Config.Chatwoot
	cfg := &Config{
		InstanceID:         externalRef,
		BaseURL:            strings.TrimRight(strings.TrimSpace(cw.URL), "/"),
		AccountToken:       strings.TrimSpace(cw.Token),
		Enabled:            cw.Enabled,
		AccountID:          int64(cw.AccountID),
		InboxID:            int64(cw.InboxID),
		BotToken:           strings.TrimSpace(cw.BotToken),
		InsecureSkipVerify: ch.Config.SkipTLSVerification,
	}

	if mainDB != nil {
		// Resolve Credential if provided
		if cw.CredentialID != "" {
			var cred struct {
				ChatwootAccountToken string `gorm:"column:chatwoot_account_token"`
				ChatwootBaseURL      string `gorm:"column:chatwoot_base_url"`
				ChatwootBotToken     string `gorm:"column:chatwoot_bot_token"`
			}
			if err := mainDB.Table("credentials").Where("id = ? AND kind = 'chatwoot'", cw.CredentialID).First(&cred).Error; err == nil {
				if cfg.AccountToken == "" {
					cfg.AccountToken = strings.TrimSpace(cred.ChatwootAccountToken)
				}
				if cfg.BaseURL == "" {
					cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cred.ChatwootBaseURL), "/")
				}
				if cfg.BotToken == "" {
					cfg.BotToken = strings.TrimSpace(cred.ChatwootBotToken)
				}
			}
		}

		// Resolve Bot Token if BotID provided
		if ch.Config.BotID != "" && cfg.BotToken == "" {
			var bot struct {
				ChatwootBotToken     string `gorm:"column:chatwoot_bot_token"`
				ChatwootCredentialID string `gorm:"column:chatwoot_credential_id"`
			}
			if err := mainDB.Table("bots").Where("id = ?", ch.Config.BotID).First(&bot).Error; err == nil {
				if strings.TrimSpace(bot.ChatwootBotToken) != "" {
					cfg.BotToken = strings.TrimSpace(bot.ChatwootBotToken)
				}
				if strings.TrimSpace(bot.ChatwootCredentialID) != "" && cfg.BotToken == "" {
					var cred struct {
						ChatwootBotToken string `gorm:"column:chatwoot_bot_token"`
					}
					if err := mainDB.Table("credentials").Where("id = ? AND kind = 'chatwoot'", bot.ChatwootCredentialID).First(&cred).Error; err == nil {
						cfg.BotToken = strings.TrimSpace(cred.ChatwootBotToken)
					}
				}
			}
		}
	}

	if cfg.BaseURL == "" || cfg.AccountToken == "" {
		return nil, nil
	}

	if cw.InboxIdentifier != "" {
		cfg.InboxIdentifier = cw.InboxIdentifier
	} else {
		cfg.InboxIdentifier = resolveInboxIdentifier(ctx, cfg)
		if cfg.InboxIdentifier == "" {
			cfg.InboxIdentifier = strconv.FormatInt(cfg.InboxID, 10)
		}
	}

	return cfg, nil
}

func resolveInboxIdentifier(ctx context.Context, cfg *Config) string {
	url := fmt.Sprintf("%s/api/v1/accounts/%d/inboxes/%d", cfg.BaseURL, cfg.AccountID, cfg.InboxID)
	var resp struct {
		Identifier string `json:"identifier"`
		Channel    struct {
			Identifier string `json:"identifier"`
		} `json:"channel"`
	}
	if err := jsonRequest(ctx, http.MethodGet, url, cfg.AccountToken, nil, &resp); err != nil {
		logrus.WithError(err).Warnf("[CHATWOOT] failed to resolve inbox identifier")
		return ""
	}
	if resp.Identifier != "" {
		return resp.Identifier
	}
	return resp.Channel.Identifier
}

// IsInstanceEnabled devuelve true si la integración de Chatwoot está habilitada y correctamente
// configurada para la instancia indicada.
func IsChannelEnabled(ctx context.Context, externalRef string) bool {
	externalRef = strings.TrimSpace(externalRef)
	if externalRef == "" {
		return false
	}
	cfg, err := loadChannelConfig(ctx, externalRef)
	if err != nil || cfg == nil {
		return false
	}
	return cfg.Enabled
}

// --- LOGIC: CONTACT MANAGEMENT ---

func ensureContact(ctx context.Context, cfg *Config, phone, name string) (int64, string, error) {
	// 1. Cache Check
	if id, src, ok := getCachedContact(cfg.InstanceID, phone); ok {
		logrus.Infof("[CHATWOOT] contact cache hit: %s", phone)
		return id, src, nil
	}

	// 2. Search API
	if id, src, err := findContactByPhone(ctx, cfg, phone); err == nil && id > 0 {
		setCachedContact(cfg.InstanceID, phone, id, src)
		return id, src, nil
	}

	// 3. Create API
	logrus.Infof("[CHATWOOT] creating contact: %s", phone)
	flag := viper.GetString("capture_chatwoot_contacts")
	if flag == "" {
		flag = viper.GetString("capture_chatwoot_payloads")
	}
	req := map[string]interface{}{
		"inbox_id":     cfg.InboxID,
		"name":         name,
		"phone_number": phone,
		"identifier":   phone,
	}
	contactURL := fmt.Sprintf("%s/api/v1/accounts/%d/contacts", cfg.BaseURL, cfg.AccountID)
	if flag == "1" {
		if data, err := json.MarshalIndent(req, "", "  "); err == nil {
			logrus.WithFields(logrus.Fields{
				"instance_id": cfg.InstanceID,
				"phone":       phone,
				"url":         contactURL,
			}).Info("[CHATWOOT_CONTACT_CAPTURE] request")
			logrus.Info(string(data))
		}
	}

	var resp struct {
		Payload struct {
			Contact struct {
				ID             int64 `json:"id"`
				ContactInboxes []struct {
					SourceID string `json:"source_id"`
				} `json:"contact_inboxes"`
			} `json:"contact"`
			ContactInbox struct {
				SourceID string `json:"source_id"`
			} `json:"contact_inbox"`
		} `json:"payload"`
	}

	err := jsonRequest(ctx, http.MethodPost, contactURL, cfg.AccountToken, req, &resp)
	if err != nil {
		// Retry if taken (race condition or soft deletion logic)
		if strings.Contains(err.Error(), "already been taken") {
			if id, src, errSearch := findContactByPhone(ctx, cfg, phone); errSearch == nil && id > 0 {
				setCachedContact(cfg.InstanceID, phone, id, src)
				return id, src, nil
			}
		}
		return 0, "", err
	}

	contact := resp.Payload.Contact
	if contact.ID == 0 {
		return 0, "", fmt.Errorf("chatwoot created contact has no ID")
	}

	srcID := phone
	if s := strings.TrimSpace(resp.Payload.ContactInbox.SourceID); s != "" {
		srcID = s
	} else if len(contact.ContactInboxes) > 0 && contact.ContactInboxes[0].SourceID != "" {
		srcID = contact.ContactInboxes[0].SourceID
	}

	if flag == "1" {
		out := map[string]interface{}{
			"contact_id": contact.ID,
			"source_id":  srcID,
		}
		if data, err := json.MarshalIndent(out, "", "  "); err == nil {
			logrus.WithFields(logrus.Fields{
				"instance_id": cfg.InstanceID,
				"phone":       phone,
			}).Info("[CHATWOOT_CONTACT_CAPTURE] response")
			logrus.Info(string(data))
		}
	}

	setCachedContact(cfg.InstanceID, phone, contact.ID, srcID)
	return contact.ID, srcID, nil
}

func findContactByPhone(ctx context.Context, cfg *Config, phone string) (int64, string, error) {
	searchURL := fmt.Sprintf("%s/api/v1/accounts/%d/contacts/search?q=%s", cfg.BaseURL, cfg.AccountID, url.QueryEscape(phone))
	var resp struct {
		Payload []struct {
			ID             int64 `json:"id"`
			ContactInboxes []struct {
				SourceID string `json:"source_id"`
				Inbox    struct {
					ID int64 `json:"id"`
				} `json:"inbox"`
			} `json:"contact_inboxes"`
		} `json:"payload"`
	}

	if err := jsonRequest(ctx, http.MethodGet, searchURL, cfg.AccountToken, nil, &resp); err != nil {
		return 0, "", err
	}

	if len(resp.Payload) == 0 || resp.Payload[0].ID == 0 {
		return 0, "", nil
	}

	contact := resp.Payload[0]
	srcID := phone
	for _, ci := range contact.ContactInboxes {
		if ci.Inbox.ID == cfg.InboxID && strings.TrimSpace(ci.SourceID) != "" {
			srcID = ci.SourceID
			break
		}
	}
	return contact.ID, srcID, nil
}

// --- LOGIC: CONVERSATION & MESSAGES ---

// createConversationWithMessage garantiza que exista una conversación para el contacto
// y envía el mensaje con el messageType indicado (0: incoming, 1: outgoing).
func createConversationWithMessage(ctx context.Context, cfg *Config, contactID int64, sourceID, text string, mediaItems []chatmedia.Item, messageType int) (int64, error) {
	// A. Find or Create Conversation
	convID, _ := findExistingConversation(ctx, cfg, contactID) // Ignoramos error, fallback a crear
	if convID == 0 {
		req := map[string]interface{}{"source_id": sourceID, "inbox_id": cfg.InboxID, "contact_id": contactID}
		var resp struct {
			ID int64 `json:"id"`
		}
		url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations", cfg.BaseURL, cfg.AccountID)
		if err := jsonRequest(ctx, http.MethodPost, url, cfg.AccountToken, req, &resp); err != nil {
			return 0, fmt.Errorf("create conversation failed: %w", err)
		}
		convID = resp.ID
		logrus.Infof("[CHATWOOT] created conversation %d", convID)
	}

	// B. Send Text
	if text != "" {
		// Según la API de Chatwoot, message_type debe ser "incoming" o "outgoing".
		mt := "incoming"
		if messageType == 1 {
			mt = "outgoing"
		}
		var attrs map[string]interface{}
		if messageType == 1 {
			attrs = map[string]interface{}{"from_bot": true}
		}
		if err := sendTextToConversation(ctx, cfg, convID, text, mt, attrs); err != nil {
			return 0, err
		}
	}

	// C. Send Media
	if len(mediaItems) > 0 {
		if err := sendAttachmentMessage(ctx, cfg, convID, sourceID, mediaItems); err != nil {
			logrus.WithError(err).Error("[CHATWOOT] failed to send media")
		}
	}
	return convID, nil
}

// forwardBotTextMessage envía un mensaje de texto generado por un Bot IA a Chatwoot
// como mensaje "agent/outgoing" (message_type = 1).
func forwardBotTextMessage(ctx context.Context, instanceID, phone, text string) error {
	instanceID = strings.TrimSpace(instanceID)
	phone = strings.TrimSpace(phone)
	text = strings.TrimSpace(text)
	if instanceID == "" || phone == "" || text == "" {
		return nil
	}

	cfg, err := loadChannelConfig(ctx, instanceID)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	contactID, sourceID, err := ensureContact(ctx, cfg, phone, "")
	if err != nil {
		return err
	}

	// 1) Intentar usar conversación cacheada para este phone
	if convID, ok := getCachedConversation(cfg.InstanceID, phone); ok && convID != 0 {
		if err := sendTextToConversation(ctx, cfg, convID, text, "outgoing", map[string]interface{}{"from_bot": true}); err == nil {
			return nil
		}
		// Si falla, continuamos a búsqueda/creación normal
	}

	// 2) Buscar o crear conversación y cachear
	convID, err := createConversationWithMessage(ctx, cfg, contactID, sourceID, text, nil, 1)
	if err != nil {
		return err
	}
	setCachedConversation(cfg.InstanceID, phone, convID)
	return nil
}

func findExistingConversation(ctx context.Context, cfg *Config, contactID int64) (int64, error) {
	if contactID <= 0 {
		return 0, nil
	}
	flag := viper.GetString("capture_chatwoot_conversations")
	if flag == "" {
		flag = viper.GetString("capture_chatwoot_payloads")
	}
	url := fmt.Sprintf("%s/api/v1/accounts/%d/contacts/%d/conversations", cfg.BaseURL, cfg.AccountID, contactID)
	var resp struct {
		Payload []struct {
			ID      int64  `json:"id"`
			InboxID int64  `json:"inbox_id"`
			Status  string `json:"status"`
		} `json:"payload"`
	}

	if err := jsonRequest(ctx, http.MethodGet, url, cfg.AccountToken, nil, &resp); err != nil {
		return 0, err
	}

	if flag == "1" {
		if data, err := json.MarshalIndent(resp, "", "  "); err == nil {
			logrus.WithFields(logrus.Fields{
				"instance_id": cfg.InstanceID,
				"contact_id":  contactID,
				"url":         url,
			}).Info("[CHATWOOT_CONVERSATIONS_CAPTURE] response")
			logrus.Info(string(data))
		}
	}

	var fallback int64
	for _, c := range resp.Payload {
		if c.InboxID != cfg.InboxID {
			continue
		}
		st := strings.ToLower(c.Status)
		if st == "open" || st == "pending" || st == "snoozed" || st == "" {
			return c.ID, nil
		}
		if fallback == 0 {
			fallback = c.ID
		}
	}
	return fallback, nil
}

func sendAttachmentMessage(ctx context.Context, cfg *Config, convID int64, sourceID string, mediaItems []chatmedia.Item) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Caption logic
	if len(mediaItems) == 1 && mediaItems[0].Caption != "" {
		_ = w.WriteField("content", mediaItems[0].Caption)
	}

	seen := make(map[string]bool)
	for i, item := range mediaItems {
		path := strings.TrimSpace(item.Path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		f, err := os.Open(path)
		if err != nil {
			continue
		}

		fname := getFileName(item, i)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="attachments[]"; filename="%s"`, strings.ReplaceAll(fname, "\"", "_")))
		if ct := getContentType(item); ct != "" {
			h.Set("Content-Type", ct)
		}
		part, _ := w.CreatePart(h)
		_, _ = io.Copy(part, f)
		f.Close()
		_ = os.Remove(path)
	}
	w.Close()

	targetURL := fmt.Sprintf("%s/public/api/v1/inboxes/%s/contacts/%s/conversations/%d/messages",
		cfg.BaseURL, url.PathEscape(cfg.InboxIdentifier), url.PathEscape(sourceID), convID)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("api_access_token", cfg.AccountToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// --- HELPERS: HTTP & UTILS ---

// jsonRequest unifica la creación, ejecución y decodificación de peticiones API.
func jsonRequest(ctx context.Context, method, url, token string, body interface{}, dest interface{}) error {
	return jsonRequestWithConfig(ctx, method, url, &Config{AccountToken: token}, body, dest)
}

func jsonRequestWithConfig(ctx context.Context, method, url string, cfg *Config, body interface{}, dest interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg != nil && cfg.AccountToken != "" {
		req.Header.Set("api_access_token", cfg.AccountToken)
	}

	client := httpClient
	if cfg != nil && cfg.InsecureSkipVerify {
		client = insecureHttpClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	if dest != nil {
		return json.Unmarshal(data, dest)
	}
	return nil
}

func normalizePhoneFromEvent(evt *events.Message) string {
	if evt == nil {
		return ""
	}
	candidate := strings.TrimSpace(evt.Info.SourceString())
	if candidate == "" {
		candidate = evt.Info.Chat.User
		if candidate == "" {
			candidate = evt.Info.Sender.User
		}
	}
	if i := strings.IndexAny(candidate, "@:"); i >= 0 {
		candidate = candidate[:i]
	}
	digits := regexp.MustCompile("[^0-9]+").ReplaceAllString(candidate, "")
	if digits == "" {
		return ""
	}
	return "+" + digits
}

// getFileName simplifica la lógica de nombrado
func getFileName(item chatmedia.Item, index int) string {
	kind := strings.ToLower(strings.TrimSpace(item.Kind))
	ext := strings.ToLower(filepath.Ext(item.Path))
	base := "file"

	switch kind {
	case "audio":
		base, ext = "voice-message", ".ogg"
	case "image":
		base = "image"
		if ext == "" {
			ext = ".jpg"
		}
	case "video":
		base = "video"
		if ext == "" || ext == ".m4v" {
			ext = ".mp4"
		}
	case "document":
		base = "document"
	}
	if ext == "" {
		ext = ".bin"
	}

	if index > 0 {
		return fmt.Sprintf("%s-%d%s", base, index+1, ext)
	}
	return base + ext
}

func getContentType(item chatmedia.Item) string {
	if mt := strings.TrimSpace(item.MimeType); mt != "" {
		if strings.Contains(mt, "m4v") {
			return "video/mp4"
		}
		return strings.Split(mt, ";")[0]
	}
	ext := strings.ToLower(filepath.Ext(item.Path))
	if ext == ".m4v" {
		return "video/mp4"
	}
	return mime.TypeByExtension(ext)
}

// --- HELPERS: CACHE ---

func makeContactCacheKey(instance, phone string) string {
	if instance == "" || phone == "" {
		return ""
	}
	return instance + "|" + phone
}

func getCachedContact(instance, phone string) (int64, string, bool) {
	key := makeContactCacheKey(instance, phone)
	if key == "" {
		return 0, "", false
	}

	contactCacheMu.RLock()
	defer contactCacheMu.RUnlock()
	entry, ok := contactCache[key]
	if ok && time.Now().After(entry.ExpiresAt) {
		go func(k string) { // Limpieza lazy asíncrona
			contactCacheMu.Lock()
			delete(contactCache, k)
			contactCacheMu.Unlock()
		}(key)
		return 0, "", false
	}
	return entry.ContactID, entry.SourceID, ok
}

func setCachedContact(instance, phone string, id int64, src string) {
	key := makeContactCacheKey(instance, phone)
	if key == "" {
		return
	}
	contactCacheMu.Lock()
	contactCache[key] = cachedContact{ContactID: id, SourceID: strings.TrimSpace(src), ExpiresAt: time.Now().Add(contactCacheTTL)}
	contactCacheMu.Unlock()
}

func makeConversationCacheKey(instance, phone string) string {
	if instance == "" || phone == "" {
		return ""
	}
	return instance + "|" + phone
}

func getCachedConversation(instance, phone string) (int64, bool) {
	key := makeConversationCacheKey(instance, phone)
	if key == "" {
		return 0, false
	}
	conversationCacheMu.RLock()
	defer conversationCacheMu.RUnlock()
	entry, ok := conversationCache[key]
	if ok && time.Now().After(entry.ExpiresAt) {
		go func(k string) {
			conversationCacheMu.Lock()
			delete(conversationCache, k)
			conversationCacheMu.Unlock()
		}(key)
		return 0, false
	}
	return entry.ConversationID, ok
}

func setCachedConversation(instance, phone string, convID int64) {
	key := makeConversationCacheKey(instance, phone)
	if key == "" || convID == 0 {
		return
	}
	conversationCacheMu.Lock()
	conversationCache[key] = cachedConversation{ConversationID: convID, ExpiresAt: time.Now().Add(convCacheTTL)}
	conversationCacheMu.Unlock()
}

package chatwoot

import (
	"bytes"
	"context"
	"database/sql"
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

	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/chatmedia"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
)

// --- CONFIG & CACHE ---

const (
	httpTimeout     = 15 * time.Second
	contactCacheTTL = 5 * time.Minute
)

var (
	httpClient     = &http.Client{Timeout: httpTimeout}
	contactCacheMu sync.RWMutex
	contactCache   = make(map[string]cachedContact)
)

type instanceChatwootConfig struct {
	InstanceID      string
	BaseURL         string
	AccountID       int64
	InboxID         int64
	InboxIdentifier string
	AccountToken    string
	BotToken        string
}

type cachedContact struct {
	ContactID int64
	SourceID  string
	ExpiresAt time.Time
}

// --- MAIN FUNCTION ---

// ForwardWhatsAppMessage envía mensajes de WhatsApp a Chatwoot preservando la lógica original.
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

	cfg, err := loadInstanceConfig(ctx, instanceID)
	if err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to load config for %s", instanceID)
		return
	}
	if cfg == nil {
		return // Instancia no configurada
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

	if err := createConversationWithMessage(ctx, cfg, contactID, sourceID, text, mediaItems); err != nil {
		logrus.WithError(err).Errorf("[CHATWOOT] failed to create conversation for %s", phone)
	}
}

// --- LOGIC: CONFIG LOADING ---

func loadInstanceConfig(ctx context.Context, instanceID string) (*instanceChatwootConfig, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var baseUrl, accId, inboxId, inboxIdent, accToken, botToken sql.NullString
	query := `SELECT chatwoot_base_url, chatwoot_account_id, chatwoot_inbox_id, chatwoot_inbox_identifier, chatwoot_account_token, chatwoot_bot_token FROM instances WHERE id = ?`
	if err := db.QueryRowContext(ctx, query, instanceID).Scan(&baseUrl, &accId, &inboxId, &inboxIdent, &accToken, &botToken); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	cfg := &instanceChatwootConfig{
		InstanceID:   instanceID,
		BaseURL:      strings.TrimRight(strings.TrimSpace(baseUrl.String), "/"),
		AccountToken: strings.TrimSpace(accToken.String),
	}

	if cfg.BaseURL == "" || cfg.AccountToken == "" {
		logrus.Debugf("[CHATWOOT] instance %s incomplete config", instanceID)
		return nil, nil
	}

	cfg.AccountID, _ = strconv.ParseInt(strings.TrimSpace(accId.String), 10, 64)
	cfg.InboxID, _ = strconv.ParseInt(strings.TrimSpace(inboxId.String), 10, 64)

	if cfg.AccountID <= 0 || cfg.InboxID <= 0 {
		return nil, nil
	}

	cfg.BotToken = strings.TrimSpace(botToken.String)
	if inboxIdent.Valid && strings.TrimSpace(inboxIdent.String) != "" {
		cfg.InboxIdentifier = strings.TrimSpace(inboxIdent.String)
	} else {
		cfg.InboxIdentifier = resolveInboxIdentifier(ctx, cfg)
		if cfg.InboxIdentifier == "" {
			cfg.InboxIdentifier = strconv.FormatInt(cfg.InboxID, 10)
		}
	}

	return cfg, nil
}

func resolveInboxIdentifier(ctx context.Context, cfg *instanceChatwootConfig) string {
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

// --- LOGIC: CONTACT MANAGEMENT ---

func ensureContact(ctx context.Context, cfg *instanceChatwootConfig, phone, name string) (int64, string, error) {
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
	req := map[string]interface{}{
		"inbox_id":     cfg.InboxID,
		"name":         name,
		"phone_number": phone,
		"identifier":   phone,
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

	err := jsonRequest(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/accounts/%d/contacts", cfg.BaseURL, cfg.AccountID), cfg.AccountToken, req, &resp)
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

	setCachedContact(cfg.InstanceID, phone, contact.ID, srcID)
	return contact.ID, srcID, nil
}

func findContactByPhone(ctx context.Context, cfg *instanceChatwootConfig, phone string) (int64, string, error) {
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

func createConversationWithMessage(ctx context.Context, cfg *instanceChatwootConfig, contactID int64, sourceID, text string, mediaItems []chatmedia.Item) error {
	// A. Find or Create Conversation
	convID, _ := findExistingConversation(ctx, cfg, contactID) // Ignoramos error, fallback a crear
	if convID == 0 {
		req := map[string]interface{}{"source_id": sourceID, "inbox_id": cfg.InboxID, "contact_id": contactID}
		var resp struct {
			ID int64 `json:"id"`
		}
		url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations", cfg.BaseURL, cfg.AccountID)
		if err := jsonRequest(ctx, http.MethodPost, url, cfg.AccountToken, req, &resp); err != nil {
			return fmt.Errorf("create conversation failed: %w", err)
		}
		convID = resp.ID
		logrus.Infof("[CHATWOOT] created conversation %d", convID)
	}

	// B. Send Text
	if text != "" {
		req := map[string]interface{}{"content": text, "message_type": 0, "private": false}
		url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", cfg.BaseURL, cfg.AccountID, convID)
		if err := jsonRequest(ctx, http.MethodPost, url, cfg.AccountToken, req, nil); err != nil {
			return fmt.Errorf("send text failed: %w", err)
		}
	}

	// C. Send Media
	if len(mediaItems) > 0 {
		if err := sendAttachmentMessage(ctx, cfg, convID, sourceID, mediaItems); err != nil {
			logrus.WithError(err).Error("[CHATWOOT] failed to send media")
		}
	}
	return nil
}

func findExistingConversation(ctx context.Context, cfg *instanceChatwootConfig, contactID int64) (int64, error) {
	if contactID <= 0 {
		return 0, nil
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

func sendAttachmentMessage(ctx context.Context, cfg *instanceChatwootConfig, convID int64, sourceID string, mediaItems []chatmedia.Item) error {
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
	if token != "" {
		req.Header.Set("api_access_token", token)
	}

	resp, err := httpClient.Do(req)
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

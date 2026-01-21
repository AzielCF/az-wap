package adapter

import (
	"context"
	"fmt"
	"strings"
	"sync"

	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	infraChatStorage "github.com/AzielCF/az-wap/infrastructure/chatstorage"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type WhatsAppAdapter struct {
	channelID    string
	workspaceID  string
	sessionID    string // Optional: for legacy migration persistence
	client       *whatsmeow.Client
	dbContainer  interface{ Close() error } // sqlstore.Container for cleanup
	chatStorage  domainChatStorage.IChatStorageRepository
	repoMu       sync.Mutex
	eventHandler func(message.IncomingMessage)
	handlerID    uint32
	manager      *workspace.Manager

	// QR & Login State
	qrMu      sync.RWMutex
	currentQR string
	qrSubs    []chan string
	subsMu    sync.RWMutex

	// Config
	configMu sync.RWMutex
	config   channel.ChannelConfig

	// Identity cache (LID <-> PN)
	identityMap sync.Map

	stopSync chan struct{}
}

// NewAdapter creates a new adapter from an existing client or prepares a new one
func NewAdapter(channelID, workspaceID, sessionID string, client *whatsmeow.Client, manager *workspace.Manager) *WhatsAppAdapter {
	return &WhatsAppAdapter{
		channelID:   channelID,
		workspaceID: workspaceID,
		sessionID:   sessionID,
		client:      client,
		manager:     manager, // Store manager reference
		qrSubs:      make([]chan string, 0),
		identityMap: sync.Map{},
		stopSync:    make(chan struct{}),
	}
}

// ID returns the channel ID
func (wa *WhatsAppAdapter) ID() string {
	return wa.channelID
}

// Type returns the channel ID
func (wa *WhatsAppAdapter) Type() channel.ChannelType {
	return channel.ChannelTypeWhatsApp
}

// getChatStorage returns the chat storage repository, initializing it if necessary
func (wa *WhatsAppAdapter) getChatStorage() domainChatStorage.IChatStorageRepository {
	wa.repoMu.Lock()
	defer wa.repoMu.Unlock()

	if wa.chatStorage != nil {
		return wa.chatStorage
	}

	repo, err := infraChatStorage.GetOrInitInstanceRepository(wa.channelID)
	if err != nil {
		logrus.Errorf("[WHATSAPP] Failed to initialize chat storage for %s: %v", wa.channelID, err)
		return nil
	}

	wa.chatStorage = repo
	return repo
}

func (wa *WhatsAppAdapter) UpdateConfig(config channel.ChannelConfig) {
	wa.configMu.Lock()
	defer wa.configMu.Unlock()
	wa.config = config
	logrus.Infof("[WHATSAPP] Updated configuration for channel %s", wa.channelID)
}

// parseJID is a helper to convert a string to a types.JID
func (wa *WhatsAppAdapter) parseJID(chatID string) (types.JID, error) {
	if strings.Contains(chatID, "@") {
		return types.ParseJID(chatID)
	}
	// Default to @s.whatsapp.net for plain numbers
	return types.NewJID(chatID, types.DefaultUserServer), nil
}

// GetMyIdentity returns the bot's own JID and LID
func (wa *WhatsAppAdapter) GetMe() (common.ContactInfo, error) {
	if wa.client == nil || wa.client.Store == nil || wa.client.Store.ID == nil {
		return common.ContactInfo{}, fmt.Errorf("no client or not logged in")
	}

	// We return the JID as the primary identifier (likely PN for standard accounts)
	// But we need to make sure we expose LID too if possible,
	// or specific fields. Since ContactInfo matches, we use ID as string.
	// But to help the tool, we return the PN JID in JID and LID in a 'Status' hack or create a new field?
	// The tool needs both to compare.
	// Actually, the Store.ID contains both if connected with LID support.

	// We return the JID as the primary identifier
	jid := wa.client.Store.ID.ToNonAD().String()
	lid := ""

	// Try to resolve LID from Store if available
	if wa.client.Store.LIDs != nil {
		// Try to see if this JID has a cached LID
		parsedJID := wa.client.Store.ID.ToNonAD()
		// If our ID is already LID (rare but possible), use it
		if parsedJID.Server == "lid" {
			lid = jid
			// We might want to find the PN then?
			// But for now, let's focus on getting LID if we have PN
		} else {
			// Try to find LID for our PN
			foundLID, err := wa.client.Store.LIDs.GetLIDForPN(context.Background(), parsedJID)
			if err == nil && !foundLID.IsEmpty() {
				lid = foundLID.ToNonAD().String()
			}
		}
	}

	return common.ContactInfo{
		JID:  jid,
		LID:  lid,
		Name: "Me",
	}, nil
}

// CloseSession ends the interaction with a specific chat
func (wa *WhatsAppAdapter) CloseSession(ctx context.Context, chatID string) error {
	// Local cleanup if needed, but DO NOT call manager.CloseSession (infinite loop)
	logrus.Infof("[WHATSAPP] Session closed for chat %s", chatID)
	return nil
}

// Note: FetchNewsletters, UnfollowNewsletter, SendNewsletterMessage, SendMessage, OnMessage
// are implemented in their respective files (newsletters.go, messaging.go, events.go).

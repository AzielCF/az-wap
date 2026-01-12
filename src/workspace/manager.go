package workspace

import (
	"context"
	"fmt"
	"sync"

	"github.com/AzielCF/az-wap/botengine"
	"github.com/AzielCF/az-wap/workspace/domain"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/sirupsen/logrus"
)

type AdapterFactory func(config domain.ChannelConfig) (ChannelAdapter, error)

type Manager struct {
	repo      repository.IWorkspaceRepository
	botEngine *botengine.Engine

	adaptersMu sync.RWMutex
	adapters   map[string]ChannelAdapter // key: channelID

	factoriesMu sync.RWMutex
	factories   map[domain.ChannelType]AdapterFactory
}

func NewManager(repo repository.IWorkspaceRepository, engine *botengine.Engine) *Manager {
	return &Manager{
		repo:      repo,
		botEngine: engine,
		adapters:  make(map[string]ChannelAdapter),
		factories: make(map[domain.ChannelType]AdapterFactory),
	}
}

// RegisterFactory registers a factory function for a specific channel type
func (m *Manager) RegisterFactory(chType domain.ChannelType, factory AdapterFactory) {
	m.factoriesMu.Lock()
	defer m.factoriesMu.Unlock()
	m.factories[chType] = factory
}

// RegisterAdapter adds an active adapter to the manager
func (m *Manager) RegisterAdapter(adapter ChannelAdapter) {
	m.adaptersMu.Lock()
	defer m.adaptersMu.Unlock()
	m.adapters[adapter.ID()] = adapter

	// Wire up the message handler
	adapter.OnMessage(m.handleIncomingMessage)
}

// UnregisterAdapter removes an adapter
func (m *Manager) UnregisterAdapter(channelID string) {
	m.adaptersMu.Lock()
	defer m.adaptersMu.Unlock()
	if adapter, ok := m.adapters[channelID]; ok {
		// Best effort stop
		_ = adapter.Stop(context.Background())
		delete(m.adapters, channelID)
	}
}

// GetAdapter returns an active adapter by ID
func (m *Manager) GetAdapter(channelID string) (ChannelAdapter, bool) {
	m.adaptersMu.RLock()
	defer m.adaptersMu.RUnlock()
	adapter, ok := m.adapters[channelID]
	return adapter, ok
}

// StartChannel initializes and starts a channel by its ID
func (m *Manager) StartChannel(ctx context.Context, channelID string) error {
	// 1. Check if already running
	if _, ok := m.GetAdapter(channelID); ok {
		return nil
	}

	// 2. Get Channel info from DB
	ch, err := m.repo.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	// 3. Find factory
	m.factoriesMu.RLock()
	factory, ok := m.factories[ch.Type]
	m.factoriesMu.RUnlock()
	if !ok {
		return fmt.Errorf("no factory registered for channel type %s", ch.Type)
	}

	// 4. Create adapter
	adapter, err := factory(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// 5. Start adapter
	if err := adapter.Start(ctx, ch.Config); err != nil {
		return fmt.Errorf("failed to start adapter: %w", err)
	}

	// 6. Register
	m.RegisterAdapter(adapter)

	// Update status
	ch.Status = domain.ChannelStatusConnected
	_ = m.repo.UpdateChannel(ctx, ch)

	return nil
}

// handleIncomingMessage is the central point for all incoming messages from all channels
func (m *Manager) handleIncomingMessage(msg IncomingMessage) {
	logrus.WithFields(logrus.Fields{
		"workspace_id": msg.WorkspaceID,
		"channel_id":   msg.ChannelID,
		"sender_id":    msg.SenderID,
	}).Info("[WorkspaceManager] Received message")

	// 1. Convert to BotInput
	// Note: In a real implementation, we would fetch the BotID from the Workspace configuration or a "Router"
	// For now, we assume the workspace might have a "default_bot_id" in its metadata, or we pass a placeholder.

	// Try to get default bot from context or similar?
	// Since we don't have the context here easily, we'll placeholder it.
	// In the future: BotID = m.ResolveBotID(msg.WorkspaceID)

	botID := "default_bot" // Placeholder

	botInput := botengine.BotInput{
		TraceID:    "",            // Engine will generate if empty
		InstanceID: msg.ChannelID, // Use ChannelID as InstanceID
		SenderID:   msg.SenderID,
		ChatID:     msg.ChatID,
		BotID:      botID,
		Platform:   botengine.PlatformWhatsApp, // TODO: Map dynamically from channel type
		Text:       msg.Text,
		Media:      nil, // msg.Media needs conversion
		Metadata:   msg.Metadata,
	}

	// 2. Process
	// We need a context here. Ideally handleIncomingMessage should take a context or creating a background one.
	ctx := context.Background()
	_, err := m.botEngine.Process(ctx, botInput)
	if err != nil {
		logrus.WithError(err).Error("[WorkspaceManager] Bot message processing failed")
	}
}

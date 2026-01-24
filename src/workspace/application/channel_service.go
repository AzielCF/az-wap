package application

import (
	"context"
	"fmt"
	"sync"

	botengine "github.com/AzielCF/az-wap/botengine"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
)

type AdapterFactory func(config channel.ChannelConfig) (channel.ChannelAdapter, error)

type ChannelService struct {
	repo      workspace.IWorkspaceRepository
	botEngine *botengine.Engine

	adaptersMu sync.RWMutex
	adapters   map[string]channel.ChannelAdapter

	factoriesMu sync.RWMutex
	factories   map[channel.ChannelType]AdapterFactory

	// Bridge for botengine transport
	transportBridge func(adapter channel.ChannelAdapter) botengineDomain.Transport

	OnAdapterRegistered func(adapter channel.ChannelAdapter)
}

func NewChannelService(repo workspace.IWorkspaceRepository, botEngine *botengine.Engine, bridge func(channel.ChannelAdapter) botengineDomain.Transport) *ChannelService {
	return &ChannelService{
		repo:            repo,
		botEngine:       botEngine,
		adapters:        make(map[string]channel.ChannelAdapter),
		factories:       make(map[channel.ChannelType]AdapterFactory),
		transportBridge: bridge,
	}
}

func (s *ChannelService) RegisterFactory(chType channel.ChannelType, factory AdapterFactory) {
	s.factoriesMu.Lock()
	defer s.factoriesMu.Unlock()
	s.factories[chType] = factory
}

func (s *ChannelService) RegisterAdapter(adapter channel.ChannelAdapter, handleMsg func(channel.ChannelAdapter, message.IncomingMessage)) {
	s.adaptersMu.Lock()
	defer s.adaptersMu.Unlock()
	s.adapters[adapter.ID()] = adapter

	// Wire up the message handler
	adapter.OnMessage(func(msg message.IncomingMessage) {
		handleMsg(adapter, msg)
	})

	// Register as transport in botEngine
	if s.botEngine != nil && s.transportBridge != nil {
		s.botEngine.RegisterTransport(s.transportBridge(adapter))
	}

	if s.OnAdapterRegistered != nil {
		s.OnAdapterRegistered(adapter)
	}
}

func (s *ChannelService) UnregisterAdapter(channelID string) {
	s.adaptersMu.Lock()
	adapter, ok := s.adapters[channelID]
	if ok {
		delete(s.adapters, channelID)
	}
	s.adaptersMu.Unlock()

	if ok {
		_ = adapter.Stop(context.Background())
		if s.botEngine != nil {
			s.botEngine.UnregisterTransport(channelID)
		}
	}
}

// UnregisterAndCleanup stops the adapter and deletes all persistent data
func (s *ChannelService) UnregisterAndCleanup(channelID string) {
	s.adaptersMu.Lock()
	adapter, ok := s.adapters[channelID]
	if ok {
		delete(s.adapters, channelID)
	}
	s.adaptersMu.Unlock()

	if ok {
		_ = adapter.Cleanup(context.Background())
		if s.botEngine != nil {
			s.botEngine.UnregisterTransport(channelID)
		}
	}
}

func (s *ChannelService) GetAdapter(channelID string) (channel.ChannelAdapter, bool) {
	s.adaptersMu.RLock()
	defer s.adaptersMu.RUnlock()
	adapter, ok := s.adapters[channelID]
	return adapter, ok
}

func (s *ChannelService) GetAdapters() []channel.ChannelAdapter {
	s.adaptersMu.RLock()
	defer s.adaptersMu.RUnlock()
	var res []channel.ChannelAdapter
	for _, a := range s.adapters {
		res = append(res, a)
	}
	return res
}

func (s *ChannelService) StartChannel(ctx context.Context, channelID string, handleMsg func(channel.ChannelAdapter, message.IncomingMessage)) error {
	if _, ok := s.GetAdapter(channelID); ok {
		return nil
	}

	ch, err := s.repo.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	s.factoriesMu.RLock()
	factory, ok := s.factories[ch.Type]
	s.factoriesMu.RUnlock()
	if !ok {
		return fmt.Errorf("no factory registered for channel type %s", ch.Type)
	}

	if ch.Config.Settings == nil {
		ch.Config.Settings = make(map[string]any)
	}
	ch.Config.Settings["channel_id"] = ch.ID
	ch.Config.Settings["workspace_id"] = ch.WorkspaceID

	adapter, err := factory(ch.Config)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	if err := adapter.Start(ctx, ch.Config); err != nil {
		return fmt.Errorf("failed to start adapter: %w", err)
	}

	s.RegisterAdapter(adapter, handleMsg)

	ch.Status = channel.ChannelStatusConnected
	_ = s.repo.UpdateChannel(ctx, ch)

	return nil
}

func (s *ChannelService) UpdateChannelConfig(channelID string, config channel.ChannelConfig) {
	s.adaptersMu.RLock()
	adapter, ok := s.adapters[channelID]
	s.adaptersMu.RUnlock()

	if ok {
		adapter.UpdateConfig(config)
	}
}

func (s *ChannelService) SetProfilePhoto(ctx context.Context, channelID string, photo []byte) (string, error) {
	s.adaptersMu.RLock()
	adapter, ok := s.adapters[channelID]
	s.adaptersMu.RUnlock()

	if !ok {
		return "", fmt.Errorf("channel adapter %s not found or not connected", channelID)
	}

	return adapter.SetProfilePhoto(ctx, photo)
}

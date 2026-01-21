package repository

import (
	"context"
	"sync"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
)

// MemoryPresenceStore implementa PresenceStore en memoria.
type MemoryPresenceStore struct {
	mu    sync.RWMutex
	store map[string]*channel.ChannelPresence
}

// NewMemoryPresenceStore crea una nueva instancia de MemoryPresenceStore.
func NewMemoryPresenceStore() *MemoryPresenceStore {
	return &MemoryPresenceStore{
		store: make(map[string]*channel.ChannelPresence),
	}
}

func (m *MemoryPresenceStore) Save(ctx context.Context, presence *channel.ChannelPresence) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[presence.ChannelID] = presence
	return nil
}

func (m *MemoryPresenceStore) Get(ctx context.Context, channelID string) (*channel.ChannelPresence, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.store[channelID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *MemoryPresenceStore) Delete(ctx context.Context, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, channelID)
	return nil
}

func (m *MemoryPresenceStore) GetAll(ctx context.Context) ([]*channel.ChannelPresence, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*channel.ChannelPresence, 0, len(m.store))
	for _, p := range m.store {
		result = append(result, p)
	}
	return result, nil
}

package repository

import (
	"context"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
)

const TypingExpiration = 7 * time.Second

type memoryTypingEntry struct {
	state channel.TypingState
}

// MemoryTypingStore implementa TypingStore en memoria.
type MemoryTypingStore struct {
	mu    sync.Mutex
	store map[string]memoryTypingEntry
}

func NewMemoryTypingStore() *MemoryTypingStore {
	return &MemoryTypingStore{
		store: make(map[string]memoryTypingEntry),
	}
}

func (m *MemoryTypingStore) key(channelID, chatID string) string {
	return channelID + "|" + chatID
}

func (m *MemoryTypingStore) Update(ctx context.Context, channelID, chatID string, isTyping bool, media channel.TypingMedia) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(channelID, chatID)
	if !isTyping {
		delete(m.store, k)
		return nil
	}

	m.store[k] = memoryTypingEntry{
		state: channel.TypingState{
			ChannelID: channelID,
			ChatID:    chatID,
			Media:     media,
			UpdatedAt: time.Now(),
		},
	}
	return nil
}

func (m *MemoryTypingStore) Get(ctx context.Context, channelID, chatID string) (*channel.TypingState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(channelID, chatID)
	e, ok := m.store[k]
	if !ok {
		return nil, nil
	}

	if time.Since(e.state.UpdatedAt) > TypingExpiration {
		delete(m.store, k)
		return nil, nil
	}

	return &e.state, nil
}

func (m *MemoryTypingStore) GetAll(ctx context.Context) ([]channel.TypingState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var active []channel.TypingState
	for k, e := range m.store {
		if now.Sub(e.state.UpdatedAt) > TypingExpiration {
			delete(m.store, k)
			continue
		}
		active = append(active, e.state)
	}
	return active, nil
}

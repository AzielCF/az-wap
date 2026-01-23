package repository

import (
	"context"
	"sync"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
)

// MemoryContextCacheStore is an in-memory implementation of ContextCacheStore.
// Used as fallback when Valkey is not enabled.
type MemoryContextCacheStore struct {
	mu      sync.RWMutex
	entries map[string]*domain.ContextCacheEntry
}

// NewMemoryContextCacheStore creates a new in-memory context cache store.
func NewMemoryContextCacheStore() *MemoryContextCacheStore {
	store := &MemoryContextCacheStore{
		entries: make(map[string]*domain.ContextCacheEntry),
	}
	go store.cleanupLoop()
	return store
}

func (s *MemoryContextCacheStore) Get(ctx context.Context, fingerprint string) (*domain.ContextCacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[fingerprint]
	if !ok {
		return nil, nil
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		return nil, nil
	}

	return entry, nil
}

func (s *MemoryContextCacheStore) Save(ctx context.Context, fingerprint string, entry *domain.ContextCacheEntry, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[fingerprint] = entry
	return nil
}

func (s *MemoryContextCacheStore) Delete(ctx context.Context, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, fingerprint)
	return nil
}

func (s *MemoryContextCacheStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.ExpiresAt) {
			delete(s.entries, key)
		}
	}
	return nil
}

func (s *MemoryContextCacheStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		_ = s.Cleanup(context.Background())
	}
}

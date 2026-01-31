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

// List returns all active (non-expired) cache entries for UI inspection.
func (s *MemoryContextCacheStore) List(ctx context.Context) ([]*domain.ContextCacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var result []*domain.ContextCacheEntry
	for _, entry := range s.entries {
		if now.Before(entry.ExpiresAt) {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (s *MemoryContextCacheStore) Lock(ctx context.Context, fingerprint string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockKey := "lock:" + fingerprint
	entry, ok := s.entries[lockKey]
	if ok && time.Now().Before(entry.ExpiresAt) {
		return false, nil
	}

	// Create a dummy entry to act as a lock
	s.entries[lockKey] = &domain.ContextCacheEntry{
		ExpiresAt: time.Now().Add(ttl),
	}
	return true, nil
}

func (s *MemoryContextCacheStore) Unlock(ctx context.Context, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, "lock:"+fingerprint)
	return nil
}

func (s *MemoryContextCacheStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		_ = s.Cleanup(context.Background())
	}
}

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	valkeylib "github.com/valkey-io/valkey-go"

	"github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/infrastructure/valkey"
)

// ValkeyContextCacheStore implements domain.ContextCacheStore using Valkey.
type ValkeyContextCacheStore struct {
	client *valkey.Client
	prefix string
}

// NewValkeyContextCacheStore creates a new ValkeyContextCacheStore instance.
func NewValkeyContextCacheStore(client *valkey.Client) *ValkeyContextCacheStore {
	return &ValkeyContextCacheStore{
		client: client,
		prefix: client.Key("context_cache") + ":",
	}
}

func (s *ValkeyContextCacheStore) fullKey(fingerprint string) string {
	return s.prefix + fingerprint
}

func (s *ValkeyContextCacheStore) inner() valkeylib.Client {
	return s.client.Inner()
}

// Get retrieves a cache entry by its fingerprint key.
func (s *ValkeyContextCacheStore) Get(ctx context.Context, fingerprint string) (*domain.ContextCacheEntry, error) {
	cmd := s.inner().B().Get().Key(s.fullKey(fingerprint)).Build()

	data, err := s.inner().Do(ctx, cmd).AsBytes()
	if err != nil {
		if valkeylib.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get context cache: %w", err)
	}

	var entry domain.ContextCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context cache: %w", err)
	}

	return &entry, nil
}

// Save stores a cache entry with the given fingerprint and TTL.
func (s *ValkeyContextCacheStore) Save(ctx context.Context, fingerprint string, entry *domain.ContextCacheEntry, ttl time.Duration) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal context cache: %w", err)
	}

	cmd := s.inner().B().Set().
		Key(s.fullKey(fingerprint)).
		Value(string(data)).
		Ex(ttl).
		Build()

	if err := s.inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to save context cache: %w", err)
	}
	return nil
}

// Delete removes a cache entry.
func (s *ValkeyContextCacheStore) Delete(ctx context.Context, fingerprint string) error {
	cmd := s.inner().B().Del().Key(s.fullKey(fingerprint)).Build()
	if err := s.inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to delete context cache: %w", err)
	}
	return nil
}

// Cleanup is a no-op for Valkey since expiration is handled by TTL.
func (s *ValkeyContextCacheStore) Cleanup(ctx context.Context) error {
	// Valkey handles expiration natively via TTL
	return nil
}

// List returns all active (non-expired) cache entries for UI inspection.
func (s *ValkeyContextCacheStore) List(ctx context.Context) ([]*domain.ContextCacheEntry, error) {
	var keys []string
	var cursor uint64

	for {
		cmd := s.inner().B().Scan().Cursor(cursor).Match(s.prefix + "*").Count(100).Build()
		result, err := s.inner().Do(ctx, cmd).AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("failed to scan context cache: %w", err)
		}

		keys = append(keys, result.Elements...)
		cursor = result.Cursor
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return []*domain.ContextCacheEntry{}, nil
	}

	// Fetch all values using MGET
	cmd := s.inner().B().Mget().Key(keys...).Build()
	values, err := s.inner().Do(ctx, cmd).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to mget context cache entries: %w", err)
	}

	entries := make([]*domain.ContextCacheEntry, 0, len(values))
	for _, val := range values {
		if val == "" {
			continue // Key might have expired between SCAN and MGET
		}
		var entry domain.ContextCacheEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			logrus.Warnf("[ValkeyContextCacheStore] Failed to unmarshal entry: %v", err)
			continue
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}
func (s *ValkeyContextCacheStore) Lock(ctx context.Context, fingerprint string, ttl time.Duration) (bool, error) {
	lockKey := s.fullKey("lock:" + fingerprint)
	cmd := s.inner().B().Set().
		Key(lockKey).
		Value("1").
		Nx().
		Ex(ttl).
		Build()

	err := s.inner().Do(ctx, cmd).Error()
	if err != nil {
		if valkeylib.IsValkeyNil(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return true, nil
}

func (s *ValkeyContextCacheStore) Unlock(ctx context.Context, fingerprint string) error {
	lockKey := s.fullKey("lock:" + fingerprint)
	cmd := s.inner().B().Del().Key(lockKey).Build()
	return s.inner().Do(ctx, cmd).Error()
}

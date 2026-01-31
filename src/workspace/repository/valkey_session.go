package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	valkeylib "github.com/valkey-io/valkey-go"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace/domain/session"
	"github.com/google/uuid"
)

const (
	lockSuffix     = ":lock"
	lockTTL        = 2 * time.Second       // Maximum time a lock can live (prevents deadlocks)
	lockWaitTime   = 50 * time.Millisecond // Time between lock acquisition attempts
	maxLockRetries = 10                    // Maximum attempts to acquire a lock
)

// Lua script for atomic lock release (only delete if token matches)
const releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`

// ValkeySessionStore implements session.SessionStore using Valkey as the backend.
// It provides distributed locking for safe concurrent updates.
type ValkeySessionStore struct {
	client *valkey.Client
	prefix string
}

// NewValkeySessionStore creates a new ValkeySessionStore instance.
// The client should be created via valkey.NewClient and passed here.
func NewValkeySessionStore(client *valkey.Client) *ValkeySessionStore {
	return &ValkeySessionStore{
		client: client,
		prefix: client.Key("session") + ":",
	}
}

func (s *ValkeySessionStore) fullKey(key string) string {
	return s.prefix + key
}

func (s *ValkeySessionStore) lockKey(key string) string {
	return s.fullKey(key) + lockSuffix
}

func (s *ValkeySessionStore) inner() valkeylib.Client {
	return s.client.Inner()
}

// Save stores a session entry with the given TTL.
func (s *ValkeySessionStore) Save(ctx context.Context, key string, entry *session.SessionEntry, ttl time.Duration) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	cmd := s.inner().B().Set().
		Key(s.fullKey(key)).
		Value(string(data)).
		Ex(ttl).
		Build()

	if err := s.inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// Get retrieves a session entry by its key.
// Returns (nil, nil) if the key does not exist.
func (s *ValkeySessionStore) Get(ctx context.Context, key string) (*session.SessionEntry, error) {
	cmd := s.inner().B().Get().Key(s.fullKey(key)).Build()

	data, err := s.inner().Do(ctx, cmd).AsBytes()
	if err != nil {
		if valkeylib.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var entry session.SessionEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &entry, nil
}

// Delete removes a session entry.
func (s *ValkeySessionStore) Delete(ctx context.Context, key string) error {
	cmd := s.inner().B().Del().Key(s.fullKey(key)).Build()
	if err := s.inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// Extend renews the TTL of an existing session without modifying its data.
// This operation is atomic in Valkey/Redis (EXPIRE command).
func (s *ValkeySessionStore) Extend(ctx context.Context, key string, ttl time.Duration) error {
	cmd := s.inner().B().Expire().Key(s.fullKey(key)).Seconds(int64(ttl.Seconds())).Build()
	result, err := s.inner().Do(ctx, cmd).AsInt64()
	if err != nil {
		return fmt.Errorf("failed to extend session TTL: %w", err)
	}
	if result == 0 {
		// Key does not exist, but we don't treat this as an error per interface contract
		return nil
	}
	return nil
}

// List returns all keys matching the given pattern.
// The pattern uses glob syntax (e.g., "channel123|*").
// Uses SCAN for production safety (non-blocking).
func (s *ValkeySessionStore) List(ctx context.Context, pattern string) ([]string, error) {
	fullPattern := s.prefix + pattern
	var keys []string
	var cursor uint64

	for {
		cmd := s.inner().B().Scan().Cursor(cursor).Match(fullPattern).Count(100).Build()
		result, err := s.inner().Do(ctx, cmd).AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("failed to scan sessions: %w", err)
		}

		// Remove prefix from returned keys
		for _, k := range result.Elements {
			if len(k) > len(s.prefix) {
				keys = append(keys, k[len(s.prefix):])
			}
		}

		cursor = result.Cursor
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// Exists checks if a session key exists.
func (s *ValkeySessionStore) Exists(ctx context.Context, key string) (bool, error) {
	cmd := s.inner().B().Exists().Key(s.fullKey(key)).Build()
	count, err := s.inner().Do(ctx, cmd).AsInt64()
	if err != nil {
		return false, fmt.Errorf("failed to check session existence: %w", err)
	}
	return count > 0, nil
}

// GetAll returns all active sessions.
// Uses pipelining (MGET) for efficiency when fetching multiple sessions.
// WARNING: This can still be expensive with very large session counts.
func (s *ValkeySessionStore) GetAll(ctx context.Context) (map[string]*session.SessionEntry, error) {
	keys, err := s.List(ctx, "*")
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return make(map[string]*session.SessionEntry), nil
	}

	// Build full keys for MGET
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = s.fullKey(k)
	}

	// Use MGET for pipelining (single round-trip)
	cmd := s.inner().B().Mget().Key(fullKeys...).Build()
	values, err := s.inner().Do(ctx, cmd).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to mget sessions: %w", err)
	}

	result := make(map[string]*session.SessionEntry)
	for i, val := range values {
		if val == "" {
			continue // Key expired between SCAN and MGET
		}
		var entry session.SessionEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			logrus.Warnf("[ValkeySessionStore] Failed to unmarshal session %s: %v", keys[i], err)
			continue
		}
		result[keys[i]] = &entry
	}

	return result, nil
}

// UpdateField updates a specific field of a session safely using a Distributed Lock.
// This prevents "lost updates" when multiple instances try to update the same session concurrently.
func (s *ValkeySessionStore) UpdateField(ctx context.Context, key string, field string, value any) error {
	// 1. Acquire Distributed Lock
	lockToken, err := s.acquireLock(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for session %s: %w", key, err)
	}
	// Ensure lock is released even if panic occurs
	defer func() {
		if releaseErr := s.releaseLock(ctx, key, lockToken); releaseErr != nil {
			logrus.Warnf("[ValkeySessionStore] Failed to release lock for %s: %v", key, releaseErr)
		}
	}()

	// 2. Get current TTL (to preserve it)
	ttlCmd := s.inner().B().Ttl().Key(s.fullKey(key)).Build()
	ttlSeconds, err := s.inner().Do(ctx, ttlCmd).AsInt64()
	if err != nil {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttlSeconds < 0 {
		// Key doesn't exist (-2) or has no TTL (-1), stop here
		return nil
	}

	// 3. Get current data (Critical Section)
	entry, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if entry == nil {
		return nil // Session vanished
	}

	// 4. Update the specific field in memory
	switch field {
	case "last_seen":
		if t, ok := value.(time.Time); ok {
			entry.LastSeen = t
		}
	case "state":
		if st, ok := value.(session.SessionState); ok {
			entry.State = st
		}
	case "focus_score":
		if sc, ok := value.(int); ok {
			entry.FocusScore = sc
		}
	case "chat_open":
		if b, ok := value.(bool); ok {
			entry.ChatOpen = b
		}
	case "last_reply_time":
		if t, ok := value.(time.Time); ok {
			entry.LastReplyTime = t
		}
	case "language":
		if lang, ok := value.(string); ok {
			entry.Language = lang
		}
	default:
		// Unknown field, just return (or could error)
		return nil
	}

	// 5. Save modified data (still inside Critical Section)
	// We use the remaining TTL to avoid extending session lifetime unintentionally
	return s.Save(ctx, key, entry, time.Duration(ttlSeconds)*time.Second)
}

// acquireLock attempts to acquire a distributed lock for a specific session key.
// It uses a spinlock mechanism with retries and exponential backoff with jitter.
func (s *ValkeySessionStore) acquireLock(ctx context.Context, key string) (string, error) {
	lockKey := s.lockKey(key)
	token := uuid.New().String() // Unique token to ensure we only release our own lock

	for i := 0; i < maxLockRetries; i++ {
		// SET key token NX EX ttl
		// NX: Only set if not exists
		// EX: Expire after lockTTL
		cmd := s.inner().B().Set().
			Key(lockKey).
			Value(token).
			Nx().
			Ex(lockTTL).
			Build()

		err := s.inner().Do(ctx, cmd).Error()
		if err == nil {
			// Lock acquired successfully
			return token, nil
		}

		if !valkeylib.IsValkeyNil(err) {
			// Real error (connection, etc), log but continue retrying
			logrus.Debugf("[ValkeySessionStore] Lock attempt %d failed for %s: %v", i+1, key, err)
		}

		// Wait with random jitter to avoid thundering herd
		sleepDuration := lockWaitTime + time.Duration(rand.Intn(20))*time.Millisecond
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(sleepDuration):
			continue
		}
	}

	return "", errors.New("lock acquisition timed out after max retries")
}

// releaseLock releases the distributed lock ONLY if the token matches.
// This prevents deleting a lock that was already re-acquired by someone else (e.g. after timeout).
// Uses a Lua script for atomicity.
func (s *ValkeySessionStore) releaseLock(ctx context.Context, key string, token string) error {
	lockKey := s.lockKey(key)

	cmd := s.inner().B().Eval().
		Script(releaseLockScript).
		Numkeys(1).
		Key(lockKey).
		Arg(token).
		Build()

	if err := s.inner().Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}

package repository

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/session"
	"github.com/sirupsen/logrus"
)

// MemorySessionStore implementa SessionStore usando un map en memoria.
// Esta es la implementación por defecto y más simple.
// Los datos se pierden al reiniciar el servidor.
type MemorySessionStore struct {
	mu      sync.RWMutex
	entries map[string]*memoryEntry
}

type memoryEntry struct {
	data     *session.SessionEntry
	expireAt time.Time
}

// NewMemorySessionStore crea un nuevo store en memoria.
// Inicia automáticamente un goroutine de limpieza que elimina sesiones expiradas.
func NewMemorySessionStore() *MemorySessionStore {
	ms := &MemorySessionStore{
		entries: make(map[string]*memoryEntry),
	}
	go ms.cleanupLoop()
	return ms
}

func (ms *MemorySessionStore) Save(ctx context.Context, key string, entry *session.SessionEntry, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	expireAt := time.Now().Add(ttl)
	ms.entries[key] = &memoryEntry{
		data:     entry,
		expireAt: expireAt,
	}
	// Sync ExpireAt in the entry itself for consistency
	entry.ExpireAt = expireAt

	return nil
}

func (ms *MemorySessionStore) Get(ctx context.Context, key string) (*session.SessionEntry, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	e, ok := ms.entries[key]
	if !ok {
		return nil, nil
	}

	// Check if expired
	if time.Now().After(e.expireAt) {
		// Don't delete here to avoid write lock, cleanup loop handles it
		return nil, nil
	}

	return e.data, nil
}

func (ms *MemorySessionStore) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.entries, key)
	return nil
}

func (ms *MemorySessionStore) Extend(ctx context.Context, key string, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	e, ok := ms.entries[key]
	if !ok {
		return nil // Silent if doesn't exist (or could return error)
	}

	e.expireAt = time.Now().Add(ttl)
	e.data.ExpireAt = e.expireAt
	return nil
}

func (ms *MemorySessionStore) List(ctx context.Context, pattern string) ([]string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var result []string
	now := time.Now()

	for key, e := range ms.entries {
		if now.After(e.expireAt) {
			continue
		}
		// Simple glob matching
		if pattern == "" || pattern == "*" {
			result = append(result, key)
		} else if matched, _ := filepath.Match(pattern, key); matched {
			result = append(result, key)
		} else if strings.HasPrefix(pattern, "*") && strings.HasSuffix(key, pattern[1:]) {
			result = append(result, key)
		} else if strings.HasSuffix(pattern, "*") && strings.HasPrefix(key, pattern[:len(pattern)-1]) {
			result = append(result, key)
		} else if strings.Contains(pattern, "|") && strings.Contains(key, strings.Split(pattern, "*")[0]) {
			// Handle patterns like "channelID|*"
			prefix := strings.Split(pattern, "*")[0]
			if strings.HasPrefix(key, prefix) {
				result = append(result, key)
			}
		}
	}

	return result, nil
}

func (ms *MemorySessionStore) Exists(ctx context.Context, key string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	e, ok := ms.entries[key]
	if !ok {
		return false, nil
	}

	if time.Now().After(e.expireAt) {
		return false, nil
	}

	return true, nil
}

func (ms *MemorySessionStore) GetAll(ctx context.Context) (map[string]*session.SessionEntry, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make(map[string]*session.SessionEntry)
	now := time.Now()

	for key, e := range ms.entries {
		if now.After(e.expireAt) {
			continue
		}
		result[key] = e.data
	}

	return result, nil
}

func (ms *MemorySessionStore) UpdateField(ctx context.Context, key string, field string, value any) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	e, ok := ms.entries[key]
	if !ok {
		return nil
	}

	// Update specific fields - memory implementation updates the struct directly
	switch field {
	case "last_seen":
		if t, ok := value.(time.Time); ok {
			e.data.LastSeen = t
		}
	case "state":
		if s, ok := value.(session.SessionState); ok {
			e.data.State = s
		}
	case "focus_score":
		if s, ok := value.(int); ok {
			e.data.FocusScore = s
		}
	case "chat_open":
		if b, ok := value.(bool); ok {
			e.data.ChatOpen = b
		}
	case "last_reply_time":
		if t, ok := value.(time.Time); ok {
			e.data.LastReplyTime = t
		}
	case "language":
		if s, ok := value.(string); ok {
			e.data.Language = s
		}
	default:
		logrus.Warnf("[MemorySessionStore] UpdateField: unknown field %s", field)
	}

	return nil
}

// cleanupLoop runs periodically to remove expired sessions
func (ms *MemorySessionStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.cleanup()
	}
}

func (ms *MemorySessionStore) cleanup() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	var expiredKeys []string

	for key, e := range ms.entries {
		if now.After(e.expireAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(ms.entries, key)
		logrus.Debugf("[MemorySessionStore] Cleaned up expired session: %s", key)
	}

	if len(expiredKeys) > 0 {
		logrus.Infof("[MemorySessionStore] Cleanup: removed %d expired sessions", len(expiredKeys))
	}
}

// GetEntry returns the raw entry with expiration info (for internal use/migration)
func (ms *MemorySessionStore) GetEntry(key string) (*session.SessionEntry, time.Time, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	e, ok := ms.entries[key]
	if !ok {
		return nil, time.Time{}, false
	}
	return e.data, e.expireAt, true
}

// Stats returns basic statistics about the store
func (ms *MemorySessionStore) Stats() (total int, expired int) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	now := time.Now()
	for _, e := range ms.entries {
		total++
		if now.After(e.expireAt) {
			expired++
		}
	}
	return
}

package kvstore

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/sirupsen/logrus"
)

// KVStore defines the standard interface for key-value storage in the system.
// It abstracts the implementation (Valkey vs Memory) while providing advanced features.
type KVStore interface {
	// Basic Operations
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Locks (Atomic - useful for distributed tasks or avoiding race conditions)
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error

	// Discovery
	Keys(ctx context.Context, pattern string) ([]string, error)
}

type cachedItem struct {
	value     string
	expiresAt time.Time
}

type smartStore struct {
	vkClient *valkey.Client
	memory   sync.Map
	locks    sync.Map // For memory-based locking
}

// NewSmartStore creates a store that automatically chooses between Valkey and Memory
func NewSmartStore(vkClient *valkey.Client) KVStore {
	s := &smartStore{
		vkClient: vkClient,
	}

	// Start background cleanup for memory-only mode
	if vkClient == nil {
		go s.memoryCleanupLoop()
	}

	return s
}

func (s *smartStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if s.vkClient != nil && s.vkClient.IsConnected() {
		err := s.vkClient.Set(ctx, key, value, ttl)
		if err != nil {
			logrus.Errorf("[KVStore] Valkey Set error for key %s: %v", key, err)
			return err
		}
		return nil
	}

	// Fallback to memory
	s.memory.Store(key, cachedItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
	return nil
}

func (s *smartStore) Get(ctx context.Context, key string) (string, error) {
	if s.vkClient != nil && s.vkClient.IsConnected() {
		val, err := s.vkClient.Get(ctx, key)
		if err != nil {
			logrus.Errorf("[KVStore] Valkey Get error for key %s: %v", key, err)
			return "", err
		}
		return val, nil
	}

	// Fallback to memory
	val, ok := s.memory.Load(key)
	if !ok {
		return "", nil
	}

	item := val.(cachedItem)
	if time.Now().After(item.expiresAt) {
		s.memory.Delete(key)
		return "", nil
	}

	return item.value, nil
}

func (s *smartStore) Exists(ctx context.Context, key string) (bool, error) {
	if s.vkClient != nil && s.vkClient.IsConnected() {
		res, err := s.vkClient.Inner().Do(ctx, s.vkClient.Inner().B().Exists().Key(key).Build()).AsInt64()
		if err != nil {
			logrus.Errorf("[KVStore] Valkey Exists error for key %s: %v", key, err)
			return false, err
		}
		return res > 0, nil
	}

	val, ok := s.memory.Load(key)
	if !ok {
		return false, nil
	}
	item := val.(cachedItem)
	if time.Now().After(item.expiresAt) {
		s.memory.Delete(key)
		return false, nil
	}
	return true, nil
}

func (s *smartStore) Delete(ctx context.Context, key string) error {
	if s.vkClient != nil && s.vkClient.IsConnected() {
		err := s.vkClient.Inner().Do(ctx, s.vkClient.Inner().B().Del().Key(key).Build()).Error()
		if err != nil {
			logrus.Errorf("[KVStore] Valkey Delete error for key %s: %v", key, err)
			return err
		}
		return nil
	}

	s.memory.Delete(key)
	return nil
}

func (s *smartStore) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := "lock:" + key
	if s.vkClient != nil && s.vkClient.IsConnected() {
		err := s.vkClient.Inner().Do(ctx, s.vkClient.Inner().B().Set().Key(lockKey).Value("1").Nx().Ex(ttl).Build()).Error()
		if err != nil {
			if valkey.IsNil(err) {
				return false, nil
			}
			logrus.Errorf("[KVStore] Valkey Lock error for key %s: %v", key, err)
			return false, err
		}
		return true, nil
	}

	// Memory locking
	now := time.Now()
	val, loaded := s.locks.LoadOrStore(lockKey, now.Add(ttl))
	if loaded {
		// Check if existing lock is expired
		expiry := val.(time.Time)
		if now.After(expiry) {
			s.locks.Store(lockKey, now.Add(ttl))
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

func (s *smartStore) Unlock(ctx context.Context, key string) error {
	lockKey := "lock:" + key
	if s.vkClient != nil && s.vkClient.IsConnected() {
		err := s.vkClient.Inner().Do(ctx, s.vkClient.Inner().B().Del().Key(lockKey).Build()).Error()
		if err != nil {
			logrus.Errorf("[KVStore] Valkey Unlock error for key %s: %v", key, err)
			return err
		}
		return nil
	}

	s.locks.Delete(lockKey)
	return nil
}

func (s *smartStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	if s.vkClient != nil && s.vkClient.IsConnected() {
		var allKeys []string
		var cursor uint64
		for {
			res, err := s.vkClient.Inner().Do(ctx, s.vkClient.Inner().B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()).AsScanEntry()
			if err != nil {
				logrus.Errorf("[KVStore] Valkey Scan error with pattern %s: %v", pattern, err)
				return nil, err
			}
			allKeys = append(allKeys, res.Elements...)
			cursor = res.Cursor
			if cursor == 0 {
				break
			}
		}
		return allKeys, nil
	}

	// Memory keys search (simple glob matching)
	var keys []string
	s.memory.Range(func(key, value any) bool {
		k := key.(string)
		item := value.(cachedItem)
		if time.Now().Before(item.expiresAt) {
			if matched, _ := filepath.Match(pattern, k); matched {
				keys = append(keys, k)
			}
		}
		return true
	})
	return keys, nil
}

func (s *smartStore) memoryCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	logrus.Debug("[KVStore] Memory cleanup loop started")
	for range ticker.C {
		now := time.Now()
		cleanedMain := 0
		cleanedLocks := 0

		s.memory.Range(func(key, value any) bool {
			item := value.(cachedItem)
			if now.After(item.expiresAt) {
				s.memory.Delete(key)
				cleanedMain++
			}
			return true
		})
		s.locks.Range(func(key, value any) bool {
			expiry := value.(time.Time)
			if now.After(expiry) {
				s.locks.Delete(key)
				cleanedLocks++
			}
			return true
		})

		if cleanedMain > 0 || cleanedLocks > 0 {
			logrus.Debugf("[KVStore] Memory Cleanup: removed %d items and %d expired locks", cleanedMain, cleanedLocks)
		}
	}
}

// Global instance helper
var Global KVStore

func Init(vkClient *valkey.Client) {
	Global = NewSmartStore(vkClient)
	if vkClient != nil {
		logrus.Info("[CORE] KVStore initialized with Valkey support")
	} else {
		logrus.Warn("[CORE] KVStore initialized in-memory mode (distributed features disabled)")
	}
}

package botengine

import (
	"sync"
)

type ChatTurn struct {
	Role string
	Text string
}

// MemoryStore gestiona el historial de conversaciones en memoria
type MemoryStore struct {
	mu     sync.RWMutex
	memory map[string][]ChatTurn // Key: memoryKey (ej: bot|id|sender)
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		memory: make(map[string][]ChatTurn),
	}
}

func (s *MemoryStore) Get(key string) []ChatTurn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if turns, ok := s.memory[key]; ok {
		// Retornar copia para evitar race conditions
		cpy := make([]ChatTurn, len(turns))
		copy(cpy, turns)
		return cpy
	}
	return nil
}

func (s *MemoryStore) Save(key string, turn ChatTurn, limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	turns := s.memory[key]
	turns = append(turns, turn)
	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	s.memory[key] = turns
}

func (s *MemoryStore) Clear(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memory, key)
}

func (s *MemoryStore) ClearPrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.memory {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.memory, k)
		}
	}
}

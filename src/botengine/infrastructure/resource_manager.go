package infrastructure

import (
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type resourceEntry struct {
	filePaths   []string
	lastUpdated time.Time
}

// ResourceManager gestiona el ciclo de vida de los archivos físicos asociados a sesiones
type ResourceManager struct {
	mu        sync.RWMutex
	resources map[string]*resourceEntry // Key: sessionKey (memoryKey)
}

func NewResourceManager() *ResourceManager {
	rm := &ResourceManager{
		resources: make(map[string]*resourceEntry),
	}
	// Start background cleanup
	go rm.startAutoPurge()
	return rm
}

// Track vincula un archivo físico a una sesión de chat
func (rm *ResourceManager) Track(sessionKey string, filePath string) {
	if filePath == "" {
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()

	entry, ok := rm.resources[sessionKey]
	if !ok {
		entry = &resourceEntry{}
		rm.resources[sessionKey] = entry
	}
	entry.filePaths = append(entry.filePaths, filePath)
	entry.lastUpdated = time.Now()
}

// PurgeSession elimina físicamente todos los archivos vinculados a una sesión
func (rm *ResourceManager) PurgeSession(sessionKey string) {
	rm.mu.Lock()
	entry, ok := rm.resources[sessionKey]
	if !ok {
		rm.mu.Unlock()
		return
	}
	files := entry.filePaths
	delete(rm.resources, sessionKey)
	rm.mu.Unlock()

	for _, path := range files {
		if err := os.Remove(path); err != nil {
			if !os.IsNotExist(err) {
				logrus.Warnf("[RESOURCE_MANAGER] Failed to delete temporary file %s: %v", path, err)
			}
		} else {
			logrus.Debugf("[RESOURCE_MANAGER] Purged temporary file: %s", path)
		}
	}
}

// GetSessionFiles devuelve la lista de archivos de una sesión
func (rm *ResourceManager) GetSessionFiles(sessionKey string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	if entry, ok := rm.resources[sessionKey]; ok {
		cpy := make([]string, len(entry.filePaths))
		copy(cpy, entry.filePaths)
		return cpy
	}
	return nil
}

func (rm *ResourceManager) startAutoPurge() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var sessionsToPurge []string

		rm.mu.RLock()
		for key, entry := range rm.resources {
			// Purge if inactive for more than 1 hour
			if time.Since(entry.lastUpdated) > 1*time.Hour {
				sessionsToPurge = append(sessionsToPurge, key)
			}
		}
		rm.mu.RUnlock()

		for _, key := range sessionsToPurge {
			logrus.Infof("[RESOURCE_MANAGER] Auto-purging inactive session resources: %s", key)
			rm.PurgeSession(key)
		}
	}
}

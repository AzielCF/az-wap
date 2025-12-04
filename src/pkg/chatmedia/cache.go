package chatmedia

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Item representa un medio extraído asociado a un mensaje específico.
// Se guarda solo en memoria por un tiempo corto (TTL) para que otras
// integraciones como Chatwoot puedan reutilizar la ruta local sin
// acoplarse a la capa de WhatsApp.
type Item struct {
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	MimeType  string    `json:"mime_type"`
	Caption   string    `json:"caption"`
	ExpiresAt time.Time `json:"expires_at"`
}

var (
	mu    sync.RWMutex
	store = make(map[string][]Item)

	// ttl define cuánto tiempo permanecen en cache los medios
	// asociados a un mensaje. Suficiente para procesar el evento
	// entrante y reenviarlo a integraciones.
	ttl = 1 * time.Minute
)

// Add agrega un Item al cache asociado a un messageID.
func Add(messageID string, item Item) {
	id := strings.TrimSpace(messageID)
	if id == "" || strings.TrimSpace(item.Path) == "" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	item.ExpiresAt = now.Add(ttl)

	// Filtrar items expirados previos para este mensaje y borrar sus ficheros.
	prev := store[id]
	filtered := prev[:0]
	for _, it := range prev {
		if now.Before(it.ExpiresAt) {
			filtered = append(filtered, it)
		} else if p := strings.TrimSpace(it.Path); p != "" {
			_ = os.Remove(p)
		}
	}

	filtered = append(filtered, item)
	store[id] = filtered
}

// Get devuelve los items válidos para un messageID. Si todos los
// items expiraron, limpia la entrada y devuelve nil.
func Get(messageID string) []Item {
	id := strings.TrimSpace(messageID)
	if id == "" {
		return nil
	}

	mu.RLock()
	items, ok := store[id]
	mu.RUnlock()
	if !ok {
		return nil
	}

	now := time.Now()
	result := make([]Item, 0, len(items))
	for _, it := range items {
		if now.Before(it.ExpiresAt) {
			result = append(result, it)
		} else if p := strings.TrimSpace(it.Path); p != "" {
			_ = os.Remove(p)
		}
	}

	if len(result) == 0 {
		mu.Lock()
		delete(store, id)
		mu.Unlock()
	}

	return result
}

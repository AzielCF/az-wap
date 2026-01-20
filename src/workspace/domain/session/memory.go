package session

import (
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
)

// SessionMemory añade capacidades de memoria de conversación a una sesión de workspace.
// Esta memoria está estrictamente ligada al ciclo de vida de la sesión (4 minutos de inactividad).
type SessionMemory struct {
	History   []botengineDomain.ChatTurn
	Resources map[string]ResourceInfo // Key: FriendlyName (o Hash si hay colisión?) -> Info
}

// AddTurn añade un nuevo mensaje al historial y mantiene el límite.
func (sm *SessionMemory) AddTurn(role, text string, limit int) {
	if sm.History == nil {
		sm.History = make([]botengineDomain.ChatTurn, 0)
	}

	sm.History = append(sm.History, botengineDomain.ChatTurn{
		Role: role,
		Text: text,
	})

	// Mantener límite de ventana de contexto (default 10)
	if limit <= 0 {
		limit = 10
	}
	if len(sm.History) > limit {
		sm.History = sm.History[len(sm.History)-limit:]
	}
}

// GetHistory devuelve el historial actual de la conversación.
func (sm *SessionMemory) GetHistory() []botengineDomain.ChatTurn {
	return sm.History
}

// ResourceInfo contiene metadatos sobre un archivo disponible en la sesión
type ResourceInfo struct {
	FriendlyName string `json:"friendly_name"` // Nombre sanitizado (ej., "factura-enero.pdf")
	FileHash     string `json:"file_hash"`     // Hash SHA256
	MimeType     string `json:"mime_type"`
	LocalPath    string `json:"local_path"`
}

// AddResource registra un archivo en la memoria de la sesión
func (sm *SessionMemory) AddResource(friendlyName, hash, mime, path string) {
	if sm.Resources == nil {
		sm.Resources = make(map[string]ResourceInfo)
	}
	sm.Resources[friendlyName] = ResourceInfo{
		FriendlyName: friendlyName,
		FileHash:     hash,
		MimeType:     mime,
		LocalPath:    path,
	}
}

// GetResources devuelve el mapa de recursos disponibles
func (sm *SessionMemory) GetResources() map[string]ResourceInfo {
	return sm.Resources
}

package session

import (
	"context"
	"time"

	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	"github.com/AzielCF/az-wap/workspace/domain/message"
)

// SessionState representa el estado actual de una sesión de chat
type SessionState string

const (
	StateDebouncing SessionState = "debouncing"
	StateProcessing SessionState = "processing"
	StateWaiting    SessionState = "waiting"
)

// SessionEntry contiene todos los datos SERIALIZABLES de una sesión de chat activa.
// Los Timers (no serializables) se manejan por separado en el SessionOrchestrator.
type SessionEntry struct {
	// Mensaje original que inició o actualizó la sesión
	Msg message.IncomingMessage `json:"msg"`

	// Buffer de textos acumulados durante el debouncing
	Texts      []string `json:"texts"`
	MessageIDs []string `json:"message_ids"`

	// Tiempos de control
	ExpireAt time.Time `json:"expire_at"`
	LastSeen time.Time `json:"last_seen"`

	// Estado de la sesión
	State SessionState `json:"state"`

	// Contadores y métricas
	LastBubbleCount int `json:"last_bubble_count"`
	FocusScore      int `json:"focus_score"`

	// Media pendiente de procesar
	Media []*message.IncomingMedia `json:"media,omitempty"`

	// Memoria de la conversación (historial IA)
	Memory SessionMemory `json:"memory"`

	// Configuración del bot asignado
	BotID string `json:"bot_id"`

	// Rutas de archivos de la sesión
	SessionPath     string   `json:"session_path,omitempty"`
	DownloadedFiles []string `json:"downloaded_files,omitempty"`

	// Estado de presencia
	ChatOpen      bool      `json:"chat_open"`
	LastReplyTime time.Time `json:"last_reply_time"`

	// Estado de la IA
	LastMindset  *botengineDomain.Mindset `json:"last_mindset,omitempty"`
	PendingTasks []string                 `json:"pending_tasks,omitempty"`

	// Configuración de la sesión (capturada al inicio/renovación)
	InactivityWarningEnabled bool `json:"inactivity_warning_enabled"`
	SessionClosingEnabled    bool `json:"session_closing_enabled"`

	// Idioma detectado/configurado
	Language string `json:"language,omitempty"`
}

func (e *SessionEntry) Clone() *SessionEntry {
	if e == nil {
		return nil
	}
	clone := *e

	// Deep copy slices
	if e.Texts != nil {
		clone.Texts = make([]string, len(e.Texts))
		copy(clone.Texts, e.Texts)
	}
	if e.MessageIDs != nil {
		clone.MessageIDs = make([]string, len(e.MessageIDs))
		copy(clone.MessageIDs, e.MessageIDs)
	}
	if e.DownloadedFiles != nil {
		clone.DownloadedFiles = make([]string, len(e.DownloadedFiles))
		copy(clone.DownloadedFiles, e.DownloadedFiles)
	}
	if e.PendingTasks != nil {
		clone.PendingTasks = make([]string, len(e.PendingTasks))
		copy(clone.PendingTasks, e.PendingTasks)
	}

	return &clone
}

// SessionStore define el contrato para almacenar y recuperar sesiones de chat.
// Esta interfaz permite intercambiar implementaciones (memoria, Valkey, etc.)
// sin modificar la lógica del SessionOrchestrator.
type SessionStore interface {
	// Save guarda o actualiza una sesión con un TTL dado.
	// Si la sesión ya existe, se sobrescribe.
	Save(ctx context.Context, key string, entry *SessionEntry, ttl time.Duration) error

	// Get recupera una sesión por su key.
	// Retorna (nil, nil) si la sesión no existe (no es error).
	Get(ctx context.Context, key string) (*SessionEntry, error)

	// Delete elimina una sesión.
	// No retorna error si la sesión no existía.
	Delete(ctx context.Context, key string) error

	// Extend renueva el TTL de una sesión existente sin modificar sus datos.
	// Retorna error si la sesión no existe.
	Extend(ctx context.Context, key string, ttl time.Duration) error

	// List devuelve todas las keys que coinciden con un patrón.
	// El patrón usa glob syntax (ej: "channel123|*" para todas las sesiones de un canal).
	// Retorna slice vacío si no hay coincidencias.
	List(ctx context.Context, pattern string) ([]string, error)

	// Exists verifica si una sesión existe.
	Exists(ctx context.Context, key string) (bool, error)

	// GetAll devuelve todas las sesiones activas.
	// Útil para obtener estadísticas o realizar operaciones en lote.
	GetAll(ctx context.Context) (map[string]*SessionEntry, error)

	// UpdateField actualiza un campo específico de una sesión sin reescribir todo.
	// Esto es útil para operaciones frecuentes como actualizar LastSeen.
	// Si la implementación no soporta updates parciales, debe hacer Get+Save.
	UpdateField(ctx context.Context, key string, field string, value any) error
}

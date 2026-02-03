package monitoring

import (
	"context"
	"time"
)

// ServerInfo represents the status of a node in the cluster
type ServerInfo struct {
	ID       string    `json:"id"`
	LastSeen time.Time `json:"last_seen"`
	Uptime   int64     `json:"uptime_seconds"`
	Version  string    `json:"version"`
}

// WorkerActivity represents what a specific worker is doing
type WorkerActivity struct {
	ServerID     string    `json:"server_id"`
	WorkerID     int       `json:"worker_id"`
	PoolType     string    `json:"pool_type"` // primary | webhook
	IsProcessing bool      `json:"is_processing"`
	ChatID       string    `json:"chat_id,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// GlobalStats groups atomic system metrics
type GlobalStats struct {
	TotalProcessed int64 `json:"total_processed"`
	TotalErrors    int64 `json:"total_errors"`
	TotalDropped   int64 `json:"total_dropped"`
	TotalPending   int64 `json:"total_pending"`

	// Tareas distribuidas
	PendingTasksMemory int64 `json:"pending_tasks_memory"` // Tareas cargadas en memoria (para hoy)
	PendingTasksDB     int64 `json:"pending_tasks_db"`     // Tareas en base de datos (largo plazo)

	// Estado de infraestructura
	ValkeyEnabled bool `json:"valkey_enabled"`
}

// MonitoringStore defines the contract for system heartbeat and metrics
type MonitoringStore interface {
	// Heartbeat: Update server status
	ReportHeartbeat(ctx context.Context, serverID string, uptime int64, version string) error

	// Servers: Get list of active servers
	GetActiveServers(ctx context.Context) ([]ServerInfo, error)
	RemoveServer(ctx context.Context, serverID string) error

	// Workers: Track what each worker is doing
	UpdateWorkerActivity(ctx context.Context, activity WorkerActivity) error
	GetClusterActivity(ctx context.Context) ([]WorkerActivity, error)

	// Atomic Counters: Increment global metrics
	IncrementStat(ctx context.Context, key string) error

	// Set value: Set a specific value (e.g. total pending)
	UpdateStat(ctx context.Context, key string, value int64) error

	// Get Stats: Get accumulated counters
	GetGlobalStats(ctx context.Context) (GlobalStats, error)
}

package monitoring

import (
	"context"
	"time"
)

// ServerInfo representa el estado de un nodo en el cluster
type ServerInfo struct {
	ID       string    `json:"id"`
	LastSeen time.Time `json:"last_seen"`
	Uptime   int64     `json:"uptime_seconds"`
	Version  string    `json:"version"`
}

// WorkerActivity representa lo que está haciendo un worker específico
type WorkerActivity struct {
	ServerID     string    `json:"server_id"`
	WorkerID     int       `json:"worker_id"`
	PoolType     string    `json:"pool_type"` // primary | webhook
	IsProcessing bool      `json:"is_processing"`
	ChatID       string    `json:"chat_id,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
}

// GlobalStats agrupa las métricas atómicas del sistema
type GlobalStats struct {
	TotalProcessed int64 `json:"total_processed"`
	TotalErrors    int64 `json:"total_errors"`
	TotalDropped   int64 `json:"total_dropped"`
}

// MonitoringStore define el contrato para el latido y métricas del sistema
type MonitoringStore interface {
	// Heartbeat: Avisar que este servidor sigue vivo
	ReportHeartbeat(ctx context.Context, serverID string, uptime int64) error

	// Server List: Obtener todos los servidores que han reportado pulso recientemente
	GetActiveServers(ctx context.Context) ([]ServerInfo, error)

	// Worker Activity: Reportar qué está haciendo un worker
	UpdateWorkerActivity(ctx context.Context, activity WorkerActivity) error

	// Worker List: Obtener la foto actual de todos los workers del cluster
	GetClusterActivity(ctx context.Context) ([]WorkerActivity, error)

	// Atomic Counters: Incrementar métricas globales
	IncrementStat(ctx context.Context, key string) error

	// Get Stats: Obtener los contadores acumulados
	GetGlobalStats(ctx context.Context) (GlobalStats, error)
}

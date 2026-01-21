package repository

import (
	"context"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
)

type MemoryMonitoringStore struct {
	mu sync.RWMutex

	servers map[string]monitoring.ServerInfo
	workers map[string]monitoring.WorkerActivity // key: "serverID:workerID"
	stats   monitoring.GlobalStats
}

func NewMemoryMonitoringStore() *MemoryMonitoringStore {
	return &MemoryMonitoringStore{
		servers: make(map[string]monitoring.ServerInfo),
		workers: make(map[string]monitoring.WorkerActivity),
	}
}

func (s *MemoryMonitoringStore) ReportHeartbeat(ctx context.Context, serverID string, uptime int64, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.servers[serverID] = monitoring.ServerInfo{
		ID:       serverID,
		LastSeen: time.Now(),
		Uptime:   uptime,
		Version:  version,
	}
	return nil
}

func (s *MemoryMonitoringStore) GetActiveServers(ctx context.Context) ([]monitoring.ServerInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var active []monitoring.ServerInfo
	now := time.Now()
	for _, srv := range s.servers {
		// En memoria, consideramos inactivo si no ha reportado en 1 minuto
		if now.Sub(srv.LastSeen) < 1*time.Minute {
			active = append(active, srv)
		}
	}
	return active, nil
}

func (s *MemoryMonitoringStore) UpdateWorkerActivity(ctx context.Context, activity monitoring.WorkerActivity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := activity.ServerID + ":" + activity.PoolType + ":" + string(rune(activity.WorkerID))
	s.workers[key] = activity
	return nil
}

func (s *MemoryMonitoringStore) GetClusterActivity(ctx context.Context) ([]monitoring.WorkerActivity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []monitoring.WorkerActivity
	for _, act := range s.workers {
		// Solo mostramos workers de servidores que siguen vivos
		if srv, ok := s.servers[act.ServerID]; ok && time.Since(srv.LastSeen) < 1*time.Minute {
			result = append(result, act)
		}
	}
	return result, nil
}

func (s *MemoryMonitoringStore) IncrementStat(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch key {
	case "processed":
		s.stats.TotalProcessed++
	case "error":
		s.stats.TotalErrors++
	case "dropped":
		s.stats.TotalDropped++
	}
	return nil
}

func (s *MemoryMonitoringStore) GetGlobalStats(ctx context.Context) (monitoring.GlobalStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats, nil
}

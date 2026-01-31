package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
)

// ValkeyMonitoringStore implements monitoring.MonitoringStore using Valkey.
// It provides cross-server visibility for cluster health and metrics.
type ValkeyMonitoringStore struct {
	client *valkey.Client
	prefix string
}

// NewValkeyMonitoringStore creates a new ValkeyMonitoringStore instance.
func NewValkeyMonitoringStore(client *valkey.Client) *ValkeyMonitoringStore {
	return &ValkeyMonitoringStore{
		client: client,
		prefix: client.Key("monitoring") + ":",
	}
}

func (s *ValkeyMonitoringStore) serversKey() string {
	return s.prefix + "servers"
}

func (s *ValkeyMonitoringStore) workersKey() string {
	return s.prefix + "workers"
}

func (s *ValkeyMonitoringStore) statsKey() string {
	return s.prefix + "stats"
}

// ReportHeartbeat updates the status of the current node.
func (s *ValkeyMonitoringStore) ReportHeartbeat(ctx context.Context, serverID string, uptime int64, version string) error {
	info := monitoring.ServerInfo{
		ID:       serverID,
		LastSeen: time.Now(),
		Uptime:   uptime,
		Version:  version,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// Use HSET to store server info
	cmd := s.client.Inner().B().Hset().
		Key(s.serversKey()).
		FieldValue().
		FieldValue(serverID, string(data)).
		Build()

	return s.client.Inner().Do(ctx, cmd).Error()
}

// GetActiveServers returns a list of servers that reported a heartbeat Recently.
func (s *ValkeyMonitoringStore) GetActiveServers(ctx context.Context) ([]monitoring.ServerInfo, error) {
	cmd := s.client.Inner().B().Hgetall().Key(s.serversKey()).Build()
	entries, err := s.client.Inner().Do(ctx, cmd).AsStrMap()
	if err != nil {
		return nil, err
	}

	var active []monitoring.ServerInfo
	now := time.Now()

	for _, val := range entries {
		var info monitoring.ServerInfo
		if err := json.Unmarshal([]byte(val), &info); err == nil {
			// Filter inactive servers (no heartbeat in last 2 minutes)
			if now.Sub(info.LastSeen) < 2*time.Minute {
				active = append(active, info)
			}
		}
	}

	return active, nil
}

// RemoveServer deletes a server and its activity from the monitoring store.
func (s *ValkeyMonitoringStore) RemoveServer(ctx context.Context, serverID string) error {
	// Remove from servers Hash
	cmd := s.client.Inner().B().Hdel().Key(s.serversKey()).Field(serverID).Build()
	if err := s.client.Inner().Do(ctx, cmd).Error(); err != nil {
		return err
	}

	// Optional: Could also scan and remove workers here, but they will expire anyway
	return nil
}

// UpdateWorkerActivity tracks what a specific worker thread is doing.
func (s *ValkeyMonitoringStore) UpdateWorkerActivity(ctx context.Context, activity monitoring.WorkerActivity) error {
	key := fmt.Sprintf("%s:%s:%d", activity.ServerID, activity.PoolType, activity.WorkerID)
	activity.UpdatedAt = time.Now()

	data, err := json.Marshal(activity)
	if err != nil {
		return err
	}

	cmd := s.client.Inner().B().Hset().
		Key(s.workersKey()).
		FieldValue().
		FieldValue(key, string(data)).
		Build()

	return s.client.Inner().Do(ctx, cmd).Error()
}

// GetClusterActivity returns activity for all workers in all active nodes.
func (s *ValkeyMonitoringStore) GetClusterActivity(ctx context.Context) ([]monitoring.WorkerActivity, error) {
	// First get active servers to filter stale data
	activeServers, err := s.GetActiveServers(ctx)
	if err != nil {
		return nil, err
	}

	aliveIDs := make(map[string]bool)
	for _, srv := range activeServers {
		aliveIDs[srv.ID] = true
	}

	cmd := s.client.Inner().B().Hgetall().Key(s.workersKey()).Build()
	entries, err := s.client.Inner().Do(ctx, cmd).AsStrMap()
	if err != nil {
		return nil, err
	}

	var result []monitoring.WorkerActivity
	now := time.Now()

	for _, val := range entries {
		var act monitoring.WorkerActivity
		if err := json.Unmarshal([]byte(val), &act); err == nil {
			// Only show if server is alive
			if !aliveIDs[act.ServerID] {
				continue
			}

			// Don't show idle workers that haven't updated in 5 minutes
			if !act.IsProcessing && now.Sub(act.UpdatedAt) > 5*time.Minute {
				continue
			}

			result = append(result, act)
		}
	}

	return result, nil
}

// IncrementStat atomically increments a global counter (TotalProcessed, etc).
func (s *ValkeyMonitoringStore) IncrementStat(ctx context.Context, key string) error {
	// Map to internal field names if necessary, for now we use the keys directly
	cmd := s.client.Inner().B().Hincrby().
		Key(s.statsKey()).
		Field(key).
		Increment(1).
		Build()

	return s.client.Inner().Do(ctx, cmd).Error()
}

// UpdateStat sets a specific value for a global metric.
func (s *ValkeyMonitoringStore) UpdateStat(ctx context.Context, key string, value int64) error {
	cmd := s.client.Inner().B().Hset().
		Key(s.statsKey()).
		FieldValue().
		FieldValue(key, fmt.Sprintf("%d", value)).
		Build()

	return s.client.Inner().Do(ctx, cmd).Error()
}

// GetGlobalStats retrieves consolidated cluster-wide metrics.
func (s *ValkeyMonitoringStore) GetGlobalStats(ctx context.Context) (monitoring.GlobalStats, error) {
	cmd := s.client.Inner().B().Hgetall().Key(s.statsKey()).Build()
	res, err := s.client.Inner().Do(ctx, cmd).AsIntMap()
	if err != nil {
		return monitoring.GlobalStats{}, err
	}

	return monitoring.GlobalStats{
		TotalProcessed: res["processed"],
		TotalErrors:    res["error"],
		TotalDropped:   res["dropped"],
		TotalPending:   res["pending"],
	}, nil
}

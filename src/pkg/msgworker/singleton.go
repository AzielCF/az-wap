package msgworker

import (
	"context"
	"sync"

	"github.com/AzielCF/az-wap/config"
	"github.com/sirupsen/logrus"
)

var (
	globalPool     *MessageWorkerPool
	globalPoolOnce sync.Once
	globalPoolCtx  context.Context
	globalCancel   context.CancelFunc
)

// GetGlobalPool returns the singleton message worker pool
func GetGlobalPool() *MessageWorkerPool {
	globalPoolOnce.Do(func() {
		globalPoolCtx, globalCancel = context.WithCancel(context.Background())

		size := config.MessageWorkerPoolSize
		if size <= 0 {
			size = 6
		}

		queue := config.MessageWorkerQueueSize
		if queue <= 0 {
			queue = 250
		}

		globalPool = NewMessageWorkerPool(size, queue)
		globalPool.Start(globalPoolCtx)
		logrus.Infof("[MSG_WORKER_POOL] Global instance started with %d workers and queue size %d", size, queue)
	})
	return globalPool
}

// StopGlobalPool stops the singleton pool
func StopGlobalPool() {
	if globalCancel != nil {
		globalCancel()
	}
	if globalPool != nil {
		globalPool.Stop()
	}
}

// GetGlobalStats returns stats from the global pool
func GetGlobalStats() PoolStats {
	return GetGlobalPool().GetStats()
}

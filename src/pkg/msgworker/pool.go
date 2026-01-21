package msgworker

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// MessageJob representa un job de procesamiento de mensaje WhatsApp
type MessageJob struct {
	InstanceID string
	ChatJID    string
	Handler    func(ctx context.Context) error
}

// PoolStats contiene métricas en tiempo real del worker pool
type PoolStats struct {
	NumWorkers      int            `json:"num_workers"`
	QueueSize       int            `json:"queue_size"`
	ActiveWorkers   int            `json:"active_workers"`
	TotalDispatched int64          `json:"total_dispatched"`
	TotalProcessed  int64          `json:"total_processed"`
	TotalDropped    int64          `json:"total_dropped"`
	TotalErrors     int64          `json:"total_errors"`
	WorkerStats     []WorkerStats  `json:"worker_stats"`
	ActiveChats     map[string]int `json:"active_chats"` // instanceID|chatJID -> worker_id
}

// WorkerStats contiene métricas por worker individual
type WorkerStats struct {
	WorkerID      int   `json:"worker_id"`
	QueueDepth    int   `json:"queue_depth"`
	IsProcessing  bool  `json:"is_processing"`
	JobsProcessed int64 `json:"jobs_processed"`
}

type activeChatEntry struct {
	workerID  int
	updatedAt time.Time
}

// MessageWorkerPool maneja un pool de workers para procesar mensajes de WhatsApp
type MessageWorkerPool struct {
	numWorkers int
	queueSize  int
	workers    []*worker
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopped    int32
	stopCh     chan struct{}

	// Métricas
	totalDispatched int64
	totalProcessed  int64
	totalDropped    int64
	totalErrors     int64
	activeChatsMu   sync.RWMutex
	activeChats     map[string]activeChatEntry // chatKey -> workerID
	startTime       time.Time

	// Hooks para monitoreo externo
	OnWorkerStart func(workerID int, chatKey string)
	OnWorkerEnd   func(workerID int, chatKey string)
}

// worker representa un worker individual con su cola
type worker struct {
	id            int
	jobQueue      chan MessageJob
	ctx           context.Context
	cancel        context.CancelFunc
	isProcessing  int32              // atomic: 1 if processing, 0 if idle
	jobsProcessed int64              // atomic counter
	pool          *MessageWorkerPool // referencia al pool para actualizar métricas globales
}

// NewMessageWorkerPool crea un nuevo pool de workers para mensajes
func NewMessageWorkerPool(numWorkers, queueSize int) *MessageWorkerPool {
	if numWorkers <= 0 {
		numWorkers = 10
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	pool := &MessageWorkerPool{
		numWorkers:  numWorkers,
		queueSize:   queueSize,
		workers:     make([]*worker, numWorkers),
		activeChats: make(map[string]activeChatEntry),
		stopCh:      make(chan struct{}),
		startTime:   time.Now(),
	}

	return pool
}

// Start inicia todos los workers del pool
func (p *MessageWorkerPool) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case <-ticker.C:
				now := time.Now()
				p.activeChatsMu.Lock()
				for k, v := range p.activeChats {
					if !v.updatedAt.IsZero() && now.Sub(v.updatedAt) > 2*time.Second {
						delete(p.activeChats, k)
					}
				}
				p.activeChatsMu.Unlock()
			}
		}
	}()

	for i := 0; i < p.numWorkers; i++ {
		workerCtx, cancel := context.WithCancel(ctx)
		w := &worker{
			id:       i,
			jobQueue: make(chan MessageJob, p.queueSize),
			ctx:      workerCtx,
			cancel:   cancel,
			pool:     p, // pasar referencia al pool
		}
		p.workers[i] = w

		p.wg.Add(1)
		go w.run(&p.wg)
	}

	logrus.Infof("[MSG_WORKER_POOL] Started with %d workers, queue size: %d", p.numWorkers, p.queueSize)
}

// TryDispatch envía un job al worker apropiado (no bloqueante) y retorna
// si el job pudo encolarse. Útil para aplicar backpressure en endpoints HTTP.
func (p *MessageWorkerPool) TryDispatch(job MessageJob) bool {
	if atomic.LoadInt32(&p.stopped) == 1 {
		atomic.AddInt64(&p.totalDropped, 1)
		return false
	}

	shard := p.shardForChat(job.InstanceID, job.ChatJID)
	atomic.AddInt64(&p.totalDispatched, 1)

	// Track active chat
	chatKey := job.InstanceID + "|" + job.ChatJID
	p.activeChatsMu.Lock()
	p.activeChats[chatKey] = activeChatEntry{workerID: shard, updatedAt: time.Now()}
	p.activeChatsMu.Unlock()

	sent := func() (ok bool) {
		defer func() {
			if r := recover(); r != nil {
				ok = false
			}
		}()
		select {
		case p.workers[shard].jobQueue <- job:
			return true
		default:
			return false
		}
	}()

	if sent {
		return true
	}
	p.activeChatsMu.Lock()
	delete(p.activeChats, chatKey)
	p.activeChatsMu.Unlock()

	atomic.AddInt64(&p.totalDropped, 1)
	logrus.Warnf("[MSG_WORKER_POOL] Worker %d queue full (or stopped), dropping job for %s|%s",
		shard, job.InstanceID, job.ChatJID)
	return false
}

// Dispatch envía un job al worker apropiado (no bloqueante)
func (p *MessageWorkerPool) Dispatch(job MessageJob) {
	_ = p.TryDispatch(job)
}

// Stop detiene el pool de forma graceful
func (p *MessageWorkerPool) Stop() {
	p.stopOnce.Do(func() {
		atomic.StoreInt32(&p.stopped, 1)
		close(p.stopCh)
		logrus.Info("[MSG_WORKER_POOL] Stopping workers...")

		// Cancelar contextos y cerrar colas
		for _, w := range p.workers {
			w.cancel()
			close(w.jobQueue)
		}

		// Esperar a que terminen los workers
		p.wg.Wait()

		logrus.Info("[MSG_WORKER_POOL] All workers stopped")
	})
}

// shardForChat calcula el shard (worker) para un chat específico usando hash consistente
func (p *MessageWorkerPool) shardForChat(instanceID, chatJID string) int {
	key := instanceID + "|" + chatJID
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % uint32(p.numWorkers))
}

// GetStats retorna estadísticas en tiempo real del pool
func (p *MessageWorkerPool) GetStats() PoolStats {
	workerStats := make([]WorkerStats, len(p.workers))
	activeWorkers := 0

	for i, w := range p.workers {
		isProcessing := atomic.LoadInt32(&w.isProcessing) == 1
		if isProcessing {
			activeWorkers++
		}

		workerStats[i] = WorkerStats{
			WorkerID:      w.id,
			QueueDepth:    len(w.jobQueue),
			IsProcessing:  isProcessing,
			JobsProcessed: atomic.LoadInt64(&w.jobsProcessed),
		}
	}

	now := time.Now()
	p.activeChatsMu.Lock()
	activeChatsSnapshot := make(map[string]int, len(p.activeChats))
	for k, v := range p.activeChats {
		if !v.updatedAt.IsZero() && now.Sub(v.updatedAt) > 2*time.Second {
			delete(p.activeChats, k)
			continue
		}
		activeChatsSnapshot[k] = v.workerID
	}
	p.activeChatsMu.Unlock()

	return PoolStats{
		NumWorkers:      p.numWorkers,
		QueueSize:       p.queueSize,
		ActiveWorkers:   activeWorkers,
		TotalDispatched: atomic.LoadInt64(&p.totalDispatched),
		TotalProcessed:  atomic.LoadInt64(&p.totalProcessed),
		TotalDropped:    atomic.LoadInt64(&p.totalDropped),
		TotalErrors:     atomic.LoadInt64(&p.totalErrors),
		WorkerStats:     workerStats,
		ActiveChats:     activeChatsSnapshot,
	}
}

// run ejecuta el loop principal del worker
func (w *worker) run(wg *sync.WaitGroup) {
	defer wg.Done()

	logrus.Debugf("[MSG_WORKER_POOL] Worker %d started", w.id)

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				// Canal cerrado, terminar
				logrus.Debugf("[MSG_WORKER_POOL] Worker %d shutting down", w.id)
				return
			}

			// Procesar job con defer para garantizar limpieza
			func() {
				chatKey := job.InstanceID + "|" + job.ChatJID

				if w.pool.OnWorkerStart != nil {
					w.pool.OnWorkerStart(w.id, chatKey)
				}
				atomic.StoreInt32(&w.isProcessing, 1)
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&w.pool.totalErrors, 1)
						logrus.Errorf("[MSG_WORKER_POOL] Worker %d panic for %s: %v", w.id, chatKey, r)
					}
					if w.pool.OnWorkerEnd != nil {
						w.pool.OnWorkerEnd(w.id, chatKey)
					}
					atomic.StoreInt32(&w.isProcessing, 0)
					atomic.AddInt64(&w.jobsProcessed, 1)
					atomic.AddInt64(&w.pool.totalProcessed, 1)
				}()

				err := job.Handler(w.ctx)

				if err != nil {
					atomic.AddInt64(&w.pool.totalErrors, 1)
					logrus.WithError(err).Errorf("[MSG_WORKER_POOL] Worker %d job failed for %s|%s",
						w.id, job.InstanceID, job.ChatJID)
				}
			}()

		case <-w.ctx.Done():
			// Contexto cancelado, procesar jobs restantes antes de terminar
			logrus.Debugf("[MSG_WORKER_POOL] Worker %d context cancelled, draining queue...", w.id)
			w.drainQueue()
			return
		}
	}
}

// drainQueue procesa jobs pendientes antes del shutdown
func (w *worker) drainQueue() {
	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				return
			}
			// Procesar job restante
			func() {
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&w.pool.totalErrors, 1)
						logrus.Errorf("[MSG_WORKER_POOL] Worker %d drain panic: %v", w.id, r)
					}
				}()
				if err := job.Handler(w.ctx); err != nil {
					logrus.WithError(err).Errorf("[MSG_WORKER_POOL] Worker %d drain job failed", w.id)
				}
			}()
		default:
			// No hay más jobs
			return
		}
	}
}

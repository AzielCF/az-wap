package msgworker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: Pool debe despachar jobs sin bloquear el caller
func TestPool_DispatchNonBlocking(t *testing.T) {
	pool := NewMessageWorkerPool(2, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	defer pool.Stop()

	start := time.Now()
	// Despachar debe retornar inmediatamente aunque el job tarde
	pool.Dispatch(MessageJob{
		InstanceID: "test",
		ChatJID:    "123",
		Handler: func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	})
	elapsed := time.Since(start)

	// Debe retornar en menos de 10ms (no bloqueante)
	assert.Less(t, elapsed, 10*time.Millisecond, "Dispatch debe ser no bloqueante")
}

// Test 2: Jobs del mismo chat deben procesarse secuencialmente (orden garantizado)
func TestPool_SameChatSequentialProcessing(t *testing.T) {
	pool := NewMessageWorkerPool(4, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	defer pool.Stop()

	var results []int
	var mu sync.Mutex

	instanceID := "inst1"
	chatJID := "chat1"

	// Enviamos 5 jobs del mismo chat
	for i := 1; i <= 5; i++ {
		val := i
		pool.Dispatch(MessageJob{
			InstanceID: instanceID,
			ChatJID:    chatJID,
			Handler: func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond) // Simula procesamiento
				mu.Lock()
				results = append(results, val)
				mu.Unlock()
				return nil
			},
		})
	}

	// Esperar a que todos los jobs se procesen
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Deben procesarse en orden: 1, 2, 3, 4, 5
	require.Equal(t, []int{1, 2, 3, 4, 5}, results, "Jobs del mismo chat deben procesarse en orden")
}

// Test 3: Jobs de distintos chats pueden procesarse en paralelo (fairness)
func TestPool_DifferentChatsParallelProcessing(t *testing.T) {
	pool := NewMessageWorkerPool(4, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	defer pool.Stop()

	var activeCount int32

	// Enviamos jobs de 4 chats diferentes
	for i := 0; i < 4; i++ {
		chatID := string(rune('A' + i))
		pool.Dispatch(MessageJob{
			InstanceID: "inst1",
			ChatJID:    chatID,
			Handler: func(ctx context.Context) error {
				atomic.AddInt32(&activeCount, 1)
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&activeCount, -1)
				return nil
			},
		})
	}

	// Esperar un poco para que arranquen los workers
	time.Sleep(10 * time.Millisecond)

	// Debería haber al menos 2 jobs activos simultáneamente (paralelismo)
	active := atomic.LoadInt32(&activeCount)
	assert.GreaterOrEqual(t, active, int32(2), "Distintos chats deben procesarse en paralelo")
}

// Test 4: Respetar límite de concurrencia (max workers)
func TestPool_RespectsMaxWorkers(t *testing.T) {
	maxWorkers := 3
	pool := NewMessageWorkerPool(maxWorkers, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	defer pool.Stop()

	var activeCount int32
	var maxActive int32

	// Enviamos 10 jobs de distintos chats
	for i := 0; i < 10; i++ {
		chatID := string(rune('A' + i))
		pool.Dispatch(MessageJob{
			InstanceID: "inst1",
			ChatJID:    chatID,
			Handler: func(ctx context.Context) error {
				current := atomic.AddInt32(&activeCount, 1)
				// Actualizar el máximo alcanzado
				for {
					max := atomic.LoadInt32(&maxActive)
					if current <= max || atomic.CompareAndSwapInt32(&maxActive, max, current) {
						break
					}
				}
				time.Sleep(30 * time.Millisecond)
				atomic.AddInt32(&activeCount, -1)
				return nil
			},
		})
	}

	// Esperar a que terminen todos
	time.Sleep(200 * time.Millisecond)

	max := atomic.LoadInt32(&maxActive)
	assert.LessOrEqual(t, max, int32(maxWorkers), "No debe exceder el límite de workers")
}

// Test 5: Graceful shutdown debe completar jobs en curso
func TestPool_GracefulShutdown(t *testing.T) {
	pool := NewMessageWorkerPool(2, 10)
	ctx, cancel := context.WithCancel(context.Background())

	pool.Start(ctx)

	var completed int32

	// Enviamos 2 jobs que tardan
	for i := 0; i < 2; i++ {
		pool.Dispatch(MessageJob{
			InstanceID: "inst1",
			ChatJID:    string(rune('A' + i)),
			Handler: func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				atomic.AddInt32(&completed, 1)
				return nil
			},
		})
	}

	time.Sleep(10 * time.Millisecond) // Dejar que arranquen

	// Cancelar contexto (graceful shutdown)
	cancel()
	pool.Stop()

	// Los jobs en curso deben completarse
	completedCount := atomic.LoadInt32(&completed)
	assert.Equal(t, int32(2), completedCount, "Jobs en curso deben completarse en shutdown")
}

// Test 6: Hash consistente - mismo chat siempre al mismo worker
func TestPool_ConsistentHashing(t *testing.T) {
	pool := NewMessageWorkerPool(4, 100)

	instanceID := "inst1"
	chatJID := "chat123"

	// Llamar varias veces con el mismo chat
	shard1 := pool.shardForChat(instanceID, chatJID)
	shard2 := pool.shardForChat(instanceID, chatJID)
	shard3 := pool.shardForChat(instanceID, chatJID)

	assert.Equal(t, shard1, shard2, "Mismo chat debe ir al mismo shard")
	assert.Equal(t, shard2, shard3, "Mismo chat debe ir al mismo shard")

	// Verificar que está en rango válido
	assert.GreaterOrEqual(t, shard1, 0)
	assert.Less(t, shard1, 4)
}

// Test 7: Distribución uniforme de chats entre workers
func TestPool_FairDistribution(t *testing.T) {
	numWorkers := 4
	pool := NewMessageWorkerPool(numWorkers, 100)

	shardCounts := make(map[int]int)

	// Simular 100 chats diferentes
	for i := 0; i < 100; i++ {
		instanceID := "inst1"
		chatJID := string(rune(i))
		shard := pool.shardForChat(instanceID, chatJID)
		shardCounts[shard]++
	}

	// Cada worker debería recibir ~25 chats (con margen de error)
	for shard, count := range shardCounts {
		assert.Greater(t, count, 15, "Worker %d debería recibir >15 chats", shard)
		assert.Less(t, count, 35, "Worker %d debería recibir <35 chats", shard)
	}
}

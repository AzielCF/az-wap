# Bot Worker Pool Architecture

## Overview

The Bot Worker Pool is a concurrent processing system that manages AI bot message handling with **guaranteed sequential processing per conversation** while maintaining **parallelism across different chats**.

## Problem Solved

**Before Worker Pool:**
```
WhatsApp Message → go gemini.HandleIncomingMessage() [Unlimited goroutines]
```

**Issues:**
- ❌ No concurrency limit (memory/CPU can explode)
- ❌ No guaranteed order per conversation (race conditions)
- ❌ Difficult to monitor/debug
- ❌ No backpressure mechanism

**After Worker Pool:**
```
WhatsApp Message → Dispatcher → Sharding (hash) → Worker Queue → Sequential Processing
```

**Benefits:**
- ✅ Controlled concurrency (configurable max workers)
- ✅ Sequential processing per conversation (same chat → same worker)
- ✅ Parallel processing across chats (different chats → different workers)
- ✅ Backpressure with buffered queues
- ✅ Observable metrics and logging

---

## Architecture Design

### Sharding Strategy

The pool uses **consistent hashing** on `instanceID|chatJID`:

```go
shard = hash(instanceID + "|" + chatJID) % numWorkers
```

**Why this matters:**
- Same conversation always routes to the same worker
- Messages from the same user are processed **in order**
- Different conversations are distributed across workers (fairness)

### Example with 4 Workers

```
Chat A (user 123) → hash → Worker 0 [sequential]
Chat B (user 456) → hash → Worker 1 [sequential]  
Chat C (user 789) → hash → Worker 0 [sequential]
Chat D (user 999) → hash → Worker 2 [sequential]
```

- Chat A and Chat C share Worker 0 but process sequentially
- All 4 workers can run in parallel
- Maximum 4 LLM calls happening simultaneously (if configured with 4 workers)

---

## Configuration

### Environment Variables

```bash
# Number of concurrent workers (default: 20)
BOT_WORKER_POOL_SIZE=20

# Queue size per worker (default: 1000)
BOT_WORKER_QUEUE_SIZE=1000
```

### CLI Flags

```bash
# Start with custom worker configuration
./whatsapp rest --bot-workers=30 --bot-queue-size=500

# For high-traffic scenarios (50-100 instances)
./whatsapp rest --bot-workers=50 --bot-queue-size=2000

# For low-resource environments
./whatsapp rest --bot-workers=5 --bot-queue-size=200
```

### Docker Compose

```yaml
services:
  whatsapp:
    image: ghcr.io/azielcf/az-wap
    environment:
      - BOT_WORKER_POOL_SIZE=30
      - BOT_WORKER_QUEUE_SIZE=1500
    command:
      - rest
      - --bot-workers=30
      - --bot-queue-size=1500
```

---

## Sizing Guide

### For 50-100 Instances

**Scenario:** Small business with multiple clients  
**Configuration:**
```bash
BOT_WORKER_POOL_SIZE=20
BOT_WORKER_QUEUE_SIZE=1000
```

**Capacity:**
- 20 concurrent LLM calls
- 20,000 messages buffered (20 workers × 1000 queue size)
- Handles ~100 instances with ~5 active chats each

### For 100-300 Instances

**Scenario:** Growing company with enterprise clients  
**Configuration:**
```bash
BOT_WORKER_POOL_SIZE=40
BOT_WORKER_QUEUE_SIZE=1500
```

**Capacity:**
- 40 concurrent LLM calls
- 60,000 messages buffered
- Handles ~300 instances with moderate traffic

### For 300-500 Instances

**Scenario:** Large deployment  
**Configuration:**
```bash
BOT_WORKER_POOL_SIZE=60
BOT_WORKER_QUEUE_SIZE=2000
```

**Capacity:**
- 60 concurrent LLM calls
- 120,000 messages buffered
- Requires ~8-16GB RAM
- Consider horizontal scaling at this point

---

## Horizontal Scaling (Future)

The architecture is designed to support horizontal scaling with minimal changes:

### Current (Single Server)
```
Server: [20 Workers] → In-memory queues → Gemini API
```

### Future (Multi-Server with Redis)
```
Server 1: [Workers 0-9]  ↘
                          → Redis Streams (shared state) → Gemini API
Server 2: [Workers 10-19] ↗
```

**Migration path:**
1. Replace in-memory channels with Redis Streams
2. Keep the same hash function: `hash(instanceID|chatJID) % totalWorkers`
3. Each server handles a range of worker shards
4. No code changes in bot logic

---

## Monitoring & Observability

### Logs to Watch

```log
[BOT_WORKER_POOL] Started with 20 workers, queue size: 1000
[BOT_WORKER_POOL] Worker 5 queue full, dropping job for inst1|123
[BOT_WORKER_POOL] Stopping workers...
[BOT_WORKER_POOL] All workers stopped
```

### Key Metrics (Future Enhancement)

- Jobs enqueued per second
- Average job processing time
- Queue depth per worker
- Jobs dropped (queue full)

---

## Failure Handling

### Queue Full Scenario

When a worker's queue is full:
```
1. Dispatcher tries non-blocking send
2. If full → logs warning + drops message
3. Bot doesn't block WhatsApp handler
4. User sees no response (graceful degradation)
```

**Mitigation:**
- Increase `BOT_WORKER_QUEUE_SIZE`
- Increase `BOT_WORKER_POOL_SIZE`
- Monitor logs for queue full warnings

### Graceful Shutdown

```go
1. Context cancelled (SIGTERM/SIGINT)
2. Workers stop accepting new jobs
3. Workers drain their queues
4. All in-flight jobs complete
5. Process exits cleanly
```

---

## Testing Strategy

### Unit Tests (Included)

Location: `src/pkg/botworker/pool_test.go`

**Tests:**
1. ✅ Dispatch is non-blocking
2. ✅ Same chat processes sequentially
3. ✅ Different chats process in parallel
4. ✅ Respects max workers limit
5. ✅ Graceful shutdown completes jobs
6. ✅ Consistent hashing per chat
7. ✅ Fair distribution across workers

Run tests:
```bash
cd src
go test -v ./pkg/botworker
```

### Integration Testing (Manual)

1. Send 2 messages from same user rapidly
2. Check logs: same worker processes both
3. Responses arrive in order

---

## Performance Comparison

### Before (Unlimited Goroutines)

```
Load: 1000 messages/minute
Result: ~1000 goroutines spawned
Memory: Unbounded growth
CPU: Spikes unpredictably
```

### After (Worker Pool)

```
Load: 1000 messages/minute
Result: Max 20 goroutines (20 workers)
Memory: Stable (buffered queues)
CPU: Smooth, predictable load
```

---

## Common Issues & Solutions

### "Worker queue full" warnings

**Cause:** More messages arriving than workers can process  
**Solution:**
```bash
# Increase workers
--bot-workers=30

# Or increase queue size
--bot-queue-size=2000
```

### Bot responses out of order

**Cause:** Bug in sharding (should not happen)  
**Check:** Verify `instanceID|chatJID` is used correctly  
**Test:** Run `TestPool_SameChatSequentialProcessing`

### High memory usage

**Cause:** Too many buffered messages  
**Solution:**
```bash
# Reduce queue size (trade latency for memory)
--bot-queue-size=500

# Or scale horizontally (future)
```

---

## Future Enhancements

### Phase 1 ✅ (Current)
- Worker pool with sharding
- Sequential per-chat processing
- Configurable workers/queue size

### Phase 2 (Planned)
- Metrics endpoint (`/metrics`)
- Prometheus integration
- Grafana dashboard

### Phase 3 (Planned)
- Redis Streams for queues
- Multi-server horizontal scaling
- Dynamic worker scaling

### Phase 4 (Planned)
- MCP Tools per bot
- Tool execution in worker context
- Shared tool connection pools

---

## Summary

The Bot Worker Pool provides:
- **Scalability:** 20-60 workers handle 50-500 instances
- **Reliability:** Sequential processing per conversation
- **Performance:** Controlled concurrency, no resource exhaustion
- **Observability:** Clear logs, future metrics support
- **Future-proof:** Ready for Redis + horizontal scaling

**Key Insight:** By hashing `instanceID|chatJID`, we guarantee order per conversation while maintaining parallelism across users.

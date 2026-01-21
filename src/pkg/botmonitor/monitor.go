package botmonitor

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// OnIncrement es un hook opcional para reportar métricas a sistemas externos (ej: cluster monitor)
var OnIncrement func(key string)

type Event struct {
	Timestamp  time.Time         `json:"timestamp"`
	TraceID    string            `json:"trace_id"`
	InstanceID string            `json:"instance_id"`
	ChatJID    string            `json:"chat_jid"`
	Provider   string            `json:"provider"`
	Stage      string            `json:"stage"`       // inbound | ai_request | ai_response | outbound
	Kind       string            `json:"kind"`        // text | image | audio | webhook
	Status     string            `json:"status"`      // ok | error | skipped
	Error      string            `json:"error"`       // optional
	Metadata   map[string]string `json:"metadata"`    // optional technical details (json strings, etc)
	DurationMs int64             `json:"duration_ms"` // optional
}

type Stats struct {
	TotalInbound    int64   `json:"total_inbound"`
	TotalAIRequests int64   `json:"total_ai_requests"`
	TotalAIReplies  int64   `json:"total_ai_replies"`
	TotalOutbound   int64   `json:"total_outbound"`
	TotalErrors     int64   `json:"total_errors"`
	RecentEvents    []Event `json:"recent_events"`
}

type Monitor struct {
	eventsMu sync.Mutex
	events   []Event
	idx      int
	count    int

	totalInbound    int64
	totalAIRequests int64
	totalAIReplies  int64
	totalOutbound   int64
	totalErrors     int64
}

func New(size int) *Monitor {
	if size <= 0 {
		size = 200
	}
	return &Monitor{events: make([]Event, size)}
}

func (m *Monitor) Record(e Event) {
	e.Timestamp = time.Now().UTC()

	switch e.Stage {
	case "inbound":
		atomic.AddInt64(&m.totalInbound, 1)
	case "ai_request":
		atomic.AddInt64(&m.totalAIRequests, 1)
	case "ai_response":
		if e.Status == "ok" {
			atomic.AddInt64(&m.totalAIReplies, 1)
		}
	case "outbound":
		if e.Status == "ok" {
			atomic.AddInt64(&m.totalOutbound, 1)
			if OnIncrement != nil {
				OnIncrement("processed")
			}
		}
	}

	if e.Status == "error" {
		atomic.AddInt64(&m.totalErrors, 1)
		if OnIncrement != nil {
			OnIncrement("error")
		}
	}

	m.eventsMu.Lock()
	m.events[m.idx] = e
	m.idx = (m.idx + 1) % len(m.events)
	if m.count < len(m.events) {
		m.count++
	}
	m.eventsMu.Unlock()
}

func (m *Monitor) GetStats() Stats {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	res := make([]Event, 0, m.count)
	cutoff := time.Time{}
	if defaultTTL > 0 {
		cutoff = time.Now().UTC().Add(-defaultTTL)
	}
	start := (m.idx - m.count) % len(m.events)
	if start < 0 {
		start += len(m.events)
	}
	for i := 0; i < m.count; i++ {
		e := m.events[(start+i)%len(m.events)]
		if !cutoff.IsZero() && !e.Timestamp.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		res = append(res, e)
	}

	return Stats{
		TotalInbound:    atomic.LoadInt64(&m.totalInbound),
		TotalAIRequests: atomic.LoadInt64(&m.totalAIRequests),
		TotalAIReplies:  atomic.LoadInt64(&m.totalAIReplies),
		TotalOutbound:   atomic.LoadInt64(&m.totalOutbound),
		TotalErrors:     atomic.LoadInt64(&m.totalErrors),
		RecentEvents:    res,
	}
}

func envInt(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err == nil {
		return d
	}
	sec, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

var defaultTTL time.Duration

var defaultMonitor = func() *Monitor {
	size := envInt("BOT_MONITOR_BUFFER", 200)
	defaultTTL = envDuration("BOT_MONITOR_TTL", 0)
	return New(size)
}()

func Record(e Event) {
	defaultMonitor.Record(e)
}

func GetStats() Stats {
	return defaultMonitor.GetStats()
}

package infrastructure

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	"github.com/sirupsen/logrus"
)

type debounceEntry struct {
	input domain.BotInput
	texts []string
	timer *time.Timer
}

type inflightEntry struct {
	cancel context.CancelFunc
	token  uint64
}

type Debouncer struct {
	mu       sync.Mutex
	entries  map[string]*debounceEntry
	inflight map[string]inflightEntry
	seq      uint64
	flushFn  func(ctx context.Context, input domain.BotInput)
}

func NewDebouncer(flushFn func(ctx context.Context, input domain.BotInput)) *Debouncer {
	return &Debouncer{
		entries:  make(map[string]*debounceEntry),
		inflight: make(map[string]inflightEntry),
		flushFn:  flushFn,
	}
}

func (d *Debouncer) Enqueue(ctx context.Context, input domain.BotInput, delay time.Duration) {
	if delay <= 0 {
		d.flushFn(ctx, input)
		return
	}

	// Key: BotID + InstanceID + ChatID + SenderID
	key := input.BotID + "|" + input.InstanceID + "|" + input.ChatID + "|" + input.SenderID

	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancelar cualquier procesamiento "en vuelo" para este mismo chat
	if prev, ok := d.inflight[key]; ok && prev.cancel != nil {
		prev.cancel()
		delete(d.inflight, key)
	}

	e, ok := d.entries[key]
	if !ok {
		e = &debounceEntry{
			input: input,
		}
		d.entries[key] = e
	}

	// Acumular texto
	if input.Text != "" {
		e.texts = append(e.texts, input.Text)
	}

	// Reiniciar timer
	if e.timer != nil {
		e.timer.Stop()
	}

	e.timer = time.AfterFunc(delay, func() {
		d.flush(key)
	})
}

func (d *Debouncer) flush(key string) {
	d.mu.Lock()
	e, ok := d.entries[key]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.entries, key)
	d.mu.Unlock()

	// Preparar input final con textos combinados
	finalInput := e.input
	if len(e.texts) > 1 {
		finalInput.Text = strings.Join(e.texts, "\n")
	} else if len(e.texts) == 1 {
		finalInput.Text = e.texts[0]
	}

	logrus.Infof("[DEBOUNCER] Flushing combined message for %s (parts: %d)", key, len(e.texts))

	// Registrar como "en vuelo" para poder cancelarlo si llega algo nuevo durante el procesamiento
	ctx, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	d.seq++
	token := d.seq
	d.inflight[key] = inflightEntry{cancel: cancel, token: token}
	d.mu.Unlock()

	go func() {
		defer func() {
			d.mu.Lock()
			if cur, ok := d.inflight[key]; ok && cur.token == token {
				delete(d.inflight, key)
			}
			d.mu.Unlock()
			cancel()
		}()
		d.flushFn(ctx, finalInput)
	}()
}

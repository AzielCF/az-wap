package application

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/sirupsen/logrus"
)

type PresenceManager struct {
	adaptersMu sync.RWMutex
	adapters   map[string]channel.ChannelAdapter

	store channel.PresenceStore

	presenceMu     sync.Mutex
	presenceTimers map[string]*time.Timer

	socketMu     sync.Mutex
	socketTimers map[string]*time.Timer

	// Callback to check if channel is active
	IsChannelActive func(channelID string) bool
}

func NewPresenceManager(store channel.PresenceStore) *PresenceManager {
	return &PresenceManager{
		adapters:       make(map[string]channel.ChannelAdapter),
		store:          store,
		presenceTimers: make(map[string]*time.Timer),
		socketTimers:   make(map[string]*time.Timer),
	}
}

func (pm *PresenceManager) RegisterAdapter(channelID string, adapter channel.ChannelAdapter) {
	pm.adaptersMu.Lock()
	defer pm.adaptersMu.Unlock()
	pm.adapters[channelID] = adapter
}

func (pm *PresenceManager) UnregisterAdapter(channelID string) {
	pm.adaptersMu.Lock()
	defer pm.adaptersMu.Unlock()
	delete(pm.adapters, channelID)

	// Detener timers activos si los hay
	pm.presenceMu.Lock()
	if t, ok := pm.presenceTimers[channelID]; ok {
		t.Stop()
		delete(pm.presenceTimers, channelID)
	}
	pm.presenceMu.Unlock()

	pm.socketMu.Lock()
	if t, ok := pm.socketTimers[channelID]; ok {
		t.Stop()
		delete(pm.socketTimers, channelID)
	}
	pm.socketMu.Unlock()
}

func (pm *PresenceManager) DeleteStatus(ctx context.Context, channelID string) error {
	pm.UnregisterAdapter(channelID)
	return pm.store.Delete(ctx, channelID)
}

func (pm *PresenceManager) HandleIncomingActivity(channelID string) {
	logrus.Debugf("[PresenceManager] Activity detected for channel %s. Cancelling hibernation timers.", channelID)

	ctx := context.Background()

	// Actualizar estado en el store
	p, _ := pm.store.Get(ctx, channelID)
	if p == nil {
		p = &channel.ChannelPresence{ChannelID: channelID}
	}
	p.LastSeen = time.Now()
	p.IsVisuallyOnline = true
	p.IsSocketConnected = true
	p.VisualOfflineAt = time.Time{} // Resetear objetivos
	p.DeepHibernateAt = time.Time{}
	_ = pm.store.Save(ctx, p)

	pm.presenceMu.Lock()
	if timer, ok := pm.presenceTimers[channelID]; ok {
		timer.Stop()
		delete(pm.presenceTimers, channelID)
	}
	pm.presenceMu.Unlock()

	pm.socketMu.Lock()
	if timer, ok := pm.socketTimers[channelID]; ok {
		timer.Stop()
		delete(pm.socketTimers, channelID)
	}
	pm.socketMu.Unlock()

	pm.adaptersMu.RLock()
	adapter, ok := pm.adapters[channelID]
	pm.adaptersMu.RUnlock()
	if ok {
		go func() {
			logrus.Infof("[PresenceManager] Resuming channel %s and setting VISUALLY ONLINE", channelID)
			_ = adapter.Resume(context.Background())

			// Wait up to 5 seconds for the connection to be ready
			for i := 0; i < 10; i++ {
				if adapter.Status() == channel.ChannelStatusConnected {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			_ = adapter.SetOnline(context.Background(), true)
		}()
	} else {
		logrus.Warnf("[PresenceManager] No adapter found for channel %s to handle activity", channelID)
	}
}

func (pm *PresenceManager) CheckChannelPresence(channelID string) {
	if pm.IsChannelActive != nil && pm.IsChannelActive(channelID) {
		return
	}

	pm.presenceMu.Lock()
	defer pm.presenceMu.Unlock()

	if _, ok := pm.presenceTimers[channelID]; ok {
		return
	}

	// Evitar programar si ya está offline visually
	ctx := context.Background()
	p, _ := pm.store.Get(ctx, channelID)
	if p != nil && !p.IsVisuallyOnline {
		return
	}

	delay := time.Duration(4+rand.Intn(7)) * time.Minute
	visualOfflineAt := time.Now().Add(delay)

	// Actualizar store con el tiempo objetivo
	if p != nil {
		p.VisualOfflineAt = visualOfflineAt
		_ = pm.store.Save(ctx, p)
	}

	pm.presenceTimers[channelID] = time.AfterFunc(delay, func() {
		pm.presenceMu.Lock()
		delete(pm.presenceTimers, channelID)
		pm.presenceMu.Unlock()

		if pm.IsChannelActive != nil && !pm.IsChannelActive(channelID) {
			pm.adaptersMu.RLock()
			adapter, ok := pm.adapters[channelID]
			pm.adaptersMu.RUnlock()
			if ok {
				logrus.Infof("[PresenceManager] Channel %s visually OFFLINE", channelID)
				_ = adapter.SetOnline(context.Background(), false)

				// Actualizar flag en store
				p, _ := pm.store.Get(context.Background(), channelID)
				if p != nil {
					p.IsVisuallyOnline = false
					_ = pm.store.Save(context.Background(), p)
				}
			}
		}
	})
}

func (pm *PresenceManager) CheckChannelSocket(channelID string) {
	if pm.IsChannelActive != nil && pm.IsChannelActive(channelID) {
		return
	}

	pm.socketMu.Lock()
	defer pm.socketMu.Unlock()

	if _, ok := pm.socketTimers[channelID]; ok {
		return
	}

	// Evitar programar si ya está hibernado
	ctx := context.Background()
	p, _ := pm.store.Get(ctx, channelID)
	if p != nil && !p.IsSocketConnected {
		return
	}

	// Stay physically connected for 2 hours of total inactivity
	delay := 2 * time.Hour
	deepHibernateAt := time.Now().Add(delay)

	// Actualizar store con el tiempo objetivo
	if p != nil {
		p.DeepHibernateAt = deepHibernateAt
		_ = pm.store.Save(ctx, p)
	}

	pm.socketTimers[channelID] = time.AfterFunc(delay, func() {
		pm.socketMu.Lock()
		delete(pm.socketTimers, channelID)
		pm.socketMu.Unlock()

		if pm.IsChannelActive != nil && !pm.IsChannelActive(channelID) {
			pm.adaptersMu.RLock()
			adapter, ok := pm.adapters[channelID]
			pm.adaptersMu.RUnlock()
			if ok {
				logrus.Warnf("[PresenceManager] DEEP HIBERNATION (Socket Close) for channel %s after 2h idle", channelID)
				_ = adapter.Hibernate(context.Background())

				// Actualizar flags en store
				p, _ := pm.store.Get(context.Background(), channelID)
				if p != nil {
					p.IsSocketConnected = false
					p.IsVisuallyOnline = false
					_ = pm.store.Save(context.Background(), p)
				}
			}
		}
	})
}

func (pm *PresenceManager) GetStatus(ctx context.Context, channelID string) (*channel.ChannelPresence, error) {
	return pm.store.Get(ctx, channelID)
}

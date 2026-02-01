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

	// Stop active timers if any
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

func (pm *PresenceManager) isNightWindow() bool {
	hour := time.Now().Hour()
	// Night window for faster visual offline: 12 AM - 6 AM
	return hour >= 0 && hour < 6
}

func (pm *PresenceManager) HandleIncomingActivity(channelID string) {
	logrus.Debugf("[PresenceManager] Activity detected for channel %s. Updating presence...", channelID)

	ctx := context.Background()

	// 1. Update internal state
	p, _ := pm.store.Get(ctx, channelID)
	if p == nil {
		p = &channel.ChannelPresence{ChannelID: channelID}
	}

	p.LastSeen = time.Now()
	p.IsVisuallyOnline = true
	p.IsSocketConnected = true // We want the socket ALWAYS connected
	p.VisualOfflineAt = time.Time{}
	p.DeepHibernateAt = time.Time{} // Reset deep hibernation target
	_ = pm.store.Save(ctx, p)

	// 2. Clear old timers
	pm.presenceMu.Lock()
	if timer, ok := pm.presenceTimers[channelID]; ok {
		timer.Stop()
		delete(pm.presenceTimers, channelID)
	}
	pm.presenceMu.Unlock()

	// 3. Set visual status as 'Online' while handling message
	pm.adaptersMu.RLock()
	adapter, ok := pm.adapters[channelID]
	pm.adaptersMu.RUnlock()

	if ok {
		go func() {
			// Ensure it's connected first (self-healing)
			if adapter.Status() != channel.ChannelStatusConnected {
				logrus.Infof("[PresenceManager] Reconnecting socket for %s...", channelID)
				_ = adapter.Resume(context.Background())
			}
			_ = adapter.SetOnline(context.Background(), true)
		}()
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

	ctx := context.Background()
	p, _ := pm.store.Get(ctx, channelID)
	if p != nil && !p.IsVisuallyOnline {
		return
	}

	// How long to stay visually 'Online' after idle
	delay := time.Duration(15+rand.Intn(10)) * time.Minute
	if pm.isNightWindow() {
		// Faster visual offline during the night (1-3 mins)
		delay = time.Duration(1+rand.Intn(2)) * time.Minute
	}

	visualOfflineAt := time.Now().Add(delay)

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
				// We keep the socket 100% OPEN, just go visually offline
				logrus.Infof("[PresenceManager] Channel %s going visually OFFLINE (Socket stays persistent)", channelID)
				_ = adapter.SetOnline(context.Background(), false)
				p, _ := pm.store.Get(context.Background(), channelID)
				if p != nil {
					p.IsVisuallyOnline = false
					_ = pm.store.Save(context.Background(), p)
				}
			}
		}
	})
}

// EnsureChannelConnectivity acts as a self-healing ticker.
// It ensures the socket is ALWAYS open, reconnecting if disconnected.
func (pm *PresenceManager) EnsureChannelConnectivity(channelID string) {
	pm.adaptersMu.RLock()
	adapter, ok := pm.adapters[channelID]
	pm.adaptersMu.RUnlock()

	if !ok {
		return
	}

	// If socket is not connected, reconnect it AUTOMATICALLY
	if adapter.Status() != channel.ChannelStatusConnected {
		logrus.Infof("[PresenceManager] Auto-healing: reconnecting socket for channel %s", channelID)
		go func() {
			_ = adapter.Resume(context.Background())

			// Update storage state
			ctx := context.Background()
			p, _ := pm.store.Get(ctx, channelID)
			if p != nil {
				p.IsSocketConnected = true
				_ = pm.store.Save(ctx, p)
			}
		}()
	}
}

func (pm *PresenceManager) GetStatus(ctx context.Context, channelID string) (*channel.ChannelPresence, error) {
	return pm.store.Get(ctx, channelID)
}

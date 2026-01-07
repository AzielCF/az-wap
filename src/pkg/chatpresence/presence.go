package chatpresence

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
)

type entry struct {
	composing bool
	media     types.ChatPresenceMedia
	updatedAt time.Time
}

var (
	mu    sync.Mutex
	store = map[string]entry{}
)

func key(instanceID, chatJID string) string {
	return instanceID + "|" + chatJID
}

func Update(instanceID, chatJID string, state types.ChatPresence, media types.ChatPresenceMedia) {
	instanceID = strings.TrimSpace(instanceID)
	chatJID = strings.TrimSpace(chatJID)
	if instanceID == "" || chatJID == "" {
		return
	}

	mu.Lock()
	store[key(instanceID, chatJID)] = entry{
		composing: state == types.ChatPresenceComposing,
		media:     media,
		updatedAt: time.Now(),
	}
	mu.Unlock()
}

func IsComposing(instanceID, chatJID string) bool {
	instanceID = strings.TrimSpace(instanceID)
	chatJID = strings.TrimSpace(chatJID)
	if instanceID == "" || chatJID == "" {
		return false
	}

	mu.Lock()
	e, ok := store[key(instanceID, chatJID)]
	if !ok {
		mu.Unlock()
		return false
	}
	if time.Since(e.updatedAt) > 12*time.Second {
		delete(store, key(instanceID, chatJID))
		mu.Unlock()
		return false
	}
	res := e.composing
	mu.Unlock()
	return res
}

func Media(instanceID, chatJID string) types.ChatPresenceMedia {
	instanceID = strings.TrimSpace(instanceID)
	chatJID = strings.TrimSpace(chatJID)
	if instanceID == "" || chatJID == "" {
		return types.ChatPresenceMediaText
	}

	mu.Lock()
	e, ok := store[key(instanceID, chatJID)]
	if !ok || time.Since(e.updatedAt) > 12*time.Second {
		if ok {
			delete(store, key(instanceID, chatJID))
		}
		mu.Unlock()
		return types.ChatPresenceMediaText
	}
	m := e.media
	mu.Unlock()
	return m
}

func WaitIdle(ctx context.Context, instanceID, chatJID string, timeout time.Duration) bool {
	if timeout <= 0 {
		return true
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()

	for {
		if !IsComposing(instanceID, chatJID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-poll.C:
		}
	}
}

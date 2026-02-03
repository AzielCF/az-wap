package application

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	botengine "github.com/AzielCF/az-wap/botengine"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	clientDomain "github.com/AzielCF/az-wap/clients/domain"
	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/domain/session"
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/AzielCF/az-wap/workspace/repository"
	"github.com/sirupsen/logrus"
)

// Re-export types from session package for backward compatibility
type SessionState = session.SessionState

const (
	StateDebouncing = session.StateDebouncing
	StateProcessing = session.StateProcessing
	StateWaiting    = session.StateWaiting
)

// SessionEntry is now an alias to session.SessionEntry
// This maintains backward compatibility while centralizing the type definition
type SessionEntry = session.SessionEntry

// timerBundle holds the non-serializable timers for each session
// These are kept locally and cannot be persisted to external stores
type timerBundle struct {
	debounce *time.Timer
	warning  *time.Timer
}

type ActiveSessionInfo struct {
	Key       string       `json:"key"`
	ChannelID string       `json:"channel_id"`
	ChatID    string       `json:"chat_id"`
	SenderID  string       `json:"sender_id"`
	State     SessionState `json:"state"`
	ExpiresIn int          `json:"expires_in"` // segundos
}

type SessionOrchestrator struct {
	botEngine *botengine.Engine
	store     session.SessionStore
	typing    channel.TypingStore

	// Timers are kept locally (not serializable)
	timerMu sync.Mutex
	timers  map[string]*timerBundle

	// Callbacks
	// Callbacks
	OnProcessFinal        func(ctx context.Context, ch channel.Channel, msg message.IncomingMessage, botID string) (botengineDomain.BotOutput, error)
	OnInactivityWarn      func(key string, ch channel.Channel)
	OnCleanupFiles        func(e *SessionEntry)
	OnChannelIdle         func(channelID string)
	OnWaitIdle            func(ctx context.Context, channelID, chatID string)
	OnSessionClosed       func(e *SessionEntry, ch channel.Channel)
	GetWorkspaceConfig    func(workspaceID string) *workspace.WorkspaceConfig
	GetSubscriptionConfig func(clientID, channelID string) *clientDomain.ClientSubscription
}

// NewSessionOrchestrator creates a new orchestrator with the default in-memory store
func NewSessionOrchestrator(botEngine *botengine.Engine) *SessionOrchestrator {
	return NewSessionOrchestratorWithStore(botEngine, repository.NewMemorySessionStore(), repository.NewMemoryTypingStore())
}

// NewSessionOrchestratorWithStore creates a new orchestrator with a custom session store
func NewSessionOrchestratorWithStore(botEngine *botengine.Engine, store session.SessionStore, typing channel.TypingStore) *SessionOrchestrator {
	return &SessionOrchestrator{
		botEngine: botEngine,
		store:     store,
		typing:    typing,
		timers:    make(map[string]*timerBundle),
	}
}

// GetSessionStore returns the underlying session store (useful for metrics/debugging)
func (s *SessionOrchestrator) GetSessionStore() session.SessionStore {
	return s.store
}

func (s *SessionOrchestrator) getTimers(key string) *timerBundle {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	return s.timers[key]
}

func (s *SessionOrchestrator) setTimers(key string, t *timerBundle) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	s.timers[key] = t
}

func (s *SessionOrchestrator) stopAndClearTimers(key string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if t, ok := s.timers[key]; ok {
		if t.debounce != nil {
			t.debounce.Stop()
		}
		if t.warning != nil {
			t.warning.Stop()
		}
		delete(s.timers, key)
	}
}

func (s *SessionOrchestrator) GetEntry(key string) (*SessionEntry, bool) {
	ctx := context.Background()
	e, err := s.store.Get(ctx, key)
	if err != nil {
		logrus.WithError(err).Warnf("[SessionOrchestrator] Failed to get entry: %s", key)
		return nil, false
	}
	return e, e != nil
}

func (s *SessionOrchestrator) DeleteEntry(key string) {
	ctx := context.Background()
	s.stopAndClearTimers(key)
	_ = s.store.Delete(ctx, key)
}

func (s *SessionOrchestrator) GetOrCreateEntry(key string, msg message.IncomingMessage) (*SessionEntry, bool) {
	ctx := context.Background()
	e, err := s.store.Get(ctx, key)
	if err != nil {
		logrus.WithError(err).Warnf("[SessionOrchestrator] Failed to get entry: %s", key)
	}

	if e != nil {
		return e, true
	}

	e = &SessionEntry{
		Msg:        msg,
		State:      StateDebouncing,
		FocusScore: 0,
	}

	// Initial short TTL, will be extended properly in EnqueueDebounced
	_ = s.store.Save(ctx, key, e, 5*time.Minute)
	return e, false
}

// calculateSessionParams determines the session duration, warning time, and storage TTL
// Priority: Client Config > Workspace Config > Channel Config > Default (4m)
// calculateSessionParams determines the session duration, warning time, storage TTL, and history limit
// Priority: Client Config > Workspace Config > Channel Config > Default (4m, 10 msgs)
func (s *SessionOrchestrator) calculateSessionParams(ch channel.Channel, msg message.IncomingMessage) (sessionDuration time.Duration, warningDelay time.Duration, valkeyTTL time.Duration, maxHistoryLimit int) {
	// Defaults
	totalMinutes := 4
	warningMinutes := 3  // Default warning at 3rd minute (4th minute = expiry)
	maxHistoryLimit = 10 // Default limit for standard users

	// 1. Check Channel Config
	if ch.Config.SessionTimeout > 0 {
		totalMinutes = ch.Config.SessionTimeout
	}
	if ch.Config.InactivityWarningTime > 0 {
		warningMinutes = ch.Config.InactivityWarningTime
	}
	if ch.Config.MaxHistoryLimit != 0 {
		maxHistoryLimit = ch.Config.MaxHistoryLimit
	}

	// 2. Check Workspace Config (Override)
	if s.GetWorkspaceConfig != nil && msg.WorkspaceID != "" {
		if wsConfig := s.GetWorkspaceConfig(msg.WorkspaceID); wsConfig != nil {
			if wsConfig.SessionTimeout > 0 {
				totalMinutes = wsConfig.SessionTimeout
			}
			if wsConfig.InactivityWarningTime > 0 {
				warningMinutes = wsConfig.InactivityWarningTime
			}
			// NOTE: WorkspaceConfig doesn't have MaxHistoryLimit yet, skipping override here for now or assume inherit
		}
	}

	// 3. Check Subscription Config (Highest Priority)
	// We use SenderID (PlatformID) and ChannelID to find the specific subscription
	if s.GetSubscriptionConfig != nil && msg.SenderID != "" {
		if sub := s.GetSubscriptionConfig(msg.SenderID, ch.ID); sub != nil {
			if sub.SessionTimeout > 0 {
				totalMinutes = sub.SessionTimeout
			}
			if sub.InactivityWarningTime > 0 {
				warningMinutes = sub.InactivityWarningTime
			}

			// Subscription History Logic:
			// If MaxHistoryLimit is explicitly set (non-nil), use it.
			// If nil (default for sub), use -1 (Unlimited).
			if sub.MaxHistoryLimit != nil {
				maxHistoryLimit = *sub.MaxHistoryLimit
			} else {
				maxHistoryLimit = -1
			}
		}
	}

	// Enforce Minimum Session Duration
	if totalMinutes < 4 {
		totalMinutes = 4
	}

	// Validation Rule: Warning cannot be earlier than 75% of total session time
	minWarningTime := int(float64(totalMinutes) * 0.75)
	if warningMinutes < minWarningTime {
		warningMinutes = minWarningTime
	}

	sessionDuration = time.Duration(totalMinutes) * time.Minute
	warningDelay = time.Duration(warningMinutes) * time.Minute
	valkeyTTL = sessionDuration + 30*time.Second

	return
}

func (s *SessionOrchestrator) CloseSession(key string) {
	ctx := context.Background()
	s.stopAndClearTimers(key)

	e, err := s.store.Get(ctx, key)
	if err != nil || e == nil {
		return
	}

	_ = s.store.Delete(ctx, key)
	if s.OnCleanupFiles != nil {
		go s.OnCleanupFiles(e)
	}
}

func (s *SessionOrchestrator) GetActiveSessions() []ActiveSessionInfo {
	ctx := context.Background()
	entries, err := s.store.GetAll(ctx)
	if err != nil {
		logrus.WithError(err).Warn("[SessionOrchestrator] Failed to get all sessions")
		return nil
	}

	var sessions []ActiveSessionInfo
	for k, e := range entries {
		parts := strings.Split(k, "|")
		info := ActiveSessionInfo{
			Key:   k,
			State: e.State,
		}
		if !e.ExpireAt.IsZero() {
			info.ExpiresIn = int(time.Until(e.ExpireAt).Seconds())
		}
		if len(parts) >= 1 {
			info.ChannelID = parts[0]
		}
		if len(parts) >= 2 {
			info.ChatID = parts[1]
		}
		if len(parts) >= 3 {
			info.SenderID = parts[2]
		}
		sessions = append(sessions, info)
	}
	return sessions
}

func (s *SessionOrchestrator) EnqueueDebounced(ctx context.Context, ch channel.Channel, msg message.IncomingMessage, botID string, markRead func(string, []string)) {
	key := ch.ID + "|" + msg.ChatID + "|" + msg.SenderID
	storeCtx := context.Background()

	// Identity Protection - handle LID migration
	if strings.Contains(msg.SenderID, "@lid") {
		keys, _ := s.store.List(storeCtx, ch.ID+"|"+msg.ChatID+"|*")
		for _, k := range keys {
			if k != key {
				logrus.Infof("[SessionOrchestrator] Identity migration detected: %s -> %s", k, msg.SenderID)
				if oldEntry, _ := s.store.Get(storeCtx, k); oldEntry != nil {
					oldEntry.State = StateWaiting
					s.stopAndClearTimers(k)
					_ = s.store.Delete(storeCtx, k)
					if s.OnCleanupFiles != nil {
						go s.OnCleanupFiles(oldEntry)
					}
				}
			}
		}
	}

	// Get or create entry
	e, err := s.store.Get(storeCtx, key)
	isNew := e == nil || err != nil

	if isNew {
		e = &SessionEntry{
			Msg:        msg,
			State:      StateDebouncing,
			BotID:      botID,
			FocusScore: 0,
			Language:   msg.Language,
		}
	} else {
		// Update language if it changes or is resolved later
		if msg.Language != "" {
			e.Language = msg.Language
		}
		if e.BotID == "" {
			e.BotID = botID
		}
	}

	// Capture latest config flags
	e.InactivityWarningEnabled = true // Default
	if ch.Config.InactivityWarning != nil {
		e.InactivityWarningEnabled = ch.Config.InactivityWarning.Enabled
	}
	e.SessionClosingEnabled = true // Default
	if ch.Config.SessionClosing != nil {
		e.SessionClosingEnabled = ch.Config.SessionClosing.Enabled
	}

	if id, ok := msg.Metadata["message_id"].(string); ok && id != "" {
		e.MessageIDs = append(e.MessageIDs, id)
	}

	isRecentlyReplied := !e.LastReplyTime.IsZero() && time.Since(e.LastReplyTime) < botengineDomain.DefaultPresenceConfig.ImmediateReadWindow
	isHighFocus := e.FocusScore > botengineDomain.DefaultPresenceConfig.HighFocusThreshold

	if (isRecentlyReplied || isHighFocus || e.State == StateProcessing || e.ChatOpen) && markRead != nil {
		idsToMark := make([]string, len(e.MessageIDs))
		copy(idsToMark, e.MessageIDs)
		go markRead(msg.ChatID, idsToMark)
	}

	sessionDuration, warningDelay, valkeyTTL, maxHistoryLimit := s.calculateSessionParams(ch, msg)

	e.LastSeen = time.Now()
	e.ExpireAt = e.LastSeen.Add(sessionDuration)
	e.MaxHistoryLimit = maxHistoryLimit

	// Stop existing timers
	s.stopAndClearTimers(key)

	debounceBase := time.Duration(globalConfig.AIDebounceMs) * time.Millisecond
	if debounceBase <= 0 {
		// Save state before processing
		_ = s.store.Save(storeCtx, key, e, valkeyTTL)

		if s.OnProcessFinal != nil {
			_, _ = s.OnProcessFinal(ctx, ch, msg, botID)
		}

		// Update state after processing
		e.State = StateWaiting
		e.ExpireAt = time.Now().Add(sessionDuration)

		// Capture latest config flags
		e.InactivityWarningEnabled = true // Default
		if ch.Config.InactivityWarning != nil {
			e.InactivityWarningEnabled = ch.Config.InactivityWarning.Enabled
		}
		e.SessionClosingEnabled = true // Default
		if ch.Config.SessionClosing != nil {
			e.SessionClosingEnabled = ch.Config.SessionClosing.Enabled
		}

		_ = s.store.Save(storeCtx, key, e, valkeyTTL)

		tb := &timerBundle{}
		if s.OnInactivityWarn != nil {
			tb.warning = time.AfterFunc(warningDelay, func() {
				parts := strings.Split(key, "|")
				chatID := ""
				if len(parts) >= 2 {
					chatID = parts[1]
				}
				msgworker.GetGlobalPool().Dispatch(msgworker.MessageJob{
					InstanceID: ch.ID,
					ChatJID:    chatID,
					Handler: func(_ context.Context) error {
						s.OnInactivityWarn(key, ch)
						return nil
					},
				})
			})
		}
		tb.debounce = time.AfterFunc(sessionDuration, func() {
			if curr, _ := s.store.Get(storeCtx, key); curr != nil && curr.State == StateWaiting {
				s.stopAndClearTimers(key)
				_ = s.store.Delete(storeCtx, key)
				if s.OnCleanupFiles != nil {
					s.OnCleanupFiles(curr)
				}
				if s.OnChannelIdle != nil {
					go s.OnChannelIdle(curr.Msg.ChannelID)
				}
				if s.OnSessionClosed != nil {
					s.OnSessionClosed(curr, ch)
				}
			}
		})
		s.setTimers(key, tb)
		return
	}

	if e.State == StateWaiting {
		e.State = StateDebouncing
		e.Texts = nil
		e.Msg = msg

		if !e.LastReplyTime.IsZero() {
			diff := time.Since(e.LastReplyTime)
			if diff < 1*time.Minute {
				e.FocusScore += 30
			} else if diff < 5*time.Minute {
				e.FocusScore += 10
			} else {
				e.FocusScore -= 20
			}
		}

		// Length based adjustment
		if len(msg.Text) > 500 {
			e.FocusScore += 15
		} else if len(msg.Text) > 100 {
			e.FocusScore += 5
		}

		if e.FocusScore < 0 {
			e.FocusScore = 0
		} else if e.FocusScore > 100 {
			e.FocusScore = 100
		}
	}

	if e.FocusScore > botengineDomain.DefaultPresenceConfig.HighFocusThreshold {
		debounceBase = 1500 * time.Millisecond
	} else if e.FocusScore > botengineDomain.DefaultPresenceConfig.MediumFocusThreshold {
		debounceBase = 3000 * time.Millisecond
	}

	if s.botEngine != nil {
		debounceBase = s.botEngine.Humanizer().GetDebounceDuration(debounceBase, len(msg.Text), len(e.Texts))
	}

	variance := time.Duration(rand.Intn(25)-10) * (debounceBase / 100)
	debounce := debounceBase + variance

	newMeta := make(map[string]any)
	for k, v := range msg.Metadata {
		newMeta[k] = v
	}
	msg.Metadata = newMeta

	e.Msg = msg
	if msg.Media != nil {
		e.Media = append(e.Media, msg.Media)
	}
	if msg.Text != "" {
		e.Texts = append(e.Texts, msg.Text)
	}

	// Save updated entry
	// (Re-calculate if needed, but we already have valid params from start)
	_ = s.store.Save(storeCtx, key, e, valkeyTTL)

	if e.State == StateProcessing {
		logrus.Infof("[SessionOrchestrator] Message enqueued during processing for %s (Session extended, Focus: %d)", key, e.FocusScore)
		return
	}

	if e.State == StateDebouncing {
		tb := &timerBundle{
			debounce: time.AfterFunc(debounce, func() {
				s.FlushDebounced(key, ch, botID, markRead)
			}),
		}
		s.setTimers(key, tb)
	}
}

func (s *SessionOrchestrator) FlushDebounced(key string, ch channel.Channel, botID string, markRead func(string, []string)) {
	storeCtx := context.Background()

	e, err := s.store.Get(storeCtx, key)
	if err != nil || e == nil || e.State != StateDebouncing {
		return
	}

	// Calculate session params immediately for use throughout the function scope
	sessionDuration, warningDelay, valkeyTTL, maxHistoryLimit := s.calculateSessionParams(ch, e.Msg)
	e.MaxHistoryLimit = maxHistoryLimit

	typingState, _ := s.typing.Get(storeCtx, ch.ID, e.Msg.ChatID)
	isComposing := typingState != nil
	if isComposing {
		debounce := time.Duration(globalConfig.AIDebounceMs) * time.Millisecond
		if debounce <= 0 {
			debounce = 2 * time.Second
		}
		s.stopAndClearTimers(key)
		tb := &timerBundle{
			debounce: time.AfterFunc(debounce, func() {
				s.FlushDebounced(key, ch, botID, markRead)
			}),
		}
		s.setTimers(key, tb)
		logrus.Debugf("[SessionOrchestrator] User STILL TYPING in %s. Rescheduling flush in %v (ID matching: %s)", key, debounce, e.Msg.ChatID)
		return
	}
	logrus.Debugf("[SessionOrchestrator] Flush check: IsComposing=%v for %s", isComposing, e.Msg.ChatID)

	e.State = StateProcessing
	s.stopAndClearTimers(key)

	batch := e.Texts
	e.Texts = nil
	ids := e.MessageIDs
	e.MessageIDs = nil

	finalMsg := e.Msg
	if len(batch) > 1 {
		finalMsg.Text = strings.Join(batch, "\n")
	} else if len(batch) == 1 {
		finalMsg.Text = batch[0]
	}

	if finalMsg.Metadata == nil {
		finalMsg.Metadata = make(map[string]any)
	}

	if len(ids) > 0 {
		finalMsg.Metadata["message_ids"] = ids
	}
	if len(e.Media) > 0 {
		finalMsg.Medias = e.Media
		e.Media = nil
	}

	// Save updated state
	_ = s.store.Save(storeCtx, key, e, valkeyTTL)

	msgworker.GetGlobalPool().Dispatch(msgworker.MessageJob{
		InstanceID: ch.ID,
		ChatJID:    finalMsg.ChatID,
		Handler: func(workerCtx context.Context) error {
			if s.OnWaitIdle != nil {
				s.OnWaitIdle(workerCtx, ch.ID, finalMsg.ChatID)
			}

			if s.OnProcessFinal != nil {
				output, _ := s.OnProcessFinal(workerCtx, ch, finalMsg, botID)

				// Re-fetch entry from store (may have been updated)
				if curr, _ := s.store.Get(storeCtx, key); curr != nil {
					if bCount, ok := output.Metadata["bubbles"].(string); ok {
						_, _ = fmt.Sscanf(bCount, "%d", &curr.LastBubbleCount)
					}

					if len(curr.Texts) > 0 {
						if curr.Msg.Metadata == nil {
							curr.Msg.Metadata = make(map[string]any)
						}
						curr.Msg.Metadata["is_delayed"] = true
						readingPause := time.Duration(0)
						if s.botEngine != nil {
							totalContent := strings.Join(curr.Texts, "")
							readingPause = s.botEngine.Humanizer().CalculateReadingTime(totalContent)
						}
						curr.State = StateDebouncing
						debounce := (time.Duration(globalConfig.AIDebounceMs) * time.Millisecond) + readingPause
						_ = s.store.Save(storeCtx, key, curr, valkeyTTL)

						tb := &timerBundle{
							debounce: time.AfterFunc(debounce, func() {
								s.FlushDebounced(key, ch, botID, markRead)
							}),
						}
						s.setTimers(key, tb)
						logrus.Debugf("[SessionOrchestrator] Re-queuing %s with reading pause of %s", key, readingPause)
					} else {
						curr.State = StateWaiting
						curr.ExpireAt = time.Now().Add(sessionDuration)
						curr.ChatOpen = false

						// Capture latest config flags
						curr.InactivityWarningEnabled = true // Default
						if ch.Config.InactivityWarning != nil {
							curr.InactivityWarningEnabled = ch.Config.InactivityWarning.Enabled
						}
						curr.SessionClosingEnabled = true // Default
						if ch.Config.SessionClosing != nil {
							curr.SessionClosingEnabled = ch.Config.SessionClosing.Enabled
						}

						_ = s.store.Save(storeCtx, key, curr, valkeyTTL)

						tb := &timerBundle{}
						if s.OnInactivityWarn != nil {
							tb.warning = time.AfterFunc(warningDelay, func() {
								parts := strings.Split(key, "|")
								chatID := ""
								if len(parts) >= 2 {
									chatID = parts[1]
								}
								msgworker.GetGlobalPool().Dispatch(msgworker.MessageJob{
									InstanceID: ch.ID,
									ChatJID:    chatID,
									Handler: func(_ context.Context) error {
										s.OnInactivityWarn(key, ch)
										return nil
									},
								})
							})
						}
						tb.debounce = time.AfterFunc(sessionDuration, func() {
							if c, _ := s.store.Get(storeCtx, key); c != nil && c.State == StateWaiting {
								if s.OnSessionClosed != nil {
									go s.OnSessionClosed(c, ch)
								}
								_ = s.store.Delete(storeCtx, key)
								if s.OnCleanupFiles != nil {
									s.OnCleanupFiles(c)
								}
								if s.OnChannelIdle != nil {
									go s.OnChannelIdle(c.Msg.ChannelID)
								}
							}
						})
						s.setTimers(key, tb)
					}
				}
			}
			return nil
		},
	})
}

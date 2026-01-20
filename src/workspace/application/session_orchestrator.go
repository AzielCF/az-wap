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
	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/chatpresence"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/message"
	"github.com/AzielCF/az-wap/workspace/domain/session"
	"github.com/sirupsen/logrus"
)

type SessionState string

const (
	StateDebouncing SessionState = "debouncing"
	StateProcessing SessionState = "processing"
	StateWaiting    SessionState = "waiting"
)

type ActiveSessionInfo struct {
	Key       string       `json:"key"`
	ChannelID string       `json:"channel_id"`
	ChatID    string       `json:"chat_id"`
	SenderID  string       `json:"sender_id"`
	State     SessionState `json:"state"`
	ExpiresIn int          `json:"expires_in"` // segundos
}

type SessionEntry struct {
	Msg             message.IncomingMessage
	Texts           []string
	MessageIDs      []string
	Timer           *time.Timer
	WarningTimer    *time.Timer
	ExpireAt        time.Time
	State           SessionState
	LastSeen        time.Time
	LastBubbleCount int
	Media           []*message.IncomingMedia
	Memory          session.SessionMemory
	BotID           string
	SessionPath     string
	DownloadedFiles []string
	ChatOpen        bool
	FocusScore      int
	LastReplyTime   time.Time
	LastMindset     *botengineDomain.Mindset
	PendingTasks    []string
	Language        string
}

type SessionOrchestrator struct {
	botEngine *botengine.Engine
	dbMu      sync.Mutex
	dbEntries map[string]*SessionEntry

	// Callbacks
	OnProcessFinal   func(ctx context.Context, ch channel.Channel, msg message.IncomingMessage, botID string) (botengineDomain.BotOutput, error)
	OnInactivityWarn func(key string, ch channel.Channel)
	OnCleanupFiles   func(e *SessionEntry)
	OnChannelIdle    func(channelID string)
	OnWaitIdle       func(ctx context.Context, channelID, chatID string)
}

func NewSessionOrchestrator(botEngine *botengine.Engine) *SessionOrchestrator {
	return &SessionOrchestrator{
		botEngine: botEngine,
		dbEntries: make(map[string]*SessionEntry),
	}
}

func (s *SessionOrchestrator) GetEntry(key string) (*SessionEntry, bool) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	e, ok := s.dbEntries[key]
	return e, ok
}

func (s *SessionOrchestrator) DeleteEntry(key string) {
	s.dbMu.Lock()
	delete(s.dbEntries, key)
	s.dbMu.Unlock()
}

func (s *SessionOrchestrator) GetOrCreateEntry(key string, msg message.IncomingMessage) (*SessionEntry, bool) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	e, ok := s.dbEntries[key]
	if !ok {
		e = &SessionEntry{
			Msg:        msg,
			State:      StateDebouncing,
			FocusScore: 0,
		}
		s.dbEntries[key] = e
	}
	return e, ok
}

func (s *SessionOrchestrator) CloseSession(key string) {
	s.dbMu.Lock()
	e, ok := s.dbEntries[key]
	if ok {
		if e.Timer != nil {
			e.Timer.Stop()
		}
		if e.WarningTimer != nil {
			e.WarningTimer.Stop()
		}
		delete(s.dbEntries, key)
		if s.OnCleanupFiles != nil {
			go s.OnCleanupFiles(e)
		}
	}
	s.dbMu.Unlock()
}

func (s *SessionOrchestrator) GetActiveSessions() []ActiveSessionInfo {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	var sessions []ActiveSessionInfo
	for k, e := range s.dbEntries {
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

	s.dbMu.Lock()
	// Identity Protection
	if strings.Contains(msg.SenderID, "@lid") {
		for k, e := range s.dbEntries {
			if strings.HasPrefix(k, ch.ID+"|"+msg.ChatID+"|") && k != key {
				logrus.Infof("[SessionOrchestrator] Identity migration detected: %s -> %s", k, msg.SenderID)
				e.State = StateWaiting
				if e.Timer != nil {
					e.Timer.Stop()
				}
				delete(s.dbEntries, k)
				if s.OnCleanupFiles != nil {
					go s.OnCleanupFiles(e)
				}
			}
		}
	}

	e, ok := s.dbEntries[key]
	if !ok {
		e = &SessionEntry{
			Msg:        msg,
			State:      StateDebouncing,
			BotID:      botID,
			FocusScore: 0,
			Language:   msg.Language,
		}
		s.dbEntries[key] = e
	} else {
		// Update language if it changes or is resolved later
		if msg.Language != "" {
			e.Language = msg.Language
		}
		if e.BotID == "" {
			e.BotID = botID
		}
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

	e.LastSeen = time.Now()
	e.ExpireAt = e.LastSeen.Add(4 * time.Minute)

	if e.Timer != nil {
		e.Timer.Stop()
		e.Timer = nil
	}
	if e.WarningTimer != nil {
		e.WarningTimer.Stop()
		e.WarningTimer = nil
	}
	s.dbMu.Unlock()

	debounceBase := time.Duration(globalConfig.AIDebounceMs) * time.Millisecond
	if debounceBase <= 0 {
		if s.OnProcessFinal != nil {
			_, _ = s.OnProcessFinal(ctx, ch, msg, botID)
		}

		s.dbMu.Lock()
		e.State = StateWaiting
		e.ExpireAt = time.Now().Add(4 * time.Minute)
		if s.OnInactivityWarn != nil {
			e.WarningTimer = time.AfterFunc(3*time.Minute, func() {
				s.OnInactivityWarn(key, ch)
			})
		}
		e.Timer = time.AfterFunc(4*time.Minute, func() {
			s.dbMu.Lock()
			if curr, still := s.dbEntries[key]; still && curr == e && curr.State == StateWaiting {
				delete(s.dbEntries, key)
				if s.OnCleanupFiles != nil {
					s.OnCleanupFiles(e)
				}
				if s.OnChannelIdle != nil {
					go s.OnChannelIdle(e.Msg.ChannelID)
				}
			}
			s.dbMu.Unlock()
		})
		s.dbMu.Unlock()
		return
	}

	s.dbMu.Lock()
	defer s.dbMu.Unlock()

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

		// Length based adjustment (from original manager)
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

	if e.State == StateProcessing {
		logrus.Infof("[SessionOrchestrator] Message enqueued during processing for %s (Session extended, Focus: %d)", key, e.FocusScore)
		return
	}

	if e.State == StateDebouncing {
		e.Timer = time.AfterFunc(debounce, func() {
			s.FlushDebounced(key, ch, botID, markRead)
		})
	}
}

func (s *SessionOrchestrator) FlushDebounced(key string, ch channel.Channel, botID string, markRead func(string, []string)) {
	s.dbMu.Lock()
	e, ok := s.dbEntries[key]
	if !ok || e.State != StateDebouncing {
		s.dbMu.Unlock()
		return
	}

	isComposing := chatpresence.IsComposing(ch.ID, e.Msg.ChatID)
	if isComposing {
		debounce := time.Duration(globalConfig.AIDebounceMs) * time.Millisecond
		if debounce <= 0 {
			debounce = 2 * time.Second
		}
		if e.Timer != nil {
			e.Timer.Stop()
		}
		e.Timer = time.AfterFunc(debounce, func() {
			s.FlushDebounced(key, ch, botID, markRead)
		})
		logrus.Infof("[SessionOrchestrator] User STILL TYPING in %s. Rescheduling flush in %v (ID matching: %s)", key, debounce, e.Msg.ChatID)
		s.dbMu.Unlock()
		return
	}
	logrus.Debugf("[SessionOrchestrator] Flush check: IsComposing=%v for %s", isComposing, e.Msg.ChatID)

	e.State = StateProcessing
	if e.Timer != nil {
		e.Timer.Stop()
		e.Timer = nil
	}

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

	if len(ids) > 0 {
		finalMsg.Metadata["message_ids"] = ids
	}
	if len(e.Media) > 0 {
		finalMsg.Medias = e.Media
		e.Media = nil
	}
	s.dbMu.Unlock()

	msgworker.GetGlobalPool().Dispatch(msgworker.MessageJob{
		InstanceID: ch.ID,
		ChatJID:    finalMsg.ChatID,
		Handler: func(workerCtx context.Context) error {
			if s.OnWaitIdle != nil {
				s.OnWaitIdle(workerCtx, ch.ID, finalMsg.ChatID)
			}

			if s.OnProcessFinal != nil {
				output, _ := s.OnProcessFinal(workerCtx, ch, finalMsg, botID)

				s.dbMu.Lock()
				if e, ok := s.dbEntries[key]; ok {
					if bCount, ok := output.Metadata["bubbles"].(string); ok {
						_, _ = fmt.Sscanf(bCount, "%d", &e.LastBubbleCount)
					}

					if len(e.Texts) > 0 {
						e.Msg.Metadata["is_delayed"] = true
						readingPause := time.Duration(0)
						if s.botEngine != nil {
							totalContent := strings.Join(e.Texts, "")
							readingPause = s.botEngine.Humanizer().CalculateReadingTime(totalContent)
						}
						e.State = StateDebouncing
						debounce := (time.Duration(globalConfig.AIDebounceMs) * time.Millisecond) + readingPause
						e.Timer = time.AfterFunc(debounce, func() {
							s.FlushDebounced(key, ch, botID, markRead)
						})
						logrus.Infof("[SessionOrchestrator] Re-queuing %s with reading pause of %s", key, readingPause)
					} else {
						e.State = StateWaiting
						e.ExpireAt = time.Now().Add(4 * time.Minute)
						if s.OnInactivityWarn != nil {
							e.WarningTimer = time.AfterFunc(3*time.Minute, func() {
								s.OnInactivityWarn(key, ch)
							})
						}
						e.Timer = time.AfterFunc(4*time.Minute, func() {
							s.dbMu.Lock()
							if curr, still := s.dbEntries[key]; still && curr == e && curr.State == StateWaiting {
								delete(s.dbEntries, key)
								if s.OnCleanupFiles != nil {
									s.OnCleanupFiles(e)
								}
								if s.OnChannelIdle != nil {
									go s.OnChannelIdle(e.Msg.ChannelID)
								}
							}
							s.dbMu.Unlock()
						})
						e.ChatOpen = false
					}
				}
				s.dbMu.Unlock()
			}
			return nil
		},
	})
}

package whatsapp

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/integrations/gemini"
	"github.com/AzielCF/az-wap/pkg/chatpresence"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/pkg/utils"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type debounceEntry struct {
	instanceID string
	chatJID    string
	phone      string
	lastEvt    *events.Message
	texts      []string
	timer      *time.Timer
}

type inflightEntry struct {
	cancel context.CancelFunc
	token  uint64
}

type aiReplyDebouncer struct {
	mu       sync.Mutex
	entries  map[string]*debounceEntry
	inflight map[string]inflightEntry
	seq      uint64
	flushFn  func(instanceID, chatJID, phone string, evt *events.Message, combinedText string)
}

func (d *aiReplyDebouncer) Enqueue(ctx context.Context, instanceID, chatJID, phone string, evt *events.Message) {
	_ = ctx
	instanceID = strings.TrimSpace(instanceID)
	chatJID = strings.TrimSpace(chatJID)
	if instanceID == "" || instanceID == "global" || chatJID == "" || evt == nil {
		return
	}
	if evt.Message == nil {
		return
	}

	hasImage := evt.Message.GetImageMessage() != nil
	hasAudio := evt.Message.GetAudioMessage() != nil

	text := strings.TrimSpace(utils.ExtractMessageTextFromProto(evt.Message))
	if text == "" && !hasImage && !hasAudio {
		return
	}

	key := instanceID + "|" + chatJID

	debounce := time.Duration(config.GeminiDebounceMs) * time.Millisecond // Uses GeminiDebounceMs for backward compatibility
	if debounce <= 0 || hasImage || hasAudio {
		d.mu.Lock()
		if prev, ok := d.inflight[key]; ok && prev.cancel != nil {
			prev.cancel()
			delete(d.inflight, key)
		}
		if e, ok := d.entries[key]; ok && e.timer != nil {
			e.timer.Stop()
			delete(d.entries, key)
		}
		d.mu.Unlock()

		combined := ""
		if !hasImage && !hasAudio {
			combined = text
		}
		d.flushFn(instanceID, chatJID, phone, evt, combined)
		return
	}

	d.mu.Lock()
	e := d.entries[key]
	if e == nil {
		e = &debounceEntry{instanceID: instanceID, chatJID: chatJID}
		d.entries[key] = e
	}
	e.instanceID = instanceID
	e.chatJID = chatJID
	if strings.TrimSpace(phone) != "" {
		e.phone = phone
	}
	e.lastEvt = evt
	e.texts = append(e.texts, text)
	if e.timer != nil {
		e.timer.Stop()
	}
	e.timer = time.AfterFunc(debounce, func() {
		d.flush(key)
	})
	d.mu.Unlock()
}

func (d *aiReplyDebouncer) flush(key string) {
	d.mu.Lock()
	e := d.entries[key]
	if e == nil {
		d.mu.Unlock()
		return
	}
	delete(d.entries, key)
	texts := append([]string(nil), e.texts...)
	evt := e.lastEvt
	instanceID := e.instanceID
	chatJID := e.chatJID
	phone := e.phone
	d.mu.Unlock()

	combined := strings.TrimSpace(strings.Join(texts, "\n"))
	if combined == "" || evt == nil {
		return
	}

	d.flushFn(instanceID, chatJID, phone, evt, combined)
}

var defaultAIReplyDebouncer = func() *aiReplyDebouncer {
	d := &aiReplyDebouncer{entries: map[string]*debounceEntry{}, inflight: map[string]inflightEntry{}}
	d.flushFn = func(instanceID, chatJID, phone string, evt *events.Message, combinedText string) {
		ctx := ContextWithInstanceID(context.Background(), instanceID)
		cli := getClientForContext(ctx)
		if cli == nil {
			return
		}

		evtCopy := *evt
		if strings.TrimSpace(combinedText) != "" {
			evtCopy.Message = &waE2E.Message{Conversation: proto.String(combinedText)}
		}

		run := func(baseCtx context.Context) {
			jobCtx, cancel := context.WithCancel(baseCtx)
			key := instanceID + "|" + chatJID

			d.mu.Lock()
			if prev, ok := d.inflight[key]; ok && prev.cancel != nil {
				prev.cancel()
			}
			d.seq++
			token := d.seq
			d.inflight[key] = inflightEntry{cancel: cancel, token: token}
			d.mu.Unlock()

			defer func() {
				cancel()
				d.mu.Lock()
				cur, ok := d.inflight[key]
				if ok && cur.token == token {
					delete(d.inflight, key)
				}
				d.mu.Unlock()
			}()

			waitIdle := time.Duration(config.GeminiWaitContactIdleMs) * time.Millisecond // Uses GeminiWaitContactIdleMs for backward compatibility
			if waitIdle > 0 {
				_ = chatpresence.WaitIdle(jobCtx, instanceID, chatJID, waitIdle)
				if jobCtx.Err() != nil {
					return
				}
			}

			gemini.HandleIncomingMessage(jobCtx, cli, instanceID, phone, &evtCopy)
		}

		if msgWorkerPool != nil {
			msgWorkerPool.Dispatch(msgworker.MessageJob{
				InstanceID: instanceID,
				ChatJID:    chatJID,
				Handler: func(workerCtx context.Context) error {
					run(workerCtx)
					return nil
				},
			})
			return
		}
		go run(ctx)
	}
	return d
}()

// enqueueAIReplyDebounced enqueues an AI reply with debouncing to batch multiple messages
func enqueueAIReplyDebounced(ctx context.Context, instanceID, phone string, evt *events.Message) {
	if evt == nil {
		return
	}
	chatJID := evt.Info.Chat.String()
	defaultAIReplyDebouncer.Enqueue(ctx, instanceID, chatJID, phone, evt)
}

package whatsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/botengine"
	"github.com/AzielCF/az-wap/botengine/domain/bot"
	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/chatpresence"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
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

			botID := GetInstanceBotID(jobCtx, instanceID)
			var b bot.Bot
			var err error
			if botID != "" && engine != nil {
				b, err = engine.GetBotUsecase().GetByID(jobCtx, botID)
			} else if engine != nil {
				b, err = engine.GetBotUsecase().GetByInstanceID(jobCtx, instanceID)
			}

			if err == nil && b.Enabled && engine != nil {
				// 1. Preparar BotInput
				input := botengine.BotInput{
					BotID:      b.ID,
					SenderID:   evtCopy.Info.Sender.String(),
					ChatID:     chatJID,
					Platform:   botengine.PlatformWhatsApp,
					Text:       utils.ExtractMessageTextFromProto(evtCopy.Message),
					InstanceID: instanceID,
					Metadata: map[string]any{
						"phone":    phone,
						"trace_id": evtCopy.Info.ID,
					},
				}

				// 2. Extraer Medios si existen (Imagen o Audio)
				var mediaObj *botengine.BotMedia
				if img := evtCopy.Message.GetImageMessage(); img != nil {
					storagePath := filepath.Join(config.PathCacheMedia, instanceID)
					utils.CreateFolder(storagePath)
					extracted, err := utils.ExtractMedia(jobCtx, cli, storagePath, img)
					if err == nil && extracted.MediaPath != "" {
						data, _ := os.ReadFile(extracted.MediaPath)
						mediaObj = &botengine.BotMedia{
							Data:     data,
							MimeType: extracted.MimeType,
							FileName: filepath.Base(extracted.MediaPath),
						}
						// En imágenes, el caption es el texto
						if input.Text == "" {
							input.Text = img.GetCaption()
						}
					}
				} else if audio := evtCopy.Message.GetAudioMessage(); audio != nil {
					storagePath := filepath.Join(config.PathCacheMedia, instanceID)
					utils.CreateFolder(storagePath)
					extracted, err := utils.ExtractMedia(jobCtx, cli, storagePath, audio)
					if err == nil && extracted.MediaPath != "" {
						data, _ := os.ReadFile(extracted.MediaPath)
						mediaObj = &botengine.BotMedia{
							Data:     data,
							MimeType: extracted.MimeType,
							FileName: filepath.Base(extracted.MediaPath),
						}
					}
				}
				input.Media = mediaObj

				// 3. Procesar si hay texto o medios
				if input.Text != "" || input.Media != nil {
					_, err := engine.Process(jobCtx, input)
					if err != nil {
						logrus.WithError(err).Error("[WHATSAPP] Bot Engine failed to process message")
					}
				}
			}
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

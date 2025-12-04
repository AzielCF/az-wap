package whatsapp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"

	"github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/pkg/chatmedia"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
)

// forwardMessageToWebhook is a helper function to forward message event to webhook url
func forwardMessageToWebhook(ctx context.Context, evt *events.Message) error {
	payload, err := createMessagePayload(ctx, evt)
	if err != nil {
		return err
	}

	return forwardPayloadToConfiguredWebhooks(ctx, payload, "message event")
}

func createMessagePayload(ctx context.Context, evt *events.Message) (map[string]any, error) {
	message := utils.BuildEventMessage(evt)
	waReaction := utils.BuildEventReaction(evt)
	forwarded := utils.BuildForwarded(evt)

	body := make(map[string]any)

	body["sender_id"] = evt.Info.Sender.User
	body["chat_id"] = evt.Info.Chat.User

	if from := evt.Info.SourceString(); from != "" {
		body["from"] = from

		from_user, from_group := from, ""
		if strings.Contains(from, " in ") {
			from_user = strings.Split(from, " in ")[0]
			from_group = strings.Split(from, " in ")[1]
		}

		if strings.HasSuffix(from_user, "@lid") {
			body["from_lid"] = from_user
			lid, err := types.ParseJID(from_user)
			if err != nil {
				logrus.Errorf("Error when parse jid: %v", err)
			} else {
				client := getClientForContext(ctx)
				if client == nil || client.Store == nil || client.Store.LIDs == nil {
					logrus.Warnf("LID store not available; skipping PN lookup for lid %s", lid.String())
				} else {
					pn, err := client.Store.LIDs.GetPNForLID(ctx, lid)
					if err != nil {
						logrus.Errorf("Error when get pn for lid %s: %v", lid.String(), err)
					}
					if !pn.IsEmpty() {
						if from_group != "" {
							body["from"] = fmt.Sprintf("%s in %s", pn.String(), from_group)
						} else {
							body["from"] = pn.String()
						}
					}
				}
			}
		}
	}
	if message.ID != "" {
		tags := regexp.MustCompile(`\B@\w+`).FindAllString(message.Text, -1)
		tagsMap := make(map[string]bool)
		for _, tag := range tags {
			tagsMap[tag] = true
		}
		for tag := range tagsMap {
			lid, err := types.ParseJID(tag[1:] + "@lid")
			if err != nil {
				logrus.Errorf("Error when parse jid: %v", err)
			} else {
				client := getClientForContext(ctx)
				if client == nil || client.Store == nil || client.Store.LIDs == nil {
					logrus.Warnf("LID store not available; skipping PN lookup for tag %s (lid %s)", tag, lid.String())
				} else {
					pn, err := client.Store.LIDs.GetPNForLID(ctx, lid)
					if err != nil {
						logrus.Errorf("Error when get pn for lid %s: %v", lid.String(), err)
					}
					if !pn.IsEmpty() {
						message.Text = strings.Replace(message.Text, tag, fmt.Sprintf("@%s", pn.User), -1)
					}
				}
			}
		}
		body["message"] = message
	}
	if pushname := evt.Info.PushName; pushname != "" {
		body["pushname"] = pushname
	}
	if waReaction.Message != "" {
		body["reaction"] = waReaction
	}
	if evt.IsViewOnce {
		body["view_once"] = evt.IsViewOnce
	}
	if forwarded {
		body["forwarded"] = forwarded
	}
	if timestamp := evt.Info.Timestamp.Format(time.RFC3339); timestamp != "" {
		body["timestamp"] = timestamp
	}

	// Handle protocol messages (revoke, etc.)
	if protocolMessage := evt.Message.GetProtocolMessage(); protocolMessage != nil {
		protocolType := protocolMessage.GetType().String()

		switch protocolType {
		case "REVOKE":
			body["action"] = "message_revoked"
			if key := protocolMessage.GetKey(); key != nil {
				body["revoked_message_id"] = key.GetID()
				body["revoked_from_me"] = key.GetFromMe()
				if key.GetRemoteJID() != "" {
					body["revoked_chat"] = key.GetRemoteJID()
				}
			}
		case "MESSAGE_EDIT":
			body["action"] = "message_edited"
			// Extract the original message ID from the protocol message key
			if key := protocolMessage.GetKey(); key != nil {
				body["original_message_id"] = key.GetID()
			}
			if editedMessage := protocolMessage.GetEditedMessage(); editedMessage != nil {
				if editedText := editedMessage.GetExtendedTextMessage(); editedText != nil {
					body["edited_text"] = editedText.GetText()
				} else if editedConv := editedMessage.GetConversation(); editedConv != "" {
					body["edited_text"] = editedConv
				}
			}
		}
	}

	if audioMedia := evt.Message.GetAudioMessage(); audioMedia != nil {
		if config.WhatsappAutoDownloadMedia {
			client := getClientForContext(ctx)
			if client != nil {
				media, err := utils.ExtractMedia(ctx, client, config.PathMedia, audioMedia)
				if err != nil {
					logrus.Errorf("Failed to download audio from %s: %v", evt.Info.SourceString(), err)
				} else {
					body["audio"] = media
					chatmedia.Add(evt.Info.ID, chatmedia.Item{
						Kind:     "audio",
						Path:     media.MediaPath,
						MimeType: media.MimeType,
						Caption:  media.Caption,
					})
				}
			}
			if _, ok := body["audio"]; !ok {
				body["audio"] = map[string]any{
					"url": audioMedia.GetURL(),
				}
			}
		} else {
			body["audio"] = map[string]any{
				"url": audioMedia.GetURL(),
			}
		}
	}

	if contactMessage := evt.Message.GetContactMessage(); contactMessage != nil {
		body["contact"] = contactMessage
	}

	if documentMedia := evt.Message.GetDocumentMessage(); documentMedia != nil {
		if config.WhatsappAutoDownloadMedia {
			client := getClientForContext(ctx)
			if client != nil {
				media, err := utils.ExtractMedia(ctx, client, config.PathMedia, documentMedia)
				if err != nil {
					logrus.Errorf("Failed to download document from %s: %v", evt.Info.SourceString(), err)
				} else {
					body["document"] = media
					chatmedia.Add(evt.Info.ID, chatmedia.Item{
						Kind:     "document",
						Path:     media.MediaPath,
						MimeType: media.MimeType,
						Caption:  media.Caption,
					})
				}
			}
			if _, ok := body["document"]; !ok {
				body["document"] = map[string]any{
					"url":      documentMedia.GetURL(),
					"filename": documentMedia.GetFileName(),
				}
			}
		} else {
			body["document"] = map[string]any{
				"url":      documentMedia.GetURL(),
				"filename": documentMedia.GetFileName(),
			}
		}
	}

	if imageMedia := evt.Message.GetImageMessage(); imageMedia != nil {
		if config.WhatsappAutoDownloadMedia {
			client := getClientForContext(ctx)
			if client != nil {
				media, err := utils.ExtractMedia(ctx, client, config.PathMedia, imageMedia)
				if err != nil {
					logrus.Errorf("Failed to download image from %s: %v", evt.Info.SourceString(), err)
				} else {
					body["image"] = media
					chatmedia.Add(evt.Info.ID, chatmedia.Item{
						Kind:     "image",
						Path:     media.MediaPath,
						MimeType: media.MimeType,
						Caption:  media.Caption,
					})
				}
			}
			if _, ok := body["image"]; !ok {
				body["image"] = map[string]any{
					"url":     imageMedia.GetURL(),
					"caption": imageMedia.GetCaption(),
				}
			}
		} else {
			body["image"] = map[string]any{
				"url":     imageMedia.GetURL(),
				"caption": imageMedia.GetCaption(),
			}
		}
	}

	if listMessage := evt.Message.GetListMessage(); listMessage != nil {
		body["list"] = listMessage
	}

	if liveLocationMessage := evt.Message.GetLiveLocationMessage(); liveLocationMessage != nil {
		body["live_location"] = liveLocationMessage
	}

	if locationMessage := evt.Message.GetLocationMessage(); locationMessage != nil {
		body["location"] = locationMessage
	}

	if orderMessage := evt.Message.GetOrderMessage(); orderMessage != nil {
		body["order"] = orderMessage
	}

	if stickerMedia := evt.Message.GetStickerMessage(); stickerMedia != nil {
		if config.WhatsappAutoDownloadMedia {
			client := getClientForContext(ctx)
			if client != nil {
				media, err := utils.ExtractMedia(ctx, client, config.PathMedia, stickerMedia)
				if err != nil {
					logrus.Errorf("Failed to download sticker from %s: %v", evt.Info.SourceString(), err)
				} else {
					body["sticker"] = media
					chatmedia.Add(evt.Info.ID, chatmedia.Item{
						Kind:     "sticker",
						Path:     media.MediaPath,
						MimeType: media.MimeType,
						Caption:  media.Caption,
					})
				}
			}
			if _, ok := body["sticker"]; !ok {
				body["sticker"] = map[string]any{
					"url": stickerMedia.GetURL(),
				}
			}
		} else {
			body["sticker"] = map[string]any{
				"url": stickerMedia.GetURL(),
			}
		}
	}

	if videoMedia := evt.Message.GetVideoMessage(); videoMedia != nil {
		if config.WhatsappAutoDownloadMedia {
			client := getClientForContext(ctx)
			if client != nil {
				media, err := utils.ExtractMedia(ctx, client, config.PathMedia, videoMedia)
				if err != nil {
					logrus.Errorf("Failed to download video from %s: %v", evt.Info.SourceString(), err)
				} else {
					body["video"] = media
					chatmedia.Add(evt.Info.ID, chatmedia.Item{
						Kind:     "video",
						Path:     media.MediaPath,
						MimeType: media.MimeType,
						Caption:  media.Caption,
					})
				}
			}
			if _, ok := body["video"]; !ok {
				body["video"] = map[string]any{
					"url":     videoMedia.GetURL(),
					"caption": videoMedia.GetCaption(),
				}
			}
		} else {
			body["video"] = map[string]any{
				"url":     videoMedia.GetURL(),
				"caption": videoMedia.GetCaption(),
			}
		}
	}

	return body, nil
}

// NormalizePhoneForChatwoot devuelve el número en formato +<digits> usando la
// misma lógica que el webhook: toma el campo final "from" (ya resuelto vía
// LID store cuando está disponible) y aplica
//
//	from.split('@')[0].split(':')[0].replace(/[^0-9]/g, '')
//
// para extraer solo los dígitos.
func NormalizePhoneForChatwoot(ctx context.Context, evt *events.Message) string {
	if evt == nil {
		return ""
	}

	payload, err := createMessagePayload(ctx, evt)
	if err != nil {
		logrus.WithError(err).Warn("[CHATWOOT] failed to build payload for phone normalization")
		return ""
	}

	rawFrom, _ := payload["from"].(string)
	from := strings.TrimSpace(rawFrom)
	if from == "" {
		return ""
	}

	// from.split('@')[0]
	candidate := from
	if idx := strings.Index(candidate, "@"); idx >= 0 {
		candidate = candidate[:idx]
	}

	// .split(':')[0]
	if idx := strings.Index(candidate, ":"); idx >= 0 {
		candidate = candidate[:idx]
	}

	reDigits := regexp.MustCompile("[^0-9]+")
	digits := reDigits.ReplaceAllString(candidate, "")
	logrus.Debugf("[CHATWOOT] normalize phone from payload: from=%q candidate=%q digits=%q", from, candidate, digits)
	if digits == "" {
		return ""
	}

	return "+" + digits
}

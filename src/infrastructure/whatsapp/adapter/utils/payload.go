package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	pkgUtils "github.com/AzielCF/az-wap/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// NOTE: This is migrated from infrastructure/whatsapp/event_message.go
// but adapted to not rely on global getClient()

// CreateMessagePayload builds the JSON payload for a message event
func CreateMessagePayload(ctx context.Context, evt *events.Message, client *whatsmeow.Client, workspaceID, channelID string, maxSize int64) (map[string]any, error) {
	message := pkgUtils.BuildEventMessage(evt)
	waReaction := pkgUtils.BuildEventReaction(evt)
	forwarded := pkgUtils.BuildForwarded(evt)

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

	if contactMessage := evt.Message.GetContactMessage(); contactMessage != nil {
		body["contact"] = contactMessage
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

	return body, nil
}

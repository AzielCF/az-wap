package adapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendMessage sends a text message
func (wa *WhatsAppAdapter) SendMessage(ctx context.Context, chatID, text, quoteMessageID string) (common.SendResponse, error) {
	if wa.client == nil {
		return common.SendResponse{}, fmt.Errorf("no client")
	}

	jid, err := types.ParseJID(chatID)
	if err != nil {
		return common.SendResponse{}, fmt.Errorf("invalid JID: %w", err)
	}

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text),
		},
	}

	// If we have a quote ID, we add ContextInfo
	if quoteMessageID != "" {
		msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{
			StanzaID:      proto.String(quoteMessageID),
			Participant:   proto.String(jid.String()),
			QuotedMessage: &waE2E.Message{Conversation: proto.String("")}, // Minimal quote
		}
	}

	resp, err := wa.client.SendMessage(ctx, jid, msg)
	if err != nil {
		return common.SendResponse{}, err
	}

	return common.SendResponse{
		MessageID: resp.ID,
		Timestamp: resp.Timestamp,
	}, nil
}

// SendMedia sends a media message
func (wa *WhatsAppAdapter) SendMedia(ctx context.Context, chatID string, media common.MediaUpload, quoteMessageID string) (common.SendResponse, error) {
	if wa.client == nil {
		return common.SendResponse{}, fmt.Errorf("no client")
	}

	jid, err := types.ParseJID(chatID)
	if err != nil {
		return common.SendResponse{}, fmt.Errorf("invalid JID: %w", err)
	}

	// Determine media type based on mimetype
	var mType whatsmeow.MediaType
	var uploaded whatsmeow.UploadResponse

	// Pre-determine whatsmeow type based on domain type or mimetype
	switch media.Type {
	case common.MediaTypeImage, common.MediaTypeSticker:
		mType = whatsmeow.MediaImage
	case common.MediaTypeVideo:
		mType = whatsmeow.MediaVideo
	case common.MediaTypeAudio:
		mType = whatsmeow.MediaAudio
	default:
		mType = whatsmeow.MediaDocument
	}

	// Double check mimetype prefix if domain type is generic or missing (fallback)
	if media.Type == "" {
		switch {
		case strings.HasPrefix(media.MimeType, "image/"):
			mType = whatsmeow.MediaImage
		case strings.HasPrefix(media.MimeType, "video/"):
			mType = whatsmeow.MediaVideo
		case strings.HasPrefix(media.MimeType, "audio/"):
			mType = whatsmeow.MediaAudio
		default:
			mType = whatsmeow.MediaDocument
		}
	}

	uploaded, err = wa.client.Upload(ctx, media.Data, mType)
	if err != nil {
		return common.SendResponse{}, fmt.Errorf("failed to upload media: %w", err)
	}

	msg := waE2E.Message{}

	switch media.Type {
	case common.MediaTypeImage:
		msg.ImageMessage = &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(media.MimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Caption:       proto.String(media.Caption),
			ViewOnce:      proto.Bool(media.ViewOnce),
		}
	case common.MediaTypeVideo:
		msg.VideoMessage = &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(media.MimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Caption:       proto.String(media.Caption),
			ViewOnce:      proto.Bool(media.ViewOnce),
		}
	case common.MediaTypeAudio:
		msg.AudioMessage = &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(media.MimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			PTT:           proto.Bool(media.PTT),
		}
	case common.MediaTypeSticker:
		msg.StickerMessage = &waE2E.StickerMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(media.MimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
		}
	default:
		msg.DocumentMessage = &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(media.MimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Caption:       proto.String(media.Caption),
			FileName:      proto.String(media.FileName),
		}
	}

	// Add context info if quoting
	if quoteMessageID != "" {
		ctxInfo := &waE2E.ContextInfo{
			StanzaID:    proto.String(quoteMessageID),
			Participant: proto.String(jid.String()),
		}
		if msg.ImageMessage != nil {
			msg.ImageMessage.ContextInfo = ctxInfo
		} else if msg.VideoMessage != nil {
			msg.VideoMessage.ContextInfo = ctxInfo
		} else if msg.AudioMessage != nil {
			msg.AudioMessage.ContextInfo = ctxInfo
		} else if msg.DocumentMessage != nil {
			msg.DocumentMessage.ContextInfo = ctxInfo
		}
	}
	resp, err := wa.client.SendMessage(ctx, jid, &msg)
	if err != nil {
		return common.SendResponse{}, err
	}

	return common.SendResponse{
		MessageID: resp.ID,
		Timestamp: resp.Timestamp,
	}, nil
}

func (wa *WhatsAppAdapter) SendPresence(ctx context.Context, chatID string, typing bool, isAudio bool) error {
	if wa.client == nil {
		return fmt.Errorf("no client")
	}
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return err
	}

	presence := types.ChatPresenceComposing
	if !typing {
		presence = types.ChatPresencePaused
	}

	media := types.ChatPresenceMediaText
	if isAudio {
		media = types.ChatPresenceMediaAudio
	}

	logrus.Debugf("[WHATSAPP] Sending chat presence (Typing: %v, Audio: %v) to %s", typing, isAudio, chatID)
	return wa.client.SendChatPresence(ctx, jid, presence, media)
}

func (wa *WhatsAppAdapter) SendContact(ctx context.Context, chatID, contactName, contactPhone string, quoteMessageID string) (common.SendResponse, error) {
	// ... implementation ...
	return common.SendResponse{}, nil
}

func (wa *WhatsAppAdapter) SendLocation(ctx context.Context, chatID string, lat, long float64, address string, quoteMessageID string) (common.SendResponse, error) {
	// ... implementation ...
	return common.SendResponse{}, nil
}

func (wa *WhatsAppAdapter) SendPoll(ctx context.Context, chatID, question string, options []string, maxSelections int, quoteMessageID string) (common.SendResponse, error) {
	// ... implementation ...
	return common.SendResponse{}, nil
}

func (wa *WhatsAppAdapter) SendLink(ctx context.Context, chatID, link, caption, title, description string, thumbnail []byte, quoteMessageID string) (common.SendResponse, error) {
	// ... implementation ...
	return common.SendResponse{}, nil
}

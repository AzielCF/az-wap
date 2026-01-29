package adapter

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/AzielCF/az-wap/workspace/domain/common"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendNewsletterMessage sends a message (text or media) to a newsletter
func (wa *WhatsAppAdapter) SendNewsletterMessage(ctx context.Context, newsletterID, text string, mediaPath string) (common.SendResponse, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return common.SendResponse{}, err
	}
	cli := wa.client
	if cli == nil {
		return common.SendResponse{}, fmt.Errorf("client not initialized")
	}

	jid, err := types.ParseJID(newsletterID)
	if err != nil {
		return common.SendResponse{}, err
	}

	var msg *waE2E.Message

	if mediaPath != "" {
		// Handle media
		data, err := os.ReadFile(mediaPath)
		if err != nil {
			return common.SendResponse{}, fmt.Errorf("failed to read media file: %w", err)
		}

		// Detect MimeType
		mimeType := http.DetectContentType(data)

		uploaded, err := cli.Upload(ctx, data, whatsmeow.MediaImage) // Defaulting to Image

		if err != nil {
			return common.SendResponse{}, fmt.Errorf("media upload failed: %w", err)
		}

		msg = &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:       proto.String(text),
				Mimetype:      proto.String(mimeType),
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(data))),
			},
		}
	} else {
		// Text only
		msg = &waE2E.Message{
			Conversation: proto.String(text),
		}
	}

	resp, err := cli.SendMessage(ctx, jid, msg)
	if err != nil {
		return common.SendResponse{}, err
	}

	return common.SendResponse{
		MessageID: resp.ID,
		Timestamp: resp.Timestamp,
	}, nil
}

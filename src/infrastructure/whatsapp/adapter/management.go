package adapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Session Management

func (wa *WhatsAppAdapter) GetQRChannel(ctx context.Context) (<-chan string, error) {
	wa.subsMu.Lock()
	defer wa.subsMu.Unlock()

	ch := make(chan string, 1)
	wa.qrSubs = append(wa.qrSubs, ch)

	// If already have a QR, send it immediately
	wa.qrMu.RLock()
	if wa.currentQR != "" {
		ch <- wa.currentQR
	}
	wa.qrMu.RUnlock()

	return ch, nil
}

func (wa *WhatsAppAdapter) Login(ctx context.Context) error {
	wa.connMu.Lock()
	defer wa.connMu.Unlock()

	if wa.client == nil {
		return fmt.Errorf("client not started")
	}
	if wa.client.IsConnected() && !wa.client.IsLoggedIn() {
		logrus.Info("[WHATSAPP] Client already connected but not logged in, disconnecting to refresh state...")
		wa.client.Disconnect()
	}
	return wa.client.Connect()
}

func (wa *WhatsAppAdapter) LoginWithCode(ctx context.Context, phone string) (string, error) {
	wa.connMu.Lock()
	defer wa.connMu.Unlock()

	if wa.client == nil {
		return "", fmt.Errorf("client not initialized")
	}

	// Ensure connected and clean state
	if wa.client.IsConnected() && !wa.client.IsLoggedIn() {
		logrus.Info("[WHATSAPP] Client already connected but not logged in, refreshing connection for pairing code...")
		wa.client.Disconnect()
	}

	if !wa.client.IsConnected() {
		if err := wa.client.Connect(); err != nil {
			return "", fmt.Errorf("failed to connect: %w", err)
		}
	}

	if wa.client.IsLoggedIn() {
		return "", fmt.Errorf("already logged in")
	}

	// Request pairing code
	logrus.WithFields(logrus.Fields{
		"phone":   phone,
		"length":  len(phone),
		"channel": wa.channelID,
	}).Info("[WHATSAPP] Requesting pairing code...")

	code, err := wa.client.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"phone":   phone,
			"channel": wa.channelID,
			"error":   err.Error(),
		}).Error("[WHATSAPP] Pairing failed (server error)")
		return "", fmt.Errorf("pairing failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"phone":   phone,
		"channel": wa.channelID,
		"code":    code,
	}).Info("[WHATSAPP] Pairing code generated successfully")

	return code, nil
}

func (wa *WhatsAppAdapter) Logout(ctx context.Context) error {
	wa.connMu.Lock()
	defer wa.connMu.Unlock()

	if wa.client == nil {
		return nil
	}

	// Try logout from WA servers
	err := wa.client.Logout(ctx)

	// Regardless of WA server logout, we disconnect and cleanup locally
	wa.client.Disconnect()

	if err != nil {
		logrus.Warnf("[WHATSAPP] Logout error (internal): %v", err)
		// We return nil if it was already logged out to not break UI flow
		if strings.Contains(err.Error(), "not logged in") || strings.Contains(err.Error(), "401") {
			return nil
		}
		return err
	}
	return nil
}

// Message Management

func (wa *WhatsAppAdapter) MarkRead(ctx context.Context, chatID string, messageIDs []string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return err
	}
	ids := make([]types.MessageID, len(messageIDs))
	for i, id := range messageIDs {
		ids[i] = types.MessageID(id)
	}
	return cli.MarkRead(ctx, ids, time.Now(), jid, *cli.Store.ID)
}

func (wa *WhatsAppAdapter) ReactMessage(ctx context.Context, chatID, messageID, emoji string) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return "", err
	}

	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				FromMe:    proto.Bool(true),
				ID:        proto.String(messageID),
				RemoteJID: proto.String(jid.String()),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}
	resp, err := cli.SendMessage(ctx, jid, msg)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (wa *WhatsAppAdapter) RevokeMessage(ctx context.Context, chatID, messageID string) (string, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return "", err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return "", err
	}
	resp, err := cli.SendMessage(ctx, jid, cli.BuildRevoke(jid, types.EmptyJID, messageID))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (wa *WhatsAppAdapter) DeleteMessageForMe(ctx context.Context, chatID, messageID string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return err
	}

	isFromMe := "1"
	if len(messageID) > 22 {
		isFromMe = "0"
	}

	patchInfo := appstate.PatchInfo{
		Timestamp: time.Now(),
		Type:      appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{{
			Index: []string{appstate.IndexDeleteMessageForMe, jid.String(), messageID, isFromMe, cli.Store.ID.String()},
			Value: &waSyncAction.SyncActionValue{
				DeleteMessageForMeAction: &waSyncAction.DeleteMessageForMeAction{
					DeleteMedia:      proto.Bool(true),
					MessageTimestamp: proto.Int64(time.Now().UnixMilli()),
				},
			},
		}},
	}
	return cli.SendAppState(ctx, patchInfo)
}

func (wa *WhatsAppAdapter) StarMessage(ctx context.Context, chatID, messageID string, starred bool) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return err
	}

	isFromMe := true
	if len(messageID) > 22 {
		isFromMe = false
	}

	patchInfo := appstate.BuildStar(jid.ToNonAD(), *cli.Store.ID, messageID, isFromMe, starred)
	return cli.SendAppState(ctx, patchInfo)
}

func (wa *WhatsAppAdapter) DownloadMedia(ctx context.Context, messageID, chatID string) (string, error) {
	return "", fmt.Errorf("DownloadMedia is not supported as persistent chat storage is disabled")
}

func (wa *WhatsAppAdapter) PinChat(ctx context.Context, chatID string, pinned bool) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	jid, err := wa.parseJID(chatID)
	if err != nil {
		return err
	}
	patchInfo := appstate.BuildPin(jid, pinned)
	return cli.SendAppState(ctx, patchInfo)
}

func (wa *WhatsAppAdapter) FetchNewsletters(ctx context.Context) ([]common.NewsletterInfo, error) {
	if err := wa.ensureConnected(ctx); err != nil {
		return nil, err
	}
	cli := wa.client
	newsletters, err := cli.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]common.NewsletterInfo, 0, len(newsletters))
	for _, nl := range newsletters {
		result = append(result, common.NewsletterInfo{
			ID:          nl.ID.String(),
			Name:        nl.ThreadMeta.Name.Text,
			Description: nl.ThreadMeta.Description.Text,
			Subscribers: int(nl.ThreadMeta.SubscriberCount),
			Role:        string(nl.ViewerMeta.Role),
			// Use Role as Subscription status for now as specific field name is elusive
			Subscription: string(nl.ViewerMeta.Role),
			CreatedAt:    nl.ThreadMeta.CreationTime.Time,
		})
	}
	return result, nil
}

func (wa *WhatsAppAdapter) UnfollowNewsletter(ctx context.Context, jid string) error {
	if err := wa.ensureConnected(ctx); err != nil {
		return err
	}
	cli := wa.client
	targetJID, err := wa.parseJID(jid)
	if err != nil {
		return err
	}
	return cli.UnfollowNewsletter(ctx, targetJID)
}

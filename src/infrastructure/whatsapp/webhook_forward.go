package whatsapp

import (
	"context"
	"fmt"
	"strings"

	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/sirupsen/logrus"
)

var submitWebhookFn = submitWebhook

// forwardPayloadToConfiguredWebhooks attempts to deliver the provided payload to every configured webhook URL.
// It only returns an error when all webhook deliveries fail. Partial failures are logged and suppressed so
// successful targets still receive the event.
func forwardPayloadToConfiguredWebhooks(ctx context.Context, payload map[string]any, eventName string) error {
	instanceID := GetInstanceIDFromContext(ctx)
	cfg := getWebhookConfigForContext(ctx)
	total := len(cfg.URLs)
	logrus.WithFields(logrus.Fields{
		"event":       eventName,
		"instance_id": instanceID,
		"webhooks":    total,
		"urls":        cfg.URLs,
	}).Info("[WEBHOOK] Forwarding event to configured webhook(s)")

	if total == 0 {
		logrus.WithFields(logrus.Fields{
			"event":       eventName,
			"instance_id": instanceID,
		}).Info("[WEBHOOK] No webhook configured; skipping dispatch")
		return nil
	}

	var (
		failed    []string
		successes int
	)
	for _, url := range cfg.URLs {
		if err := submitWebhookFn(ctx, payload, url); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", url, err))
			logrus.Warnf("Failed forwarding %s to %s: %v", eventName, url, err)
			continue
		}
		successes++
	}

	if len(failed) == total {
		return pkgError.WebhookError(fmt.Sprintf("all webhook URLs failed for %s: %s", eventName, strings.Join(failed, "; ")))
	}

	if len(failed) > 0 {
		logrus.Warnf("Some webhook URLs failed for %s (succeeded: %d/%d): %s", eventName, successes, total, strings.Join(failed, "; "))
	} else {
		logrus.Infof("%s forwarded to all webhook(s)", eventName)
	}

	return nil
}

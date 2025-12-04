package cmd

import (
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	"github.com/AzielCF/az-wap/ui/rest/helpers"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

// getValidWhatsAppClient returns an initialized WhatsApp client if available.
func getValidWhatsAppClient() *whatsmeow.Client {
	client := whatsappCli
	if client == nil {
		client = whatsapp.GetClient()
	}
	return client
}

// startAutoReconnectCheckerIfClientAvailable guards the reconnect checker behind a valid client reference.
func startAutoReconnectCheckerIfClientAvailable() {
	client := getValidWhatsAppClient()
	if client == nil {
		logrus.Warn("whatsapp client is nil; auto-reconnect checker not started")
		return
	}
	go helpers.SetAutoReconnectChecking(client)
}

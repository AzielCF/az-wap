package onlyclients

import "github.com/AzielCF/az-wap/botengine/domain"

// IsClientRegistered checks if the current interaction is with a registered client.
// It is used as a visibility condition for native tools that require
// an active client profile.
func IsClientRegistered(input domain.BotInput) bool {
	return input.ClientContext != nil && input.ClientContext.IsRegistered
}

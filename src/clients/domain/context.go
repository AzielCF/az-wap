package domain

import "fmt"

// ClientContext representa toda la info resuelta para un mensaje entrante
type ClientContext struct {
	// Cliente global (nil si no está registrado)
	Client *Client `json:"client,omitempty"`

	// Suscripción activa al canal actual (nil si no tiene)
	Subscription *ClientSubscription `json:"subscription,omitempty"`

	// Bot ID a usar (resuelto)
	ResolvedBotID string `json:"resolved_bot_id"`

	// System prompt adicional (si hay override)
	AdditionalPrompt string `json:"additional_prompt,omitempty"`

	// Flags de conveniencia
	IsRegistered    bool `json:"is_registered"`    // ¿Existe en clients?
	HasSubscription bool `json:"has_subscription"` // ¿Tiene suscripción activa?
	IsVIP           bool `json:"is_vip"`           // ¿Es VIP o superior?
	IsPremium       bool `json:"is_premium"`       // ¿Es Premium o superior?
}

// ForPrompt genera texto para inyectar en el system prompt del bot
func (ctx *ClientContext) ForPrompt() string {
	if !ctx.IsRegistered || ctx.Client == nil {
		return ""
	}

	tierInfo := fmt.Sprintf("El usuario que te habla es un cliente registrado (Tier: %s).", ctx.Client.Tier)

	if ctx.IsVIP {
		tierInfo += " Tratalo con atención prioritaria y personalizada."
	}

	if ctx.Client.DisplayName != "" {
		tierInfo += fmt.Sprintf(" Su nombre es: %s.", ctx.Client.DisplayName)
	}

	if len(ctx.Client.Tags) > 0 {
		tierInfo += fmt.Sprintf(" Tags: %v.", ctx.Client.Tags)
	}

	return tierInfo
}

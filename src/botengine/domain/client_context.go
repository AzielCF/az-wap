package domain

import "strings"

// ClientContext representa el contexto de un cliente resuelto para el bot.
// Esta es una estructura simplificada que se pasa al BotInput.
type ClientContext struct {
	// Cliente global
	ClientID    string         `json:"client_id,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Email       string         `json:"email,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Tier        string         `json:"tier,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Language    string         `json:"language,omitempty"`
	Timezone    string         `json:"timezone,omitempty"`
	Country     string         `json:"country,omitempty"`
	AllowedBots []string       `json:"allowed_bots,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`

	// SocialName es el nombre que el usuario ha compartido voluntariamente con la IA
	// y se almacena en metadata["name"]. Distinto del DisplayName del sistema.
	SocialName string `json:"social_name,omitempty"`
	PushName   string `json:"push_name,omitempty"`

	// Flags de conveniencia
	IsRegistered    bool `json:"is_registered"`
	HasSubscription bool `json:"has_subscription"`
	IsVIP           bool `json:"is_vip"`
	IsPremium       bool `json:"is_premium"`

	// Override de suscripción
	CustomSystemPrompt string `json:"custom_system_prompt,omitempty"`
}

// ForPrompt generates text to inject into the bot's system prompt.
// PRIVACY: By default, the AI does NOT see the client's name, email, or phone.
// The client can share this information voluntarily, and it will be stored in metadata.
// Use the get_my_info tool to retrieve full client details when the client requests it.
func (ctx *ClientContext) ForPrompt() string {
	if ctx == nil || !ctx.IsRegistered {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("CLIENT STATUS:\n")
	sb.WriteString("- You are talking to a REGISTERED client (they have an account in the system).\n")
	sb.WriteString("- Tier: " + ctx.Tier + "\n")

	// PRIVACY: We do NOT expose DisplayName, Email, or Phone by default.
	// The AI should use get_my_info tool if the client asks about their data.
	// We check if the client has provided a SOCIAL NAME in metadata.
	if ctx.SocialName == "" {
		sb.WriteString("\n!!! CRITICAL ACTION REQUIRED !!!\n")
		sb.WriteString("This is a NEW or UNNAMED registered client. You DO NOT know their name yet.\n")
		sb.WriteString("YOUR PRIMARY GOAL RIGHT NOW is to politely ask for their name to welcome them properly.\n")
		sb.WriteString("Example: 'Hello! I see you are a registered member, but I don't have your name yet. How should I call you?'\n")
		sb.WriteString("Once they reply with their name, you MUST IMMEDIATELY use the 'update_my_info' tool with the 'name' field to save it.\n")
	} else {
		// Only provide the first name or a greeting-friendly version
		sb.WriteString("- The client has a name on file (" + ctx.SocialName + "). You may address them by this name.\n")
	}

	if len(ctx.Tags) > 0 {
		sb.WriteString("- Tags: " + strings.Join(ctx.Tags, ", ") + "\n")
	}

	if ctx.Language != "" {
		sb.WriteString("- Preferred Language: " + ctx.Language + " (MANDATORY: Prioritize this language over any other general instruction)\n")
	}

	if ctx.IsVIP {
		sb.WriteString("- NOTE: This client is VIP. Prioritize their requests and maintain a high level of exclusivity.\n")
	}

	if ctx.IsPremium && !ctx.IsVIP {
		sb.WriteString("- This client has PREMIUM status.\n")
	}

	if ctx.HasSubscription {
		sb.WriteString("- The client has an active subscription for this specific channel.\n")
	}

	sb.WriteString("- TOOLS AVAILABLE: You have 'update_my_info', 'get_my_info', and 'delete_my_field' tools to help the client manage their personal information.\n")

	return sb.String()
}

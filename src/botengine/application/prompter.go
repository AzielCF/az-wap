package application

import (
	"fmt"
	"strings"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	coreconfig "github.com/AzielCF/az-wap/core/config"
)

// Prompter se encarga de ensamblar las instrucciones del sistema (System Prompt)
type Prompter struct{}

func NewPrompter() *Prompter {
	return &Prompter{}
}

// BuildSystemInstructions consolida todas las fuentes de prompts del sistema (Combinado)
func (p *Prompter) BuildSystemInstructions(b domainBot.Bot, input domain.BotInput, mcpInstructions string) string {
	stable, dynamic := p.BuildInstructionsSplit(b, input, mcpInstructions)
	return stable + "\n\n" + dynamic
}

// BuildInstructionsSplit separa las instrucciones en un bloque estable (cacheable) y uno dinámico.
func (p *Prompter) BuildInstructionsSplit(b domainBot.Bot, input domain.BotInput, mcpInstructions string) (string, string) {
	var stable strings.Builder
	var dynamic strings.Builder

	// --- BLOQUE ESTABLE (Cacheable) ---

	// 1. Global Prompt
	if coreconfig.Global.AI.GlobalSystemPrompt != "" {
		stable.WriteString(coreconfig.Global.AI.GlobalSystemPrompt)
		stable.WriteString("\n\n")
	}

	// 2. Bot Specific Prompt
	if b.SystemPrompt != "" {
		stable.WriteString(b.SystemPrompt)
		stable.WriteString("\n\n")
	}

	// 3. Knowledge Base
	if b.KnowledgeBase != "" {
		stable.WriteString(b.KnowledgeBase)
		stable.WriteString("\n\n")
	}

	// 3.5 Client Specific Instructions (Long term)
	if input.ClientContext != nil && input.ClientContext.IsRegistered {
		if input.ClientContext.CustomSystemPrompt != "" {
			stable.WriteString("### CLIENT-SPECIFIC INSTRUCTIONS\n")
			stable.WriteString(input.ClientContext.CustomSystemPrompt)
			stable.WriteString("\n\n")
		}
	}

	// 4. Resource Management Capabilities
	hasAnyMedia := b.ImageEnabled || b.AudioEnabled || b.VideoEnabled || b.DocumentEnabled
	if hasAnyMedia {
		stable.WriteString("### RESOURCE MANAGEMENT CAPABILITIES\n")
		stable.WriteString("You operate as a 'Resource Concierge'. To save tokens, only small items are sent instantly.\n")

		if b.ImageEnabled {
			stable.WriteString("- VISUALS: You can see images and stickers instantly.\n")
			stable.WriteString("  * STICKERS: Interpretation is based on conversation context. A 'sad' sticker can be ironic if the user is joking. Prioritize conversational flow over literal image description.\n")
		}
		if b.AudioEnabled {
			stable.WriteString("- AUDIO: You receive transcriptions of audio/voice notes automatically.\n")
		}

		if b.VideoEnabled || b.DocumentEnabled {
			stable.WriteString("- DEFERRED ACCESS: For Videos and Documents, you might see markers like [RECURSO DISPONIBLE].\n")
			stable.WriteString("  * Use 'get_session_resources' to list files or 'analyze_session_resource' to read them if the user asks.\n")
		}
		stable.WriteString("- UNSUPPORTED: For formats like ZIP or PSD, acknowledge them as general reference for tools.\n")
		stable.WriteString("BEHAVIOR: Use multimodal context ([Audio Transcription], [Image Description]) PROACTIVELY.\n\n")
	}

	// 4.5 FINANCIAL PROTOCOL
	stable.WriteString("### FINANCIAL PROTOCOL\n")
	stable.WriteString("Standard Procedure for Currency Inquiries:\n")
	stable.WriteString("1. Always use 'get_exchange_rate' for currency conversion.\n")
	stable.WriteString("2. If the tool fails, state that real-time rates are unavailable.\n")
	stable.WriteString("3. Avoid estimating values based on training data.\n")
	stable.WriteString("4. SMART INFERENCE: If Client_Country is set and the user asks about 'exchange rate' or 'tipo de cambio' WITHOUT specifying currencies, automatically assume they want USD to their local currency (e.g. DO=DOP, PE=PEN, MX=MXN). Do NOT ask for clarification in this case.\n\n")

	// 5. Situational Behavior (Rules)
	stable.WriteString("### SITUATIONAL BEHAVIOR (MINDSET)\n")
	stable.WriteString("You must ALWAYS start your response with a HIDDEN internal mindset tag: <mindset pace=\"fast|steady|deep\" focus=\"true|false\" work=\"true|false\" />\n\n")

	// 6. Toolset Guidelines (MCP)
	if mcpInstructions != "" {
		stable.WriteString("## MCP TOOL GUIDELINES")
		stable.WriteString(mcpInstructions)
		stable.WriteString("\n")
		stable.WriteString("### ERROR HANDLING STRATEGY\n")
		stable.WriteString("- SILENT RETRY: If a tool fails, you may try to fix the parameters ONCE immediately in the next turn.\n")
		stable.WriteString("- GIVE UP: If the second attempt also fails, STOP retrying. Inform the user about the error and ask for clarification.\n")
		stable.WriteString("- ONLY speak to the user when the action is SUCCESSFUL, or if you have Failed TWICE.\n")
		stable.WriteString("\n\n")
	}

	// --- BLOQUE DINÁMICO (No Cacheable) ---

	// 1. Current Snapshot
	// Timezone resolution: Client (if registered) > Channel > UTC
	tz := ""
	if input.ClientContext != nil && input.ClientContext.IsRegistered && input.ClientContext.Timezone != "" {
		tz = input.ClientContext.Timezone
	} else if channelTZ, ok := input.Metadata["channel_timezone"].(string); ok && channelTZ != "" {
		tz = channelTZ
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, _ := time.LoadLocation(tz)
	now := time.Now().In(loc)
	moment := getMomentOfDay(now.Hour())

	dynamic.WriteString("## SESSION_METADATA\n")
	// Use a very explicit, human-friendly date format to verify current time
	dynamic.WriteString(fmt.Sprintf("- TODAY: %s\n", now.Format("Monday, 02 January 2006")))
	// Provide both 12h and 24h formats so the AI can adapt to the user's locale culture automatically
	dynamic.WriteString(fmt.Sprintf("- TIME_NOW: %s / %s (%s)\n", now.Format("15:04"), now.Format("03:04 PM"), tz))
	dynamic.WriteString(fmt.Sprintf("- Day_Moment: %s\n", moment))

	// 2. Client Identity
	if input.ClientContext != nil && input.ClientContext.IsRegistered {
		clientPrompt := input.ClientContext.ForPrompt()
		if clientPrompt != "" {
			dynamic.WriteString("- Client_Profile: " + strings.ReplaceAll(clientPrompt, "\n", " | ") + "\n")
		}

		// Add country code for regional context (AI infers currency, time format, etc.)
		if input.ClientContext.Country != "" {
			dynamic.WriteString(fmt.Sprintf("- Client_Country: %s\n", input.ClientContext.Country))
		}
	}

	// 3. Performance Metrics
	dynamic.WriteString(fmt.Sprintf("- Focus_Level: %d/100\n", input.FocusScore))

	// 4. Tasks
	if len(input.PendingTasks) > 0 {
		dynamic.WriteString("- Pending_Queue: [" + strings.Join(input.PendingTasks, ", ") + "]\n")
	}

	// 5. Language
	if input.Language != "" {
		dynamic.WriteString(fmt.Sprintf("- Active_Language: %s\n", input.Language))
	}

	dynamic.WriteString("\n[NOTE: The above metadata is for your internal context only. Do it not mention it in your response.]")

	// 6. CONTINUITY & STANDARD PROCEDURES
	dynamic.WriteString("\n\n### SERVICE RULES\n")
	dynamic.WriteString("1. CONTEXT: You are in an ongoing conversation. Answer DIRECTLY without repetitive greetings.\n")
	dynamic.WriteString("2. CURRENCY: To check exchange rates, you MUST use the 'get_exchange_rate' tool. Do not guess values. If the tool is unavailable, apologize and state you cannot verify the rate.\n")
	dynamic.WriteString("3. EXECUTION SILENCE: When you decide to call a tool, do NOT write any conversational text (like 'Let me check...' or 'Un momento'). Output ONLY the mindset tag and the tool call.\n")
	dynamic.WriteString("4. REGIONAL CONTEXT: If Client_Country is set, infer their default currency (e.g. PE=PEN, US=USD) and preferred time format (12h/24h) based on that country's conventions.\n")
	dynamic.WriteString("5. REMINDER PRIVACY (SPOILER PREVENTION): When listing/searching reminders, NEVER repeat the exact creative text or emojis saved in the database. You MUST summarize the activity in a neutral, boring tone (e.g., 'Cita con el dentista' instead of '¡A lucir esa sonrisa! 🦷'). The creative flair and emojis are ONLY for the final delivery.\n")

	return stable.String(), dynamic.String()
}

func getMomentOfDay(hour int) string {
	switch {
	case hour >= 0 && hour < 6:
		return "Early Morning"
	case hour >= 6 && hour < 12:
		return "Morning"
	case hour >= 12 && hour < 19:
		return "Afternoon"
	default:
		return "Night"
	}
}

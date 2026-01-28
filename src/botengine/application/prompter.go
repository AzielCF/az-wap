package application

import (
	"fmt"
	"strings"
	"time"

	domain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	configGlobal "github.com/AzielCF/az-wap/config"
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
	if configGlobal.AIGlobalSystemPrompt != "" {
		stable.WriteString(configGlobal.AIGlobalSystemPrompt)
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

	// 5. Situational Behavior (Rules)
	stable.WriteString("### SITUATIONAL BEHAVIOR (MINDSET)\n")
	stable.WriteString("You must ALWAYS start your response with a HIDDEN internal mindset tag: <mindset pace=\"fast|steady|deep\" focus=\"true|false\" work=\"true|false\" />\n\n")

	// 6. Toolset Guidelines (MCP)
	if mcpInstructions != "" {
		stable.WriteString("## MCP TOOL GUIDELINES")
		stable.WriteString(mcpInstructions)
		stable.WriteString("\n\n")
	}

	// --- BLOQUE DINÁMICO (No Cacheable) ---

	// 1. Current Snapshot
	tz := b.Timezone
	if tz == "" {
		tz = configGlobal.AITimezone
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, _ := time.LoadLocation(tz)
	now := time.Now().In(loc)
	moment := getMomentOfDay(now.Hour())

	dynamic.WriteString("## SESSION_METADATA\n")
	dynamic.WriteString(fmt.Sprintf("- Current_Time: %s\n", now.Format(time.RFC3339)))
	dynamic.WriteString(fmt.Sprintf("- Timeformat: %s\n", tz))
	dynamic.WriteString(fmt.Sprintf("- Day_Moment: %s\n", moment))

	// 2. Client Identity
	if input.ClientContext != nil && input.ClientContext.IsRegistered {
		clientPrompt := input.ClientContext.ForPrompt()
		if clientPrompt != "" {
			dynamic.WriteString("- Client_Profile: " + strings.ReplaceAll(clientPrompt, "\n", " | ") + "\n")
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

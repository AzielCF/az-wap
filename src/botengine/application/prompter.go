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

// BuildSystemInstructions consolida todas las fuentes de prompts del sistema
func (p *Prompter) BuildSystemInstructions(b domainBot.Bot, input domain.BotInput, mcpInstructions string) string {
	var sb strings.Builder

	// 1. Global Prompt
	if configGlobal.AIGlobalSystemPrompt != "" {
		sb.WriteString(configGlobal.AIGlobalSystemPrompt)
		sb.WriteString("\n\n")
	}

	// 2. Bot Specific Prompt
	if b.SystemPrompt != "" {
		sb.WriteString(b.SystemPrompt)
		sb.WriteString("\n\n")
	}

	// 3. Knowledge Base
	if b.KnowledgeBase != "" {
		sb.WriteString(b.KnowledgeBase)
		sb.WriteString("\n\n")
	}

	// 4. Timezone & Current Time
	tz := b.Timezone
	if tz == "" {
		tz = configGlobal.AITimezone
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	moment := getMomentOfDay(now.Hour())
	sb.WriteString(fmt.Sprintf("IMPORTANT - Current date and time (%s): %s (Time of day: %s)", tz, now.Format(time.RFC3339), moment))

	// 5. Resource Management Capabilities
	hasAnyMedia := b.ImageEnabled || b.AudioEnabled || b.VideoEnabled || b.DocumentEnabled
	if hasAnyMedia {
		sb.WriteString("\n\n### RESOURCE MANAGEMENT CAPABILITIES\n")
		sb.WriteString("You operate as a 'Resource Concierge'. To save tokens, only small items are sent instantly.\n")

		if b.ImageEnabled {
			sb.WriteString("- VISUALS: You can see images and stickers instantly.\n")
			sb.WriteString("  * STICKERS: Interpretation is based on conversation context. A 'sad' sticker can be ironic if the user is joking. Prioritize conversational flow over literal image description.\n")
		}
		if b.AudioEnabled {
			sb.WriteString("- AUDIO: You receive transcriptions of audio/voice notes automatically.\n")
		}

		if b.VideoEnabled || b.DocumentEnabled {
			sb.WriteString("- DEFERRED ACCESS: For Videos and Documents, you might see markers like [RECURSO DISPONIBLE].\n")
			sb.WriteString("  * Use 'get_session_resources' to list files or 'analyze_session_resource' to read them if the user asks.\n")
		}
		sb.WriteString("- UNSUPPORTED: For formats like ZIP or PSD, acknowledge them as general reference for tools.\n")

		sb.WriteString("\nBEHAVIOR: Use multimodal context ([Audio Transcription], [Image Description]) PROACTIVELY. Do not ask for details already provided in these descriptions.")
	}

	// 6. Situational Behavior (Mindset)
	sb.WriteString("\n\n### SITUATIONAL BEHAVIOR (MINDSET)\n")
	sb.WriteString("You must ALWAYS start your response with a HIDDEN internal mindset tag that defines your focus and effort level.\n")
	sb.WriteString("Format: <mindset pace=\"fast|steady|deep\" focus=\"true|false\" work=\"true|false\" />\n")
	sb.WriteString("- pace: 'fast' for greetings/trivialities, 'steady' for normal talk, 'deep' for analysis.\n")
	sb.WriteString("- focus: Set to 'true' if the topic is interesting, requires follow-up, or the user is providing high-value input.\n")
	sb.WriteString("- work: Set to 'true' if you used tools or performed complex reasoning/file analysis.\n")

	// 7. Session Context & Focus Score
	sb.WriteString(fmt.Sprintf("\n\n### CURRENT SESSION CONTEXT\nYour current Focus Score is %d/100. ", input.FocusScore))
	if input.FocusScore > 70 {
		sb.WriteString("You are in HIGH FOCUS MODE. The user has your full attention. Be direct, fast, and proactive. The chat stays open for you.")
	} else if input.FocusScore > 30 {
		sb.WriteString("You are in STEADY FOCUS. You are engaged in the conversation but still maintaining a natural human pace.")
	} else {
		sb.WriteString("You are just noticing this conversation. You might need to take a moment to understand the context.")
	}

	// 8. Pending Tasks Queue
	if len(input.PendingTasks) > 0 {
		sb.WriteString("\n\n### PENDING TASKS QUEUE\n")
		sb.WriteString("You have the following tasks in your waiting list:\n")
		for _, task := range input.PendingTasks {
			sb.WriteString(fmt.Sprintf("- %s\n", task))
		}
		sb.WriteString("\nIf current conversation is idle, consider mentioning or starting one of these tasks.")
	}

	// 9. Toolset Guidelines (MCP) - IDENTIDAD ORIGINAL: Al final de todo
	if mcpInstructions != "" {
		sb.WriteString("\n\n## MCP TOOL GUIDELINES")
		sb.WriteString(mcpInstructions)
	}

	return sb.String()
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

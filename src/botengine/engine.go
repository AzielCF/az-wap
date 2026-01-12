package botengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type PostReplyHook func(ctx context.Context, b bot.Bot, input BotInput, output BotOutput)

type Engine struct {
	botUsecase  bot.IBotUsecase
	mcpUsecase  domainMCP.IMCPUsecase
	providers   map[string]AIProvider
	transports  map[string]Transport
	memory      *MemoryStore
	humanizer   *Humanizer
	onPostReply []PostReplyHook
}

func NewEngine(botService bot.IBotUsecase, mcpService domainMCP.IMCPUsecase) *Engine {
	return &Engine{
		botUsecase: botService,
		mcpUsecase: mcpService,
		providers:  make(map[string]AIProvider),
		transports: make(map[string]Transport),
		memory:     NewMemoryStore(),
		humanizer:  NewHumanizer(true),
	}
}

func (e *Engine) RegisterPostReplyHook(h PostReplyHook) {
	e.onPostReply = append(e.onPostReply, h)
}

func (e *Engine) RegisterProvider(name string, p AIProvider) {
	e.providers[name] = p
}

func (e *Engine) RegisterTransport(t Transport) {
	e.transports[t.ID()] = t
}

func (e *Engine) GetMemoryStore() *MemoryStore {
	return e.memory
}

func (e *Engine) GetBotUsecase() bot.IBotUsecase {
	return e.botUsecase
}

// Process maneja el ciclo de vida completo de un mensaje
func (e *Engine) Process(ctx context.Context, input BotInput) (BotOutput, error) {
	// 0. Ensure TraceID
	if input.TraceID == "" {
		input.TraceID = uuid.NewString()
	}

	meta := map[string]string{"trace_id": input.TraceID}

	// 0. Record Inbound
	botmonitor.Record(botmonitor.Event{
		TraceID:    input.TraceID,
		InstanceID: input.InstanceID,
		ChatJID:    input.ChatID,
		Provider:   string(input.Platform), // Initial provider is the platform
		Stage:      "inbound",
		Kind:       "text",
		Status:     "ok",
		Metadata: map[string]string{
			"trace_id": input.TraceID,
			"input":    input.Text,
			"platform": string(input.Platform),
		},
	})

	// 1. Cargar configuración del bot
	b, err := e.botUsecase.GetByID(ctx, input.BotID)
	if err != nil {
		botmonitor.Record(botmonitor.Event{
			TraceID:    input.TraceID,
			InstanceID: input.InstanceID, ChatJID: input.ChatID,
			Stage: "ai_response", Status: "error", Error: err.Error(),
			Metadata: meta,
		})
		return BotOutput{}, fmt.Errorf("failed to load bot %s: %w", input.BotID, err)
	}

	// 2. Whitelist logic
	if len(b.Whitelist) > 0 {
		allowed := false
		for _, jid := range b.Whitelist {
			if strings.TrimSpace(jid) == strings.TrimSpace(input.SenderID) ||
				strings.Contains(input.SenderID, strings.TrimSpace(jid)) {
				allowed = true
				break
			}
		}
		if !allowed {
			logrus.Infof("[ENGINE] Message from %s ignored (not in whitelist for bot %s)", input.SenderID, b.ID)
			botmonitor.Record(botmonitor.Event{
				TraceID:    input.TraceID,
				InstanceID: input.InstanceID, ChatJID: input.ChatID,
				Stage: "ai_response", Status: "skipped",
				Metadata: meta,
			})
			return BotOutput{}, nil
		}
	}

	// 3. Seleccionar Proveedor
	providerName := string(b.Provider)
	if providerName == "" {
		providerName = "gemini"
	}

	p, ok := e.providers[providerName]
	if !ok {
		return BotOutput{}, fmt.Errorf("provider %s not registered", providerName)
	}

	// 4. Cargar Herramientas MCP
	var tools []domainMCP.Tool
	if e.mcpUsecase != nil {
		tools, _ = e.mcpUsecase.GetBotTools(ctx, b.ID)
	}

	// 5. Generar Respuesta
	output, err := p.GenerateReply(ctx, b, input, tools)
	if err != nil {
		return BotOutput{}, fmt.Errorf("AI provider failed: %w", err)
	}

	if output.Text == "" {
		return output, nil
	}

	// 6. Post-Proceso: Simular escritura y Enviar (Async si hay transporte)
	transport, hasTransport := e.transports[input.InstanceID]
	if hasTransport {
		go func() {
			// Usamos Background o un contexto derivado para que no se cancele
			// si la petición REST termina pero queremos seguir "escribiendo" en WA.
			if ok := e.humanizer.SimulateTyping(context.Background(), transport, input.ChatID, output.Text); !ok {
				return
			}
			_ = transport.SendMessage(context.Background(), input.ChatID, output.Text)
			botmonitor.Record(botmonitor.Event{
				TraceID:    input.TraceID,
				InstanceID: input.InstanceID, ChatJID: input.ChatID,
				Provider: fmt.Sprintf("%s / %s", input.Platform, providerName),
				Stage:    "outbound", Status: "ok",
				Metadata: map[string]string{
					"trace_id": input.TraceID,
					"output":   output.Text,
				},
			})

			// Ejecutar Hooks (ej: Chatwoot)
			for _, h := range e.onPostReply {
				h(context.Background(), b, input, output)
			}
		}()
	} else {
		// Si no hay transporte (REST API), hooks sincrónicos.
		for _, h := range e.onPostReply {
			h(ctx, b, input, output)
		}
	}

	return output, nil
}

// Memory Utilities
func (e *Engine) ClearBotMemory(botID string) {
	e.memory.ClearPrefix(fmt.Sprintf("bot|%s|", botID))
}

func (e *Engine) CloseChatMemory(botID, senderID string) {
	e.memory.Clear(fmt.Sprintf("bot|%s|%s", botID, senderID))
}

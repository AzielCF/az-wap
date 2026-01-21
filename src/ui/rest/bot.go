package rest

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/AzielCF/az-wap/botengine"
	"github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/pkg/msgworker"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/gofiber/fiber/v2"
)

var (
	GenerateBotTextReplyFunc func(ctx context.Context, botID string, memoryID string, input string) (string, error)
	ClearBotMemoryFunc       func(botID string)

	botWebhookPoolOnce   sync.Once
	botWebhookPool       *msgworker.MessageWorkerPool
	botWebhookPoolCtx    context.Context
	botWebhookPoolCancel context.CancelFunc

	workspaceManager *workspace.Manager
	engine           *botengine.Engine
)

func SetBotEngine(e *botengine.Engine, wm *workspace.Manager) {
	engine = e
	workspaceManager = wm
	if wm != nil {
		ClearBotMemoryFunc = wm.ClearBotMemory
	}
}

func initBotWebhookPool() {
	botWebhookPoolOnce.Do(func() {
		botWebhookPoolCtx, botWebhookPoolCancel = context.WithCancel(context.Background())

		size := 6
		if v := strings.TrimSpace(os.Getenv("BOT_WEBHOOK_POOL_SIZE")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				size = n
			}
		}

		queue := 250
		if v := strings.TrimSpace(os.Getenv("BOT_WEBHOOK_QUEUE_SIZE")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				queue = n
			}
		}

		botWebhookPool = msgworker.NewMessageWorkerPool(size, queue)
		botWebhookPool.Start(botWebhookPoolCtx)

		// Register with global monitor if manager is available
		if workspaceManager != nil {
			workspaceManager.RegisterExternalPool(botWebhookPool, "webhook")
		}
	})
}

type Bot struct {
	Service    domainBot.IBotUsecase
	MCPService domainMCP.IMCPUsecase
}

func InitRestBot(app fiber.Router, service domainBot.IBotUsecase, mcpService domainMCP.IMCPUsecase, wm *workspace.Manager) Bot {
	workspaceManager = wm
	initBotWebhookPool()
	rest := Bot{Service: service, MCPService: mcpService}
	app.Get("/bots", rest.ListBots)
	app.Post("/bots", rest.CreateBot)
	app.Get("/bots/:id", rest.GetBot)
	app.Put("/bots/:id", rest.UpdateBot)
	app.Delete("/bots/:id", rest.DeleteBot)
	app.Post("/bots/:id/webhook", rest.HandleWebhook)
	app.Post("/bots/:id/memory/clear", rest.ClearMemory)
	app.Get("/bots/config/models", rest.ListModels)

	// Bot-MCP relations
	app.Get("/bots/:id/mcp", rest.ListBotMCPs)
	app.Post("/bots/:id/mcp", rest.AddBotMCP)
	app.Put("/bots/:id/mcp/:server_id", rest.UpdateBotMCPConfig) // Granular config
	app.Delete("/bots/:id/mcp/:server_id", rest.RemoveBotMCP)

	return rest
}

func (h *Bot) ListModels(c *fiber.Ctx) error {
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Models fetched",
		Results: domainBot.ProviderModels,
	})
}

func (h *Bot) ListBots(c *fiber.Ctx) error {
	bots, err := h.Service.List(c.UserContext())
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bots fetched",
		Results: bots,
	})
}

func (h *Bot) CreateBot(c *fiber.Ctx) error {
	var req domainBot.CreateBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	bot, err := h.Service.Create(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bot created",
		Results: bot,
	})
}

func (h *Bot) GetBot(c *fiber.Ctx) error {
	id := c.Params("id")
	bot, err := h.Service.GetByID(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(utils.ResponseData{
			Status:  404,
			Code:    "NOT_FOUND",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bot fetched",
		Results: bot,
	})
}

func (h *Bot) UpdateBot(c *fiber.Ctx) error {
	id := c.Params("id")
	var req domainBot.UpdateBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	bot, err := h.Service.Update(c.UserContext(), id, req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bot updated",
		Results: bot,
	})
}

func (h *Bot) DeleteBot(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.Service.Delete(c.UserContext(), id); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bot deleted",
		Results: nil,
	})
}

func (h *Bot) ClearMemory(c *fiber.Ctx) error {
	id := c.Params("id")
	if strings.TrimSpace(id) == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "id: cannot be blank.",
		})
	}

	// Validamos que el bot exista para evitar limpiar claves de un ID inválido.
	if _, err := h.Service.GetByID(c.UserContext(), id); err != nil {
		return c.Status(404).JSON(utils.ResponseData{
			Status:  404,
			Code:    "NOT_FOUND",
			Message: err.Error(),
		})
	}

	if ClearBotMemoryFunc != nil {
		ClearBotMemoryFunc(id)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Bot memory cleared",
		Results: nil,
	})
}

func (h *Bot) HandleWebhook(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "id: cannot be blank.",
		})
	}

	var req struct {
		MemoryID string `json:"memory_id"`
		Input    string `json:"input"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	text := strings.TrimSpace(req.Input)
	if text == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "input: cannot be blank.",
		})
	}

	initBotWebhookPool()
	if botWebhookPool == nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "bot webhook worker pool not initialized",
		})
	}

	monitorChatID := strings.TrimSpace(req.MemoryID)
	if monitorChatID == "" {
		monitorChatID = "(no-memory-id)"
	}

	type res struct {
		reply string
		err   error
	}
	resCh := make(chan res, 1)

	ok := botWebhookPool.TryDispatch(msgworker.MessageJob{
		InstanceID: "bot:" + id,
		ChatJID:    monitorChatID,
		Handler: func(ctx context.Context) error {
			defer func() {
				if r := recover(); r != nil {
					select {
					case resCh <- res{reply: "", err: fmt.Errorf("bot webhook handler panic: %v", r)}:
					default:
					}
				}
			}()

			if engine != nil && req.MemoryID != "" {
				output, err := engine.Process(ctx, domain.BotInput{
					BotID:      id,
					SenderID:   req.MemoryID,
					ChatID:     req.MemoryID,
					Platform:   domain.PlatformWeb,
					Text:       text,
					InstanceID: "webhook",
				})
				select {
				case resCh <- res{reply: output.Text, err: err}:
				default:
				}
				return err
			}

			reply, err := GenerateBotTextReplyFunc(ctx, id, req.MemoryID, text)
			select {
			case resCh <- res{reply: reply, err: err}:
			default:
			}
			return err
		},
	})
	if !ok {
		return c.Status(429).JSON(utils.ResponseData{
			Status:  429,
			Code:    "TOO_MANY_REQUESTS",
			Message: "bot webhook queue is full",
		})
	}

	select {
	case r := <-resCh:
		if r.err != nil {
			return c.Status(400).JSON(utils.ResponseData{
				Status:  400,
				Code:    "BAD_REQUEST",
				Message: r.err.Error(),
			})
		}
		reply := r.reply
		response := map[string]any{
			"bot_id":    id,
			"memory_id": req.MemoryID,
			"input":     text,
			"reply":     reply,
		}

		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "SUCCESS",
			Message: "Bot reply generated",
			Results: response,
		})
	case <-c.UserContext().Done():
		return c.Status(499).JSON(utils.ResponseData{
			Status:  499,
			Code:    "CLIENT_CLOSED_REQUEST",
			Message: "request cancelled",
		})
	}
}

func (h *Bot) ListBotMCPs(c *fiber.Ctx) error {
	id := c.Params("id")
	servers, err := h.MCPService.ListServersForBot(c.UserContext(), id)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Results: servers,
	})
}

func (h *Bot) AddBotMCP(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		ServerID string `json:"server_id"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}
	err := h.MCPService.ToggleServerForBot(c.UserContext(), id, req.ServerID, req.Enabled)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{Status: 200, Message: "Bot MCP toggled"})
}

func (h *Bot) UpdateBotMCPConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	serverID := c.Params("server_id")
	var req struct {
		Enabled       bool              `json:"enabled"`
		DisabledTools []string          `json:"disabled_tools"`
		CustomHeaders map[string]string `json:"custom_headers"`
		Instructions  string            `json:"instructions"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}
	err := h.MCPService.UpdateBotMCPConfig(c.UserContext(), domainMCP.BotMCPConfig{
		BotID:         id,
		ServerID:      serverID,
		Enabled:       req.Enabled,
		DisabledTools: req.DisabledTools,
		CustomHeaders: req.CustomHeaders,
		Instructions:  req.Instructions,
	})
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{Status: 200, Message: "Bot MCP config updated"})
}

func (h *Bot) RemoveBotMCP(c *fiber.Ctx) error {
	id := c.Params("id")
	serverID := c.Params("server_id")
	err := h.MCPService.ToggleServerForBot(c.UserContext(), id, serverID, false)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{Status: 200, Message: "Bot MCP removed"})
}

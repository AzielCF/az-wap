package rest

import (
	"strings"

	domainBot "github.com/AzielCF/az-wap/domains/bot"
	integrationGemini "github.com/AzielCF/az-wap/integrations/gemini"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

var (
	generateBotTextReplyFunc = integrationGemini.GenerateBotTextReply
	clearBotMemoryFunc       = integrationGemini.ClearBotMemory
)

type Bot struct {
	Service domainBot.IBotUsecase
}

func InitRestBot(app fiber.Router, service domainBot.IBotUsecase) Bot {
	rest := Bot{Service: service}
	app.Get("/bots", rest.ListBots)
	app.Post("/bots", rest.CreateBot)
	app.Get("/bots/:id", rest.GetBot)
	app.Put("/bots/:id", rest.UpdateBot)
	app.Delete("/bots/:id", rest.DeleteBot)
	app.Post("/bots/:id/webhook", rest.HandleWebhook)
	app.Post("/bots/:id/memory/clear", rest.ClearMemory)
	return rest
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

	clearBotMemoryFunc(id)

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

	reply, err := generateBotTextReplyFunc(c.UserContext(), id, req.MemoryID, text)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

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
}

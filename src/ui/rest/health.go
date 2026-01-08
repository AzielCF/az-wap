package rest

import (
	"github.com/AzielCF/az-wap/domains/health"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Health struct {
	Service health.IHealthUsecase
}

func InitRestHealth(app fiber.Router, service health.IHealthUsecase) Health {
	handler := Health{Service: service}

	group := app.Group("/api/health")
	group.Get("/status", handler.GetStatus)
	group.Post("/check-all", handler.CheckAll)
	group.Post("/mcp/:id/check", handler.CheckMCP)
	group.Post("/credentials/:id/check", handler.CheckCredential)

	return handler
}

func (h *Health) GetStatus(c *fiber.Ctx) error {
	records, err := h.Service.GetStatus(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_SERVER_ERROR",
			Message: err.Error(),
		})
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Health status retrieved",
		Results: records,
	})
}

func (h *Health) CheckAll(c *fiber.Ctx) error {
	records, err := h.Service.CheckAll(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_SERVER_ERROR",
			Message: err.Error(),
		})
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Verification started for all entities",
		Results: records,
	})
}

func (h *Health) CheckMCP(c *fiber.Ctx) error {
	id := c.Params("id")
	record, err := h.Service.CheckMCP(c.UserContext(), id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_SERVER_ERROR",
			Message: err.Error(),
		})
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "MCP health check completed",
		Results: record,
	})
}

func (h *Health) CheckCredential(c *fiber.Ctx) error {
	id := c.Params("id")
	record, err := h.Service.CheckCredential(c.UserContext(), id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_SERVER_ERROR",
			Message: err.Error(),
		})
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credential health check completed",
		Results: record,
	})
}

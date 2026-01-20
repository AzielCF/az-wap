package rest

import (
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type MCP struct {
	Service domainMCP.IMCPUsecase
}

func InitRestMCP(app fiber.Router, service domainMCP.IMCPUsecase) MCP {
	rest := MCP{Service: service}

	group := app.Group("/api/mcp")
	group.Get("/servers", rest.ListServers)
	group.Post("/servers", rest.AddServer)
	group.Get("/servers/:id", rest.GetServer)
	group.Put("/servers/:id", rest.UpdateServer)
	group.Delete("/servers/:id", rest.DeleteServer)

	group.Get("/servers/:id/tools", rest.ListTools)

	return rest
}

func (handler *MCP) ListServers(c *fiber.Ctx) error {
	servers, err := handler.Service.ListServers(c.UserContext())
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "MCP servers retrieved",
		Results: servers,
	})
}

func (handler *MCP) AddServer(c *fiber.Ctx) error {
	var req domainMCP.MCPServer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}
	server, err := handler.Service.AddServer(c.UserContext(), req)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  201,
		Code:    "SUCCESS",
		Message: "MCP server added successfully",
		Results: server,
	})
}

func (handler *MCP) GetServer(c *fiber.Ctx) error {
	id := c.Params("id")
	server, err := handler.Service.GetServer(c.UserContext(), id)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Results: server,
	})
}

func (handler *MCP) UpdateServer(c *fiber.Ctx) error {
	id := c.Params("id")
	var req domainMCP.MCPServer
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}
	server, err := handler.Service.UpdateServer(c.UserContext(), id, req)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "MCP server updated successfully",
		Results: server,
	})
}

func (handler *MCP) DeleteServer(c *fiber.Ctx) error {
	id := c.Params("id")
	err := handler.Service.DeleteServer(c.UserContext(), id)
	utils.PanicIfNeeded(err)
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "MCP server deleted successfully",
	})
}

func (handler *MCP) ListTools(c *fiber.Ctx) error {
	id := c.Params("id")
	tools, err := handler.Service.ListTools(c.UserContext(), id)
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
		Results: tools,
	})
}

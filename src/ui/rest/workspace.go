package rest

import (
	"github.com/AzielCF/az-wap/workspace/domain"
	"github.com/AzielCF/az-wap/workspace/usecase"
	"github.com/gofiber/fiber/v2"
)

type WorkspaceHandler struct {
	uc *usecase.WorkspaceUsecase
}

func NewWorkspaceHandler(uc *usecase.WorkspaceUsecase) *WorkspaceHandler {
	return &WorkspaceHandler{uc: uc}
}

func (h *WorkspaceHandler) Register(router fiber.Router) {
	g := router.Group("/workspaces")
	g.Post("/", h.CreateWorkspace)
	g.Get("/", h.ListWorkspaces)
	g.Get("/:id", h.GetWorkspace)
	g.Post("/:id/channels", h.CreateChannel)
	g.Get("/:id/channels", h.ListChannels)
}

func (h *WorkspaceHandler) CreateWorkspace(c *fiber.Ctx) error {
	type req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     string `json:"owner_id"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if r.Name == "" || r.OwnerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and owner_id are required"})
	}

	ws, err := h.uc.CreateWorkspace(c.Context(), r.Name, r.Description, r.OwnerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ws)
}

func (h *WorkspaceHandler) ListWorkspaces(c *fiber.Ctx) error {
	workspaces, err := h.uc.ListWorkspaces(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(workspaces)
}

func (h *WorkspaceHandler) GetWorkspace(c *fiber.Ctx) error {
	id := c.Params("id")
	ws, err := h.uc.GetWorkspace(c.Context(), id)
	if err == domain.ErrWorkspaceNotFound {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "workspace not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ws)
}

func (h *WorkspaceHandler) CreateChannel(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	type req struct {
		Type domain.ChannelType `json:"type"`
		Name string             `json:"name"`
	}
	var r req
	if err := c.BodyParser(&r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	if r.Name == "" || r.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and type are required"})
	}

	ch, err := h.uc.CreateChannel(c.Context(), workspaceID, r.Type, r.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ch)
}

func (h *WorkspaceHandler) ListChannels(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	channels, err := h.uc.ListChannels(c.Context(), workspaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(channels)
}

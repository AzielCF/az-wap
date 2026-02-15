package infrastructure

import (
	"strings"

	"github.com/AzielCF/az-wap/clients_portal/auth/application"
	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	authService *application.AuthService
}

func NewAuthHandler(service *application.AuthService) *AuthHandler {
	return &AuthHandler{authService: service}
}

// Request Models
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	ClientID string            `json:"client_id"` // Typically comes from an admin token or context
	Username string            `json:"username"`
	Password string            `json:"password"`
	FullName string            `json:"full_name"`
	Role     domain.PortalRole `json:"role"` // Optional, default MEMBER
}

// Login handles portal user authentication
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	token, user, err := h.authService.Login(c.Context(), req.Username, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"id":        user.ID,
			"username":  user.Username,
			"full_name": user.FullName,
			"role":      user.Role,
			"client_id": user.ClientID,
		},
	})
}

// Register handles new user registration (ideally protected by Admin/Owner role)
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Basic validation
	if req.Username == "" || req.Password == "" || req.ClientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing required fields"})
	}

	// Default role
	if req.Role == "" {
		req.Role = domain.RoleMember
	}

	user, err := h.authService.Register(c.Context(), req.ClientID, req.Username, req.Password, req.FullName, req.Role)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "user created successfully",
		"user_id": user.ID,
	})
}

// Me returns the logged-in user information including account details
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	// Get UserID from context (injected by middleware)
	userID, ok := c.Locals("portal_user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// Get full profile using the service
	profile, err := h.authService.GetUserProfile(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch profile"})
	}

	return c.JSON(profile)
}

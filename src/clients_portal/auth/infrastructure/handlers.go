package infrastructure

import (
	"strings"

	"github.com/AzielCF/az-wap/clients_portal/auth/application"
	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	coreconfig "github.com/AzielCF/az-wap/core/config"
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

// GenerateMagicLink creates a magic link URL for a phone number (Admin/Bot use only)
func (h *AuthHandler) GenerateMagicLink(c *fiber.Ctx) error {
	// 1. Security Check: Validate internal master key
	secret := c.Get("X-Internal-Key")
	masterKey := coreconfig.Global.Security.PortalInternalKey

	if secret == "" || secret != masterKey {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden: invalid internal key"})
	}

	type Request struct {
		ClientID string `json:"client_id"`
		Phone    string `json:"phone"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	token, err := h.authService.GenerateMagicLink(c.Context(), req.ClientID, req.Phone)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// In production, this would be the full frontend URL
	magicURL := "/auth/magic-login?token=" + token

	return c.JSON(fiber.Map{
		"token": token,
		"url":   magicURL,
	})
}

// RedeemMagicLink handles the exchange of magic token for session token
func (h *AuthHandler) RedeemMagicLink(c *fiber.Ctx) error {
	type Request struct {
		Token string `json:"token"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	sessionToken, user, err := h.authService.RedeemMagicLink(c.Context(), req.Token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link"})
	}

	return c.JSON(fiber.Map{
		"token": sessionToken,
		"user":  user,
	})
}

// UpdateProfile handles user profile updates
func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("portal_user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	type Request struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	var emailPtr *string
	if req.Email != "" {
		emailPtr = &req.Email
	}

	err := h.authService.UpdateProfile(c.Context(), userID, emailPtr, req.FullName, req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "profile updated successfully"})
}

// ListAccountsState allows admin to query the portal status of multiple clients
func (h *AuthHandler) ListAccountsState(c *fiber.Ctx) error {
	idsParam := c.Query("ids")
	var ids []string
	if idsParam != "" {
		ids = strings.Split(idsParam, ",")
	}

	accounts, err := h.authService.ListAccountsState(c.Context(), ids)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(accounts)
}

// CreatePortalAccount allows admin to provision a portal account for a client
func (h *AuthHandler) CreatePortalAccount(c *fiber.Ctx) error {
	clientID := c.Params("id")
	type Request struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	user, err := h.authService.CreateAccountByAdmin(c.Context(), clientID, req.Email, req.FullName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "portal account provisioned successfully",
		"user_id": user.ID,
	})
}

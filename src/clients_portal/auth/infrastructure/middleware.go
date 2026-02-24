package infrastructure

import (
	"strings"

	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	portalSecurity "github.com/AzielCF/az-wap/clients_portal/shared/security"
	"github.com/gofiber/fiber/v2"
)

// Config for middleware (future: support BetterAuth remote keys)
type AuthConfig struct {
	SecretKey []byte
}

// NewAuthMiddleware creates the middleware to protect portal routes
func NewAuthMiddleware(userRepo domain.IAuthRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract token
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization header"})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format"})
		}

		tokenString := parts[1]

		// 2. Validate token (This is where we would switch logic if using BetterAuth)
		// In the future, we could validate against an external public key here.
		claims, err := portalSecurity.ValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		// 3. Validate user existence and get fresh data
		user, err := userRepo.GetByID(c.Context(), claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found or inactive"})
		}

		if !user.Active {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "account is deactivated"})
		}

		// 4. Inject context for next handlers
		c.Locals("user", user) // Crucial for feature handlers
		c.Locals("portal_user_id", user.ID)
		c.Locals("portal_client_id", user.ClientID)
		c.Locals("portal_role", user.Role)

		return c.Next()
	}
}

// RequireRole is an additional middleware for granular permissions
func RequireRole(requiredRole domain.PortalRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("portal_role").(domain.PortalRole)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "role not found in context"})
		}

		if role != requiredRole && role != domain.RoleOwner { // Owner can always do everything
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient permissions"})
		}

		return c.Next()
	}
}

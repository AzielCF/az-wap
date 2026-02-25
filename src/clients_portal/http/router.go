package http

import (
	authInfra "github.com/AzielCF/az-wap/clients_portal/auth/infrastructure"
	featuresInfra "github.com/AzielCF/az-wap/clients_portal/features/infrastructure"
	coreConfig "github.com/AzielCF/az-wap/core/config"
	"github.com/gofiber/fiber/v2"
)

// RegisterPortalRoutes centralizes all Client Portal route definitions.
// It manages:
// 1. Internal/Admin routes (via baseAPI, protected by system BasicAuth)
// 2. Public Portal routes (via app, separated from system auth)
// 3. Protected Portal routes (via middleware)
func RegisterPortalRoutes(
	app fiber.Router,
	baseAPI fiber.Router,
	authHandler *authInfra.AuthHandler,
	featuresHandler *featuresInfra.FeaturesHandler,
	portalAuthMiddleware fiber.Handler,
) {
	// 1. Internal Admin Routes (Attached to system API group)
	// Used by admin dashboard relative to /api/internal
	baseAPI.Post("/internal/clients/:id/portal-account", authHandler.CreatePortalAccount)
	baseAPI.Get("/internal/portal-accounts", authHandler.ListAccountsState)
	baseAPI.Post("/internal/magic-link/generate", authHandler.GenerateMagicLink)

	// 2. Public Portal Routes (/api/portal)
	// Created directly on app router to manage middleware stack independently
	portalGroup := app.Group(coreConfig.Global.App.BasePath + "/api/portal")

	portalGroup.Post("/login", authHandler.Login)
	portalGroup.Post("/magic-link/redeem", authHandler.RedeemMagicLink)

	// 3. Protected Portal Routes (Require valid Portal Token)
	protected := portalGroup.Group("", portalAuthMiddleware)

	// Auth Module Routes
	protected.Get("/me", authHandler.Me)
	protected.Put("/profile", authHandler.UpdateProfile)

	// Features Module Routes
	protected.Get("/reminders", featuresHandler.ListReminders)
	protected.Get("/info", featuresHandler.GetGeneralInfo)
	protected.Get("/owned-channels", featuresHandler.GetOwnedChannels)
	protected.Get("/authorized-agents", featuresHandler.GetAuthorizedAgents)

	// Access Rules for Channels
	protected.Get("/owned-channels/:cid/access-rules", featuresHandler.GetChannelAccessRules)
	protected.Post("/owned-channels/:cid/access-rules", featuresHandler.AddChannelAccessRule)
	protected.Delete("/owned-channels/:cid/access-rules/:rid", featuresHandler.DeleteChannelAccessRule)
	protected.Get("/owned-channels/:cid/resolve-identity", featuresHandler.ResolveIdentity)

	// Edit Channel
	protected.Put("/owned-channels/:cid/name", featuresHandler.UpdateChannelName)

	// WhatsApp Channel Controls
	protected.Get("/owned-channels/:cid/whatsapp/status", featuresHandler.GetWhatsAppStatus)
	protected.Get("/owned-channels/:cid/whatsapp/login", featuresHandler.WhatsAppLogin)
	protected.Post("/owned-channels/:cid/whatsapp/login-code", featuresHandler.WhatsAppLoginWithCode)
	protected.Get("/owned-channels/:cid/whatsapp/logout", featuresHandler.WhatsAppLogout)

	protected.Post("/owned-channels/:cid/enable", featuresHandler.EnableChannel)
	protected.Post("/owned-channels/:cid/disable", featuresHandler.DisableChannel)

	// Workspace Management (ABAC restricted)
	protected.Get("/workspaces", featuresHandler.ListWorkspaces)
	protected.Get("/workspaces/:wid/channels", featuresHandler.ListWorkspaceChannels)
	protected.Get("/workspaces/:wid/guests", featuresHandler.ListGuests)

	wsGroup := protected.Group("/workspaces", featuresHandler.EnforceWorkspaceFeature)
	wsGroup.Post("", featuresHandler.CreateWorkspace)
	wsGroup.Put("/:wid", featuresHandler.UpdateWorkspace)
	wsGroup.Delete("/:wid", featuresHandler.DeleteWorkspace)

	// Workspace Channels (Write Actions)
	wsGroup.Post("/:wid/channels/:cid", featuresHandler.LinkChannel)
	wsGroup.Delete("/:wid/channels/:cid", featuresHandler.UnlinkChannel)

	// Workspace Guests (Write Actions)
	wsGroup.Post("/:wid/guests", featuresHandler.CreateGuest)
	wsGroup.Put("/:wid/guests/:gid", featuresHandler.UpdateGuest)
	wsGroup.Delete("/:wid/guests/:gid", featuresHandler.DeleteGuest)
}

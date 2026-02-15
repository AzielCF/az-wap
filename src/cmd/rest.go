package cmd

import (
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	clientsRepo "github.com/AzielCF/az-wap/clients/repository"
	portalAuthApp "github.com/AzielCF/az-wap/clients_portal/auth/application"
	portalAuthInfra "github.com/AzielCF/az-wap/clients_portal/auth/infrastructure"
	portalAuthRepo "github.com/AzielCF/az-wap/clients_portal/auth/repository"
	coreconfig "github.com/AzielCF/az-wap/core/config"
	coreDB "github.com/AzielCF/az-wap/core/database"
	"github.com/AzielCF/az-wap/ui/rest"
	"github.com/AzielCF/az-wap/ui/rest/middleware"
	"github.com/AzielCF/az-wap/ui/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var restCmd = &cobra.Command{
	Use:   "rest",
	Short: "Send whatsapp API over http",
	Long:  `This application is from clone https://az-wap`,
	Run:   restServer,
}

func init() {
	restCmd.Flags().String("basic-auth", "", "Basic auth for API (format: user:pass,user2:pass2)")
	rootCmd.AddCommand(restCmd)
}
func restServer(cmd *cobra.Command, _ []string) {
	// Override basic auth if flag is provided
	if baFlag, _ := cmd.Flags().GetString("basic-auth"); baFlag != "" {
		coreconfig.Global.App.BasicAuth = strings.Split(baFlag, ",")
	}

	fiberConfig := fiber.Config{
		EnableTrustedProxyCheck: true,
		BodyLimit:               int(coreconfig.Global.Whatsapp.MaxVideoSize),
		Network:                 "tcp",
		AppName:                 "Az-Wap Enterprise Engine",
		DisableStartupMessage:   false, // Keep generic startup message
		ServerHeader:            "Hidden",
	}

	// Configure proxy settings if trusted proxies are specified
	if len(coreconfig.Global.App.TrustedProxies) > 0 {
		fiberConfig.TrustedProxies = coreconfig.Global.App.TrustedProxies
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedHost
	}

	app := fiber.New(fiberConfig)

	// Security: RequestID for audit trails
	app.Use(requestid.New())

	// Security: Strict CORS
	// In production, this should be restricted to the actual frontend domain.
	origins := strings.Join(coreconfig.Global.App.CorsAllowedOrigins, ", ")
	if !strings.Contains(origins, coreconfig.Global.App.BaseUrl) {
		origins += ", " + coreconfig.Global.App.BaseUrl
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: origins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Instance-Token, X-Request-ID",
	}))
	app.Use(middleware.Recovery())

	// Security: Hardened Headers
	app.Use(helmet.New(helmet.Config{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		HSTSMaxAge:            31536000, // 1 Year
		HSTSExcludeSubdomains: false,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https://*.whatsapp.net; connect-src 'self' http://localhost:* ws://localhost:*;",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        1000,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
	}))

	if coreconfig.Global.App.Debug {
		app.Use(logger.New())
	}

	if len(coreconfig.Global.App.BasicAuth) == 0 {
		logrus.Fatalln("APP_BASIC_AUTH is required. Nothing should be public; please set APP_BASIC_AUTH=<user>:<secret>[,<user2>:<secret2>] and restart.")
	}

	account := make(map[string]string)
	for _, basicAuth := range coreconfig.Global.App.BasicAuth {
		ba := strings.Split(basicAuth, ":")
		if len(ba) != 2 {
			logrus.Fatalln("Basic auth is not valid, please this following format <user>:<secret>")
		}
		account[ba[0]] = ba[1]
	}

	// System statics
	app.Static(coreconfig.Global.App.BasePath+"/statics", "./statics")

	// Create API group
	apiGroup := app.Group(coreconfig.Global.App.BasePath + "/api")

	// Apply BasicAuth ONLY to the API group
	apiGroup.Use(basicauth.New(basicauth.Config{
		Users: account,
		Next: func(c *fiber.Ctx) bool {
			// Allow CORS preflight without credentials.
			if c.Method() == fiber.MethodOptions {
				return true
			}
			return false
		},
	}))

	// Graceful shutdown handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Info("[REST] Reception of termination signal, shutting down gracefully...")
		if err := app.Shutdown(); err != nil {
			logrus.Errorf("[REST] Error during Fiber shutdown: %v", err)
		}

		// Stop all app subsystems (DBs, clients, etc.)
		StopApp()
	}()

	// Register Workspace Handlers
	rest.InitRestApp(apiGroup, appUsecase)
	// rest.InitRestChat(apiGroup, chatUsecase) // Discontinued
	rest.InitRestSend(apiGroup, sendUsecase)
	rest.InitRestUser(apiGroup, userUsecase)
	rest.InitRestMessage(apiGroup, messageUsecase)
	rest.InitRestGroup(apiGroup, groupUsecase)
	rest.InitRestNewsletter(apiGroup, newsletterUsecase)
	rest.InitRestBot(apiGroup, botUsecase, mcpUsecase, workspaceManager)
	// rest.InitRestInstance(apiGroup, instanceUsecase, sendUsecase) // DEPRECATED
	rest.InitChannelAPI(apiGroup, wkUsecase, workspaceManager, sendUsecase, settingsSvc)
	rest.InitRestCredential(apiGroup, credentialUsecase)
	rest.InitRestCache(apiGroup, cacheUsecase)
	rest.InitRestMCP(apiGroup, mcpUsecase)
	rest.InitRestHealth(apiGroup, healthUsecase)

	// --- CLIENTS PORTAL (NEW) ---
	// 1. Initialize Portal Auth Components
	portalAuthRepository := portalAuthRepo.NewGormAuthRepository(coreDB.GlobalDB)
	// Inject existing CRM Client Repository
	crmClientRepo := clientsRepo.NewClientGormRepository(coreDB.GlobalDB)
	// Auto-migrate portal users table
	if err := portalAuthRepository.AutoMigrate(); err != nil {
		logrus.Errorf("Failed to migrate portal tables: %v", err)
	}

	portalAuthService := portalAuthApp.NewAuthService(portalAuthRepository, crmClientRepo)
	portalAuthHandler := portalAuthInfra.NewAuthHandler(portalAuthService)
	portalAuthMiddleware := portalAuthInfra.NewAuthMiddleware(portalAuthRepository)

	// 2. Create API Group for Portal (Isolated from Admin)
	portalGroup := app.Group(coreconfig.Global.App.BasePath + "/api/portal")

	// Public Portal Routes (Login)
	portalGroup.Post("/login", portalAuthHandler.Login)
	// TODO: Register should be protected or invitation-only, currently open for initial testing or move to admin
	// portalGroup.Post("/register", portalAuthHandler.Register)

	// Protected Portal Routes
	portalProtected := portalGroup.Group("/")
	portalProtected.Use(portalAuthMiddleware)
	portalProtected.Get("/me", portalAuthHandler.Me) // Logged-in user profile

	// Unified Monitoring System (Multi-server aware)
	rest.InitRestMonitoring(apiGroup, monitorStore, workspaceManager, contextCacheStore)

	// Register Workspace Handlers
	rest.InitRestWorkspace(apiGroup, wkUsecase, workspaceManager, appUsecase)

	// Register Client Handlers
	clientHandler.RegisterRoutes(apiGroup)

	// Websocket
	websocket.SetValkeyClient(vkClient, serverID)
	websocket.RegisterRoutes(apiGroup, appUsecase)
	go websocket.RunHub()

	// 404 Handler ONLY for API group to prevent fallthrough to SPA fallback
	apiGroup.All("/*", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "API Endpoint not found",
			"path":  c.Path(),
		})
	})

	// Static assets from frontend/dist
	app.Use(coreconfig.Global.App.BasePath+"/", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedFrontend),
		PathPrefix: "frontend/dist",
		Browse:     false,
		Index:      "index.html",
	}))

	// SPA Fallback: Serve index.html for any unknown routes
	app.Get(coreconfig.Global.App.BasePath+"/*", func(c *fiber.Ctx) error {
		path := c.Path()
		// Only serve index.html for non-API and non-static routes
		// If it has a dot, it's a file that should have been caught by the filesystem middleware.
		// If it's not a file and not an API route, it's a frontend route.
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/statics") || strings.Contains(path, ".") {
			return c.Next()
		}

		file, err := EmbedFrontend.ReadFile("frontend/dist/index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Frontend not found")
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(file)
	})

	if err := app.Listen(":" + coreconfig.Global.App.Port); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}

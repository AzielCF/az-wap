package cmd

import (
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	globalConfig "github.com/AzielCF/az-wap/config"
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
	rootCmd.AddCommand(restCmd)
}
func restServer(_ *cobra.Command, _ []string) {
	fiberConfig := fiber.Config{
		EnableTrustedProxyCheck: true,
		BodyLimit:               int(globalConfig.WhatsappSettingMaxVideoSize),
		Network:                 "tcp",
		AppName:                 "Az-Wap Enterprise Engine",
		DisableStartupMessage:   false, // Keep generic startup message
		ServerHeader:            "Hidden",
	}

	// Configure proxy settings if trusted proxies are specified
	if len(globalConfig.AppTrustedProxies) > 0 {
		fiberConfig.TrustedProxies = globalConfig.AppTrustedProxies
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedHost
	}

	app := fiber.New(fiberConfig)

	// Security: RequestID for audit trails
	app.Use(requestid.New())

	// Security: Strict CORS
	// In production, this should be restricted to the actual frontend domain.
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000, http://localhost:5173, " + globalConfig.AppBaseUrl,
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
		ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self' http://localhost:* ws://localhost:*;",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 30 * time.Second,
	}))

	if globalConfig.AppDebug {
		app.Use(logger.New())
	}

	if len(globalConfig.AppBasicAuthCredential) == 0 {
		logrus.Fatalln("APP_BASIC_AUTH is required. Nothing should be public; please set APP_BASIC_AUTH=<user>:<secret>[,<user2>:<secret2>] and restart.")
	}

	account := make(map[string]string)
	for _, basicAuth := range globalConfig.AppBasicAuthCredential {
		ba := strings.Split(basicAuth, ":")
		if len(ba) != 2 {
			logrus.Fatalln("Basic auth is not valid, please this following format <user>:<secret>")
		}
		account[ba[0]] = ba[1]
	}

	// System statics
	app.Static(globalConfig.AppBasePath+"/statics", "./statics")

	// Create API group
	apiGroup := app.Group(globalConfig.AppBasePath + "/api")

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
	rest.InitRestChat(apiGroup, chatUsecase)
	rest.InitRestSend(apiGroup, sendUsecase)
	rest.InitRestUser(apiGroup, userUsecase)
	rest.InitRestMessage(apiGroup, messageUsecase)
	rest.InitRestGroup(apiGroup, groupUsecase)
	rest.InitRestNewsletter(apiGroup, newsletterUsecase)
	rest.InitRestBot(apiGroup, botUsecase, mcpUsecase)
	// rest.InitRestInstance(apiGroup, instanceUsecase, sendUsecase) // DEPRECATED
	rest.InitChannelAPI(apiGroup, wkUsecase, workspaceManager, sendUsecase)
	rest.InitRestCredential(apiGroup, credentialUsecase)
	rest.InitRestCache(apiGroup, cacheUsecase)
	rest.InitRestMCP(apiGroup, mcpUsecase)
	rest.InitRestHealth(apiGroup, healthUsecase)

	// Worker Pool monitoring endpoint
	apiGroup.Get("/worker-pool/stats", rest.GetWorkerPoolStats)
	apiGroup.Get("/bot-webhook-pool/stats", rest.GetBotWebhookPoolStats)

	// Bot monitor endpoint
	apiGroup.Get("/bot-monitor/stats", rest.GetBotMonitorStats)
	apiGroup.Get("/monitoring/typing", rest.GetTypingStatus)

	// Register Workspace Handlers
	rest.InitRestWorkspace(apiGroup, wkUsecase, workspaceManager, appUsecase)

	// Websocket
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
	app.Use(globalConfig.AppBasePath+"/", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedFrontend),
		PathPrefix: "frontend/dist",
		Browse:     false,
		Index:      "index.html",
	}))

	// SPA Fallback: Serve index.html for any unknown routes
	app.Get(globalConfig.AppBasePath+"/*", func(c *fiber.Ctx) error {
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

	if err := app.Listen(":" + globalConfig.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}

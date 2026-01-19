package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	globalConfig "github.com/AzielCF/az-wap/config"
	"github.com/AzielCF/az-wap/ui/rest"
	"github.com/AzielCF/az-wap/ui/rest/middleware"
	"github.com/AzielCF/az-wap/ui/websocket"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
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
	engine := html.NewFileSystem(http.FS(EmbedIndex), ".html")
	engine.AddFunc("isEnableBasicAuth", func(token any) bool {
		return token != nil
	})
	fiberConfig := fiber.Config{
		Views:                   engine,
		EnableTrustedProxyCheck: true,
		BodyLimit:               int(globalConfig.WhatsappSettingMaxVideoSize),
		Network:                 "tcp",
	}

	// Configure proxy settings if trusted proxies are specified
	if len(globalConfig.AppTrustedProxies) > 0 {
		fiberConfig.TrustedProxies = globalConfig.AppTrustedProxies
		fiberConfig.ProxyHeader = fiber.HeaderXForwardedHost
	}

	app := fiber.New(fiberConfig)

	app.Use(middleware.Recovery())
	app.Use(middleware.BasicAuth())
	if globalConfig.AppDebug {
		app.Use(logger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Instance-Token",
	}))

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

	app.Use(basicauth.New(basicauth.Config{
		Users: account,
		Next: func(c *fiber.Ctx) bool {
			// Allow CORS preflight without credentials.
			if c.Method() == fiber.MethodOptions {
				return true
			}
			// Allow Chatwoot webhooks without BasicAuth. They are authenticated at handler-level.
			path := c.Path()
			basePath := globalConfig.AppBasePath
			if basePath != "" && !strings.HasPrefix(basePath, "/") {
				basePath = "/" + basePath
			}
			prefix := basePath + "/instances/"
			suffix := "/chatwoot/webhook"
			if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
				return true
			}
			return false
		},
	}))

	// Static assets should also be protected by auth. Register them AFTER auth middleware.
	app.Static(globalConfig.AppBasePath+"/statics", "./statics")
	app.Use(globalConfig.AppBasePath+"/components", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/components",
		Browse:     true,
	}))
	app.Use(globalConfig.AppBasePath+"/assets", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/assets",
		Browse:     true,
	}))

	// Create base path group or use app directly
	var apiGroup fiber.Router = app
	if globalConfig.AppBasePath != "" {
		apiGroup = app.Group(globalConfig.AppBasePath)
	}

	// Rest
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
	apiGroup.Get("/api/worker-pool/stats", rest.GetWorkerPoolStats)
	apiGroup.Get("/api/bot-webhook-pool/stats", rest.GetBotWebhookPoolStats)

	// Bot monitor endpoint
	apiGroup.Get("/api/bot-monitor/stats", rest.GetBotMonitorStats)
	apiGroup.Get("/api/monitoring/typing", rest.GetTypingStatus)

	apiGroup.Get("/", func(c *fiber.Ctx) error {
		return c.Render("views/index", fiber.Map{
			"AppHost":        fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname()),
			"AppVersion":     globalConfig.AppVersion,
			"AppBasePath":    globalConfig.AppBasePath,
			"BasicAuthToken": c.UserContext().Value(middleware.AuthorizationValue("BASIC_AUTH")),
			"MaxFileSize":    humanize.Bytes(uint64(globalConfig.WhatsappSettingMaxFileSize)),
			"MaxVideoSize":   humanize.Bytes(uint64(globalConfig.WhatsappSettingMaxVideoSize)),
		})
	})

	websocket.RegisterRoutes(apiGroup, appUsecase)
	go websocket.RunHub()

	// Set auto reconnect to whatsapp server after booting
	// go helpers.SetAutoConnectAfterBooting(appUsecase) // Disabled Legacy AutoConnect

	// Set auto reconnect checking with a guaranteed client instance
	// startAutoReconnectCheckerIfClientAvailable() // Disabled Legacy AutoCheck

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
	rest.InitRestWorkspace(apiGroup, wkUsecase, workspaceManager, appUsecase)

	if err := app.Listen(":" + globalConfig.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}

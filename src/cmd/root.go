package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/botengine"
	botUsecaseLayer "github.com/AzielCF/az-wap/botengine/application"
	"github.com/AzielCF/az-wap/botengine/domain"
	botengineDomain "github.com/AzielCF/az-wap/botengine/domain"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/botengine/providers"
	botTools "github.com/AzielCF/az-wap/botengine/tools"
	globalConfig "github.com/AzielCF/az-wap/config"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	domainChat "github.com/AzielCF/az-wap/domains/chat"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	domainGroup "github.com/AzielCF/az-wap/domains/group"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	domainMessage "github.com/AzielCF/az-wap/domains/message"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	domainSend "github.com/AzielCF/az-wap/domains/send"
	domainUser "github.com/AzielCF/az-wap/domains/user"
	"github.com/AzielCF/az-wap/infrastructure/chatstorage"

	whatsappadapter "github.com/AzielCF/az-wap/infrastructure/whatsapp/adapter"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/utils"
	uiRest "github.com/AzielCF/az-wap/ui/rest"
	"github.com/AzielCF/az-wap/usecase"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/repository"
	workspaceUsecaseLayer "github.com/AzielCF/az-wap/workspace/usecase"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
	// "go.mau.fi/whatsmeow/store/sqlstore"
)

var (
	EmbedIndex embed.FS
	EmbedViews embed.FS

	// Whatsapp
	whatsappCli *whatsmeow.Client

	// Usecase
	appUsecase        domainApp.IAppUsecase
	chatUsecase       domainChat.IChatUsecase
	sendUsecase       domainSend.ISendUsecase
	userUsecase       domainUser.IUserUsecase
	messageUsecase    domainMessage.IMessageUsecase
	groupUsecase      domainGroup.IGroupUsecase
	newsletterUsecase domainNewsletter.INewsletterUsecase
	botUsecase        domainBot.IBotUsecase
	credentialUsecase domainCredential.ICredentialUsecase
	// instanceUsecase   domainInstance.IInstanceUsecase
	cacheUsecase  domainCache.ICacheUsecase
	mcpUsecase    domainMCP.IMCPUsecase
	healthUsecase domainHealth.IHealthUsecase

	// Bot Engine
	botEngine *botengine.Engine

	// Workspace
	workspaceDB      *sql.DB
	wkRepo           repository.IWorkspaceRepository
	workspaceManager *workspace.Manager
	wkUsecase        *workspaceUsecaseLayer.WorkspaceUsecase
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Short: "Send free whatsapp API",
	Long: `This application is from clone https://az-wap, 
you can send whatsapp over http api but your whatsapp account have to be multi device version`,
}

func init() {
	// Load environment variables first
	utils.LoadConfig(".")

	time.Local = time.UTC

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Initialize flags first, before any subcommands are added
	initFlags()

	// Then initialize other components
	cobra.OnInitialize(initEnvConfig, initApp)
}

// initEnvConfig loads configuration from environment variables
func initEnvConfig() {
	fmt.Println(viper.AllSettings())
	// Application settings
	if envPort := viper.GetString("app_port"); envPort != "" {
		globalConfig.AppPort = envPort
	}
	if envDebug := viper.GetBool("app_debug"); envDebug {
		globalConfig.AppDebug = envDebug
	}
	if envOs := viper.GetString("app_os"); envOs != "" {
		globalConfig.AppOs = envOs
	}
	envBasicAuth := viper.GetString("app_basic_auth")
	if envBasicAuth == "" {
		envBasicAuth = os.Getenv("APP_BASIC_AUTH")
	}
	if envBasicAuth != "" {
		credential := strings.Split(envBasicAuth, ",")
		globalConfig.AppBasicAuthCredential = credential
	}
	if envBasePath := viper.GetString("app_base_path"); envBasePath != "" {
		globalConfig.AppBasePath = envBasePath
	}
	if envTrustedProxies := viper.GetString("app_trusted_proxies"); envTrustedProxies != "" {
		proxies := strings.Split(envTrustedProxies, ",")
		globalConfig.AppTrustedProxies = proxies
	}

	// Database settings
	if envDBURI := viper.GetString("db_uri"); envDBURI != "" {
		globalConfig.DBURI = envDBURI
	}
	if envDBKEYSURI := viper.GetString("db_keys_uri"); envDBKEYSURI != "" {
		globalConfig.DBKeysURI = envDBKEYSURI
	}

	// WhatsApp settings
	if envAutoReply := viper.GetString("whatsapp_auto_reply"); envAutoReply != "" {
		trimmed := strings.TrimSpace(envAutoReply)
		trimmed = strings.Trim(trimmed, "\"'")
		if trimmed != "" && !strings.EqualFold(trimmed, "Auto reply message") {
			globalConfig.WhatsappAutoReplyMessage = envAutoReply
		}
	}
	if viper.IsSet("whatsapp_auto_mark_read") {
		globalConfig.WhatsappAutoMarkRead = viper.GetBool("whatsapp_auto_mark_read")
	}
	if viper.IsSet("whatsapp_auto_download_media") {
		globalConfig.WhatsappAutoDownloadMedia = viper.GetBool("whatsapp_auto_download_media")
	}
	if envWebhook := viper.GetString("whatsapp_webhook"); envWebhook != "" {
		webhook := strings.Split(envWebhook, ",")
		globalConfig.WhatsappWebhook = webhook
	}
	if envWebhookSecret := viper.GetString("whatsapp_webhook_secret"); envWebhookSecret != "" {
		globalConfig.WhatsappWebhookSecret = envWebhookSecret
	}
	if viper.IsSet("whatsapp_webhook_insecure_skip_verify") {
		globalConfig.WhatsappWebhookInsecureSkipVerify = viper.GetBool("whatsapp_webhook_insecure_skip_verify")
	}
	if viper.IsSet("whatsapp_account_validation") {
		globalConfig.WhatsappAccountValidation = viper.GetBool("whatsapp_account_validation")
	}
}

func initFlags() {
	// Application flags
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.AppPort,
		"port", "p",
		globalConfig.AppPort,
		"change port number with --port <number> | example: --port=8080",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&globalConfig.AppDebug,
		"debug", "d",
		globalConfig.AppDebug,
		"hide or displaying log with --debug <true/false> | example: --debug=true",
	)
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.AppOs,
		"os", "",
		globalConfig.AppOs,
		`os name --os <string> | example: --os="Chrome"`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&globalConfig.AppBasicAuthCredential,
		"basic-auth", "b",
		globalConfig.AppBasicAuthCredential,
		"basic auth credential | -b=yourUsername:yourPassword",
	)
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.AppBasePath,
		"base-path", "",
		globalConfig.AppBasePath,
		`base path for subpath deployment --base-path <string> | example: --base-path="/gowa"`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&globalConfig.AppTrustedProxies,
		"trusted-proxies", "",
		globalConfig.AppTrustedProxies,
		`trusted proxy IP ranges for reverse proxy deployments --trusted-proxies <string> | example: --trusted-proxies="0.0.0.0/0" or --trusted-proxies="10.0.0.0/8,172.16.0.0/12"`,
	)

	// Database flags
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.DBURI,
		"db-uri", "",
		globalConfig.DBURI,
		`the database uri to store the connection data database uri (by default, we'll use sqlite3 under storages/whatsapp.db). database uri --db-uri <string> | example: --db-uri="file:storages/whatsapp.db?_foreign_keys=on or postgres://user:password@localhost:5432/whatsapp"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.DBKeysURI,
		"db-keys-uri", "",
		globalConfig.DBKeysURI,
		`the database uri to store the keys database uri (by default, we'll use the same database uri). database uri --db-keys-uri <string> | example: --db-keys-uri="file::memory:?cache=shared&_foreign_keys=on"`,
	)

	// WhatsApp flags
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.WhatsappAutoReplyMessage,
		"autoreply", "",
		globalConfig.WhatsappAutoReplyMessage,
		`auto reply when received message --autoreply <string> | example: --autoreply="Don't reply this message"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&globalConfig.WhatsappAutoMarkRead,
		"auto-mark-read", "",
		globalConfig.WhatsappAutoMarkRead,
		`auto mark incoming messages as read --auto-mark-read <true/false> | example: --auto-mark-read=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&globalConfig.WhatsappAutoDownloadMedia,
		"auto-download-media", "",
		globalConfig.WhatsappAutoDownloadMedia,
		`auto download media from incoming messages --auto-download-media <true/false> | example: --auto-download-media=false`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&globalConfig.WhatsappWebhook,
		"webhook", "w",
		globalConfig.WhatsappWebhook,
		`forward event to webhook --webhook <string> | example: --webhook="https://yourcallback.com/callback"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&globalConfig.WhatsappWebhookSecret,
		"webhook-secret", "",
		globalConfig.WhatsappWebhookSecret,
		`secure webhook request --webhook-secret <string> | example: --webhook-secret="super-secret-key"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&globalConfig.WhatsappWebhookInsecureSkipVerify,
		"webhook-insecure-skip-verify", "",
		globalConfig.WhatsappWebhookInsecureSkipVerify,
		`skip TLS certificate verification for webhooks (INSECURE - use only for development/self-signed certs) --webhook-insecure-skip-verify <true/false> | example: --webhook-insecure-skip-verify=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&globalConfig.WhatsappAccountValidation,
		"account-validation", "",
		globalConfig.WhatsappAccountValidation,
		`enable or disable account validation --account-validation <true/false> | example: --account-validation=true`,
	)

	// Message Worker Pool flags
	rootCmd.PersistentFlags().IntVarP(
		&globalConfig.MessageWorkerPoolSize,
		"message-workers", "",
		globalConfig.MessageWorkerPoolSize,
		`number of concurrent message workers --message-workers <number> | example: --message-workers=30 (default: 20)`,
	)
	rootCmd.PersistentFlags().IntVarP(
		&globalConfig.MessageWorkerQueueSize,
		"message-queue-size", "",
		globalConfig.MessageWorkerQueueSize,
		`queue size per message worker --message-queue-size <number> | example: --message-queue-size=1500 (default: 1000)`,
	)
}

func initApp() {
	if globalConfig.AppDebug {
		globalConfig.WhatsappLogLevel = "DEBUG"
		logrus.SetLevel(logrus.DebugLevel)
	}

	//preparing folder if not exist
	err := utils.CreateFolder(globalConfig.PathQrCode, globalConfig.PathSendItems, globalConfig.PathStorages, globalConfig.PathCacheMedia)
	if err != nil {
		logrus.Errorln(err)
	}

	ctx := context.Background()

	// Workspace Initialization
	workspaceDBPath := "storages/workspaces.db"
	workspaceDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL", workspaceDBPath))
	if err != nil {
		logrus.Fatalf("failed to open workspace db: %v", err)
	}

	wkRepo = repository.NewSQLiteRepository(workspaceDB)
	if err := wkRepo.Init(ctx); err != nil {
		logrus.Fatalf("failed to init workspace repo: %v", err)
	}

	// 1. Basic Usecases (No complex dependencies)
	credentialUsecase = usecase.NewCredentialService()
	botUsecase = botUsecaseLayer.NewBotService(credentialUsecase)
	cacheUsecase = usecase.NewCacheService()
	cacheUsecase.StartBackgroundCleanup(ctx)
	mcpUsecase = botUsecaseLayer.NewMCPService()

	// 2. Bot Engine Initialization (Needs BotUsecase, MCPUsecase)
	botEngine = botengine.NewEngine(botUsecase, mcpUsecase)
	geminiProvider := providers.NewGeminiProvider(mcpUsecase)
	botEngine.RegisterProvider(string(domainBot.ProviderAI), geminiProvider)
	botEngine.RegisterProvider(string(domainBot.ProviderGemini), geminiProvider)
	// botEngine.RegisterProvider(string(domainBot.ProviderOpenAI), geminiProvider)
	// botEngine.RegisterProvider(string(domainBot.ProviderClaude), geminiProvider)

	// 3. Workspace Manager (Needs wkRepo, BotEngine)
	workspaceManager = workspace.NewManager(wkRepo, botEngine)

	// 4. Domain Usecases (Need WorkspaceManager)
	// instanceUsecase = usecase.NewInstanceService(workspaceManager, wkRepo) // DEPRECATED
	appUsecase = usecase.NewAppService(workspaceManager)
	chatUsecase = usecase.NewChatService(workspaceManager)
	userUsecase = usecase.NewUserService(workspaceManager)
	groupUsecase = usecase.NewGroupService(workspaceManager)
	newsletterUsecase = usecase.NewNewsletterService(workspaceManager, wkRepo)
	sendUsecase = usecase.NewSendService(appUsecase, workspaceManager)
	messageUsecase = usecase.NewMessageService(workspaceManager)
	wkUsecase = workspaceUsecaseLayer.NewWorkspaceUsecase(wkRepo, workspaceManager)

	// 5. WhatsApp Adapter Factory
	workspaceManager.RegisterFactory(channel.ChannelTypeWhatsApp, func(conf channel.ChannelConfig) (channel.ChannelAdapter, error) {
		channelID, _ := conf.Settings["channel_id"].(string)
		workspaceID, _ := conf.Settings["workspace_id"].(string)
		instanceID, _ := conf.Settings["instance_id"].(string)
		if channelID == "" || workspaceID == "" {
			return nil, fmt.Errorf("channel_id or workspace_id missing in channel settings")
		}
		return whatsappadapter.NewAdapter(channelID, workspaceID, instanceID, nil, workspaceManager), nil
	})

	// 6. Post-initialization
	healthUsecase = usecase.NewHealthService(mcpUsecase, credentialUsecase, botUsecase, workspaceManager, wkUsecase)
	mcpUsecase.SetHealthUsecase(healthUsecase)
	healthUsecase.StartPeriodicChecks(ctx)
	uiRest.SetBotEngine(botEngine, workspaceManager)

	// Hooks (Bot Engine depends on wkRepo)
	botEngine.RegisterPostReplyHook(func(ctx context.Context, b domainBot.Bot, input botengineDomain.BotInput, output botengineDomain.BotOutput) {
		// 1. Acumular costos en DB (Independiente de phone)
		if output.TotalCost > 0 && input.InstanceID != "" {
			breakdown := make(map[string]float64)
			for _, d := range output.CostDetails {
				key := d.BotID + ":" + d.Model
				breakdown[key] += d.Cost
			}
			if err := wkRepo.AddChannelComplexCost(ctx, input.InstanceID, output.TotalCost, breakdown); err != nil {
				logrus.WithError(err).Error("[ENGINE] Failed to accumulate channel cost")
			}
		}

		// 2. Notificaciones Chatwoot (Dependiente de phone)
		phone, _ := input.Metadata["phone"].(string)
		if phone == "" || input.InstanceID == "" {
			return
		}

		ch, err := wkRepo.GetChannel(ctx, input.InstanceID)
		if err != nil {
			go chatwoot.ForwardBotReplyFromEvent(ctx, input.InstanceID, phone, output.Text)
			return
		}
		if ch.Config.Chatwoot != nil && ch.Config.Chatwoot.Enabled {
			cwCfg := &chatwoot.Config{
				InstanceID:         ch.ID,
				BaseURL:            ch.Config.Chatwoot.URL,
				AccountID:          int64(ch.Config.Chatwoot.AccountID),
				InboxID:            int64(ch.Config.Chatwoot.InboxID),
				AccountToken:       ch.Config.Chatwoot.Token,
				BotToken:           ch.Config.Chatwoot.BotToken,
				InboxIdentifier:    ch.Config.Chatwoot.InboxIdentifier,
				InsecureSkipVerify: ch.Config.SkipTLSVerification,
				Enabled:            ch.Config.Chatwoot.Enabled,
			}
			go chatwoot.ForwardBotReplyWithConfig(ctx, cwCfg, phone, output.Text)
		} else {
			go chatwoot.ForwardBotReplyFromEvent(ctx, input.InstanceID, phone, output.Text)
		}
	})

	// Register Default Native Tools
	sessionTool := botTools.NewSessionResourcesTool()
	botEngine.RegisterNativeTool(&domain.NativeTool{
		Tool:    sessionTool.Tool,
		Handler: sessionTool.Handler,
	})

	analyzeTool := botTools.NewAnalyzeSessionResourceTool()
	botEngine.RegisterNativeTool(&domain.NativeTool{
		Tool:    analyzeTool.Tool,
		Handler: analyzeTool.Handler,
	})

	terminateTool := botTools.NewTerminateSessionTool()
	botEngine.RegisterNativeTool(&domain.NativeTool{
		Tool:    terminateTool.Tool,
		Handler: terminateTool.Handler,
	})

	// Register Newsletter Tools
	nTools := botTools.NewNewsletterTools(newsletterUsecase, workspaceManager)
	botEngine.RegisterNativeTool(nTools.ListNewslettersTool())
	botEngine.RegisterNativeTool(nTools.SchedulePostTool())

	gTools := botTools.NewGroupTools(workspaceManager)
	botEngine.RegisterNativeTool(gTools.ListGroupsTool())

	// Register Reminder Tools
	rTools := botTools.NewReminderTools(newsletterUsecase)
	botEngine.RegisterNativeTool(rTools.ScheduleReminderTool())
	botEngine.RegisterNativeTool(rTools.ListRemindersTool())

	// Workspace Channels Auto-Start
	go func() {
		time.Sleep(5 * time.Second) // Small delay to ensure all infrastructure is ready
		if err := wkUsecase.StartEnabledChannels(ctx, workspaceManager); err != nil {
			logrus.WithError(err).Error("[WORKSPACE] Failed to auto-start enabled channels")
		}
	}()

	// Start Newsletter Scheduler
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := newsletterUsecase.ProcessScheduledPosts(context.Background()); err != nil {
				logrus.WithError(err).Error("[SCHEDULER] Failed to process posts")
			}
		}
	}()

}

func GetBotEngine() *botengine.Engine {
	return botEngine
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(embedIndex embed.FS, embedViews embed.FS) {
	EmbedIndex = embedIndex
	EmbedViews = embedViews
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// StopApp performs a clean shutdown of all database connections and services.
func StopApp() {
	logrus.Info("[APP] Stopping application...")

	// 1. Shutdown WhatsApp subsystem (handles all instance clients and their DBs)
	// whatsapp.GracefulShutdown() // Deleted via refactor
	// TODO: Stop all workspace channels if manager exposes a method
	if workspaceManager != nil {
		// workspaceManager.StopAll()
	}

	// 2. Clear in-memory chat storage caches or close connections
	chatstorage.CloseInstanceRepositories()

	// 3. Shutdown MCP Usecase (closes persistent SSE connections)
	if mcpUsecase != nil {
		mcpUsecase.Shutdown()
	}

	logrus.Info("[APP] Application stopped cleanly.")
}

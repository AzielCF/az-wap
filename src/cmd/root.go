/*
AZ-WAP - Open Source WhatsApp Web API
Copyright (C) 2025-2026 Aziel Cruzado <contacto@azielcruzado.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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
	botengineRepo "github.com/AzielCF/az-wap/botengine/repository"
	botTools "github.com/AzielCF/az-wap/botengine/tools"
	onlyClients "github.com/AzielCF/az-wap/botengine/tools/only-clients"
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
	"github.com/AzielCF/az-wap/infrastructure/valkey"

	botDomain "github.com/AzielCF/az-wap/botengine/domain"
	whatsappadapter "github.com/AzielCF/az-wap/infrastructure/whatsapp/adapter"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/botmonitor"
	"github.com/AzielCF/az-wap/pkg/utils"
	uiRest "github.com/AzielCF/az-wap/ui/rest"
	"github.com/AzielCF/az-wap/usecase"
	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/monitoring"
	"github.com/AzielCF/az-wap/workspace/repository"
	workspaceUsecaseLayer "github.com/AzielCF/az-wap/workspace/usecase"

	// Clients Module
	clientsRest "github.com/AzielCF/az-wap/clients/adapter/rest"
	clientsApp "github.com/AzielCF/az-wap/clients/application"
	clientsRepo "github.com/AzielCF/az-wap/clients/repository"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
	// "go.mau.fi/whatsmeow/store/sqlstore"
)

var (
	EmbedFrontend embed.FS

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
	workspaceDB       *sql.DB
	wkRepo            repository.IWorkspaceRepository
	workspaceManager  *workspace.Manager
	wkUsecase         *workspaceUsecaseLayer.WorkspaceUsecase
	typingStore       channel.TypingStore
	monitorStore      monitoring.MonitoringStore
	contextCacheStore botDomain.ContextCacheStore

	// Clients Module
	appDB          *sql.DB
	clientService  *clientsApp.ClientService
	subService     *clientsApp.SubscriptionService
	clientResolver *clientsApp.ClientResolver
	clientHandler  *clientsRest.ClientHandler

	// Shared Infra
	vkClient *valkey.Client
	serverID string
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
	// Bind environment variables to viper keys
	viper.BindEnv("app_port", "APP_PORT")
	viper.BindEnv("app_debug", "APP_DEBUG")
	viper.BindEnv("debug", "DEBUG")
	viper.BindEnv("whatsapp_log_level", "WHATSAPP_LOG_LEVEL")
	viper.BindEnv("db_uri", "DB_URI")
	viper.BindEnv("db_keys_uri", "DB_KEYS_URI")

	// Valkey settings
	viper.BindEnv("valkey_enabled", "VALKEY_ENABLED")
	viper.BindEnv("valkey_address", "VALKEY_ADDRESS")
	viper.BindEnv("valkey_password", "VALKEY_PASSWORD")
	viper.BindEnv("valkey_db", "VALKEY_DB")
	viper.BindEnv("valkey_key_prefix", "VALKEY_KEY_PREFIX")

	// If config already loaded variables via os.Getenv in init(), we sync them with viper if needed
	// or just ensure viper doesn't overwrite them with empty defaults

	// Application settings
	if envPort := viper.GetString("app_port"); envPort != "" {
		globalConfig.AppPort = envPort
	}
	if viper.IsSet("app_debug") {
		globalConfig.AppDebug = viper.GetBool("app_debug")
	} else if viper.IsSet("debug") {
		globalConfig.AppDebug = viper.GetBool("debug")
	}

	if envLogLevel := viper.GetString("whatsapp_log_level"); envLogLevel != "" {
		globalConfig.WhatsappLogLevel = strings.ToUpper(envLogLevel)
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
	if envBaseUrl := viper.GetString("app_base_url"); envBaseUrl != "" {
		globalConfig.AppBaseUrl = envBaseUrl
	}
	if envCors := viper.GetString("app_cors_allowed_origins"); envCors != "" {
		globalConfig.AppCorsAllowedOrigins = envCors
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

	// Valkey settings sync
	if viper.IsSet("valkey_enabled") {
		globalConfig.ValkeyEnabled = viper.GetBool("valkey_enabled")
	}
	if v := viper.GetString("valkey_address"); v != "" {
		globalConfig.ValkeyAddress = v
	}
	if v := viper.GetString("valkey_password"); v != "" {
		globalConfig.ValkeyPassword = v
	}
	if viper.IsSet("valkey_db") {
		globalConfig.ValkeyDB = viper.GetInt("valkey_db")
	}
	if v := viper.GetString("valkey_key_prefix"); v != "" {
		globalConfig.ValkeyKeyPrefix = v
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
	// Generate or Load a persistent unique ID for this server instance
	serverID = utils.GetPersistentServerID(globalConfig.AppServerID, globalConfig.PathStorages)

	// Priority: Explicit WHATSAPP_LOG_LEVEL > APP_DEBUG logic
	if globalConfig.AppDebug {
		logrus.SetLevel(logrus.DebugLevel)
		if globalConfig.WhatsappLogLevel == "" || globalConfig.WhatsappLogLevel == "ERROR" {
			globalConfig.WhatsappLogLevel = "INFO" // For WhatsApp, INFO is enough for debug without being binary-heavy
		}
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Set specific Level for WhatsApp if provided
	if globalConfig.WhatsappLogLevel != "" {
		// This will be used when creating adapters
	}

	//preparing folder if not exist
	err := utils.CreateFolder(globalConfig.PathSendItems, globalConfig.PathStorages)
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

	// Initialize Shared Infrastrucutre (Valkey)
	if globalConfig.ValkeyEnabled {
		var vkErr error
		vkClient, vkErr = valkey.NewClient(valkey.Config{
			Address:        globalConfig.ValkeyAddress,
			Password:       globalConfig.ValkeyPassword,
			DB:             globalConfig.ValkeyDB,
			KeyPrefix:      globalConfig.ValkeyKeyPrefix,
			ConnectTimeout: 5 * time.Second,
		})
		if vkErr != nil {
			logrus.WithError(vkErr).Warn("[STARTUP] Failed to connect to Valkey, some features will use in-memory fallback")
		}
	}

	// Initialize context cache store for AI providers
	if vkClient != nil {
		contextCacheStore = botengineRepo.NewValkeyContextCacheStore(vkClient)
		logrus.Info("[STARTUP] Using Valkey for AI context caching")
	} else {
		contextCacheStore = botengineRepo.NewMemoryContextCacheStore()
		logrus.Info("[STARTUP] Using in-memory store for AI context caching")
	}
	geminiProvider := providers.NewGeminiProvider(mcpUsecase, contextCacheStore)
	openaiProvider := providers.NewOpenAIProvider(mcpUsecase)

	botEngine.RegisterProvider(string(domainBot.ProviderAI), geminiProvider)
	botEngine.RegisterProvider(string(domainBot.ProviderGemini), geminiProvider)
	botEngine.RegisterProvider(string(domainBot.ProviderOpenAI), openaiProvider)
	// botEngine.RegisterProvider(string(domainBot.ProviderClaude), geminiProvider)

	// 2.1 Clients Module Initialization (Needed for Workspace Manager)
	appDBPath := "storages/app.db"
	appDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL", appDBPath))
	if err != nil {
		logrus.Fatalf("failed to open app db: %v", err)
	}

	// Client Repository (app.db)
	clientRepo := clientsRepo.NewSQLiteClientRepository(appDB)
	if err := clientRepo.InitSchema(ctx); err != nil {
		logrus.Fatalf("failed to init client repo: %v", err)
	}

	// Subscription Repository (workspaces.db) - reuse workspaceDB opened earlier
	subRepo := clientsRepo.NewSQLiteSubscriptionRepository(workspaceDB)
	if err := subRepo.InitSchema(ctx); err != nil {
		logrus.Fatalf("failed to init subscription repo: %v", err)
	}

	// Client Services
	clientService = clientsApp.NewClientService(clientRepo, subRepo)
	subService = clientsApp.NewSubscriptionService(subRepo, clientRepo)

	// Client Resolver (for runtime context resolution)
	clientResolver = clientsApp.NewClientResolver(clientRepo, subRepo, wkRepo)

	// Client REST Handler
	clientHandler = clientsRest.NewClientHandler(clientService, subService)

	logrus.Info("[CLIENTS] Client module initialized successfully")

	// 3. Monitoring and Typing (Distributed if Valkey is available)
	if vkClient != nil {
		typingStore = repository.NewValkeyTypingStore(vkClient)
		monitorStore = repository.NewValkeyMonitoringStore(vkClient)
		botmonitor.SetValkeyClient(vkClient, serverID) // Sync events across nodes
		logrus.Info("[STARTUP] Using Valkey for distributed Monitoring and Typing")
	} else {
		typingStore = repository.NewMemoryTypingStore()
		monitorStore = repository.NewMemoryMonitoringStore()
		logrus.Info("[STARTUP] Using in-memory stores for Monitoring and Typing")
	}

	// 4. Workspace Manager (Needs wkRepo, BotEngine, ClientResolver, stores, serverID)
	workspaceManager = workspace.NewManager(wkRepo, botEngine, clientResolver, typingStore, monitorStore, vkClient, serverID)

	// 5. Connect Bot Monitor to Cluster Stats
	botmonitor.OnIncrement = func(key string) {
		_ = monitorStore.IncrementStat(ctx, key)
	}

	// 4. Domain Usecases (Need WorkspaceManager)
	// instanceUsecase = usecase.NewInstanceService(workspaceManager, wkRepo) // DEPRECATED
	appUsecase = usecase.NewAppService(workspaceManager)
	chatUsecase = usecase.NewChatService(workspaceManager)
	userUsecase = usecase.NewUserService(workspaceManager)
	groupUsecase = usecase.NewGroupService(workspaceManager)
	newsletterUsecase = usecase.NewNewsletterService(workspaceManager, wkRepo, monitorStore, vkClient)
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
	nTools := onlyClients.NewNewsletterTools(newsletterUsecase, workspaceManager)
	botEngine.RegisterNativeTool(nTools.ListNewslettersTool())
	botEngine.RegisterNativeTool(nTools.SchedulePostTool())

	gTools := onlyClients.NewGroupTools(workspaceManager)
	botEngine.RegisterNativeTool(gTools.ListGroupsTool())

	// Register Reminder Tools
	rTools := onlyClients.NewReminderTools(newsletterUsecase)
	botEngine.RegisterNativeTool(rTools.ScheduleReminderTool())
	botEngine.RegisterNativeTool(rTools.ListRemindersTool())

	// Register Client Profile Tools (allow users to manage their personal info via AI)
	cTools := onlyClients.NewClientTools(clientRepo)
	botEngine.RegisterNativeTool(cTools.UpdateMyInfoTool())
	botEngine.RegisterNativeTool(cTools.GetMyInfoTool())
	botEngine.RegisterNativeTool(cTools.DeleteMyFieldTool())

	// Workspace Channels Auto-Start
	go func() {
		time.Sleep(5 * time.Second) // Small delay to ensure all infrastructure is ready
		if err := wkUsecase.StartEnabledChannels(ctx, workspaceManager); err != nil {
			logrus.WithError(err).Error("[WORKSPACE] Failed to auto-start enabled channels")
		}
	}()

	// Start Newsletter Scheduler (Promoter and Worker)
	go func() {
		ticker := time.NewTicker(1 * time.Hour) // Promoter runs once per hour
		defer ticker.Stop()
		// Initial promotion run
		_ = newsletterUsecase.ProcessScheduledPosts(context.Background())
		for range ticker.C {
			if err := newsletterUsecase.ProcessScheduledPosts(context.Background()); err != nil {
				logrus.WithError(err).Error("[SCHEDULER] Failed to process posts")
			}
		}
	}()

	go func() {
		if err := newsletterUsecase.RunTaskWorker(ctx); err != nil {
			logrus.WithError(err).Error("[SCHEDULER] Worker failed")
		}
	}()

}

func GetBotEngine() *botengine.Engine {
	return botEngine
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(embedFrontend embed.FS) {
	EmbedFrontend = embedFrontend
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

	// 4. Report shutdown to monitoring then close Valkey
	if monitorStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = monitorStore.RemoveServer(ctx, serverID)
		cancel()
	}

	if vkClient != nil {
		vkClient.Close()
	}

	logrus.Info("[APP] Application stopped cleanly.")
}

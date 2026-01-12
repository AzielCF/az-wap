package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/store/sqlstore"

	"github.com/AzielCF/az-wap/botengine"
	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	"github.com/AzielCF/az-wap/botengine/providers"
	botUsecaseLayer "github.com/AzielCF/az-wap/botengine/usecase"
	"github.com/AzielCF/az-wap/config"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	domainChat "github.com/AzielCF/az-wap/domains/chat"
	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	domainGroup "github.com/AzielCF/az-wap/domains/group"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	domainMessage "github.com/AzielCF/az-wap/domains/message"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	domainSend "github.com/AzielCF/az-wap/domains/send"
	domainUser "github.com/AzielCF/az-wap/domains/user"
	"github.com/AzielCF/az-wap/infrastructure/chatstorage"
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	"github.com/AzielCF/az-wap/integrations/chatwoot"
	"github.com/AzielCF/az-wap/pkg/utils"
	uiRest "github.com/AzielCF/az-wap/ui/rest"
	"github.com/AzielCF/az-wap/usecase"
	"github.com/AzielCF/az-wap/workspace"
	workspaceDomain "github.com/AzielCF/az-wap/workspace/domain"
	workspaceRepo "github.com/AzielCF/az-wap/workspace/repository"
	workspaceUsecaseLayer "github.com/AzielCF/az-wap/workspace/usecase"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
)

var (
	EmbedIndex embed.FS
	EmbedViews embed.FS

	// Whatsapp
	whatsappCli *whatsmeow.Client

	// Chat Storage
	chatStorageDB   *sql.DB
	chatStorageRepo domainChatStorage.IChatStorageRepository

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
	instanceUsecase   domainInstance.IInstanceUsecase
	cacheUsecase      domainCache.ICacheUsecase
	mcpUsecase        domainMCP.IMCPUsecase
	healthUsecase     domainHealth.IHealthUsecase

	// Bot Engine
	botEngine *botengine.Engine

	// Workspace
	workspaceDB      *sql.DB
	wkRepo           workspaceRepo.IWorkspaceRepository
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
		config.AppPort = envPort
	}
	if envDebug := viper.GetBool("app_debug"); envDebug {
		config.AppDebug = envDebug
	}
	if envOs := viper.GetString("app_os"); envOs != "" {
		config.AppOs = envOs
	}
	envBasicAuth := viper.GetString("app_basic_auth")
	if envBasicAuth == "" {
		envBasicAuth = os.Getenv("APP_BASIC_AUTH")
	}
	if envBasicAuth != "" {
		credential := strings.Split(envBasicAuth, ",")
		config.AppBasicAuthCredential = credential
	}
	if envBasePath := viper.GetString("app_base_path"); envBasePath != "" {
		config.AppBasePath = envBasePath
	}
	if envTrustedProxies := viper.GetString("app_trusted_proxies"); envTrustedProxies != "" {
		proxies := strings.Split(envTrustedProxies, ",")
		config.AppTrustedProxies = proxies
	}

	// Database settings
	if envDBURI := viper.GetString("db_uri"); envDBURI != "" {
		config.DBURI = envDBURI
	}
	if envDBKEYSURI := viper.GetString("db_keys_uri"); envDBKEYSURI != "" {
		config.DBKeysURI = envDBKEYSURI
	}

	// WhatsApp settings
	if envAutoReply := viper.GetString("whatsapp_auto_reply"); envAutoReply != "" {
		trimmed := strings.TrimSpace(envAutoReply)
		trimmed = strings.Trim(trimmed, "\"'")
		if trimmed != "" && !strings.EqualFold(trimmed, "Auto reply message") {
			config.WhatsappAutoReplyMessage = envAutoReply
		}
	}
	if viper.IsSet("whatsapp_auto_mark_read") {
		config.WhatsappAutoMarkRead = viper.GetBool("whatsapp_auto_mark_read")
	}
	if viper.IsSet("whatsapp_auto_download_media") {
		config.WhatsappAutoDownloadMedia = viper.GetBool("whatsapp_auto_download_media")
	}
	if envWebhook := viper.GetString("whatsapp_webhook"); envWebhook != "" {
		webhook := strings.Split(envWebhook, ",")
		config.WhatsappWebhook = webhook
	}
	if envWebhookSecret := viper.GetString("whatsapp_webhook_secret"); envWebhookSecret != "" {
		config.WhatsappWebhookSecret = envWebhookSecret
	}
	if viper.IsSet("whatsapp_webhook_insecure_skip_verify") {
		config.WhatsappWebhookInsecureSkipVerify = viper.GetBool("whatsapp_webhook_insecure_skip_verify")
	}
	if viper.IsSet("whatsapp_account_validation") {
		config.WhatsappAccountValidation = viper.GetBool("whatsapp_account_validation")
	}
}

func initFlags() {
	// Application flags
	rootCmd.PersistentFlags().StringVarP(
		&config.AppPort,
		"port", "p",
		config.AppPort,
		"change port number with --port <number> | example: --port=8080",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&config.AppDebug,
		"debug", "d",
		config.AppDebug,
		"hide or displaying log with --debug <true/false> | example: --debug=true",
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AppOs,
		"os", "",
		config.AppOs,
		`os name --os <string> | example: --os="Chrome"`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.AppBasicAuthCredential,
		"basic-auth", "b",
		config.AppBasicAuthCredential,
		"basic auth credential | -b=yourUsername:yourPassword",
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.AppBasePath,
		"base-path", "",
		config.AppBasePath,
		`base path for subpath deployment --base-path <string> | example: --base-path="/gowa"`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.AppTrustedProxies,
		"trusted-proxies", "",
		config.AppTrustedProxies,
		`trusted proxy IP ranges for reverse proxy deployments --trusted-proxies <string> | example: --trusted-proxies="0.0.0.0/0" or --trusted-proxies="10.0.0.0/8,172.16.0.0/12"`,
	)

	// Database flags
	rootCmd.PersistentFlags().StringVarP(
		&config.DBURI,
		"db-uri", "",
		config.DBURI,
		`the database uri to store the connection data database uri (by default, we'll use sqlite3 under storages/whatsapp.db). database uri --db-uri <string> | example: --db-uri="file:storages/whatsapp.db?_foreign_keys=on or postgres://user:password@localhost:5432/whatsapp"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.DBKeysURI,
		"db-keys-uri", "",
		config.DBKeysURI,
		`the database uri to store the keys database uri (by default, we'll use the same database uri). database uri --db-keys-uri <string> | example: --db-keys-uri="file::memory:?cache=shared&_foreign_keys=on"`,
	)

	// WhatsApp flags
	rootCmd.PersistentFlags().StringVarP(
		&config.WhatsappAutoReplyMessage,
		"autoreply", "",
		config.WhatsappAutoReplyMessage,
		`auto reply when received message --autoreply <string> | example: --autoreply="Don't reply this message"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAutoMarkRead,
		"auto-mark-read", "",
		config.WhatsappAutoMarkRead,
		`auto mark incoming messages as read --auto-mark-read <true/false> | example: --auto-mark-read=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAutoDownloadMedia,
		"auto-download-media", "",
		config.WhatsappAutoDownloadMedia,
		`auto download media from incoming messages --auto-download-media <true/false> | example: --auto-download-media=false`,
	)
	rootCmd.PersistentFlags().StringSliceVarP(
		&config.WhatsappWebhook,
		"webhook", "w",
		config.WhatsappWebhook,
		`forward event to webhook --webhook <string> | example: --webhook="https://yourcallback.com/callback"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&config.WhatsappWebhookSecret,
		"webhook-secret", "",
		config.WhatsappWebhookSecret,
		`secure webhook request --webhook-secret <string> | example: --webhook-secret="super-secret-key"`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappWebhookInsecureSkipVerify,
		"webhook-insecure-skip-verify", "",
		config.WhatsappWebhookInsecureSkipVerify,
		`skip TLS certificate verification for webhooks (INSECURE - use only for development/self-signed certs) --webhook-insecure-skip-verify <true/false> | example: --webhook-insecure-skip-verify=true`,
	)
	rootCmd.PersistentFlags().BoolVarP(
		&config.WhatsappAccountValidation,
		"account-validation", "",
		config.WhatsappAccountValidation,
		`enable or disable account validation --account-validation <true/false> | example: --account-validation=true`,
	)

	// Message Worker Pool flags
	rootCmd.PersistentFlags().IntVarP(
		&config.MessageWorkerPoolSize,
		"message-workers", "",
		config.MessageWorkerPoolSize,
		`number of concurrent message workers --message-workers <number> | example: --message-workers=30 (default: 20)`,
	)
	rootCmd.PersistentFlags().IntVarP(
		&config.MessageWorkerQueueSize,
		"message-queue-size", "",
		config.MessageWorkerQueueSize,
		`queue size per message worker --message-queue-size <number> | example: --message-queue-size=1500 (default: 1000)`,
	)
}

func initChatStorage() (*sql.DB, error) {
	connStr := fmt.Sprintf("%s?_journal_mode=WAL", config.ChatStorageURI)
	if config.ChatStorageEnableForeignKeys {
		connStr += "&_foreign_keys=on"
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func autoConnectAllInstances(ctx context.Context) {
	if instanceUsecase == nil {
		return
	}

	instances, err := instanceUsecase.List(ctx)
	if err != nil {
		logrus.WithError(err).Error("[AUTO_CONNECT] failed to list instances for auto-connect")
		return
	}

	for _, inst := range instances {
		trimmedID := strings.TrimSpace(inst.ID)
		if trimmedID == "" {
			continue
		}

		go func(id string) {
			client, _, err := whatsapp.GetOrInitInstanceClient(context.Background(), id, chatStorageRepo)
			if err != nil {
				logrus.WithError(err).Errorf("[AUTO_CONNECT] failed to init client for instance %s", id)
				return
			}

			// Client may be nil if instance has no device yet (needs login)
			if client == nil {
				logrus.Infof("[AUTO_CONNECT] instance %s has no device yet (needs login), skipping", id)
				return
			}

			if client.IsConnected() && client.IsLoggedIn() {
				logrus.Infof("[AUTO_CONNECT] instance %s already connected", id)
			} else {
				if err := client.Connect(); err != nil {
					logrus.WithError(err).Errorf("[AUTO_CONNECT] failed to connect instance %s", id)
				} else {
					logrus.Infof("[AUTO_CONNECT] instance %s connected on startup", id)
				}
			}
		}(trimmedID)
	}

	// Start global auto-reconnect worker for all instances
	startGlobalInstanceAutoReconnectWorker(instanceUsecase, chatStorageRepo)
}

func startGlobalInstanceAutoReconnectWorker(instanceService domainInstance.IInstanceUsecase, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	go func() {
		logrus.Info("[AUTO_RECONNECT] starting global instance auto-reconnect worker")
		// Initial wait to let boot connections settle
		time.Sleep(1 * time.Minute)
		for {
			instances, err := instanceService.List(context.Background())
			if err == nil {
				for _, inst := range instances {
					if !inst.AutoReconnect {
						continue
					}
					// Only try if there's already a client (meaning it has been initialized/logged in before)
					client := whatsapp.GetInstanceClient(inst.ID)
					if client != nil && !client.IsConnected() && client.IsLoggedIn() {
						logrus.Infof("[AUTO_RECONNECT] instance %s disconnected, attempting reconnection...", inst.ID)
						if err := client.Connect(); err != nil {
							logrus.WithError(err).Warnf("[AUTO_RECONNECT] failed to reconnect instance %s", inst.ID)
						} else {
							logrus.Infof("[AUTO_RECONNECT] instance %s reconnected successfully", inst.ID)
						}
					}
				}
			}
			time.Sleep(5 * time.Minute)
		}
	}()
}

func initApp() {
	if config.AppDebug {
		config.WhatsappLogLevel = "DEBUG"
		logrus.SetLevel(logrus.DebugLevel)
	}

	//preparing folder if not exist
	err := utils.CreateFolder(config.PathQrCode, config.PathSendItems, config.PathStorages, config.PathMedia, config.PathCacheMedia)
	if err != nil {
		logrus.Errorln(err)
	}

	ctx := context.Background()

	chatStorageDB, err = initChatStorage()
	if err != nil {
		// Terminate the application if chat storage fails to initialize to avoid nil pointer panics later.
		logrus.Fatalf("failed to initialize chat storage: %v", err)
	}

	chatStorageRepo = chatstorage.NewStorageRepository(chatStorageDB)
	chatStorageRepo.InitializeSchema()

	whatsappDB := whatsapp.InitWaDB(ctx, config.DBURI)
	var keysDB *sqlstore.Container
	if config.DBKeysURI != "" {
		keysDB = whatsapp.InitWaDB(ctx, config.DBKeysURI)
	}

	whatsappCli = whatsapp.InitWaCLI(ctx, whatsappDB, keysDB, chatStorageRepo)

	// Usecase
	instanceUsecase = usecase.NewInstanceService()
	appUsecase = usecase.NewAppService(chatStorageRepo, instanceUsecase)
	chatUsecase = usecase.NewChatService(chatStorageRepo, instanceUsecase)
	sendUsecase = usecase.NewSendService(appUsecase, chatStorageRepo, instanceUsecase)
	userUsecase = usecase.NewUserService(instanceUsecase)
	messageUsecase = usecase.NewMessageService(appUsecase, chatStorageRepo, instanceUsecase)
	groupUsecase = usecase.NewGroupService(appUsecase, instanceUsecase)
	newsletterUsecase = usecase.NewNewsletterService(instanceUsecase)
	credentialUsecase = usecase.NewCredentialService()
	botUsecase = botUsecaseLayer.NewBotService(credentialUsecase)
	cacheUsecase = usecase.NewCacheService()
	cacheUsecase.StartBackgroundCleanup(ctx)
	mcpUsecase = botUsecaseLayer.NewMCPService()
	healthUsecase = usecase.NewHealthService(mcpUsecase, credentialUsecase, botUsecase)
	mcpUsecase.SetHealthUsecase(healthUsecase)
	healthUsecase.StartPeriodicChecks(ctx)

	// Bot Engine Initialization
	botEngine = botengine.NewEngine(botUsecase, mcpUsecase)
	botEngine.RegisterProvider(string(domainBot.ProviderGemini), providers.NewGeminiProvider(mcpUsecase, botEngine.GetMemoryStore()))

	// Hooks: Chatwoot integration
	botEngine.RegisterPostReplyHook(func(ctx context.Context, b domainBot.Bot, input botengine.BotInput, output botengine.BotOutput) {
		phone, _ := input.Metadata["phone"].(string)
		if phone != "" {
			go chatwoot.ForwardBotReplyFromEvent(ctx, input.InstanceID, phone, output.Text)
		}
	})

	// Workspace Initialization
	// We use a separate DB file for workspaces to keep it clean
	workspaceDBPath := "storages/workspaces.db"
	if config.DBURI != "" && strings.Contains(config.DBURI, "postgres") {
		// If using postgres generally, we might want to use it here too, but for MVP sticking to SQLite file
		// or parsing the URI. For now, let's force SQLite for workspace MVP to avoid migration complexity.
	}

	workspaceDB, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&_journal_mode=WAL", workspaceDBPath))
	if err != nil {
		logrus.Fatalf("failed to open workspace db: %v", err)
	}

	wkRepo = workspaceRepo.NewSQLiteRepository(workspaceDB)
	if err := wkRepo.Init(ctx); err != nil {
		logrus.Fatalf("failed to init workspace repo: %v", err)
	}

	workspaceManager = workspace.NewManager(wkRepo, botEngine)

	// Register WhatsApp Adapter Factory
	workspaceManager.RegisterFactory(workspaceDomain.ChannelTypeWhatsApp, func(conf workspaceDomain.ChannelConfig) (workspace.ChannelAdapter, error) {
		// Extract instance ID from config
		instanceID, ok := conf.Settings["instance_id"].(string)
		if !ok || instanceID == "" {
			return nil, fmt.Errorf("instance_id missing in channel config")
		}

		// Get client from existing infrastructure
		// Note: We might need to ensure the client exists.
		// For now, we assume standard infrastructure/whatsapp/init.go manages clients map.
		// We need a way to get it. infrastructure/whatsapp/init.go has GetInstanceClient(id)

		client := whatsapp.GetInstanceClient(instanceID)
		if client == nil {
			// Try to init? Or fail?
			// If it's not in memory, we might need to load it.
			// Let's try GetOrInitInstanceClient (but it needs chatRepo)
			var err error
			client, _, err = whatsapp.GetOrInitInstanceClient(context.Background(), instanceID, chatStorageRepo)
			if err != nil {
				return nil, fmt.Errorf("failed to load whatsapp client: %w", err)
			}
		}

		return whatsapp.NewAdapter(conf.Settings["channel_id"].(string), conf.Settings["workspace_id"].(string), client), nil
	})

	wkUsecase = workspaceUsecaseLayer.NewWorkspaceUsecase(wkRepo)

	// Run Migration
	if err := AutoMigrateLegacyInstances(ctx, instanceUsecase, wkUsecase); err != nil {
		logrus.Errorf("Migration failed: %v", err)
	}

	whatsapp.SetBotEngine(botEngine)
	whatsapp.SetInstanceUsecase(instanceUsecase)
	uiRest.SetBotEngine(botEngine)

	go autoConnectAllInstances(ctx)
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
	whatsapp.GracefulShutdown()

	// 2. Clear in-memory chat storage caches or close connections
	chatstorage.CloseInstanceRepositories()

	// 3. Close the global chat storage database handle
	if chatStorageDB != nil {
		logrus.Info("[APP] Closing chat storage database...")
		if err := chatStorageDB.Close(); err != nil {
			logrus.Errorf("[APP] Error closing chat storage database: %v", err)
		}
	}

	logrus.Info("[APP] Application stopped cleanly.")
}

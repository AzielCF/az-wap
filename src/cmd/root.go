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
	"embed"
	"fmt"
	"os"
	"time"

	coreconfig "github.com/AzielCF/az-wap/core/config"
	coreDB "github.com/AzielCF/az-wap/core/database"
	coreSettings "github.com/AzielCF/az-wap/core/settings/application"

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

	domainApp "github.com/AzielCF/az-wap/domains/app"
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	domainGroup "github.com/AzielCF/az-wap/domains/group"
	domainHealth "github.com/AzielCF/az-wap/domains/health"
	domainMessage "github.com/AzielCF/az-wap/domains/message"
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	domainSend "github.com/AzielCF/az-wap/domains/send"
	domainUser "github.com/AzielCF/az-wap/domains/user"
	"github.com/AzielCF/az-wap/infrastructure/valkey"

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
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow"
	// "go.mau.fi/whatsmeow/store/sqlstore"
)

var (
	EmbedFrontend embed.FS

	// Whatsapp
	whatsappCli *whatsmeow.Client

	// Usecase
	appUsecase        domainApp.IAppUsecase
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
	wkRepo            repository.IWorkspaceRepository
	workspaceManager  *workspace.Manager
	wkUsecase         *workspaceUsecaseLayer.WorkspaceUsecase
	typingStore       channel.TypingStore
	monitorStore      monitoring.MonitoringStore
	contextCacheStore botengineDomain.ContextCacheStore

	// Clients Module
	clientService  *clientsApp.ClientService
	subService     *clientsApp.SubscriptionService
	clientResolver *clientsApp.ClientResolver
	clientHandler  *clientsRest.ClientHandler

	// Shared Infra
	vkClient *valkey.Client
	serverID string

	// Core Services
	settingsSvc *coreSettings.SettingsService
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Short: "Send free whatsapp API",
	Long: `This application is from clone https://az-wap, 
you can send whatsapp over http api but your whatsapp account have to be multi device version`,
}

func init() {
	// Config via coreconfig
	cobra.OnInitialize(initApp)
}

// ... (imports remain)

func initApp() {
	var err error
	ctx := context.Background()

	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		logrus.Warn("[CORE] No .env file found or failed to load, relying on environment variables")
	}

	// --- CORE INITIALIZATION ---
	// 1. Load Configuration (Implicitly reads env vars)
	cfg, err := coreconfig.LoadConfig()
	if err != nil {
		logrus.WithError(err).Warn("[CORE] Failed to load core configuration, using defaults")
		cfg = &coreconfig.Config{}
	}

	// Priority: Explicit WHATSAPP_LOG_LEVEL > APP_DEBUG logic
	if cfg.App.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		if cfg.Whatsapp.LogLevel == "" || cfg.Whatsapp.LogLevel == "ERROR" {
			cfg.Whatsapp.LogLevel = "INFO" // For WhatsApp, INFO is enough for debug without being binary-heavy
		}
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	// Set specific Level for WhatsApp if provided
	if coreconfig.Global.Whatsapp.LogLevel != "" {
		// This will be used when creating adapters
	}

	// Generate or Load a persistent unique ID for this server instance
	serverID = utils.GetPersistentServerID(cfg.App.ServerID, cfg.Paths.Storages)

	// preparing folder if not exist
	err = utils.CreateFolder(cfg.Paths.SendItems, cfg.Paths.Storages)
	if err != nil {
		logrus.Errorln(err)
	}

	// 2. Initialize Database (GORM)
	gormDB, err := coreDB.NewDatabase(cfg)
	if err != nil {
		logrus.Fatalf("[CORE] Failed to initialize database: %v", err)
	}

	// 3. Load Dynamic Settings (replacing settings.go init logic)
	settingsSvc = coreSettings.NewSettingsService(gormDB)
	if dynSettings, err := settingsSvc.GetDynamicSettings(ctx); err != nil {
		logrus.WithError(err).Warn("[CORE] Failed to load dynamic settings from DB")
	} else {
		// Update Global Config from Dynamic Settings
		if dynSettings.AIGlobalSystemPrompt != "" {
			coreconfig.Global.AI.GlobalSystemPrompt = dynSettings.AIGlobalSystemPrompt
		}
		if dynSettings.AITimezone != "" {
			coreconfig.Global.AI.Timezone = dynSettings.AITimezone
		}
		if dynSettings.AIDebounceMs != nil {
			coreconfig.Global.AI.DebounceMs = *dynSettings.AIDebounceMs
		}
		if dynSettings.AIWaitContactIdleMs != nil {
			coreconfig.Global.AI.WaitContactIdleMs = *dynSettings.AIWaitContactIdleMs
		}
		if dynSettings.AITypingEnabled != nil {
			coreconfig.Global.AI.TypingEnabled = *dynSettings.AITypingEnabled
		}
		if dynSettings.WhatsappMaxDownloadSize != nil {
			coreconfig.Global.Whatsapp.MaxDownloadSize = *dynSettings.WhatsappMaxDownloadSize
		}

		logrus.Info("[CORE] Dynamic settings loaded successfully")
	}

	// 3. Load Dynamic Settings (replacing settings.go init logic)
	settingsSvc = coreSettings.NewSettingsService(gormDB)

	// Workspace Initialization (Separate DB file, but using GORM for robust schema management)
	workspaceDBPath := "storages/workspaces.db"

	// Create a dedicated GORM connection for the workspaces database
	wkGormDB, err := coreDB.NewDatabaseWithCustomPath(cfg, workspaceDBPath)
	if err != nil {
		logrus.Fatalf("failed to open workspace db via GORM: %v", err)
	}

	wkRepo = repository.NewWorkspaceGormRepository(wkGormDB)
	if err := wkRepo.Init(ctx); err != nil {
		logrus.Fatalf("failed to init workspace repo: %v", err)
	}

	// 1. Basic Usecases (No complex dependencies)
	credentialUsecase = usecase.NewCredentialService(gormDB)
	botUsecase = botUsecaseLayer.NewBotService(credentialUsecase)
	cacheUsecase = usecase.NewCacheService(settingsSvc)
	cacheUsecase.StartBackgroundCleanup(ctx)
	mcpUsecase = botUsecaseLayer.NewMCPService(gormDB)

	// 2. Bot Engine Initialization (Needs BotUsecase, MCPUsecase)
	botEngine = botengine.NewEngine(botUsecase, mcpUsecase)

	// Initialize Shared Infrastrucutre (Valkey)
	if coreconfig.Global.Database.ValkeyEnabled {
		var vkErr error
		vkClient, vkErr = valkey.NewClient(valkey.Config{
			Address:        coreconfig.Global.Database.ValkeyAddress,
			Password:       coreconfig.Global.Database.ValkeyPassword,
			DB:             coreconfig.Global.Database.ValkeyDB,
			KeyPrefix:      coreconfig.Global.Database.ValkeyKeyPrefix,
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

	// 2.1 Clients Module Initialization (Using appDB from Core)

	// Client Repository (app.db)
	clientRepo := clientsRepo.NewClientGormRepository(gormDB)
	if err := clientRepo.InitSchema(ctx); err != nil {
		logrus.Fatalf("failed to init client repo: %v", err)
	}

	// Subscription Repository (Uses the separate workspaceDB)
	subRepo := clientsRepo.NewSubscriptionGormRepository(wkGormDB)
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
	appUsecase = usecase.NewAppService(workspaceManager, settingsSvc)
	userUsecase = usecase.NewUserService(workspaceManager)
	groupUsecase = usecase.NewGroupService(workspaceManager)
	newsletterUsecase = usecase.NewNewsletterService(workspaceManager, wkRepo, subRepo, monitorStore, vkClient)
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
	healthUsecase = usecase.NewHealthService(mcpUsecase, credentialUsecase, botUsecase, workspaceManager, wkUsecase, vkClient)
	mcpUsecase.SetHealthUsecase(healthUsecase)
	healthUsecase.StartPeriodicChecks(ctx)
	uiRest.SetBotEngine(botEngine, workspaceManager)

	// Initialize Integrations
	chatwoot.SetRepositories(wkRepo, gormDB)

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
	botEngine.RegisterNativeTool(rTools.ListPendingRemindersTool())
	botEngine.RegisterNativeTool(rTools.SearchRemindersHistoryTool())
	botEngine.RegisterNativeTool(rTools.CancelReminderTool())
	botEngine.RegisterNativeTool(rTools.UpdateReminderTool())
	botEngine.RegisterNativeTool(rTools.CountRemindersTool())

	// Register Client Profile Tools (allow users to manage their personal info via AI)
	cTools := onlyClients.NewClientTools(clientRepo)
	botEngine.RegisterNativeTool(cTools.UpdateMyInfoTool())
	botEngine.RegisterNativeTool(cTools.GetMyInfoTool())
	botEngine.RegisterNativeTool(cTools.DeleteMyFieldTool())

	// Register Currency Tools
	cxTools := onlyClients.NewExchangeRateTools()
	botEngine.RegisterNativeTool(cxTools.GetExchangeRateTool())

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
	// Discontinued: chatstorage.CloseInstanceRepositories()

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

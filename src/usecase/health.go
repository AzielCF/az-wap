package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	"github.com/AzielCF/az-wap/domains/health"
	"github.com/AzielCF/az-wap/infrastructure/valkey"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type healthService struct {
	mu            sync.RWMutex
	memoryRecords map[string]health.HealthRecord
	vkClient      *valkey.Client

	mcpUsecase        domainMCP.IMCPUsecase
	credentialUsecase domainCredential.ICredentialUsecase
	botUsecase        domainBot.IBotUsecase
	workspaceManager  *workspace.Manager
	workspaceUsecase  interface {
		ListWorkspaces(ctx context.Context) ([]wsDomain.Workspace, error)
		GetWorkspace(ctx context.Context, id string) (wsDomain.Workspace, error)
		ListChannels(ctx context.Context, workspaceID string) ([]wsChannelDomain.Channel, error)
		GetChannel(ctx context.Context, id string) (wsChannelDomain.Channel, error)
	}
}

func NewHealthService(mcp domainMCP.IMCPUsecase, cred domainCredential.ICredentialUsecase, bot domainBot.IBotUsecase, wm *workspace.Manager, wu interface {
	ListWorkspaces(ctx context.Context) ([]wsDomain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (wsDomain.Workspace, error)
	ListChannels(ctx context.Context, workspaceID string) ([]wsChannelDomain.Channel, error)
	GetChannel(ctx context.Context, id string) (wsChannelDomain.Channel, error)
}, vk *valkey.Client) health.IHealthUsecase {
	return &healthService{
		memoryRecords:     make(map[string]health.HealthRecord),
		vkClient:          vk,
		mcpUsecase:        mcp,
		credentialUsecase: cred,
		botUsecase:        bot,
		workspaceManager:  wm,
		workspaceUsecase:  wu,
	}
}

func (s *healthService) healthKey() string {
	return "monitoring:health"
}

func (s *healthService) entityKey(t health.EntityType, id string) string {
	return fmt.Sprintf("%s:%s", t, id)
}

func (s *healthService) GetStatus(ctx context.Context) ([]health.HealthRecord, error) {
	var results []health.HealthRecord

	if s.vkClient != nil {
		cmd := s.vkClient.Inner().B().Hgetall().Key(s.vkClient.Key(s.healthKey())).Build()
		res, err := s.vkClient.Inner().Do(ctx, cmd).AsMap()
		if err == nil {
			for _, val := range res {
				var r health.HealthRecord
				sData, _ := val.ToString()
				if err := json.Unmarshal([]byte(sData), &r); err == nil {
					results = append(results, r)
				}
			}
		}
	}

	// Always merge with local memory or use it as fallback
	s.mu.RLock()
	if len(results) == 0 {
		for _, r := range s.memoryRecords {
			results = append(results, r)
		}
	}
	s.mu.RUnlock()

	// Sort by EntityType and then Name for consistency
	sort.Slice(results, func(i, j int) bool {
		if results[i].EntityType != results[j].EntityType {
			return results[i].EntityType < results[j].EntityType
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func (s *healthService) GetEntityStatus(ctx context.Context, entityType health.EntityType, entityID string) (health.HealthRecord, error) {
	key := s.entityKey(entityType, entityID)

	if s.vkClient != nil {
		cmd := s.vkClient.Inner().B().Hget().Key(s.vkClient.Key(s.healthKey())).Field(key).Build()
		res, err := s.vkClient.Inner().Do(ctx, cmd).ToString()
		if err == nil {
			var r health.HealthRecord
			if err := json.Unmarshal([]byte(res), &r); err == nil {
				return r, nil
			}
		}
		// If Valkey is active but result not found, return Unknown
		return health.HealthRecord{
			EntityType: entityType,
			EntityID:   entityID,
			Status:     health.StatusUnknown,
		}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.memoryRecords[key]; ok {
		return r, nil
	}

	return health.HealthRecord{
		EntityType: entityType,
		EntityID:   entityID,
		Status:     health.StatusUnknown,
	}, nil
}

func (s *healthService) upsertStatus(ctx context.Context, r health.HealthRecord) error {
	key := s.entityKey(r.EntityType, r.EntityID)

	if r.ID == "" {
		existing, _ := s.GetEntityStatus(ctx, r.EntityType, r.EntityID)
		if existing.ID != "" {
			r.ID = existing.ID
			if r.Name == "" {
				r.Name = existing.Name
			}
			if r.LastSuccess == nil {
				r.LastSuccess = existing.LastSuccess
			}
		} else {
			r.ID = uuid.NewString()
		}
	}

	now := time.Now()
	r.LastChecked = now
	if r.Status == health.StatusOk {
		r.LastSuccess = &now
	}

	// Store in distributed Valkey OR local memory
	if s.vkClient != nil {
		data, _ := json.Marshal(r)
		cmd := s.vkClient.Inner().B().Hset().Key(s.vkClient.Key(s.healthKey())).FieldValue().FieldValue(key, string(data)).Build()
		_ = s.vkClient.Inner().Do(ctx, cmd).Error()
	} else {
		// Only use Go RAM if Valkey is NOT active
		s.mu.Lock()
		s.memoryRecords[key] = r
		s.mu.Unlock()
	}

	return nil
}

func (s *healthService) ReportFailure(ctx context.Context, entityType health.EntityType, entityID string, message string) {
	record := health.HealthRecord{
		EntityType:  entityType,
		EntityID:    entityID,
		Status:      health.StatusError,
		LastMessage: message,
	}
	s.upsertStatus(ctx, record)

	// Dependency propagation: If an MCP fails, check all bots using it
	if entityType == health.EntityMCP {
		go func() {
			// Use a fresh context for async loop
			asyncCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			if bots, err := s.mcpUsecase.ListBotsUsingServer(asyncCtx, entityID); err == nil {
				for _, botID := range bots {
					// Don't leak too many goroutines, check them sequentially or with a worker pool if many
					_, _ = s.CheckBot(asyncCtx, botID)
				}
			}
		}()
	}
}

func (s *healthService) ReportSuccess(ctx context.Context, entityType health.EntityType, entityID string) {
	record := health.HealthRecord{
		EntityType:  entityType,
		EntityID:    entityID,
		Status:      health.StatusOk,
		LastMessage: "OK",
	}
	s.upsertStatus(ctx, record)

	// If an MCP is back up, bots using it might be OK now
	if entityType == health.EntityMCP {
		go func() {
			asyncCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			if bots, err := s.mcpUsecase.ListBotsUsingServer(asyncCtx, entityID); err == nil {
				for _, botID := range bots {
					_, _ = s.CheckBot(asyncCtx, botID)
				}
			}
		}()
	}
}

func (s *healthService) CheckMCP(ctx context.Context, id string) (health.HealthRecord, error) {
	record := health.HealthRecord{
		EntityType: health.EntityMCP,
		EntityID:   id,
		Status:     health.StatusOk,
	}

	// Fetch name if possible
	if srv, err := s.mcpUsecase.GetServer(ctx, id); err == nil {
		record.Name = srv.Name
	}

	err := s.mcpUsecase.Validate(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = err.Error()
	} else {
		record.LastMessage = "Connection successful"
	}

	err = s.upsertStatus(ctx, record)
	return record, err
}

func (s *healthService) CheckCredential(ctx context.Context, id string) (health.HealthRecord, error) {
	record := health.HealthRecord{
		EntityType: health.EntityCredential,
		EntityID:   id,
		Status:     health.StatusOk,
	}

	// Fetch name if possible
	if cred, err := s.credentialUsecase.GetByID(ctx, id); err == nil {
		record.Name = cred.Name
	}

	err := s.credentialUsecase.Validate(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = err.Error()
	} else {
		record.LastMessage = "Key validated successfully"
	}

	err = s.upsertStatus(ctx, record)
	return record, err
}

func (s *healthService) CheckBot(ctx context.Context, id string) (health.HealthRecord, error) {
	record := health.HealthRecord{
		EntityType: health.EntityBot,
		EntityID:   id,
		Status:     health.StatusOk,
	}

	// Fetch name if possible
	if b, err := s.botUsecase.GetByID(ctx, id); err == nil {
		record.Name = b.Name
	}

	// Check status of MCP servers for this bot
	servers, err := s.mcpUsecase.ListServersForBot(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = fmt.Sprintf("failed to list bot servers: %v", err)
	} else {
		var failingServers []string
		for _, srv := range servers {
			if srv.Enabled {
				// Use CACHED status instead of re-triggering network check
				status, _ := s.GetEntityStatus(ctx, health.EntityMCP, srv.ID)
				if status.Status == health.StatusError {
					failingServers = append(failingServers, srv.Name)
				}
			}
		}

		if len(failingServers) > 0 {
			record.Status = health.StatusError
			record.LastMessage = fmt.Sprintf("Failing MCP dependencies: %s", strings.Join(failingServers, ", "))
		} else {
			record.LastMessage = "All dependencies healthy"
		}
	}

	err = s.upsertStatus(ctx, record)
	return record, err
}

func (s *healthService) CheckWorkspace(ctx context.Context, id string) (health.HealthRecord, error) {
	record := health.HealthRecord{
		EntityType: health.EntityWorkspace,
		EntityID:   id,
		Status:     health.StatusOk,
	}

	ws, err := s.workspaceUsecase.GetWorkspace(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = fmt.Sprintf("failed to get workspace: %v", err)
	} else {
		record.Name = ws.Name
		if !ws.Enabled {
			record.Status = health.StatusError
			record.LastMessage = "Workspace is disabled"
		} else {
			// Check channels
			channels, err := s.workspaceUsecase.ListChannels(ctx, id)
			if err != nil {
				record.Status = health.StatusError
				record.LastMessage = fmt.Sprintf("failed to list channels: %v", err)
			} else {
				failing := 0
				for _, ch := range channels {
					if ch.Enabled {
						cStatus, _ := s.CheckChannel(ctx, ch.ID)
						if cStatus.Status == health.StatusError {
							failing++
						}
					}
				}
				if failing > 0 {
					record.Status = health.StatusError
					record.LastMessage = fmt.Sprintf("%d channels failing", failing)
				} else {
					record.LastMessage = "All channels healthy"
				}
			}
		}
	}

	err = s.upsertStatus(ctx, record)
	return record, err
}

func (s *healthService) CheckChannel(ctx context.Context, id string) (health.HealthRecord, error) {
	record := health.HealthRecord{
		EntityType: health.EntityChannel,
		EntityID:   id,
		Status:     health.StatusOk,
	}

	ch, err := s.workspaceUsecase.GetChannel(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = fmt.Sprintf("failed to get channel: %v", err)
	} else {
		record.Name = ch.Name
		if !ch.Enabled {
			record.Status = health.StatusError
			record.LastMessage = "Channel is disabled"
		} else {
			// Check active adapter
			if adapter, ok := s.workspaceManager.GetAdapter(id); ok {
				status := adapter.Status()
				if status != wsChannelDomain.ChannelStatusConnected {
					record.Status = health.StatusError
					record.LastMessage = fmt.Sprintf("Adapter status: %s", status)
				} else {
					record.LastMessage = "Connected"
				}
			} else {
				record.Status = health.StatusError
				record.LastMessage = "Adapter not found (not running)"
			}
		}
	}

	err = s.upsertStatus(ctx, record)
	return record, err
}

func (s *healthService) CheckAll(ctx context.Context) ([]health.HealthRecord, error) {
	var results []health.HealthRecord

	// Check MCP Servers (network heavy)
	servers, err := s.mcpUsecase.ListServers(ctx)
	if err == nil {
		for _, srv := range servers {
			res, _ := s.CheckMCP(ctx, srv.ID)
			results = append(results, res)
			// Wait 2 seconds between servers to be extremely gentle
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return results, ctx.Err()
			}
		}
	}

	// Check Credentials (network heavy)
	creds, err := s.credentialUsecase.List(ctx, nil)
	if err == nil {
		for _, cred := range creds {
			res, _ := s.CheckCredential(ctx, cred.ID)
			results = append(results, res)
			select {
			case <-time.After(200 * time.Millisecond):
			case <-ctx.Done():
				return results, ctx.Err()
			}
		}
	}

	// Check Bots (CPU/DB heavy)
	bots, err := s.botUsecase.List(ctx)
	if err == nil {
		for _, b := range bots {
			res, _ := s.CheckBot(ctx, b.ID)
			results = append(results, res)
		}
	}

	// Check Workspaces (DB/Memory heavy)
	if workspaces, err := s.workspaceUsecase.ListWorkspaces(ctx); err == nil {
		for _, ws := range workspaces {
			res, _ := s.CheckWorkspace(ctx, ws.ID)
			results = append(results, res)
		}
	}

	return results, nil
}

func (s *healthService) StartPeriodicChecks(ctx context.Context) {
	logrus.Info("[Health] starting periodic health checks loop (interval: 12h)")
	ticker := time.NewTicker(12 * time.Hour)

	// Run once at start
	go func() {
		logrus.Info("[Health] performing initial health check")
		s.CheckAll(ctx)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				logrus.Info("[Health] performing scheduled health check")
				s.CheckAll(ctx)
			}
		}
	}()
}

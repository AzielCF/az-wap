package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	domainBot "github.com/AzielCF/az-wap/botengine/domain/bot"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
	globalConfig "github.com/AzielCF/az-wap/config"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	"github.com/AzielCF/az-wap/domains/health"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	wsDomain "github.com/AzielCF/az-wap/workspace/domain/workspace"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type healthService struct {
	db                *sql.DB
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

func initHealthStorageDB() (*sql.DB, error) {
	db, err := globalConfig.GetAppDB()
	if err != nil {
		return nil, err
	}

	createHealthTable := `
		CREATE TABLE IF NOT EXISTS health_checks (
			id TEXT PRIMARY KEY,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			status TEXT NOT NULL,
			last_message TEXT,
			last_checked TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_success TIMESTAMP,
			UNIQUE(entity_type, entity_id)
		);
	`

	if _, err := db.Exec(createHealthTable); err != nil {
		return nil, err
	}

	return db, nil
}

func NewHealthService(mcp domainMCP.IMCPUsecase, cred domainCredential.ICredentialUsecase, bot domainBot.IBotUsecase, wm *workspace.Manager, wu interface {
	ListWorkspaces(ctx context.Context) ([]wsDomain.Workspace, error)
	GetWorkspace(ctx context.Context, id string) (wsDomain.Workspace, error)
	ListChannels(ctx context.Context, workspaceID string) ([]wsChannelDomain.Channel, error)
	GetChannel(ctx context.Context, id string) (wsChannelDomain.Channel, error)
}) health.IHealthUsecase {
	db, err := initHealthStorageDB()
	if err != nil {
		logrus.WithError(err).Error("[Health] failed to initialize storage")
	}

	return &healthService{
		db:                db,
		mcpUsecase:        mcp,
		credentialUsecase: cred,
		botUsecase:        bot,
		workspaceManager:  wm,
		workspaceUsecase:  wu,
	}
}

func (s *healthService) ensureDB() error {
	if s.db == nil {
		return fmt.Errorf("health storage not initialized")
	}
	return nil
}

func (s *healthService) GetStatus(ctx context.Context) ([]health.HealthRecord, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}

	query := `SELECT id, entity_type, entity_id, status, last_message, last_checked, last_success FROM health_checks`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []health.HealthRecord
	for rows.Next() {
		var r health.HealthRecord
		var lastSuccess sql.NullTime
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.LastMessage, &r.LastChecked, &lastSuccess); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			r.LastSuccess = &lastSuccess.Time
		}
		records = append(records, r)
	}
	return records, nil
}

func (s *healthService) GetEntityStatus(ctx context.Context, entityType health.EntityType, entityID string) (health.HealthRecord, error) {
	if err := s.ensureDB(); err != nil {
		return health.HealthRecord{}, err
	}

	var r health.HealthRecord
	var lastSuccess sql.NullTime
	query := `SELECT id, entity_type, entity_id, status, last_message, last_checked, last_success FROM health_checks WHERE entity_type = ? AND entity_id = ?`
	err := s.db.QueryRowContext(ctx, query, string(entityType), entityID).Scan(&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.LastMessage, &r.LastChecked, &lastSuccess)
	if err != nil {
		if err == sql.ErrNoRows {
			return health.HealthRecord{
				EntityType: entityType,
				EntityID:   entityID,
				Status:     health.StatusUnknown,
			}, nil
		}
		return r, err
	}
	if lastSuccess.Valid {
		r.LastSuccess = &lastSuccess.Time
	}
	return r, nil
}

func (s *healthService) upsertStatus(ctx context.Context, r health.HealthRecord) error {
	if err := s.ensureDB(); err != nil {
		return err
	}

	if r.ID == "" {
		// Try to find existing ID
		existing, _ := s.GetEntityStatus(ctx, r.EntityType, r.EntityID)
		if existing.ID != "" {
			r.ID = existing.ID
		} else {
			r.ID = uuid.NewString()
		}
	}

	query := `
		INSERT INTO health_checks (id, entity_type, entity_id, status, last_message, last_checked, last_success)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			status = excluded.status,
			last_message = excluded.last_message,
			last_checked = excluded.last_checked,
			last_success = CASE WHEN excluded.status = 'OK' THEN excluded.last_checked ELSE last_success END
	`

	now := time.Now()
	_, err := s.db.ExecContext(ctx, query, r.ID, string(r.EntityType), r.EntityID, string(r.Status), r.LastMessage, now, now)
	return err
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
	} else if !ws.Enabled {
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
	} else if !ch.Enabled {
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

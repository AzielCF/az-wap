package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainBot "github.com/AzielCF/az-wap/domains/bot"
	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	"github.com/AzielCF/az-wap/domains/health"
	domainMCP "github.com/AzielCF/az-wap/domains/mcp"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type healthService struct {
	db                *sql.DB
	mcpUsecase        domainMCP.IMCPUsecase
	credentialUsecase domainCredential.ICredentialUsecase
	botUsecase        domainBot.IBotUsecase
}

func initHealthStorageDB() (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)

	db, err := sql.Open("sqlite3", connStr)
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

func NewHealthService(mcp domainMCP.IMCPUsecase, cred domainCredential.ICredentialUsecase, bot domainBot.IBotUsecase) health.IHealthUsecase {
	db, err := initHealthStorageDB()
	if err != nil {
		logrus.WithError(err).Error("[Health] failed to initialize storage")
		return &healthService{db: nil}
	}
	return &healthService{
		db:                db,
		mcpUsecase:        mcp,
		credentialUsecase: cred,
		botUsecase:        bot,
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

	// Check if any of the MCP servers for this bot are down
	servers, err := s.mcpUsecase.ListServersForBot(ctx, id)
	if err != nil {
		record.Status = health.StatusError
		record.LastMessage = fmt.Sprintf("failed to list bot servers: %v", err)
	} else {
		var failingServers []string
		for _, srv := range servers {
			if srv.Enabled {
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

func (s *healthService) CheckAll(ctx context.Context) ([]health.HealthRecord, error) {
	var results []health.HealthRecord

	// Check MCP Servers
	servers, err := s.mcpUsecase.ListServers(ctx)
	if err == nil {
		for _, srv := range servers {
			res, _ := s.CheckMCP(ctx, srv.ID)
			results = append(results, res)
		}
	}

	// Check Credentials
	creds, err := s.credentialUsecase.List(ctx, nil)
	if err == nil {
		for _, cred := range creds {
			res, _ := s.CheckCredential(ctx, cred.ID)
			results = append(results, res)
		}
	}

	// Check Bots (last, so they pick up potentially new MCP status)
	bots, err := s.botUsecase.List(ctx)
	if err == nil {
		for _, b := range bots {
			res, _ := s.CheckBot(ctx, b.ID)
			results = append(results, res)
		}
	}

	return results, nil
}

func (s *healthService) StartPeriodicChecks(ctx context.Context) {
	logrus.Info("[Health] starting periodic health checks loop (interval: 30m)")
	ticker := time.NewTicker(30 * time.Minute)

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

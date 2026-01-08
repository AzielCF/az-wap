package health

import (
	"context"
	"time"
)

type EntityType string

const (
	EntityMCP        EntityType = "mcp_server"
	EntityCredential EntityType = "ia_credential"
	EntityBot        EntityType = "bot"
)

type Status string

const (
	StatusOk      Status = "OK"
	StatusError   Status = "ERROR"
	StatusUnknown Status = "UNKNOWN"
)

type HealthRecord struct {
	ID          string     `json:"id"`
	EntityType  EntityType `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	Status      Status     `json:"status"`
	LastMessage string     `json:"last_message"`
	LastChecked time.Time  `json:"last_checked"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
}

type IHealthUsecase interface {
	CheckMCP(ctx context.Context, id string) (HealthRecord, error)
	CheckCredential(ctx context.Context, id string) (HealthRecord, error)
	CheckBot(ctx context.Context, id string) (HealthRecord, error)
	CheckAll(ctx context.Context) ([]HealthRecord, error)
	GetStatus(ctx context.Context) ([]HealthRecord, error)
	GetEntityStatus(ctx context.Context, entityType EntityType, entityID string) (HealthRecord, error)
	ReportFailure(ctx context.Context, entityType EntityType, entityID string, message string)
	ReportSuccess(ctx context.Context, entityType EntityType, entityID string)
	StartPeriodicChecks(ctx context.Context)
}

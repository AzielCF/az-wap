package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/AzielCF/az-wap/workspace/domain"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Init(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workspaces (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			owner_id TEXT NOT NULL,
			config_timezone TEXT DEFAULT 'UTC',
			config_default_language TEXT DEFAULT 'en',
			config_metadata TEXT,
			limits_max_messages_per_day INTEGER DEFAULT 10000,
			limits_max_channels INTEGER DEFAULT 5,
			limits_max_bots INTEGER DEFAULT 10,
			limits_rate_limit_per_minute INTEGER DEFAULT 60,
			enabled BOOLEAN DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS channels (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			config TEXT,
			status TEXT DEFAULT 'pending',
			external_ref TEXT UNIQUE,
			last_seen DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_channels_workspace ON channels(workspace_id);`,
		`CREATE INDEX IF NOT EXISTS idx_channels_external_ref ON channels(external_ref);`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to init schema: %w", err)
		}
	}
	return nil
}

// Workspace CRUD

func (r *SQLiteRepository) Create(ctx context.Context, ws domain.Workspace) error {
	metadata, _ := json.Marshal(ws.Config.Metadata)
	query := `INSERT INTO workspaces (id, name, description, owner_id, config_timezone, config_default_language, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, ws.ID, ws.Name, ws.Description, ws.OwnerID, ws.Config.Timezone, ws.Config.DefaultLanguage, string(metadata), ws.Limits.MaxMessagesPerDay, ws.Limits.MaxChannels, ws.Limits.MaxBots, ws.Limits.RateLimitPerMinute, ws.Enabled, ws.CreatedAt, ws.UpdatedAt)
	return err
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id string) (domain.Workspace, error) {
	query := `SELECT id, name, description, owner_id, config_timezone, config_default_language, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at FROM workspaces WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var ws domain.Workspace
	var metadata string
	err := row.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.OwnerID, &ws.Config.Timezone, &ws.Config.DefaultLanguage, &metadata, &ws.Limits.MaxMessagesPerDay, &ws.Limits.MaxChannels, &ws.Limits.MaxBots, &ws.Limits.RateLimitPerMinute, &ws.Enabled, &ws.CreatedAt, &ws.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Workspace{}, domain.ErrWorkspaceNotFound
	}
	if err != nil {
		return domain.Workspace{}, err
	}
	_ = json.Unmarshal([]byte(metadata), &ws.Config.Metadata)
	return ws, nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]domain.Workspace, error) {
	query := `SELECT id, name, description, owner_id, config_timezone, config_default_language, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at FROM workspaces`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []domain.Workspace
	for rows.Next() {
		var ws domain.Workspace
		var metadata string
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.OwnerID, &ws.Config.Timezone, &ws.Config.DefaultLanguage, &metadata, &ws.Limits.MaxMessagesPerDay, &ws.Limits.MaxChannels, &ws.Limits.MaxBots, &ws.Limits.RateLimitPerMinute, &ws.Enabled, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadata), &ws.Config.Metadata)
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, ws domain.Workspace) error {
	metadata, _ := json.Marshal(ws.Config.Metadata)
	query := `UPDATE workspaces SET name=?, description=?, owner_id=?, config_timezone=?, config_default_language=?, config_metadata=?, limits_max_messages_per_day=?, limits_max_channels=?, limits_max_bots=?, limits_rate_limit_per_minute=?, enabled=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, ws.Name, ws.Description, ws.OwnerID, ws.Config.Timezone, ws.Config.DefaultLanguage, string(metadata), ws.Limits.MaxMessagesPerDay, ws.Limits.MaxChannels, ws.Limits.MaxBots, ws.Limits.RateLimitPerMinute, ws.Enabled, ws.UpdatedAt, ws.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrWorkspaceNotFound
	}
	return nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM workspaces WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrWorkspaceNotFound
	}
	return nil
}

// Channel CRUD

func (r *SQLiteRepository) CreateChannel(ctx context.Context, ch domain.Channel) error {
	config, _ := json.Marshal(ch.Config)
	query := `INSERT INTO channels (id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, ch.ID, ch.WorkspaceID, ch.Type, ch.Name, ch.Enabled, string(config), ch.Status, ch.ExternalRef, ch.LastSeen, ch.CreatedAt, ch.UpdatedAt)
	return err
}

func (r *SQLiteRepository) GetChannel(ctx context.Context, channelID string) (domain.Channel, error) {
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, created_at, updated_at FROM channels WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, channelID)

	var ch domain.Channel
	var config string
	if err := row.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return domain.Channel{}, domain.ErrChannelNotFound
		}
		return domain.Channel{}, err
	}
	_ = json.Unmarshal([]byte(config), &ch.Config)
	return ch, nil
}

func (r *SQLiteRepository) ListChannels(ctx context.Context, workspaceID string) ([]domain.Channel, error) {
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, created_at, updated_at FROM channels WHERE workspace_id = ?`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []domain.Channel
	for rows.Next() {
		var ch domain.Channel
		var config string
		if err := rows.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(config), &ch.Config)
		channels = append(channels, ch)
	}
	return channels, nil
}

func (r *SQLiteRepository) UpdateChannel(ctx context.Context, ch domain.Channel) error {
	config, _ := json.Marshal(ch.Config)
	query := `UPDATE channels SET name=?, enabled=?, config=?, status=?, external_ref=?, last_seen=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, ch.Name, ch.Enabled, string(config), ch.Status, ch.ExternalRef, ch.LastSeen, ch.UpdatedAt, ch.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteChannel(ctx context.Context, channelID string) error {
	query := `DELETE FROM channels WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, channelID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrChannelNotFound
	}
	return nil
}

func (r *SQLiteRepository) GetChannelByExternalRef(ctx context.Context, externalRef string) (domain.Channel, error) {
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, created_at, updated_at FROM channels WHERE external_ref = ?`
	row := r.db.QueryRowContext(ctx, query, externalRef)

	var ch domain.Channel
	var config string
	if err := row.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return domain.Channel{}, domain.ErrChannelNotFound
		}
		return domain.Channel{}, err
	}
	_ = json.Unmarshal([]byte(config), &ch.Config)
	return ch, nil
}

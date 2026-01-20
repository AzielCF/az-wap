package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/workspace/domain/channel"
	"github.com/AzielCF/az-wap/workspace/domain/common"
	"github.com/AzielCF/az-wap/workspace/domain/workspace"
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
		`CREATE TABLE IF NOT EXISTS access_rules (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			identity TEXT NOT NULL,
			action TEXT NOT NULL,
			label TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE,
			UNIQUE(channel_id, identity)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_access_channel ON access_rules(channel_id);`,
		`CREATE INDEX IF NOT EXISTS idx_access_identity ON access_rules(identity);`,
		`CREATE INDEX IF NOT EXISTS idx_channels_workspace ON channels(workspace_id);`,
		`CREATE INDEX IF NOT EXISTS idx_channels_external_ref ON channels(external_ref);`,
		// Migraciones incrementales
		`ALTER TABLE channels ADD COLUMN accumulated_cost REAL DEFAULT 0;`,
		`ALTER TABLE channels ADD COLUMN cost_breakdown TEXT DEFAULT '{}';`,
		`CREATE TABLE IF NOT EXISTS scheduled_posts (
			id TEXT PRIMARY KEY,
			channel_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			text TEXT,
			media_path TEXT,
			media_type TEXT,
			scheduled_at DATETIME NOT NULL,
			status TEXT DEFAULT 'pending',
			error TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_posts_channel ON scheduled_posts(channel_id);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_posts_status ON scheduled_posts(status);`,
		`CREATE INDEX IF NOT EXISTS idx_scheduled_posts_channel_target ON scheduled_posts(channel_id, target_id);`,
		// Migration for sender_id
		`ALTER TABLE scheduled_posts ADD COLUMN sender_id TEXT DEFAULT '';`,
	}

	for _, query := range queries {
		if _, err := r.db.ExecContext(ctx, query); err != nil {
			// Ignorar errores de "duplicate column" en migraciones ALTER TABLE
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return fmt.Errorf("failed to init schema: %w", err)
		}
	}
	return nil
}

// Workspace CRUD

func (r *SQLiteRepository) Create(ctx context.Context, ws workspace.Workspace) error {
	metadata, _ := json.Marshal(ws.Config.Metadata)
	query := `INSERT INTO workspaces (id, name, description, owner_id, config_timezone, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, ws.ID, ws.Name, ws.Description, ws.OwnerID, ws.Config.Timezone, string(metadata), ws.Limits.MaxMessagesPerDay, ws.Limits.MaxChannels, ws.Limits.MaxBots, ws.Limits.RateLimitPerMinute, ws.Enabled, ws.CreatedAt, ws.UpdatedAt)
	return err
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id string) (workspace.Workspace, error) {
	query := `SELECT id, name, description, owner_id, config_timezone, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at FROM workspaces WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var ws workspace.Workspace
	var metadata string
	err := row.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.OwnerID, &ws.Config.Timezone, &metadata, &ws.Limits.MaxMessagesPerDay, &ws.Limits.MaxChannels, &ws.Limits.MaxBots, &ws.Limits.RateLimitPerMinute, &ws.Enabled, &ws.CreatedAt, &ws.UpdatedAt)
	if err == sql.ErrNoRows {
		return workspace.Workspace{}, common.ErrWorkspaceNotFound
	}
	if err != nil {
		return workspace.Workspace{}, err
	}
	_ = json.Unmarshal([]byte(metadata), &ws.Config.Metadata)
	return ws, nil
}

func (r *SQLiteRepository) List(ctx context.Context) ([]workspace.Workspace, error) {
	query := `SELECT id, name, description, owner_id, config_timezone, config_metadata, limits_max_messages_per_day, limits_max_channels, limits_max_bots, limits_rate_limit_per_minute, enabled, created_at, updated_at FROM workspaces`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []workspace.Workspace
	for rows.Next() {
		var ws workspace.Workspace
		var metadata string
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.OwnerID, &ws.Config.Timezone, &metadata, &ws.Limits.MaxMessagesPerDay, &ws.Limits.MaxChannels, &ws.Limits.MaxBots, &ws.Limits.RateLimitPerMinute, &ws.Enabled, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadata), &ws.Config.Metadata)
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, ws workspace.Workspace) error {
	metadata, _ := json.Marshal(ws.Config.Metadata)
	query := `UPDATE workspaces SET name=?, description=?, owner_id=?, config_timezone=?, config_metadata=?, limits_max_messages_per_day=?, limits_max_channels=?, limits_max_bots=?, limits_rate_limit_per_minute=?, enabled=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, ws.Name, ws.Description, ws.OwnerID, ws.Config.Timezone, string(metadata), ws.Limits.MaxMessagesPerDay, ws.Limits.MaxChannels, ws.Limits.MaxBots, ws.Limits.RateLimitPerMinute, ws.Enabled, ws.UpdatedAt, ws.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return common.ErrWorkspaceNotFound
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
		return common.ErrWorkspaceNotFound
	}
	return nil
}

// Channel CRUD

func (r *SQLiteRepository) CreateChannel(ctx context.Context, ch channel.Channel) error {
	config, _ := json.Marshal(ch.Config)
	breakdown, _ := json.Marshal(ch.CostBreakdown)
	query := `INSERT INTO channels (id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, accumulated_cost, cost_breakdown, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, ch.ID, ch.WorkspaceID, ch.Type, ch.Name, ch.Enabled, string(config), ch.Status, ch.ExternalRef, ch.LastSeen, ch.AccumulatedCost, string(breakdown), ch.CreatedAt, ch.UpdatedAt)
	return err
}

func (r *SQLiteRepository) GetChannel(ctx context.Context, channelID string) (channel.Channel, error) {
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, accumulated_cost, cost_breakdown, created_at, updated_at FROM channels WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, channelID)

	var ch channel.Channel
	var config string
	var breakdown sql.NullString
	if err := row.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.AccumulatedCost, &breakdown, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return channel.Channel{}, common.ErrChannelNotFound
		}
		return channel.Channel{}, err
	}
	_ = json.Unmarshal([]byte(config), &ch.Config)
	if breakdown.Valid {
		_ = json.Unmarshal([]byte(breakdown.String), &ch.CostBreakdown)
	}
	return ch, nil
}

func (r *SQLiteRepository) ListChannels(ctx context.Context, workspaceID string) ([]channel.Channel, error) {
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, accumulated_cost, cost_breakdown, created_at, updated_at FROM channels WHERE workspace_id = ?`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []channel.Channel
	for rows.Next() {
		var ch channel.Channel
		var config string
		var breakdown sql.NullString
		if err := rows.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.AccumulatedCost, &breakdown, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(config), &ch.Config)
		if breakdown.Valid {
			_ = json.Unmarshal([]byte(breakdown.String), &ch.CostBreakdown)
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

func (r *SQLiteRepository) UpdateChannel(ctx context.Context, ch channel.Channel) error {
	config, _ := json.Marshal(ch.Config)
	query := `UPDATE channels SET name=?, enabled=?, config=?, status=?, external_ref=?, last_seen=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, ch.Name, ch.Enabled, string(config), ch.Status, ch.ExternalRef, ch.LastSeen, ch.UpdatedAt, ch.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return common.ErrChannelNotFound
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
		return common.ErrChannelNotFound
	}
	return nil
}

func (r *SQLiteRepository) GetChannelByExternalRef(ctx context.Context, externalRef string) (channel.Channel, error) {
	trimmedRef := strings.TrimSpace(externalRef)
	query := `SELECT id, workspace_id, type, name, enabled, config, status, external_ref, last_seen, accumulated_cost, cost_breakdown, created_at, updated_at FROM channels WHERE TRIM(external_ref) = ?`
	row := r.db.QueryRowContext(ctx, query, trimmedRef)

	var ch channel.Channel
	var config string
	var breakdown sql.NullString
	if err := row.Scan(&ch.ID, &ch.WorkspaceID, &ch.Type, &ch.Name, &ch.Enabled, &config, &ch.Status, &ch.ExternalRef, &ch.LastSeen, &ch.AccumulatedCost, &breakdown, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return channel.Channel{}, common.ErrChannelNotFound
		}
		return channel.Channel{}, err
	}
	_ = json.Unmarshal([]byte(config), &ch.Config)
	if breakdown.Valid {
		_ = json.Unmarshal([]byte(breakdown.String), &ch.CostBreakdown)
	}
	return ch, nil
}

// Access Rules

func (r *SQLiteRepository) GetAccessRules(ctx context.Context, channelID string) ([]common.AccessRule, error) {
	query := `SELECT id, channel_id, identity, action, label, created_at, updated_at FROM access_rules WHERE channel_id = ? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []common.AccessRule
	for rows.Next() {
		var rule common.AccessRule
		if err := rows.Scan(&rule.ID, &rule.ChannelID, &rule.Identity, &rule.Action, &rule.Label, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *SQLiteRepository) AddAccessRule(ctx context.Context, rule common.AccessRule) error {
	query := `INSERT INTO access_rules (id, channel_id, identity, action, label, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, rule.ID, rule.ChannelID, rule.Identity, rule.Action, rule.Label, rule.CreatedAt, rule.UpdatedAt)
	return err
}

func (r *SQLiteRepository) DeleteAccessRule(ctx context.Context, id string) error {
	query := `DELETE FROM access_rules WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *SQLiteRepository) DeleteAllAccessRules(ctx context.Context, channelID string) error {
	query := `DELETE FROM access_rules WHERE channel_id = ?`
	_, err := r.db.ExecContext(ctx, query, channelID)
	return err
}

// AddChannelCost añade un costo al acumulado del canal de forma atómica
func (r *SQLiteRepository) AddChannelCost(ctx context.Context, channelID string, cost float64) error {
	// 1. Obtener breakdown actual
	query := `SELECT cost_breakdown FROM channels WHERE id = ?`
	var breakdownJSON sql.NullString
	err := r.db.QueryRowContext(ctx, query, channelID).Scan(&breakdownJSON)
	if err != nil {
		return err
	}

	// breakdown := make(map[string]float64)
	// No tenemos los detalles aquí directamente de la firma legacy,
	// pero podemos actualizar el total.
	// Para soportar el breakdown nuevo, necesitamos cambiar la firma o usar un truco.

	queryUpdate := `UPDATE channels SET accumulated_cost = accumulated_cost + ? WHERE id = ?`
	_, err = r.db.ExecContext(ctx, queryUpdate, cost, channelID)
	return err
}

// AddChannelComplexCost actualiza tanto el total como el desglose
func (r *SQLiteRepository) AddChannelComplexCost(ctx context.Context, channelID string, total float64, details map[string]float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Obtener breakdown actual
	var currentJSON sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT cost_breakdown FROM channels WHERE id = ?`, channelID).Scan(&currentJSON)
	if err != nil {
		return err
	}

	breakdown := make(map[string]float64)
	if currentJSON.Valid && currentJSON.String != "" {
		_ = json.Unmarshal([]byte(currentJSON.String), &breakdown)
	}

	// 2. Sumar nuevos detalles
	for k, v := range details {
		breakdown[k] += v
	}

	newJSON, _ := json.Marshal(breakdown)

	// 3. Update total y breakdown
	_, err = tx.ExecContext(ctx, `UPDATE channels SET accumulated_cost = accumulated_cost + ?, cost_breakdown = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, total, string(newJSON), channelID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Scheduled Post CRUD

func (r *SQLiteRepository) CreateScheduledPost(ctx context.Context, post common.ScheduledPost) error {
	query := `INSERT INTO scheduled_posts (id, channel_id, target_id, sender_id, text, media_path, media_type, scheduled_at, status, error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, post.ID, post.ChannelID, post.TargetID, post.SenderID, post.Text, post.MediaPath, post.MediaType, post.ScheduledAt, post.Status, post.Error, post.CreatedAt, post.UpdatedAt)
	return err
}

func (r *SQLiteRepository) GetScheduledPost(ctx context.Context, id string) (common.ScheduledPost, error) {
	query := `SELECT id, channel_id, target_id, sender_id, text, media_path, media_type, scheduled_at, status, error, created_at, updated_at FROM scheduled_posts WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var post common.ScheduledPost
	if err := row.Scan(&post.ID, &post.ChannelID, &post.TargetID, &post.SenderID, &post.Text, &post.MediaPath, &post.MediaType, &post.ScheduledAt, &post.Status, &post.Error, &post.CreatedAt, &post.UpdatedAt); err != nil {
		return common.ScheduledPost{}, err
	}
	return post, nil
}

func (r *SQLiteRepository) ListScheduledPosts(ctx context.Context, channelID string) ([]common.ScheduledPost, error) {
	query := `SELECT id, channel_id, target_id, sender_id, text, media_path, media_type, scheduled_at, status, error, created_at, updated_at FROM scheduled_posts WHERE channel_id = ? ORDER BY scheduled_at ASC`
	rows, err := r.db.QueryContext(ctx, query, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []common.ScheduledPost
	for rows.Next() {
		var post common.ScheduledPost
		if err := rows.Scan(&post.ID, &post.ChannelID, &post.TargetID, &post.SenderID, &post.Text, &post.MediaPath, &post.MediaType, &post.ScheduledAt, &post.Status, &post.Error, &post.CreatedAt, &post.UpdatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (r *SQLiteRepository) ListPendingScheduledPosts(ctx context.Context) ([]common.ScheduledPost, error) {
	query := `SELECT id, channel_id, target_id, sender_id, text, media_path, media_type, scheduled_at, status, error, created_at, updated_at FROM scheduled_posts WHERE status = 'pending' AND scheduled_at <= datetime('now')`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []common.ScheduledPost
	for rows.Next() {
		var post common.ScheduledPost
		if err := rows.Scan(&post.ID, &post.ChannelID, &post.TargetID, &post.SenderID, &post.Text, &post.MediaPath, &post.MediaType, &post.ScheduledAt, &post.Status, &post.Error, &post.CreatedAt, &post.UpdatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (r *SQLiteRepository) ListUpcomingScheduledPosts(ctx context.Context, limitTime time.Time) ([]common.ScheduledPost, error) {
	query := `SELECT id, channel_id, target_id, sender_id, text, media_path, media_type, scheduled_at, status, error, created_at, updated_at FROM scheduled_posts WHERE status = 'pending' AND scheduled_at <= ?`
	rows, err := r.db.QueryContext(ctx, query, limitTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []common.ScheduledPost
	for rows.Next() {
		var post common.ScheduledPost
		if err := rows.Scan(&post.ID, &post.ChannelID, &post.TargetID, &post.SenderID, &post.Text, &post.MediaPath, &post.MediaType, &post.ScheduledAt, &post.Status, &post.Error, &post.CreatedAt, &post.UpdatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (r *SQLiteRepository) UpdateScheduledPost(ctx context.Context, post common.ScheduledPost) error {
	query := `UPDATE scheduled_posts SET text=?, media_path=?, media_type=?, scheduled_at=?, status=?, error=?, updated_at=? WHERE id=?`
	res, err := r.db.ExecContext(ctx, query, post.Text, post.MediaPath, post.MediaType, post.ScheduledAt, post.Status, post.Error, post.UpdatedAt, post.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("scheduled post not found")
	}
	return nil
}

func (r *SQLiteRepository) DeleteScheduledPost(ctx context.Context, id string) error {
	query := `DELETE FROM scheduled_posts WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

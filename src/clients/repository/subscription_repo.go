package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/clients/domain"
	"github.com/google/uuid"
)

// SQLiteSubscriptionRepository implementa domain.SubscriptionRepository para SQLite (workspaces.db)
type SQLiteSubscriptionRepository struct {
	db *sql.DB
}

// NewSQLiteSubscriptionRepository crea una nueva instancia del repositorio
func NewSQLiteSubscriptionRepository(db *sql.DB) *SQLiteSubscriptionRepository {
	return &SQLiteSubscriptionRepository{db: db}
}

// InitSchema crea la tabla client_subscriptions si no existe
func (r *SQLiteSubscriptionRepository) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS client_subscriptions (
		id TEXT PRIMARY KEY,
		client_id TEXT NOT NULL,
		channel_id TEXT NOT NULL,
		custom_bot_id TEXT,
		custom_system_prompt TEXT,
		custom_config TEXT DEFAULT '{}',
		priority INTEGER DEFAULT 0,
		status TEXT DEFAULT 'active',
		expires_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		session_timeout INTEGER DEFAULT 0,
		inactivity_warning_time INTEGER DEFAULT 0,
		max_history_limit INTEGER,
		FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE,
		UNIQUE(client_id, channel_id)
	);
	
	CREATE INDEX IF NOT EXISTS idx_subscriptions_client ON client_subscriptions(client_id);
	CREATE INDEX IF NOT EXISTS idx_subscriptions_channel ON client_subscriptions(channel_id);
	CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON client_subscriptions(status);
	`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return err
	}

	// Migrations for existing tables
	// Ignore errors if columns already exist
	_ = r.addColumnIfNotExists(ctx, "client_subscriptions", "session_timeout", "INTEGER DEFAULT 0")
	_ = r.addColumnIfNotExists(ctx, "client_subscriptions", "inactivity_warning_time", "INTEGER DEFAULT 0")
	_ = r.addColumnIfNotExists(ctx, "client_subscriptions", "max_history_limit", "INTEGER")

	return nil
}

func (r *SQLiteSubscriptionRepository) addColumnIfNotExists(ctx context.Context, table, column, typeDef string) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typeDef))
	return err
}

// Create inserta una nueva suscripción
func (r *SQLiteSubscriptionRepository) Create(ctx context.Context, sub *domain.ClientSubscription) error {
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	if sub.CustomConfig == nil {
		sub.CustomConfig = make(map[string]any)
	}

	configJSON, err := json.Marshal(sub.CustomConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal custom_config: %w", err)
	}

	query := `
	INSERT INTO client_subscriptions (id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		sub.ID,
		sub.ClientID,
		sub.ChannelID,
		nullString(sub.CustomBotID),
		nullString(sub.CustomSystemPrompt),
		string(configJSON),
		sub.Priority,
		string(sub.Status),
		sub.ExpiresAt,
		sub.CreatedAt,
		sub.UpdatedAt,
		sub.SessionTimeout,
		sub.InactivityWarningTime,
		sub.MaxHistoryLimit,
	)

	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return domain.ErrDuplicateSubscription
	}

	return err
}

// GetByID obtiene una suscripción por su ID
func (r *SQLiteSubscriptionRepository) GetByID(ctx context.Context, id string) (*domain.ClientSubscription, error) {
	query := `SELECT id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit FROM client_subscriptions WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanSubscription(row)
}

// Update actualiza una suscripción existente
func (r *SQLiteSubscriptionRepository) Update(ctx context.Context, sub *domain.ClientSubscription) error {
	sub.UpdatedAt = time.Now()

	configJSON, err := json.Marshal(sub.CustomConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal custom_config: %w", err)
	}

	query := `
	UPDATE client_subscriptions SET
		client_id = ?,
		channel_id = ?,
		custom_bot_id = ?,
		custom_system_prompt = ?,
		custom_config = ?,
		priority = ?,
		status = ?,
		expires_at = ?,
		updated_at = ?,
		session_timeout = ?,
		inactivity_warning_time = ?,
		max_history_limit = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		sub.ClientID,
		sub.ChannelID,
		nullString(sub.CustomBotID),
		nullString(sub.CustomSystemPrompt),
		string(configJSON),
		sub.Priority,
		string(sub.Status),
		sub.ExpiresAt,
		sub.UpdatedAt,
		sub.SessionTimeout,
		sub.InactivityWarningTime,
		sub.MaxHistoryLimit,
		sub.ID,
	)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}

	return nil
}

// Delete elimina una suscripción por su ID
func (r *SQLiteSubscriptionRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM client_subscriptions WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrSubscriptionNotFound
	}

	return nil
}

// GetByClientAndChannel obtiene una suscripción por cliente y canal
func (r *SQLiteSubscriptionRepository) GetByClientAndChannel(ctx context.Context, clientID, channelID string) (*domain.ClientSubscription, error) {
	query := `SELECT id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit FROM client_subscriptions WHERE client_id = ? AND channel_id = ?`

	row := r.db.QueryRowContext(ctx, query, clientID, channelID)
	return r.scanSubscription(row)
}

// ListByClient lista todas las suscripciones de un cliente
func (r *SQLiteSubscriptionRepository) ListByClient(ctx context.Context, clientID string) ([]*domain.ClientSubscription, error) {
	query := `SELECT id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit FROM client_subscriptions WHERE client_id = ? ORDER BY priority DESC`

	rows, err := r.db.QueryContext(ctx, query, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSubscriptions(rows)
}

// ListByChannel lista todas las suscripciones de un canal
func (r *SQLiteSubscriptionRepository) ListByChannel(ctx context.Context, channelID string) ([]*domain.ClientSubscription, error) {
	query := `SELECT id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit FROM client_subscriptions WHERE channel_id = ? ORDER BY priority DESC`

	rows, err := r.db.QueryContext(ctx, query, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanSubscriptions(rows)
}

// GetActiveSubscription obtiene la suscripción activa de un cliente en un canal (para resolución en runtime)
func (r *SQLiteSubscriptionRepository) GetActiveSubscription(ctx context.Context, clientID, channelID string) (*domain.ClientSubscription, error) {
	query := `
	SELECT id, client_id, channel_id, custom_bot_id, custom_system_prompt, custom_config, priority, status, expires_at, created_at, updated_at, session_timeout, inactivity_warning_time, max_history_limit
	FROM client_subscriptions 
	WHERE client_id = ? AND channel_id = ? AND status = 'active' AND (expires_at IS NULL OR expires_at > ?)
	ORDER BY priority DESC
	LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, clientID, channelID, time.Now())
	return r.scanSubscription(row)
}

// ExpireOldSubscriptions marca como expiradas las suscripciones vencidas
func (r *SQLiteSubscriptionRepository) ExpireOldSubscriptions(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE client_subscriptions SET status = 'expired', updated_at = ? WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at < ?`, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// DeleteByClientID elimina todas las suscripciones de un cliente (cuando se elimina el cliente)
func (r *SQLiteSubscriptionRepository) DeleteByClientID(ctx context.Context, clientID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_subscriptions WHERE client_id = ?`, clientID)
	return err
}

// CountByChannel cuenta suscripciones en un canal
func (r *SQLiteSubscriptionRepository) CountByChannel(ctx context.Context, channelID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM client_subscriptions WHERE channel_id = ?`, channelID).Scan(&count)
	return count, err
}

// CountActiveByChannel cuenta suscripciones activas en un canal
func (r *SQLiteSubscriptionRepository) CountActiveByChannel(ctx context.Context, channelID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM client_subscriptions WHERE channel_id = ? AND status = 'active' AND (expires_at IS NULL OR expires_at > ?)`, channelID, time.Now()).Scan(&count)
	return count, err
}

// scanSubscription escanea una fila en una ClientSubscription
func (r *SQLiteSubscriptionRepository) scanSubscription(row *sql.Row) (*domain.ClientSubscription, error) {
	var sub domain.ClientSubscription
	var customBotID, customSystemPrompt sql.NullString
	var configJSON, status string
	var expiresAt sql.NullTime
	var maxHistoryLimit sql.NullInt64

	err := row.Scan(
		&sub.ID,
		&sub.ClientID,
		&sub.ChannelID,
		&customBotID,
		&customSystemPrompt,
		&configJSON,
		&sub.Priority,
		&status,
		&expiresAt,
		&sub.CreatedAt,
		&sub.UpdatedAt,
		&sub.SessionTimeout,
		&sub.InactivityWarningTime,
		&maxHistoryLimit,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}

	sub.CustomBotID = customBotID.String
	sub.CustomSystemPrompt = customSystemPrompt.String
	sub.Status = domain.SubscriptionStatus(status)

	if expiresAt.Valid {
		sub.ExpiresAt = &expiresAt.Time
	}

	if maxHistoryLimit.Valid {
		limit := int(maxHistoryLimit.Int64)
		sub.MaxHistoryLimit = &limit
	}

	if err := json.Unmarshal([]byte(configJSON), &sub.CustomConfig); err != nil {
		sub.CustomConfig = make(map[string]any)
	}

	return &sub, nil
}

// scanSubscriptions escanea múltiples filas en slices de ClientSubscription
func (r *SQLiteSubscriptionRepository) scanSubscriptions(rows *sql.Rows) ([]*domain.ClientSubscription, error) {
	var subs []*domain.ClientSubscription

	for rows.Next() {
		var sub domain.ClientSubscription
		var customBotID, customSystemPrompt sql.NullString
		var configJSON, status string
		var expiresAt sql.NullTime
		var maxHistoryLimit sql.NullInt64

		err := rows.Scan(
			&sub.ID,
			&sub.ClientID,
			&sub.ChannelID,
			&customBotID,
			&customSystemPrompt,
			&configJSON,
			&sub.Priority,
			&status,
			&expiresAt,
			&sub.CreatedAt,
			&sub.UpdatedAt,
			&sub.SessionTimeout,
			&sub.InactivityWarningTime,
			&maxHistoryLimit,
		)

		if err != nil {
			return nil, err
		}

		sub.CustomBotID = customBotID.String
		sub.CustomSystemPrompt = customSystemPrompt.String
		sub.Status = domain.SubscriptionStatus(status)

		if expiresAt.Valid {
			sub.ExpiresAt = &expiresAt.Time
		}

		if maxHistoryLimit.Valid {
			limit := int(maxHistoryLimit.Int64)
			sub.MaxHistoryLimit = &limit
		}

		if err := json.Unmarshal([]byte(configJSON), &sub.CustomConfig); err != nil {
			sub.CustomConfig = make(map[string]any)
		}

		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// nullString convierte un string vacío en sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

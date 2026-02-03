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

// SQLiteClientRepository implementa domain.ClientRepository para SQLite (app.db)
type SQLiteClientRepository struct {
	db *sql.DB
}

// NewSQLiteClientRepository crea una nueva instancia del repositorio
func NewSQLiteClientRepository(db *sql.DB) *SQLiteClientRepository {
	return &SQLiteClientRepository{db: db}
}

// InitSchema crea la tabla clients si no existe
func (r *SQLiteClientRepository) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS clients (
		id TEXT PRIMARY KEY,
		platform_id TEXT NOT NULL,
		platform_type TEXT NOT NULL,
		display_name TEXT,
		email TEXT,
		phone TEXT,
		tier TEXT DEFAULT 'standard',
		tags TEXT DEFAULT '[]',
		metadata TEXT DEFAULT '{}',
		notes TEXT,
		language TEXT DEFAULT 'en',
		allowed_bots TEXT DEFAULT '[]',
		enabled BOOLEAN DEFAULT 1,
		last_interaction DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(platform_id, platform_type)
	);
	
	CREATE INDEX IF NOT EXISTS idx_clients_platform ON clients(platform_id, platform_type);
	CREATE INDEX IF NOT EXISTS idx_clients_tier ON clients(tier);
	CREATE INDEX IF NOT EXISTS idx_clients_email ON clients(email);
	`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return err
	}

	// Manual migration: Add language column if it doesn't exist (ignores error if already exists)
	_, _ = r.db.ExecContext(ctx, "ALTER TABLE clients ADD COLUMN language TEXT DEFAULT 'en'")
	_, _ = r.db.ExecContext(ctx, "ALTER TABLE clients ADD COLUMN allowed_bots TEXT DEFAULT '[]'")
	_, _ = r.db.ExecContext(ctx, "ALTER TABLE clients ADD COLUMN timezone TEXT")
	_, _ = r.db.ExecContext(ctx, "ALTER TABLE clients ADD COLUMN country TEXT")

	return nil
}

// Create inserta un nuevo cliente en la base de datos
func (r *SQLiteClientRepository) Create(ctx context.Context, client *domain.Client) error {
	if client.ID == "" {
		client.ID = uuid.New().String()
	}
	now := time.Now()
	client.CreatedAt = now
	client.UpdatedAt = now

	tagsJSON, err := json.Marshal(client.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(client.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	botsJSON, err := json.Marshal(client.AllowedBots)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed_bots: %w", err)
	}

	query := `
	INSERT INTO clients (id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		client.ID,
		client.PlatformID,
		string(client.PlatformType),
		client.DisplayName,
		client.Email,
		client.Phone,
		string(client.Tier),
		string(tagsJSON),
		string(metadataJSON),
		client.Notes,
		client.Language,
		client.Timezone,
		client.Country,
		string(botsJSON),
		client.Enabled,
		client.LastInteraction,
		client.CreatedAt,
		client.UpdatedAt,
	)

	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return domain.ErrDuplicateClient
	}

	return err
}

// GetByID obtiene un cliente por su ID
func (r *SQLiteClientRepository) GetByID(ctx context.Context, id string) (*domain.Client, error) {
	query := `SELECT id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at FROM clients WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanClient(row)
}

// GetByPlatform obtiene un cliente por su platform_id y platform_type
func (r *SQLiteClientRepository) GetByPlatform(ctx context.Context, platformID string, platformType domain.PlatformType) (*domain.Client, error) {
	query := `SELECT id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at FROM clients WHERE platform_id = ? AND platform_type = ?`

	row := r.db.QueryRowContext(ctx, query, platformID, string(platformType))
	return r.scanClient(row)
}

// GetByPhone obtiene un cliente por su número de teléfono
func (r *SQLiteClientRepository) GetByPhone(ctx context.Context, phone string) (*domain.Client, error) {
	// Limpiar el teléfono de caracteres no numéricos para la búsqueda
	cleanPhone := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	query := `SELECT id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at FROM clients WHERE phone LIKE ? OR phone = ?`

	// Intentar con LIKE para manejar prefijos (ej: +51...)
	row := r.db.QueryRowContext(ctx, query, "%"+cleanPhone+"%", phone)
	return r.scanClient(row)
}

// Update actualiza un cliente existente
func (r *SQLiteClientRepository) Update(ctx context.Context, client *domain.Client) error {
	client.UpdatedAt = time.Now()

	tagsJSON, err := json.Marshal(client.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(client.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	botsJSON, err := json.Marshal(client.AllowedBots)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed_bots: %w", err)
	}

	query := `
	UPDATE clients SET
		platform_id = ?,
		platform_type = ?,
		display_name = ?,
		email = ?,
		phone = ?,
		tier = ?,
		tags = ?,
		metadata = ?,
		notes = ?,
		language = ?,
		timezone = ?,
		country = ?,
		allowed_bots = ?,
		enabled = ?,
		last_interaction = ?,
		updated_at = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		client.PlatformID,
		string(client.PlatformType),
		client.DisplayName,
		client.Email,
		client.Phone,
		string(client.Tier),
		string(tagsJSON),
		string(metadataJSON),
		client.Notes,
		client.Language,
		client.Timezone,
		client.Country,
		string(botsJSON),
		client.Enabled,
		client.LastInteraction,
		client.UpdatedAt,
		client.ID,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrDuplicateClient
		}
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrClientNotFound
	}

	return nil
}

// Delete elimina un cliente por su ID
func (r *SQLiteClientRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM clients WHERE id = ?`, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrClientNotFound
	}

	return nil
}

// List obtiene una lista de clientes con filtros
func (r *SQLiteClientRepository) List(ctx context.Context, filter domain.ClientFilter) ([]*domain.Client, error) {
	query := `SELECT id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at FROM clients WHERE 1=1`
	args := []any{}

	if filter.Tier != nil {
		query += " AND tier = ?"
		args = append(args, string(*filter.Tier))
	}

	if filter.Enabled != nil {
		query += " AND enabled = ?"
		args = append(args, *filter.Enabled)
	}

	if filter.Search != "" {
		query += " AND (display_name LIKE ? OR email LIKE ? OR phone LIKE ?)"
		searchPattern := "%" + filter.Search + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	// Order
	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	orderDir := "ASC"
	if filter.OrderDesc {
		orderDir = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

	// Pagination
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []*domain.Client
	for rows.Next() {
		client, err := r.scanClientFromRows(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}

	return clients, rows.Err()
}

// ListByTier obtiene clientes por tier
func (r *SQLiteClientRepository) ListByTier(ctx context.Context, tier domain.ClientTier) ([]*domain.Client, error) {
	return r.List(ctx, domain.ClientFilter{Tier: &tier})
}

// ListByTag obtiene clientes que tengan un tag específico
func (r *SQLiteClientRepository) ListByTag(ctx context.Context, tag string) ([]*domain.Client, error) {
	query := `SELECT id, platform_id, platform_type, display_name, email, phone, tier, tags, metadata, notes, language, timezone, country, allowed_bots, enabled, last_interaction, created_at, updated_at FROM clients WHERE tags LIKE ?`

	rows, err := r.db.QueryContext(ctx, query, "%\""+tag+"\"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []*domain.Client
	for rows.Next() {
		client, err := r.scanClientFromRows(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}

	return clients, rows.Err()
}

// Search busca clientes por texto libre
func (r *SQLiteClientRepository) Search(ctx context.Context, query string) ([]*domain.Client, error) {
	return r.List(ctx, domain.ClientFilter{Search: query, Limit: 50})
}

// CountByTier cuenta clientes agrupados por tier
func (r *SQLiteClientRepository) CountByTier(ctx context.Context) (map[domain.ClientTier]int, error) {
	query := `SELECT tier, COUNT(*) FROM clients GROUP BY tier`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[domain.ClientTier]int)
	for rows.Next() {
		var tier string
		var count int
		if err := rows.Scan(&tier, &count); err != nil {
			return nil, err
		}
		result[domain.ClientTier(tier)] = count
	}

	return result, rows.Err()
}

// UpdateLastInteraction actualiza la última interacción de un cliente
func (r *SQLiteClientRepository) UpdateLastInteraction(ctx context.Context, id string, t time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET last_interaction = ?, updated_at = ? WHERE id = ?`, t, time.Now(), id)
	return err
}

// AddTag agrega un tag a un cliente
func (r *SQLiteClientRepository) AddTag(ctx context.Context, id string, tag string) error {
	client, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Verificar si ya tiene el tag
	for _, t := range client.Tags {
		if t == tag {
			return nil // Ya existe, no hacer nada
		}
	}

	client.Tags = append(client.Tags, tag)
	return r.Update(ctx, client)
}

// RemoveTag elimina un tag de un cliente
func (r *SQLiteClientRepository) RemoveTag(ctx context.Context, id string, tag string) error {
	client, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	newTags := []string{}
	for _, t := range client.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}

	client.Tags = newTags
	return r.Update(ctx, client)
}

// scanClient escanea una fila en un Client
func (r *SQLiteClientRepository) scanClient(row *sql.Row) (*domain.Client, error) {
	var client domain.Client
	var tagsJSON, metadataJSON, botsJSON string
	var platformType, tier string
	var lastInteraction sql.NullTime
	var timezone, country sql.NullString

	err := row.Scan(
		&client.ID,
		&client.PlatformID,
		&platformType,
		&client.DisplayName,
		&client.Email,
		&client.Phone,
		&tier,
		&tagsJSON,
		&metadataJSON,
		&client.Notes,
		&client.Language,
		&timezone,
		&country,
		&botsJSON,
		&client.Enabled,
		&lastInteraction,
		&client.CreatedAt,
		&client.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrClientNotFound
	}
	if err != nil {
		return nil, err
	}

	client.PlatformType = domain.PlatformType(platformType)
	client.Tier = domain.ClientTier(tier)
	client.Timezone = timezone.String
	client.Country = country.String

	if lastInteraction.Valid {
		client.LastInteraction = &lastInteraction.Time
	}

	if err := json.Unmarshal([]byte(tagsJSON), &client.Tags); err != nil {
		client.Tags = []string{}
	}

	if err := json.Unmarshal([]byte(metadataJSON), &client.Metadata); err != nil {
		client.Metadata = make(map[string]any)
	}

	if err := json.Unmarshal([]byte(botsJSON), &client.AllowedBots); err != nil {
		client.AllowedBots = []string{}
	}

	return &client, nil
}

// scanClientFromRows escanea una fila de sql.Rows en un Client
func (r *SQLiteClientRepository) scanClientFromRows(rows *sql.Rows) (*domain.Client, error) {
	var client domain.Client
	var tagsJSON, metadataJSON, botsJSON string
	var platformType, tier string
	var lastInteraction sql.NullTime
	var timezone, country sql.NullString

	err := rows.Scan(
		&client.ID,
		&client.PlatformID,
		&platformType,
		&client.DisplayName,
		&client.Email,
		&client.Phone,
		&tier,
		&tagsJSON,
		&metadataJSON,
		&client.Notes,
		&client.Language,
		&timezone,
		&country,
		&botsJSON,
		&client.Enabled,
		&lastInteraction,
		&client.CreatedAt,
		&client.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	client.PlatformType = domain.PlatformType(platformType)
	client.Tier = domain.ClientTier(tier)
	client.Timezone = timezone.String
	client.Country = country.String

	if lastInteraction.Valid {
		client.LastInteraction = &lastInteraction.Time
	}

	if err := json.Unmarshal([]byte(tagsJSON), &client.Tags); err != nil {
		client.Tags = []string{}
	}

	if err := json.Unmarshal([]byte(metadataJSON), &client.Metadata); err != nil {
		client.Metadata = make(map[string]any)
	}

	if err := json.Unmarshal([]byte(botsJSON), &client.AllowedBots); err != nil {
		client.AllowedBots = []string{}
	}

	return &client, nil
}

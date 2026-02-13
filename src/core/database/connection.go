package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/AzielCF/az-wap/core/config"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GlobalDB holds the singleton database connection
var GlobalDB *gorm.DB

// GetLegacyDB returns the underlying *sql.DB for legacy compatibility
func GetLegacyDB() (*sql.DB, error) {
	if GlobalDB == nil {
		return nil, fmt.Errorf("global database not initialized")
	}
	return GlobalDB.DB()
}

// NewDatabase initializes a database connection based on the provided configuration.
func NewDatabase(cfg *config.Config) (*gorm.DB, error) {
	db, err := NewDatabaseWithCustomPath(cfg, cfg.Database.Name)
	if err == nil {
		GlobalDB = db
	}
	return db, err
}

// NewDatabaseWithCustomPath allows opening a secondary database file (for SQLite) with the same global settings.
func NewDatabaseWithCustomPath(cfg *config.Config, path string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
			cfg.Database.Host,
			cfg.Database.User,
			cfg.Database.Password,
			path, // Path acts as dbname in Postgres
			cfg.Database.Port,
		)
		dialector = postgres.Open(dsn)
	case "sqlite", "": // Default to SQLite
		dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", path)
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database (%s): %w", path, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	if cfg.Database.Driver == "sqlite" || cfg.Database.Driver == "" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	return db, nil
}

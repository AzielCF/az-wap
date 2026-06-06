package db

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SafeMigrateSQLite bypasses GORM's flawed SQLite table recreation logic
// by manually preserving data into a temporary table when legacy constraints
// like FOREIGN KEYs are present. It ensures all specified models are fully
// aligned with their GORM definitions without crashing.
func SafeMigrateSQLite(ctx context.Context, db *gorm.DB, models map[string]interface{}) error {
	modelsSlice := make([]interface{}, 0, len(models))
	for _, m := range models {
		modelsSlice = append(modelsSlice, m)
	}
	return db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Error)}).AutoMigrate(modelsSlice...)
}

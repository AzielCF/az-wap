package db

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// SafeMigrateSQLite bypasses GORM's flawed SQLite table recreation logic
// by manually preserving data into a temporary table when legacy constraints
// like FOREIGN KEYs are present. It ensures all specified models are fully
// aligned with their GORM definitions without crashing.
func SafeMigrateSQLite(ctx context.Context, db *gorm.DB, models map[string]interface{}) error {
	// Phase 1: Create new tables and copy data. 
	// We DO NOT drop old tables yet, to prevent ON DELETE CASCADE from wiping related tables before they are copied.
	for tableName, model := range models {
		var count int64
		db.WithContext(ctx).Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
		if count == 0 {
			continue // AutoMigrate will handle fresh creation
		}

		oldTableName := tableName + "_old_mig"
		db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", oldTableName))

		var oldCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tableName).Scan(&oldCols)

		var indexes []string
		db.WithContext(ctx).Raw("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?", tableName).Scan(&indexes)
		for _, idx := range indexes {
			if !strings.HasPrefix(idx, "sqlite_autoindex") {
				db.WithContext(ctx).Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx))
			}
		}

		// Disable foreign keys at connection level (best effort)
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = OFF")
		
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tableName, oldTableName)).Error; err != nil {
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to rename old table %s: %w", tableName, err)
		}

		if err := db.WithContext(ctx).AutoMigrate(model); err != nil {
			db.WithContext(ctx).Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldTableName, tableName))
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to create new table %s: %w", tableName, err)
		}

		var newCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tableName).Scan(&newCols)

		commonCols := make([]string, 0)
		for _, o := range oldCols {
			for _, n := range newCols {
				if o == n {
					commonCols = append(commonCols, o)
					break
				}
			}
		}

		if len(commonCols) > 0 {
			colsStr := strings.Join(commonCols, ", ")
			insertQuery := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) SELECT %s FROM %s", tableName, colsStr, colsStr, oldTableName)
			if err := db.WithContext(ctx).Exec(insertQuery).Error; err != nil {
				db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to copy data for %s: %w", tableName, err)
			}
		}
		
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
	}

	// Phase 2: Data Cleanup for legacy NULLs that break Go's string scanner
	// If a table exists in models, we can sanitize known problematic columns.
	if _, ok := models["workspaces"]; ok {
		db.WithContext(ctx).Exec("UPDATE workspaces SET name = 'Workspace' WHERE name IS NULL OR name = ''")
		db.WithContext(ctx).Exec("UPDATE workspaces SET owner_id = 'system' WHERE owner_id IS NULL OR owner_id = ''")
	}
	if _, ok := models["channels"]; ok {
		db.WithContext(ctx).Exec("UPDATE channels SET owner_id = 'system' WHERE owner_id IS NULL OR owner_id = ''")
	}

	// Phase 3: Safely drop all old tables. We disabled foreign keys so this doesn't cascade,
	// but even if it does, it only cascades to other _old_mig tables.
	db.WithContext(ctx).Exec("PRAGMA foreign_keys = OFF")
	for tableName := range models {
		oldTableName := tableName + "_old_mig"
		db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", oldTableName))
	}
	db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")

	// Final AutoMigrate just to be absolutely sure relationships are linked
	modelsSlice := make([]interface{}, 0, len(models))
	for _, m := range models {
		modelsSlice = append(modelsSlice, m)
	}
	return db.WithContext(ctx).AutoMigrate(modelsSlice...)
}

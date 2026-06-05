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
	for tableName, model := range models {
		// 1. Check if table exists
		var count int64
		db.WithContext(ctx).Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
		if count == 0 {
			continue // AutoMigrate will handle fresh creation
		}

		oldTableName := tableName + "_old_mig"
		
		// Ensure old temp table is cleaned up in case of previous failure
		db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", oldTableName))

		// 2. Get columns of the old table
		var oldCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tableName).Scan(&oldCols)

		// 3. Drop all indexes on the old table to free up their global names
		// SQLite requires index names to be globally unique.
		var indexes []string
		db.WithContext(ctx).Raw("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?", tableName).Scan(&indexes)
		for _, idx := range indexes {
			if !strings.HasPrefix(idx, "sqlite_autoindex") {
				db.WithContext(ctx).Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx))
			}
		}

		// 4. Rename old table with foreign keys disabled
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = OFF")
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tableName, oldTableName)).Error; err != nil {
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to rename old table %s: %w", tableName, err)
		}

		// 5. Use GORM to create the perfectly clean NEW table
		if err := db.WithContext(ctx).AutoMigrate(model); err != nil {
			// Attempt recovery
			db.WithContext(ctx).Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldTableName, tableName))
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to create new table %s: %w", tableName, err)
		}

		// 6. Get columns of the new table
		var newCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tableName).Scan(&newCols)

		// 7. Find intersection
		commonCols := make([]string, 0)
		for _, o := range oldCols {
			for _, n := range newCols {
				if o == n {
					commonCols = append(commonCols, o)
					break
				}
			}
		}

		// 8. Copy data safely
		if len(commonCols) > 0 {
			colsStr := strings.Join(commonCols, ", ")
			insertQuery := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) SELECT %s FROM %s", tableName, colsStr, colsStr, oldTableName)
			if err := db.WithContext(ctx).Exec(insertQuery).Error; err != nil {
				db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
				return fmt.Errorf("failed to copy data for %s: %w", tableName, err)
			}
		}

		// 9. Drop the old table and re-enable foreign keys
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE %s", oldTableName)).Error; err != nil {
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to drop old table %s: %w", oldTableName, err)
		}
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
	}

	// 10. Final AutoMigrate just to be absolutely sure relationships are linked
	modelsSlice := make([]interface{}, 0, len(models))
	for _, m := range models {
		modelsSlice = append(modelsSlice, m)
	}
	return db.WithContext(ctx).AutoMigrate(modelsSlice...)
}

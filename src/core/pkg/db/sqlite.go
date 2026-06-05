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

		// 2. We create a temporary table mimicking the NEW schema
		tempName := tableName + "_safe_mig"
		
		// Ensure temp table is cleaned up in case of previous failure
		db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tempName))

		// 3. Use GORM to create the temp table with the perfect new schema
		if err := db.WithContext(ctx).Table(tempName).AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to create temp table %s: %w", tempName, err)
		}

		// 4. Get columns for both tables
		var oldCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tableName).Scan(&oldCols)
		
		var newCols []string
		db.WithContext(ctx).Raw("SELECT name FROM PRAGMA_table_info(?)", tempName).Scan(&newCols)

		// 5. Find intersection
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
			// 6. Copy data safely
			insertQuery := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) SELECT %s FROM %s", tempName, colsStr, colsStr, tableName)
			if err := db.WithContext(ctx).Exec(insertQuery).Error; err != nil {
				return fmt.Errorf("failed to copy data for %s: %w", tableName, err)
			}
		}

		// 7. Swap tables safely with foreign keys disabled
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = OFF")
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE %s", tableName)).Error; err != nil {
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to drop old table %s: %w", tableName, err)
		}
		
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tempName, tableName)).Error; err != nil {
			db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")
			return fmt.Errorf("failed to rename temp table to %s: %w", tableName, err)
		}
		db.WithContext(ctx).Exec("PRAGMA foreign_keys = ON")

		// 8. Drop all indexes on the renamed table (they have temp names).
		// AutoMigrate will recreate them correctly in the final step.
		var indexes []string
		db.WithContext(ctx).Raw("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?", tableName).Scan(&indexes)
		for _, idx := range indexes {
			if !strings.HasPrefix(idx, "sqlite_autoindex") {
				db.WithContext(ctx).Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx))
			}
		}
	}

	// 9. Finally, let GORM do a clean AutoMigrate to establish any missing indexes
	modelsSlice := make([]interface{}, 0, len(models))
	for _, m := range models {
		modelsSlice = append(modelsSlice, m)
	}
	return db.WithContext(ctx).AutoMigrate(modelsSlice...)
}

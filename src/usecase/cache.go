package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
)

type cacheService struct{}

func NewCacheService() domainCache.ICacheUsecase {
	return &cacheService{}
}

func (s *cacheService) GetSettings(ctx context.Context) (domainCache.CacheSettings, error) {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return domainCache.CacheSettings{}, err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS global_settings (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return domainCache.CacheSettings{}, err
	}

	settings := domainCache.CacheSettings{
		Enabled:         false,
		MaxAgeDays:      30,
		MaxSizeMB:       1024, // 1GB default
		CleanupInterval: 60,   // 1 hour default
	}

	rows, err := db.Query(`SELECT key, value FROM global_settings WHERE key LIKE 'cache_%'`)
	if err != nil {
		return settings, nil
	}
	defer rows.Close()

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err == nil {
			switch key {
			case "cache_enabled":
				settings.Enabled = val == "1" || val == "true"
			case "cache_max_age_days":
				if n, err := strconv.Atoi(val); err == nil {
					settings.MaxAgeDays = n
				}
			case "cache_max_size_mb":
				if n, err := strconv.ParseInt(val, 10, 64); err == nil {
					settings.MaxSizeMB = n
				}
			case "cache_cleanup_interval":
				if n, err := strconv.Atoi(val); err == nil {
					settings.CleanupInterval = n
				}
			}
		}
	}

	return settings, nil
}

func (s *cacheService) SaveSettings(ctx context.Context, settings domainCache.CacheSettings) error {
	dbPath := fmt.Sprintf("%s/instances.db", config.PathStorages)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	save := func(key, val string) {
		db.Exec(`INSERT INTO global_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, val)
	}

	enabledStr := "0"
	if settings.Enabled {
		enabledStr = "1"
	}

	save("cache_enabled", enabledStr)
	save("cache_max_age_days", strconv.Itoa(settings.MaxAgeDays))
	save("cache_max_size_mb", strconv.FormatInt(settings.MaxSizeMB, 10))
	save("cache_cleanup_interval", strconv.Itoa(settings.CleanupInterval))

	return nil
}

func (s *cacheService) StartBackgroundCleanup(ctx context.Context) {
	go func() {
		for {
			settings, err := s.GetSettings(context.Background())
			if err != nil || !settings.Enabled {
				// Wait 5 minutes and check again if not enabled or error
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Minute):
					continue
				}
			}

			logrus.Info("[CACHE] Running scheduled cleanup...")
			s.runCleanup(settings)

			interval := time.Duration(settings.CleanupInterval) * time.Minute
			if interval < 5*time.Minute {
				interval = 5 * time.Minute
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
		}
	}()
}

func (s *cacheService) runCleanup(settings domainCache.CacheSettings) {
	// 1. Delete files by age
	maxAge := time.Duration(settings.MaxAgeDays) * 24 * time.Hour
	cutoff := time.Now().Add(-maxAge)

	dirs := []string{config.PathMedia, config.PathQrCode, config.PathSendItems}
	for _, dir := range dirs {
		s.pruneByAge(dir, cutoff)
	}
	s.pruneFilesByPatternAge(config.PathStorages, "history-*", cutoff)

	// 2. Delete files by size limit
	maxSizeBytes := settings.MaxSizeMB * 1024 * 1024
	if maxSizeBytes > 0 {
		s.pruneBySize(maxSizeBytes)
	}
}

func (s *cacheService) pruneByAge(path string, cutoff time.Time) {
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() != ".gitignore" && info.ModTime().Before(cutoff) {
			os.Remove(p)
		}
		return nil
	})
}

func (s *cacheService) pruneFilesByPatternAge(dir, pattern string, cutoff time.Time) {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && !info.IsDir() && info.Name() != ".gitignore" && info.ModTime().Before(cutoff) {
			os.Remove(match)
		}
	}
}

type fileInfo struct {
	Path string
	Size int64
	Time time.Time
}

func (s *cacheService) pruneBySize(limit int64) {
	var files []fileInfo
	var totalSize int64

	collect := func(path string) {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && info.Name() != ".gitignore" {
				files = append(files, fileInfo{Path: p, Size: info.Size(), Time: info.ModTime()})
				totalSize += info.Size()
			}
			return nil
		})
	}

	dirs := []string{config.PathMedia, config.PathQrCode, config.PathSendItems}
	for _, dir := range dirs {
		collect(dir)
	}

	// Historiales también cuentan
	matches, _ := filepath.Glob(filepath.Join(config.PathStorages, "history-*"))
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil {
			files = append(files, fileInfo{Path: m, Size: info.Size(), Time: info.ModTime()})
			totalSize += info.Size()
		}
	}

	if totalSize <= limit {
		return
	}

	// Sort files by time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Time.Before(files[j].Time)
	})

	for _, f := range files {
		if totalSize <= limit {
			break
		}
		if err := os.Remove(f.Path); err == nil {
			totalSize -= f.Size
		}
	}
}

func (s *cacheService) GetGlobalStats(ctx context.Context) (domainCache.CacheStats, error) {
	var totalSize int64

	// Directories to scan
	dirs := []string{config.PathMedia, config.PathQrCode, config.PathSendItems}
	for _, dir := range dirs {
		size, _ := s.getDirSize(dir)
		totalSize += size
	}

	// Scan storages for history files
	historySize, _ := s.getFilesByPatternSize(config.PathStorages, "history-*")
	totalSize += historySize

	return domainCache.CacheStats{
		TotalSize: totalSize,
		HumanSize: humanize.Bytes(uint64(totalSize)),
	}, nil
}

func (s *cacheService) ClearGlobalCache(ctx context.Context) error {
	dirs := []string{config.PathMedia, config.PathQrCode, config.PathSendItems}
	for _, dir := range dirs {
		s.clearDir(dir)
	}

	// Clear history files
	s.clearFilesByPattern(config.PathStorages, "history-*")

	return nil
}

func (s *cacheService) GetInstanceStats(ctx context.Context, instanceID string) (domainCache.CacheStats, error) {
	var totalSize int64

	// Media for this instance (if stored in subfolder)
	instanceMediaDir := filepath.Join(config.PathMedia, instanceID)
	size, _ := s.getDirSize(instanceMediaDir)
	totalSize += size

	// History files for this instance
	// Pattern: history-*-<instanceID>-*.json
	pattern := fmt.Sprintf("history-*-%s-*.json", instanceID)
	histSize, _ := s.getFilesByPatternSize(config.PathStorages, pattern)
	totalSize += histSize

	return domainCache.CacheStats{
		TotalSize: totalSize,
		HumanSize: humanize.Bytes(uint64(totalSize)),
	}, nil
}

func (s *cacheService) ClearInstanceCache(ctx context.Context, instanceID string) error {
	// Clear media subfolder
	instanceMediaDir := filepath.Join(config.PathMedia, instanceID)
	os.RemoveAll(instanceMediaDir)

	// Clear history files
	pattern := fmt.Sprintf("history-*-%s-*.json", instanceID)
	s.clearFilesByPattern(config.PathStorages, pattern)

	return nil
}

// Helper: Get directory size
func (s *cacheService) getDirSize(path string) (int64, error) {
	var size int64
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() != ".gitignore" {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// Helper: Get size of files matching a pattern in a directory
func (s *cacheService) getFilesByPatternSize(dir, pattern string) (int64, error) {
	var size int64
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0, err
	}
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil && !info.IsDir() && info.Name() != ".gitignore" {
			size += info.Size()
		}
	}
	return size, nil
}

// Helper: Clear directory contents (keep .gitignore)
func (s *cacheService) clearDir(path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.Name() == ".gitignore" {
			continue
		}
		os.RemoveAll(filepath.Join(path, f.Name()))
	}
}

// Helper: Clear files matching pattern
func (s *cacheService) clearFilesByPattern(dir, pattern string) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		logrus.Errorf("[CACHE] Failed to glob pattern %s: %v", pattern, err)
		return
	}
	for _, match := range matches {
		if filepath.Base(match) == ".gitignore" {
			continue
		}
		os.Remove(match)
	}
}

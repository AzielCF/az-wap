package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	coreconfig "github.com/AzielCF/az-wap/core/config"
	coreSettings "github.com/AzielCF/az-wap/core/settings/application"
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	"github.com/AzielCF/az-wap/pkg/chatmedia"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
)

type cacheService struct {
	settingsSvc *coreSettings.SettingsService
}

func NewCacheService(settingsSvc *coreSettings.SettingsService) domainCache.ICacheUsecase {
	return &cacheService{settingsSvc: settingsSvc}
}

func (s *cacheService) GetSettings(ctx context.Context) (domainCache.CacheSettings, error) {
	settings := domainCache.CacheSettings{
		Enabled:         false,
		MaxAgeDays:      30,
		MaxSizeMB:       1024, // 1GB default
		CleanupInterval: 60,   // 1 hour default
	}

	if s.settingsSvc == nil {
		return settings, nil
	}

	ds, err := s.settingsSvc.GetDynamicSettings(ctx)
	if err != nil {
		return settings, nil
	}

	if ds.CacheEnabled != nil {
		settings.Enabled = *ds.CacheEnabled
	}
	if ds.CacheMaxAgeDays != nil {
		settings.MaxAgeDays = *ds.CacheMaxAgeDays
	}
	if ds.CacheMaxSizeMB != nil {
		settings.MaxSizeMB = *ds.CacheMaxSizeMB
	}
	if ds.CacheCleanupInterval != nil {
		settings.CleanupInterval = *ds.CacheCleanupInterval
	}

	return settings, nil
}

func (s *cacheService) SaveSettings(ctx context.Context, settings domainCache.CacheSettings) error {
	if s.settingsSvc == nil {
		return fmt.Errorf("settings service not initialized")
	}

	if err := s.settingsSvc.SetCacheEnabled(ctx, settings.Enabled); err != nil {
		return err
	}
	if err := s.settingsSvc.SetCacheMaxAge(ctx, settings.MaxAgeDays); err != nil {
		return err
	}
	if err := s.settingsSvc.SetCacheMaxSize(ctx, settings.MaxSizeMB); err != nil {
		return err
	}
	if err := s.settingsSvc.SetCacheCleanupInterval(ctx, settings.CleanupInterval); err != nil {
		return err
	}

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

	dirs := []string{filepath.Join(coreconfig.Global.Paths.Statics, "workspaces"), coreconfig.Global.Paths.SendItems}
	for _, dir := range dirs {
		s.pruneByAge(dir, cutoff)
	}
	s.pruneFilesByPatternAge(coreconfig.Global.Paths.Storages, "history-*", cutoff)
	s.pruneFilesByPatternAge(coreconfig.Global.Paths.Storages, "*.jfif", cutoff)

	// 2. Delete files by size limit
	maxSizeBytes := settings.MaxSizeMB * 1024 * 1024
	if maxSizeBytes > 0 {
		s.pruneBySize(maxSizeBytes)
	}

	// 3. Chatmedia cleanup (in-memory and short-lived files)
	chatmedia.Cleanup()
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

	dirs := []string{filepath.Join(coreconfig.Global.Paths.Statics, "workspaces"), coreconfig.Global.Paths.SendItems}
	for _, dir := range dirs {
		collect(dir)
	}

	// Historiales también cuentan
	matches, _ := filepath.Glob(filepath.Join(coreconfig.Global.Paths.Storages, "history-*"))
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
	dirs := []string{filepath.Join(coreconfig.Global.Paths.Statics, "workspaces"), coreconfig.Global.Paths.SendItems}
	for _, dir := range dirs {
		size, _ := s.getDirSize(dir)
		totalSize += size
	}

	// Scan storages for history files
	historySize, _ := s.getFilesByPatternSize(coreconfig.Global.Paths.Storages, "history-*")
	totalSize += historySize

	return domainCache.CacheStats{
		TotalSize: totalSize,
		HumanSize: humanize.Bytes(uint64(totalSize)),
	}, nil
}

func (s *cacheService) ClearGlobalCache(ctx context.Context) error {
	dirs := []string{filepath.Join(coreconfig.Global.Paths.Statics, "workspaces"), coreconfig.Global.Paths.SendItems}
	for _, dir := range dirs {
		s.clearDir(dir)
	}

	// Clear history files and jfif files from storages
	s.clearFilesByPattern(coreconfig.Global.Paths.Storages, "history-*")
	s.clearFilesByPattern(coreconfig.Global.Paths.Storages, "*.jfif")

	return nil
}

func (s *cacheService) GetInstanceStats(ctx context.Context, instanceID string) (domainCache.CacheStats, error) {
	var totalSize int64

	// Media for this instance (look inside workspaces)
	instanceMediaDir := filepath.Join(coreconfig.Global.Paths.Statics, "workspaces", "*", instanceID)
	size, _ := s.getDirSize(instanceMediaDir)
	totalSize += size

	// contextCache is separate and managed by botengine
	totalSize += 0 // Fallback if needed

	// History files for this instance
	// Pattern: history-*-<instanceID>-*.json
	pattern := fmt.Sprintf("history-*-%s-*.json", instanceID)
	histSize, _ := s.getFilesByPatternSize(coreconfig.Global.Paths.Storages, pattern)
	totalSize += histSize

	return domainCache.CacheStats{
		TotalSize: totalSize,
		HumanSize: humanize.Bytes(uint64(totalSize)),
	}, nil
}

func (s *cacheService) ClearInstanceCache(ctx context.Context, instanceID string) error {
	// Clear media subfolder (look inside workspaces)
	instanceMediaDir := filepath.Join(coreconfig.Global.Paths.Statics, "workspaces", "*", instanceID)
	os.RemoveAll(instanceMediaDir)

	// instanceCacheDir is now gone
	_ = instanceID

	// Clear history files
	pattern := fmt.Sprintf("history-*-%s-*.json", instanceID)
	s.clearFilesByPattern(coreconfig.Global.Paths.Storages, pattern)

	// Clear jfif files from storages
	s.clearFilesByPattern(coreconfig.Global.Paths.Storages, "*.jfif")

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

package chatstorage

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/AzielCF/az-wap/config"
	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	"github.com/sirupsen/logrus"
)

var (
	instanceRepoMu sync.RWMutex
	instanceRepos  = make(map[string]domainChatStorage.IChatStorageRepository)
)

// GetOrInitInstanceRepository returns a chatstorage repository bound to a
// specific logical instance. Each instance gets its own SQLite file under
// storages/chat-<instanceID>.db with WAL and foreign keys enabled.
func GetOrInitInstanceRepository(instanceID string) (domainChatStorage.IChatStorageRepository, error) {
	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return nil, fmt.Errorf("instanceID cannot be blank")
	}

	instanceRepoMu.RLock()
	repo, ok := instanceRepos[trimmed]
	instanceRepoMu.RUnlock()
	if ok && repo != nil {
		return repo, nil
	}

	instanceRepoMu.Lock()
	defer instanceRepoMu.Unlock()
	if repo, ok := instanceRepos[trimmed]; ok && repo != nil {
		return repo, nil
	}

	dbPath := fmt.Sprintf("%s/chat-%s.db", config.PathStorages, trimmed)
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL", dbPath)
	if config.ChatStorageEnableForeignKeys {
		connStr += "&_foreign_keys=on"
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	repo = NewStorageRepository(db)
	if err := repo.InitializeSchema(); err != nil {
		logrus.Errorf("[CHATSTORAGE_INSTANCE] failed to initialize schema for %s: %v", trimmed, err)
		_ = db.Close()
		return nil, err
	}

	instanceRepos[trimmed] = repo
	logrus.Infof("[CHATSTORAGE_INSTANCE] initialized chatstorage DB for instance %s at %s", trimmed, dbPath)
	return repo, nil
}

func CleanupInstanceRepository(instanceID string) error {
	trimmed := strings.TrimSpace(instanceID)
	if trimmed == "" {
		return fmt.Errorf("instanceID cannot be blank")
	}

	instanceRepoMu.Lock()
	repo, ok := instanceRepos[trimmed]
	if ok {
		delete(instanceRepos, trimmed)
	}
	instanceRepoMu.Unlock()

	if ok && repo != nil {
		if sqliteRepo, ok := repo.(*SQLiteRepository); ok && sqliteRepo.db != nil {
			if err := sqliteRepo.db.Close(); err != nil {
				logrus.Errorf("[CHATSTORAGE_INSTANCE] failed to close DB for instance %s: %v", trimmed, err)
			}
		}
	}

	dbPath := fmt.Sprintf("%s/chat-%s.db", config.PathStorages, trimmed)
	if err := os.Remove(dbPath); err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("[CHATSTORAGE_INSTANCE] failed to remove DB file for instance %s: %v", trimmed, err)
			return err
		}
	}

	return nil
}

// CloseInstanceRepositories closes all open instance database connections.
func CloseInstanceRepositories() {
	instanceRepoMu.Lock()
	defer instanceRepoMu.Unlock()

	for id, repo := range instanceRepos {
		if sqliteRepo, ok := repo.(*SQLiteRepository); ok && sqliteRepo.db != nil {
			logrus.Infof("[CHATSTORAGE_INSTANCE] Closing DB for instance %s...", id)
			if err := sqliteRepo.db.Close(); err != nil {
				logrus.Errorf("[CHATSTORAGE_INSTANCE] Failed to close DB for instance %s: %v", id, err)
			}
		}
	}
	instanceRepos = make(map[string]domainChatStorage.IChatStorageRepository)
}

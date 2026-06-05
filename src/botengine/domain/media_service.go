package domain

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FileStorage interface abstracts local file system operations
type FileStorage interface {
	ReadFile(path string) ([]byte, error)
	CreateTempFile(prefix, filename string) (string, error)
	RemoveFile(path string) error
}

// HTTPContentFetcher interface abstracts remote downloading
type HTTPContentFetcher interface {
	FetchMetadata(ctx context.Context, url string) (mimeType string, size int64, err error)
	FetchToRAM(ctx context.Context, url string) ([]byte, error)
	FetchToDisk(ctx context.Context, url string, destPath string) error
}

// MediaService is the Domain Service responsible for handling media resources.
// It decides if a file should be loaded into RAM or streamed to disk based on its size and global RAM usage.
type MediaService struct {
	fetcher     HTTPContentFetcher
	storage     FileStorage
	maxRamBytes int64
	globalMax   int64
	mu          sync.Mutex
	currentRam  int64
}

func NewMediaService(fetcher HTTPContentFetcher, storage FileStorage, maxRamMB int, globalMaxRamMB int) *MediaService {
	return &MediaService{
		fetcher:     fetcher,
		storage:     storage,
		maxRamBytes: int64(maxRamMB) * 1024 * 1024,
		globalMax:   int64(globalMaxRamMB) * 1024 * 1024,
	}
}

func (s *MediaService) ProcessMedia(ctx context.Context, isURL bool, pathOrURL string, mime string, fname string) (*BotMedia, error) {
	if !isURL {
		// Handled locally
		data, err := s.storage.ReadFile(pathOrURL)
		if err != nil {
			return nil, err
		}
		return &BotMedia{Data: data, MimeType: mime, FileName: fname, LocalPath: pathOrURL}, nil
	}

	// Remote URL: Business Logic for RAM vs Disk
	actualMime, size, err := s.fetcher.FetchMetadata(ctx, pathOrURL)
	if err != nil {
		return nil, fmt.Errorf("failed fetching remote metadata: %v", err)
	}
	if actualMime != "" {
		mime = actualMime
	}
	if size > 50*1024*1024 {
		return nil, fmt.Errorf("remote file is too large (%.2f MB), maximum allowed is 50MB", float64(size)/(1024*1024))
	}

	useRam := size > 0 && size <= s.maxRamBytes
	if useRam {
		// Wait in queue until global RAM is available
		acquired := false
		for !acquired {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting for available RAM: %v", ctx.Err())
			default:
				s.mu.Lock()
				if s.currentRam+size <= s.globalMax {
					s.currentRam += size
					acquired = true
				}
				s.mu.Unlock()

				if !acquired {
					time.Sleep(150 * time.Millisecond) // Short wait to yield processing
				}
			}
		}

		data, err := s.fetcher.FetchToRAM(ctx, pathOrURL)
		if err != nil {
			s.mu.Lock()
			s.currentRam -= size
			s.mu.Unlock()
			return nil, fmt.Errorf("failed fetching to RAM: %v", err)
		}
		m := &BotMedia{
			Data:     data,
			MimeType: mime,
			FileName: fname,
			URL:      pathOrURL,
			State:    MediaStateAnalyzed,
		}
		// Inversion of control for cleanup
		m.CleanupFunc = func() {
			s.mu.Lock()
			s.currentRam -= size
			s.mu.Unlock()
		}
		return m, nil
	}

	// Process as temporary file on disk
	tempPath, err := s.storage.CreateTempFile("smartdl-*-", fname)
	if err != nil {
		return nil, fmt.Errorf("could not create temp file: %v", err)
	}

	err = s.fetcher.FetchToDisk(ctx, pathOrURL, tempPath)
	if err != nil {
		_ = s.storage.RemoveFile(tempPath)
		return nil, fmt.Errorf("failed streaming to disk: %v", err)
	}

	m := &BotMedia{
		MimeType:  mime,
		FileName:  fname,
		URL:       pathOrURL,
		State:     MediaStateAnalyzed,
		LocalPath: tempPath,
	}
	m.CleanupFunc = func() {
		_ = s.storage.RemoveFile(tempPath)
	}
	return m, nil
}

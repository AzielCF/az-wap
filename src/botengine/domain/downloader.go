package domain

import "context"

// SmartDownloader handles the intelligent downloading of heavy resources.
// It instinctively decides whether to send the file to RAM or save it as a stream to hard disk.
type SmartDownloader interface {
	Download(ctx context.Context, url string, mimeType string, filename string, maxRamSizeMB int) (*BotMedia, error)
}

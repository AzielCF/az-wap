package infrastructure

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
)

type LocalFileStorage struct{}

func NewLocalFileStorage() domain.FileStorage {
	return &LocalFileStorage{}
}

func (s *LocalFileStorage) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (s *LocalFileStorage) CreateTempFile(prefix, filename string) (string, error) {
	f, err := os.CreateTemp("", prefix+filename)
	if err != nil {
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func (s *LocalFileStorage) RemoveFile(path string) error {
	return os.Remove(path)
}

type StandardHTTPFetcher struct {
	client *http.Client
}

func NewStandardHTTPFetcher() domain.HTTPContentFetcher {
	customTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &StandardHTTPFetcher{
		client: &http.Client{Transport: customTransport, Timeout: 60 * time.Second},
	}
}

func (f *StandardHTTPFetcher) FetchMetadata(ctx context.Context, url string) (string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := f.client.Do(req)

	// If HEAD is blocked, fallback to GET and drop body
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		reqGet, errGet := http.NewRequestWithContext(ctx, "GET", url, nil)
		if errGet != nil {
			return "", 0, errGet
		}
		reqGet.Header.Set("User-Agent", "Mozilla/5.0")
		respGet, errGet := f.client.Do(reqGet)
		if errGet != nil {
			return "", 0, errGet
		}
		defer respGet.Body.Close()
		return respGet.Header.Get("Content-Type"), respGet.ContentLength, nil
	}

	defer resp.Body.Close()
	return resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

func (f *StandardHTTPFetcher) FetchToRAM(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (f *StandardHTTPFetcher) FetchToDisk(ctx context.Context, url string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

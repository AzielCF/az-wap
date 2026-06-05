package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AzielCF/az-wap/workspace/infrastructure/telegram/domain"
)

type TelegramHTTPClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewTelegramHTTPClient(token string) *TelegramHTTPClient {
	return &TelegramHTTPClient{
		token:   token,
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20, // Mayor concurrencia para envío rápido
				IdleConnTimeout:     90 * time.Second,
			},
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TelegramHTTPClient) request(ctx context.Context, method string, payload interface{}, response interface{}) error {
	url := fmt.Sprintf("%s/%s", c.baseURL, method)

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	if response != nil {
		return json.NewDecoder(resp.Body).Decode(response)
	}

	return nil
}

func (c *TelegramHTTPClient) SendMessage(ctx context.Context, chatID interface{}, text string) (string, error) {
	req := domain.SendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}
	var resp domain.TelegramResponse[domain.Message]
	err := c.request(ctx, "sendMessage", req, &resp)
	if err != nil {
		return "", err
	}
	if !resp.Ok {
		return "", fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return fmt.Sprintf("%d", resp.Result.MessageID), nil
}

func (c *TelegramHTTPClient) GetMe(ctx context.Context) (map[string]interface{}, error) {
	var resp domain.TelegramResponse[map[string]interface{}]
	err := c.request(ctx, "getMe", nil, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return resp.Result, nil
}

func (c *TelegramHTTPClient) GetUpdates(ctx context.Context, offset int) ([]domain.Update, error) {
	payload := map[string]interface{}{
		"offset":  offset,
		"timeout": 20, // Long polling
	}
	var resp domain.TelegramResponse[[]domain.Update]
	err := c.request(ctx, "getUpdates", payload, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return resp.Result, nil
}

func (c *TelegramHTTPClient) SetWebhook(ctx context.Context, url string) error {
	payload := map[string]interface{}{"url": url}
	var resp domain.TelegramResponse[interface{}]
	err := c.request(ctx, "setWebhook", payload, &resp)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return nil
}

func (c *TelegramHTTPClient) DeleteWebhook(ctx context.Context) error {
	var resp domain.TelegramResponse[interface{}]
	err := c.request(ctx, "deleteWebhook", nil, &resp)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return nil
}

func (c *TelegramHTTPClient) SendChatAction(ctx context.Context, chatID interface{}, action string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"action":  action,
	}
	// No esperamos respuesta estructural, solo el OK
	var resp domain.TelegramResponse[interface{}]
	err := c.request(ctx, "sendChatAction", payload, &resp)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return nil
}

func (c *TelegramHTTPClient) GetFile(ctx context.Context, fileID string) (domain.File, error) {
	payload := map[string]interface{}{"file_id": fileID}
	var resp domain.TelegramResponse[domain.File]
	err := c.request(ctx, "getFile", payload, &resp)
	if err != nil {
		return domain.File{}, err
	}
	if !resp.Ok {
		return domain.File{}, fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return resp.Result, nil
}

func (c *TelegramHTTPClient) DownloadFile(ctx context.Context, filePath string) ([]byte, error) {
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.token, filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("telegram file download error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TelegramClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewTelegramClient(token string) *TelegramClient {
	return &TelegramClient{
		token:   token,
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", token),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TelegramClient) request(ctx context.Context, method string, payload any, response any) error {
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

type SendMessageRequest struct {
	ChatID                any    `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
	ReplyToMessageID      int64  `json:"reply_to_message_id,omitempty"`
}

type TelegramResponse[T any] struct {
	Ok     bool   `json:"ok"`
	Result T      `json:"result"`
	Desc   string `json:"description"`
}

func (c *TelegramClient) SendMessage(ctx context.Context, chatID any, text string) error {
	req := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	var resp TelegramResponse[any]
	err := c.request(ctx, "sendMessage", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("telegram api error: %s", resp.Desc)
	}
	return nil
}

func (c *TelegramClient) GetMe(ctx context.Context) (map[string]any, error) {
	var resp TelegramResponse[map[string]any]
	err := c.request(ctx, "getMe", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

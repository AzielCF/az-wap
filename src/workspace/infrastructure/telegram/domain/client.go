package domain

import (
	"context"
)

// TelegramClient define las operaciones que podemos realizar contra la API de Telegram
type ITelegramClient interface {
	SendMessage(ctx context.Context, chatID interface{}, text string) (string, error)
	GetMe(ctx context.Context) (map[string]interface{}, error)
	GetUpdates(ctx context.Context, offset int) ([]Update, error)
	SetWebhook(ctx context.Context, url string) error
	DeleteWebhook(ctx context.Context) error
	SendChatAction(ctx context.Context, chatID interface{}, action string) error
	GetFile(ctx context.Context, fileID string) (File, error)
	DownloadFile(ctx context.Context, filePath string) ([]byte, error)
}

type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size"`
}

type Audio struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

type Voice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

type Video struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

type Document struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int64  `json:"file_size"`
	FilePath     string `json:"file_path"`
}

type Message struct {
	MessageID int `json:"message_id"`
	From      *struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
	} `json:"from"`
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
	Text           string      `json:"text"`
	Caption        string      `json:"caption,omitempty"`
	Photo          []PhotoSize `json:"photo,omitempty"`
	Voice          *Voice      `json:"voice,omitempty"`
	Audio          *Audio      `json:"audio,omitempty"`
	Video          *Video      `json:"video,omitempty"`
	Document       *Document   `json:"document,omitempty"`
	ReplyToMessage *Message    `json:"reply_to_message,omitempty"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

// Tipos de datos genéricos de Telegram
type SendMessageRequest struct {
	ChatID                interface{} `json:"chat_id"`
	Text                  string      `json:"text"`
	ParseMode             string      `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool        `json:"disable_web_page_preview,omitempty"`
}

type TelegramResponse[T any] struct {
	Ok     bool   `json:"ok"`
	Result T      `json:"result"`
	Desc   string `json:"description"`
}

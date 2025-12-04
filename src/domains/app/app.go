package app

import (
	"context"
	"time"
)

type IAppUsecase interface {
	Login(ctx context.Context, token string) (response LoginResponse, err error)
	LoginWithCode(ctx context.Context, token string, phoneNumber string) (loginCode string, err error)
	Logout(ctx context.Context, token string) (err error)
	Reconnect(ctx context.Context, token string) (err error)
	FirstDevice(ctx context.Context, token string) (response DevicesResponse, err error)
	FetchDevices(ctx context.Context, token string) (response []DevicesResponse, err error)
}

type DevicesResponse struct {
	Name   string `json:"name"`
	Device string `json:"device"`
}

type LoginResponse struct {
	ImagePath string        `json:"image_path"`
	Duration  time.Duration `json:"duration"`
	Code      string        `json:"code"`
}

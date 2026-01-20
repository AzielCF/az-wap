package usecase

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
)

type serviceApp struct {
	workspaceMgr *workspace.Manager
}

func NewAppService(workspaceMgr *workspace.Manager) domainApp.IAppUsecase {
	return &serviceApp{
		workspaceMgr: workspaceMgr,
	}
}

func (service *serviceApp) validateToken(ctx context.Context, token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return pkgError.ValidationError("token: cannot be blank.")
	}
	_, ok := service.workspaceMgr.GetAdapter(trimmed)
	if !ok {
		// If not in manager, we should check if it exists in repo/DB
		// For now we assume the caller ensures existence or we try to Start it.
	}
	return nil
}

func (service *serviceApp) getAdapter(ctx context.Context, token string) (wsChannelDomain.ChannelAdapter, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, pkgError.ValidationError("token: cannot be blank.")
	}

	adapter, ok := service.workspaceMgr.GetAdapter(trimmed)
	if !ok {
		// Try to start it
		if err := service.workspaceMgr.StartChannel(ctx, trimmed); err != nil {
			return nil, err
		}
		adapter, _ = service.workspaceMgr.GetAdapter(trimmed)
	}

	return adapter, nil
}

func (service *serviceApp) Login(ctx context.Context, token string) (response domainApp.LoginResponse, err error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return response, err
	}

	if adapter.IsLoggedIn() {
		return response, pkgError.ErrAlreadyLoggedIn
	}

	qrChan, err := adapter.GetQRChannel(ctx)
	if err != nil {
		return response, err
	}

	// Start login (Connect)
	if err := adapter.Login(ctx); err != nil {
		return response, err
	}

	// Wait for the first QR code
	select {
	case code := <-qrChan:
		response.Code = code
		response.Duration = 20 // Default WhatsApp QR duration approx

		qrPath := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, fiberUtils.UUIDv4())
		err = qrcode.WriteFile(code, qrcode.Medium, 512, qrPath)
		if err != nil {
			logrus.Error("Error when write qr code to file: ", err)
		}

		response.ImagePath = qrPath

		// Cleanup timer
		go func() {
			time.Sleep(20 * time.Second)
			_ = os.Remove(qrPath)
		}()

	case <-time.After(30 * time.Second):
		return response, fmt.Errorf("timeout waiting for QR code")
	case <-ctx.Done():
		return response, ctx.Err()
	}

	return response, nil
}

func (service *serviceApp) LoginWithCode(ctx context.Context, token string, phoneNumber string) (loginCode string, err error) {
	if err = validations.ValidateLoginWithCode(ctx, phoneNumber); err != nil {
		return loginCode, err
	}

	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return loginCode, err
	}

	if adapter.IsLoggedIn() {
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	return adapter.LoginWithCode(ctx, phoneNumber)
}

func (service *serviceApp) Logout(ctx context.Context, token string) (err error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return err
	}

	if err := adapter.Logout(ctx); err != nil {
		return err
	}

	service.workspaceMgr.UnregisterAdapter(token)
	return nil
}

func (service *serviceApp) Reconnect(ctx context.Context, token string) (err error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return err
	}

	return adapter.Login(ctx)
}

func (service *serviceApp) FirstDevice(ctx context.Context, token string) (response domainApp.DevicesResponse, err error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return response, err
	}

	// For now, return generic "Adapter Account" as we moved away from raw DB device access here
	response.Device = adapter.ID()
	response.Name = "Active Session"
	return response, nil
}

func (service *serviceApp) FetchDevices(ctx context.Context, token string) (response []domainApp.DevicesResponse, err error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return response, err
	}

	response = append(response, domainApp.DevicesResponse{
		Device: adapter.ID(),
		Name:   "Active Session",
	})
	return response, nil
}

func (service *serviceApp) GetConnectionStatus(ctx context.Context, token string) (bool, bool, string, error) {
	adapter, err := service.getAdapter(ctx, token)
	if err != nil {
		return false, false, "", err
	}

	status := adapter.Status()
	// Check against domain constant if possible, or just string
	isConnected := status == wsChannelDomain.ChannelStatusConnected
	isLoggedIn := adapter.IsLoggedIn()
	deviceID := adapter.ID()

	return isConnected, isLoggedIn, deviceID, nil
}

func (service *serviceApp) GetSettings(ctx context.Context) (map[string]any, error) {
	return config.GetAllSettings(), nil
}

func (service *serviceApp) UpdateSettings(ctx context.Context, key string, value any) error {
	switch key {
	case "whatsapp_max_download_size":
		var val int64
		switch v := value.(type) {
		case float64:
			val = int64(v)
		case int64:
			val = v
		case int:
			val = int64(v)
		case string:
			parsed, _ := strconv.ParseInt(v, 10, 64)
			val = parsed
		}
		return config.SaveWhatsappMaxDownloadSize(val)
	case "ai_global_system_prompt":
		return config.SaveAIGlobalSystemPrompt(fmt.Sprintf("%v", value))
	case "ai_timezone":
		return config.SaveAITimezone(fmt.Sprintf("%v", value))
	case "ai_debounce_ms":
		var val int
		if f, ok := value.(float64); ok {
			val = int(f)
		} else if i, ok := value.(int); ok {
			val = i
		}
		return config.SaveAIDebounceMs(val)
	case "ai_wait_contact_idle_ms":
		var val int
		if f, ok := value.(float64); ok {
			val = int(f)
		} else if i, ok := value.(int); ok {
			val = i
		}
		return config.SaveAIWaitContactIdleMs(val)
	case "ai_typing_enabled":
		var val bool
		if b, ok := value.(bool); ok {
			val = b
		} else if s, ok := value.(string); ok {
			val = s == "true" || s == "1"
		}
		return config.SaveAITypingEnabled(val)
	}
	return fmt.Errorf("setting key %s not supported", key)
}

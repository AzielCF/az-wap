package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	coreconfig "github.com/AzielCF/az-wap/core/config"
	coreSettings "github.com/AzielCF/az-wap/core/settings/application"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/AzielCF/az-wap/validations"
	"github.com/AzielCF/az-wap/workspace"
	wsChannelDomain "github.com/AzielCF/az-wap/workspace/domain/channel"
)

type serviceApp struct {
	workspaceMgr *workspace.Manager
	settingsSvc  *coreSettings.SettingsService
}

func NewAppService(workspaceMgr *workspace.Manager, settingsSvc *coreSettings.SettingsService) domainApp.IAppUsecase {
	return &serviceApp{
		workspaceMgr: workspaceMgr,
		settingsSvc:  settingsSvc,
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
		response.Duration = 20    // Default WhatsApp QR duration approx
		response.ImagePath = code // Devolver el código directamente como "path" para retrocompatibilidad simple o mejorando el campo después

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
	// 1. Get baseline from memory
	settings := coreconfig.GetAllSettings()

	// 2. Overlay with dynamic settings from DB
	dynamic, err := service.settingsSvc.GetDynamicSettings(ctx)
	if err != nil {
		// If DB fails, we at least return the defaults from memory
		return settings, nil
	}

	// Update the map with latest from DB
	if dynamic.AIGlobalSystemPrompt != "" {
		settings["ai_global_system_prompt"] = dynamic.AIGlobalSystemPrompt
	}
	if dynamic.AITimezone != "" {
		settings["ai_timezone"] = dynamic.AITimezone
	}
	if dynamic.AIDebounceMs != nil {
		settings["ai_debounce_ms"] = *dynamic.AIDebounceMs
	}
	if dynamic.AIWaitContactIdleMs != nil {
		settings["ai_wait_contact_idle_ms"] = *dynamic.AIWaitContactIdleMs
	}
	if dynamic.AITypingEnabled != nil {
		settings["ai_typing_enabled"] = *dynamic.AITypingEnabled
	}
	if dynamic.WhatsappMaxDownloadSize != nil {
		settings["whatsapp_setting_max_download_size"] = *dynamic.WhatsappMaxDownloadSize
	}

	return settings, nil
}

func (service *serviceApp) UpdateSettings(ctx context.Context, key string, value any) error {
	switch key {
	case "whatsapp_setting_max_download_size", "whatsapp_max_download_size":
		val := parseToInt64(value)
		return service.settingsSvc.SetMaxDownloadSize(ctx, val)
	case "whatsapp_setting_max_file_size":
		// TODO: Implement field in service if needed, for now ignore to avoid error
		return nil
	case "whatsapp_setting_max_video_size":
		// TODO: Implement field in service if needed, for now ignore to avoid error
		return nil
	case "whatsapp_webhook_secret":
		// TODO: Implement field in service if needed, for now ignore to avoid error
		return nil
	case "whatsapp_webhook_insecure_skip_verify":
		// TODO: Implement field in service if needed, for now ignore to avoid error
		return nil
	case "whatsapp_account_validation":
		// TODO: Implement field in service if needed, for now ignore to avoid error
		return nil
	case "ai_global_system_prompt":
		strVal := fmt.Sprintf("%v", value)
		if strings.TrimSpace(strVal) == "" {
			return nil
		}
		if err := service.settingsSvc.SetSystemPrompt(ctx, strVal); err != nil {
			return err
		}
		coreconfig.Global.AI.GlobalSystemPrompt = strVal
		return nil
	case "ai_timezone":
		strVal := fmt.Sprintf("%v", value)
		if strings.TrimSpace(strVal) == "" {
			return nil
		}
		if err := service.settingsSvc.SetTimezone(ctx, strVal); err != nil {
			return err
		}
		coreconfig.Global.AI.Timezone = strVal
		return nil
	case "ai_debounce_ms":
		val := parseToInt(value)
		if err := service.settingsSvc.SetDebounce(ctx, val); err != nil {
			return err
		}
		coreconfig.Global.AI.DebounceMs = val
		return nil
	case "ai_wait_contact_idle_ms":
		val := parseToInt(value)
		if err := service.settingsSvc.SetContactIdle(ctx, val); err != nil {
			return err
		}
		coreconfig.Global.AI.WaitContactIdleMs = val
		return nil
	case "ai_typing_enabled":
		val := parseToBool(value)
		if err := service.settingsSvc.SetTypingEnabled(ctx, val); err != nil {
			return err
		}
		coreconfig.Global.AI.TypingEnabled = val
		return nil
	case "app_version", "app_debug":
		return nil // Read-only, ignore
	}
	return fmt.Errorf("setting key %s not supported", key)
}

func parseToInt(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	}
	return 0
}

func parseToInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	}
	return 0
}

func parseToBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	}
	return false
}

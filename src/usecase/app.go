package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	domainChatStorage "github.com/AzielCF/az-wap/domains/chatstorage"
	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	"github.com/AzielCF/az-wap/infrastructure/whatsapp"
	pkgError "github.com/AzielCF/az-wap/pkg/error"
	"github.com/AzielCF/az-wap/validations"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/libsignal/logger"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type serviceApp struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
	instanceService domainInstance.IInstanceUsecase
}

func NewAppService(chatStorageRepo domainChatStorage.IChatStorageRepository, instanceService domainInstance.IInstanceUsecase) domainApp.IAppUsecase {
	return &serviceApp{
		chatStorageRepo: chatStorageRepo,
		instanceService: instanceService,
	}
}

func (service *serviceApp) validateToken(ctx context.Context, token string) error {
	if service.instanceService == nil {
		return nil
	}

	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return pkgError.ValidationError("token: cannot be blank.")
	}

	_, err := service.instanceService.GetByToken(ctx, trimmed)
	return err
}

func (service *serviceApp) getClientAndDB(ctx context.Context, token string) (*whatsmeow.Client, *sqlstore.Container, error) {
	if service.instanceService == nil {
		client := whatsapp.GetClient()
		if client == nil {
			return nil, nil, pkgError.ErrWaCLI
		}
		return client, whatsapp.GetDB(), nil
	}

	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, nil, pkgError.ValidationError("token: cannot be blank.")
	}

	instance, err := service.instanceService.GetByToken(ctx, trimmed)
	if err != nil {
		return nil, nil, err
	}

	// Registrar configuración de webhooks específica de la instancia (si está disponible).
	whatsapp.SetInstanceWebhookConfig(
		instance.ID,
		instance.WebhookURLs,
		instance.WebhookSecret,
		instance.WebhookInsecureSkipVerify,
	)

	client, instDB, err := whatsapp.GetOrInitInstanceClient(ctx, instance.ID, service.chatStorageRepo)
	if err != nil {
		return nil, nil, err
	}

	// If client is nil, the instance needs login first
	if client == nil {
		logrus.Warnf("[INSTANCE] Client for instance %s is nil (needs login), using DB only", instance.ID)
		return nil, instDB, nil
	}

	return client, instDB, nil
}

func (service *serviceApp) Login(ctx context.Context, token string) (response domainApp.LoginResponse, err error) {
	client, currentDB, err := service.getClientAndDB(ctx, token)
	if err != nil {
		return response, err
	}

	// [DEBUG] Log database state before login
	logrus.Info("[DEBUG] Starting login process...")
	if currentDB != nil {
		devices, dbErr := currentDB.GetAllDevices(ctx)
		if dbErr != nil {
			logrus.Errorf("[DEBUG] Error getting devices before login: %v", dbErr)
		} else {
			logrus.Infof("[DEBUG] Devices before login: %d found", len(devices))
			for _, device := range devices {
				logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
			}
		}
	}

	// If client is nil, create a new one for this instance (fresh login)
	if client == nil {
		logrus.Info("[DEBUG] Client is nil - creating new client for fresh login")
		if currentDB == nil {
			return response, pkgError.ErrWaCLI
		}
		device := currentDB.NewDevice()
		client = whatsmeow.NewClient(device, waLog.Stdout("Client", config.WhatsappLogLevel, true))
		client.EnableAutoReconnect = true
		client.AutoTrustIdentity = true
	}

	// [DEBUG] Log client state
	if client.Store != nil && client.Store.ID != nil {
		logrus.Infof("[DEBUG] Client has existing store ID: %s", client.Store.ID.String())
	} else {
		logrus.Info("[DEBUG] Client has no store ID")
	}

	// Disconnect for reconnecting
	client.Disconnect()

	chImage := make(chan string)

	logrus.Info("[DEBUG] Attempting to get QR channel...")
	ch, err := client.GetQRChannel(context.Background())
	if err != nil {
		logrus.Errorf("[DEBUG] GetQRChannel failed: %v", err)
		logrus.Error(err.Error())
		// This error means that we're already logged in, so ignore it.
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			logrus.Info("[DEBUG] Error is ErrQRStoreContainsID - attempting to connect")
			_ = client.Connect() // just connect to websocket
			if client.IsLoggedIn() {
				return response, pkgError.ErrAlreadyLoggedIn
			}
			return response, pkgError.ErrSessionSaved
		} else {
			return response, pkgError.ErrQrChannel
		}
	} else {
		logrus.Info("[DEBUG] QR channel obtained successfully")
		go func() {
			for evt := range ch {
				response.Code = evt.Code
				response.Duration = evt.Timeout / time.Second / 2
				if evt.Event == "code" {
					qrPath := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, fiberUtils.UUIDv4())
					err = qrcode.WriteFile(evt.Code, qrcode.Medium, 512, qrPath)
					if err != nil {
						logrus.Error("Error when write qr code to file: ", err)
					}
					go func() {
						time.Sleep(response.Duration * time.Second)
						err := os.Remove(qrPath)
						if err != nil {
							// Only log if it's not a "file not found" error
							if !os.IsNotExist(err) {
								logrus.Error("error when remove qrImage file", err.Error())
							}
						}
					}()
					chImage <- qrPath
				} else {
					logrus.Error("error when get qrCode", evt.Event, evt.Error)
				}
			}
		}()
	}

	err = client.Connect()
	if err != nil {
		logger.Error("Error when connect to whatsapp", err)
		return response, pkgError.ErrReconnect
	}
	response.ImagePath = <-chImage

	// [DEBUG] Verify connection state and sync global client
	logrus.Infof("[DEBUG] Login connection established - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Store the client for this instance after successful login
	whatsapp.SetInstanceClient(whatsapp.GetActiveInstanceID(), client, currentDB)

	return response, nil
}

func (service *serviceApp) LoginWithCode(ctx context.Context, token string, phoneNumber string) (loginCode string, err error) {
	if err = validations.ValidateLoginWithCode(ctx, phoneNumber); err != nil {
		logrus.Errorf("Error when validate login with code: %s", err.Error())
		return loginCode, err
	}

	client, _, err := service.getClientAndDB(ctx, token)
	if err != nil {
		return loginCode, err
	}
	// detect is already logged in
	if client.Store.ID != nil || client.IsLoggedIn() {
		logrus.Warn("User is already logged in")
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	// reconnect first
	if err = service.Reconnect(ctx, token); err != nil {
		logrus.Errorf("Error when reconnecting before login with code: %s", err.Error())
		return loginCode, err
	}

	// refresh client reference after reconnect
	client = whatsapp.GetClient()
	if client.IsLoggedIn() || client.Store.ID != nil {
		logrus.Warn("User is already logged in after reconnect")
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	logrus.Infof("[DEBUG] Starting phone pairing for number: %s", phoneNumber)
	loginCode, err = client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logrus.Errorf("Error when pairing phone: %s", err.Error())
		return loginCode, err
	}

	// [DEBUG] Verify pairing state and sync global client
	logrus.Infof("[DEBUG] Phone pairing completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	logrus.Infof("Successfully paired phone with code: %s", loginCode)
	return loginCode, nil
}

func (service *serviceApp) Logout(ctx context.Context, token string) (err error) {
	if err = service.validateToken(ctx, token); err != nil {
		return err
	}

	// Legacy/global logout when no token is provided or instanceService is not available
	if token == "" || service.instanceService == nil {
		logrus.Info("[DEBUG] Starting logout process (global)...")
		devices, dbErr := whatsapp.GetDB().GetAllDevices(ctx)
		if dbErr != nil {
			logrus.Errorf("[DEBUG] Error getting devices before logout: %v", dbErr)
		} else {
			logrus.Infof("[DEBUG] Devices before logout: %d found", len(devices))
			for _, device := range devices {
				logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
			}
		}

		logrus.Info("[DEBUG] Calling WhatsApp client logout (global)...")
		if err = whatsapp.GetClient().Logout(ctx); err != nil {
			logrus.Errorf("[DEBUG] WhatsApp logout failed: %v", err)
		} else {
			logrus.Info("[DEBUG] WhatsApp logout completed successfully")
		}

		devices, dbErr = whatsapp.GetDB().GetAllDevices(ctx)
		if dbErr != nil {
			logrus.Errorf("[DEBUG] Error getting devices after logout: %v", dbErr)
		} else {
			logrus.Infof("[DEBUG] Devices after logout: %d found", len(devices))
		}

		newDB, newCli, err := whatsapp.PerformCleanupAndUpdateGlobals(ctx, "MANUAL_LOGOUT", service.chatStorageRepo)
		if err != nil {
			logrus.Errorf("[DEBUG] Cleanup failed: %v", err)
			return err
		}

		whatsapp.UpdateGlobalClient(newCli, newDB)
		logrus.Info("[DEBUG] Logout process completed successfully (global)")
		return nil
	}

	// Instance-scoped logout when a token is provided
	client, currentDB, err := service.getClientAndDB(ctx, token)
	if err != nil {
		return err
	}

	logrus.Info("[DEBUG] Starting logout process (instance)...")
	devices, dbErr := currentDB.GetAllDevices(ctx)
	if dbErr != nil {
		logrus.Errorf("[DEBUG] Error getting devices before logout (instance): %v", dbErr)
	} else {
		logrus.Infof("[DEBUG] Devices before logout (instance): %d found", len(devices))
		for _, device := range devices {
			logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
		}
	}

	logrus.Info("[DEBUG] Calling WhatsApp client logout (instance)...")
	if err = client.Logout(ctx); err != nil {
		logrus.Errorf("[DEBUG] WhatsApp logout failed (instance): %v", err)
	} else {
		logrus.Info("[DEBUG] WhatsApp logout completed successfully (instance)")
	}

	devices, dbErr = currentDB.GetAllDevices(ctx)
	if dbErr != nil {
		logrus.Errorf("[DEBUG] Error getting devices after logout (instance): %v", dbErr)
	} else {
		logrus.Infof("[DEBUG] Devices after logout (instance): %d found", len(devices))
	}

	inst, err := service.instanceService.GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if err := whatsapp.CleanupInstanceSession(ctx, inst.ID, service.chatStorageRepo); err != nil {
		logrus.Errorf("[DEBUG] Instance cleanup failed: %v", err)
		return err
	}

	logrus.Info("[DEBUG] Logout process completed successfully (instance)")
	return nil
}

func (service *serviceApp) Reconnect(_ context.Context, token string) (err error) {
	// Reconnect sigue usando el cliente global; el token se valida en Login/LoginWithCode cuando no está vacío.
	logrus.Info("[DEBUG] Starting reconnect process...")

	client, db, err := service.getClientAndDB(context.Background(), token)
	if err != nil {
		return err
	}
	client.Disconnect()
	err = client.Connect()

	if err != nil {
		logrus.Errorf("[DEBUG] Reconnect failed: %v", err)
		return err
	}

	// [DEBUG] Verify reconnection state and sync global client
	logrus.Infof("[DEBUG] Reconnection completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, db)

	logrus.Info("[DEBUG] Reconnect process completed successfully")
	return err
}

func (service *serviceApp) FirstDevice(ctx context.Context, token string) (response domainApp.DevicesResponse, err error) {
	_, currentDB, err := service.getClientAndDB(ctx, token)
	if err != nil {
		return response, err
	}

	devices, err := currentDB.GetFirstDevice(ctx)
	if err != nil {
		return response, err
	}

	response.Device = devices.ID.String()
	if devices.PushName != "" {
		response.Name = devices.PushName
	} else {
		response.Name = devices.BusinessName
	}

	return response, nil
}

func (service *serviceApp) FetchDevices(ctx context.Context, token string) (response []domainApp.DevicesResponse, err error) {
	_, currentDB, err := service.getClientAndDB(ctx, token)
	if err != nil {
		return response, err
	}

	devices, err := currentDB.GetAllDevices(ctx)
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		var d domainApp.DevicesResponse
		d.Device = device.ID.String()
		if device.PushName != "" {
			d.Name = device.PushName
		} else {
			d.Name = device.BusinessName
		}

		response = append(response, d)
	}

	return response, nil
}

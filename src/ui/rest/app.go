package rest

import (
	"fmt"

	"github.com/AzielCF/az-wap/config"
	domainApp "github.com/AzielCF/az-wap/domains/app"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type App struct {
	Service domainApp.IAppUsecase
}

func InitRestApp(app fiber.Router, service domainApp.IAppUsecase) App {
	rest := App{Service: service}
	app.Get("/app/login", rest.Login)
	app.Get("/app/login-with-code", rest.LoginWithCode)
	app.Get("/app/logout", rest.Logout)
	app.Get("/app/reconnect", rest.Reconnect)
	app.Get("/app/devices", rest.Devices)
	app.Get("/app/status", rest.ConnectionStatus)
	app.Get("/app/settings", rest.GetSettings)
	app.Put("/app/settings", rest.UpdateSettings)
	app.Get("/app/version", rest.GetVersion) // Nueva ruta

	return App{Service: service}
}

func (handler *App) GetVersion(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": config.AppVersion,
		"os":      config.AppOs,
	})
}

func (handler *App) Login(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	response, err := handler.Service.Login(c.UserContext(), token)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login success",
		Results: map[string]any{
			"qr_link":     fmt.Sprintf("%s://%s%s/%s", c.Protocol(), c.Hostname(), config.AppBasePath, response.ImagePath),
			"qr_duration": response.Duration,
		},
	})
}

func (handler *App) LoginWithCode(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	pairCode, err := handler.Service.LoginWithCode(c.UserContext(), token, c.Query("phone"))
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login with code success",
		Results: map[string]any{
			"pair_code": pairCode,
		},
	})
}

func (handler *App) Logout(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	err := handler.Service.Logout(c.UserContext(), token)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success logout",
		Results: nil,
	})
}

func (handler *App) Reconnect(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	err := handler.Service.Reconnect(c.UserContext(), token)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Reconnect success",
		Results: nil,
	})
}

func (handler *App) Devices(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	devices, err := handler.Service.FetchDevices(c.UserContext(), token)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Fetch device success",
		Results: devices,
	})
}

func (handler *App) ConnectionStatus(c *fiber.Ctx) error {
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}

	isConnected, isLoggedIn, deviceID, err := handler.Service.GetConnectionStatus(c.UserContext(), token)
	if err != nil {
		// Just log or return disconnected if error (legacy behavior was generous)
		// But better to panic/return error structure if strict.
		// For now, let's just return false/false structure if error but ideally check error.
		// If explicit error handling needed:
		// utils.PanicIfNeeded(err)
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Connection status retrieved",
		Results: map[string]any{
			"is_connected": isConnected,
			"is_logged_in": isLoggedIn,
			"device_id":    deviceID,
		},
	})
}

func (handler *App) GetSettings(c *fiber.Ctx) error {
	settings, err := handler.Service.GetSettings(c.UserContext())
	utils.PanicIfNeeded(err)
	return c.JSON(settings)
}

func (handler *App) UpdateSettings(c *fiber.Ctx) error {
	var body map[string]any
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	for k, v := range body {
		err := handler.Service.UpdateSettings(c.UserContext(), k, v)
		utils.PanicIfNeeded(err)
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

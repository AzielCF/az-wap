package rest

import (
	domainCache "github.com/AzielCF/az-wap/domains/cache"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Cache struct {
	Service domainCache.ICacheUsecase
}

func InitRestCache(app fiber.Router, service domainCache.ICacheUsecase) Cache {
	rest := Cache{Service: service}
	app.Get("/cache/stats", rest.GetGlobalStats)
	app.Post("/cache/clear", rest.ClearGlobalCache)
	app.Get("/cache/settings", rest.GetSettings)
	app.Put("/cache/settings", rest.UpdateSettings)
	app.Get("/instances/:id/cache/stats", rest.GetInstanceStats)
	app.Post("/instances/:id/cache/clear", rest.ClearInstanceCache)

	return rest
}

func (handler *Cache) GetGlobalStats(c *fiber.Ctx) error {
	stats, err := handler.Service.GetGlobalStats(c.UserContext())
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Global cache stats retrieved",
		Results: stats,
	})
}

func (handler *Cache) ClearGlobalCache(c *fiber.Ctx) error {
	err := handler.Service.ClearGlobalCache(c.UserContext())
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Global cache cleared successfully",
	})
}

func (handler *Cache) GetInstanceStats(c *fiber.Ctx) error {
	id := c.Params("id")
	stats, err := handler.Service.GetInstanceStats(c.UserContext(), id)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance cache stats retrieved",
		Results: stats,
	})
}

func (handler *Cache) ClearInstanceCache(c *fiber.Ctx) error {
	id := c.Params("id")
	err := handler.Service.ClearInstanceCache(c.UserContext(), id)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance cache cleared successfully",
	})
}

func (handler *Cache) GetSettings(c *fiber.Ctx) error {
	settings, err := handler.Service.GetSettings(c.UserContext())
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Cache settings retrieved",
		Results: settings,
	})
}

func (handler *Cache) UpdateSettings(c *fiber.Ctx) error {
	var settings domainCache.CacheSettings
	if err := c.BodyParser(&settings); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
		})
	}

	err := handler.Service.SaveSettings(c.UserContext(), settings)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Cache settings updated successfully",
	})
}

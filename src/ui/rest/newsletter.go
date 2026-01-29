package rest

import (
	domainNewsletter "github.com/AzielCF/az-wap/domains/newsletter"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Newsletter struct {
	Service domainNewsletter.INewsletterUsecase
}

func InitRestNewsletter(app fiber.Router, service domainNewsletter.INewsletterUsecase) Newsletter {
	rest := Newsletter{Service: service}
	app.Post("/newsletter/unfollow", rest.Unfollow)
	app.Get("/newsletter/list/:channel_id", rest.List)
	app.Post("/newsletter/schedule", rest.SchedulePost)
	app.Get("/newsletter/scheduled/:channel_id", rest.ListScheduled)
	app.Delete("/newsletter/scheduled/:id", rest.CancelScheduled)
	return rest
}

func (controller *Newsletter) Unfollow(c *fiber.Ctx) error {
	var request domainNewsletter.UnfollowRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	err = controller.Service.Unfollow(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success unfollow newsletter",
	})
}

func (controller *Newsletter) List(c *fiber.Ctx) error {
	channelID := c.Params("channel_id")
	if channelID == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "channel_id is required",
		})
	}

	newsletters, err := controller.Service.List(c.UserContext(), channelID)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success fetch newsletters",
		Results: newsletters,
	})
}

func (controller *Newsletter) SchedulePost(c *fiber.Ctx) error {
	var request domainNewsletter.SchedulePostRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	post, err := controller.Service.SchedulePost(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success schedule post",
		Results: post,
	})
}

func (controller *Newsletter) ListScheduled(c *fiber.Ctx) error {
	channelID := c.Params("channel_id")
	if channelID == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "channel_id is required",
		})
	}

	posts, err := controller.Service.ListScheduled(c.UserContext(), channelID)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success fetch scheduled posts",
		Results: posts,
	})
}

func (controller *Newsletter) CancelScheduled(c *fiber.Ctx) error {
	postID := c.Params("id")
	if postID == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "post_id is required",
		})
	}

	err := controller.Service.CancelScheduled(c.UserContext(), postID)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success cancel scheduled post",
	})
}

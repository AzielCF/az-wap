package rest

import (
	domainMessage "github.com/AzielCF/az-wap/domains/message"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Message struct {
	Service domainMessage.IMessageUsecase
}

func InitRestMessage(app fiber.Router, service domainMessage.IMessageUsecase) Message {
	rest := Message{Service: service}

	// Message action endpoints
	app.Post("/message/:message_id/reaction", rest.ReactMessage)
	app.Post("/message/:message_id/revoke", rest.RevokeMessage)
	app.Post("/message/:message_id/delete", rest.DeleteMessage)
	app.Post("/message/:message_id/update", rest.UpdateMessage)
	app.Post("/message/:message_id/read", rest.MarkAsRead)
	app.Post("/message/:message_id/star", rest.StarMessage)
	app.Post("/message/:message_id/unstar", rest.UnstarMessage)
	app.Get("/message/:message_id/download", rest.DownloadMedia)
	return rest
}

func (controller *Message) RevokeMessage(c *fiber.Ctx) error {
	var request domainMessage.RevokeRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	request.Token = token
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.RevokeMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) DeleteMessage(c *fiber.Ctx) error {
	var request domainMessage.DeleteRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	err = controller.Service.DeleteMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Message deleted successfully",
		Results: nil,
	})
}

func (controller *Message) UpdateMessage(c *fiber.Ctx) error {
	var request domainMessage.UpdateMessageRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.UpdateMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) ReactMessage(c *fiber.Ctx) error {
	var request domainMessage.ReactionRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.ReactMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) MarkAsRead(c *fiber.Ctx) error {
	var request domainMessage.MarkAsReadRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)

	response, err := controller.Service.MarkAsRead(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) StarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)
	request.IsStarred = true
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	request.Token = token

	err = controller.Service.StarMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Starred message successfully",
		Results: nil,
	})
}

func (controller *Message) UnstarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	err := c.BodyParser(&request)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Message: err.Error()})
	}

	request.MessageID = c.Params("message_id")
	utils.SanitizePhone(&request.Phone)
	request.IsStarred = false
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	request.Token = token
	err = controller.Service.StarMessage(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Unstarred message successfully",
		Results: nil,
	})
}

func (controller *Message) DownloadMedia(c *fiber.Ctx) error {
	var request domainMessage.DownloadMediaRequest

	request.MessageID = c.Params("message_id")
	request.Phone = c.Query("phone")
	utils.SanitizePhone(&request.Phone)
	token := c.Get("X-Instance-Token")
	if token == "" {
		token = c.Query("token")
	}
	request.Token = token

	response, err := controller.Service.DownloadMedia(c.UserContext(), request)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

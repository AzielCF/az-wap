package rest

import (
	"strings"

	domainCredential "github.com/AzielCF/az-wap/domains/credential"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type Credential struct {
	Service domainCredential.ICredentialUsecase
}

func InitRestCredential(app fiber.Router, service domainCredential.ICredentialUsecase) Credential {
	rest := Credential{Service: service}
	app.Get("/credentials", rest.ListCredentials)
	app.Post("/credentials", rest.CreateCredential)
	app.Get("/credentials/:id", rest.GetCredential)
	app.Put("/credentials/:id", rest.UpdateCredential)
	app.Delete("/credentials/:id", rest.DeleteCredential)
	return rest
}

func (h *Credential) ListCredentials(c *fiber.Ctx) error {
	kindParam := strings.TrimSpace(c.Query("kind"))
	var kindPtr *domainCredential.Kind
	if kindParam != "" {
		k := domainCredential.Kind(kindParam)
		kindPtr = &k
	}

	creds, err := h.Service.List(c.UserContext(), kindPtr)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credentials fetched",
		Results: creds,
	})
}

func (h *Credential) CreateCredential(c *fiber.Ctx) error {
	var req domainCredential.CreateCredentialRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	cred, err := h.Service.Create(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credential created",
		Results: cred,
	})
}

func (h *Credential) GetCredential(c *fiber.Ctx) error {
	id := c.Params("id")
	cred, err := h.Service.GetByID(c.UserContext(), id)
	if err != nil {
		return c.Status(404).JSON(utils.ResponseData{
			Status:  404,
			Code:    "NOT_FOUND",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credential fetched",
		Results: cred,
	})
}

func (h *Credential) UpdateCredential(c *fiber.Ctx) error {
	id := c.Params("id")
	var req domainCredential.UpdateCredentialRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	cred, err := h.Service.Update(c.UserContext(), id, req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credential updated",
		Results: cred,
	})
}

func (h *Credential) DeleteCredential(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.Service.Delete(c.UserContext(), id); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Credential deleted",
		Results: nil,
	})
}

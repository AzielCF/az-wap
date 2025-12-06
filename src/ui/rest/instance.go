package rest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/config"
	domainInstance "github.com/AzielCF/az-wap/domains/instance"
	domainSend "github.com/AzielCF/az-wap/domains/send"
	integrationGemini "github.com/AzielCF/az-wap/integrations/gemini"
	"github.com/AzielCF/az-wap/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Instance struct {
	Service     domainInstance.IInstanceUsecase
	SendService domainSend.ISendUsecase
}

type typingState struct {
	typing        bool
	timer         *time.Timer
	lastStartSent time.Time
	lastStopSent  time.Time
}

type presenceState struct {
	online            bool
	timer             *time.Timer
	lastAvailableSent time.Time
}

var (
	typingMu       sync.Mutex
	typingStates   = make(map[string]*typingState)
	presenceMu     sync.Mutex
	presenceStates = make(map[string]*presenceState)
)

func (handler *Instance) handleTypingState(instanceID, phone, token string, isOn bool) string {
	key := instanceID + "|" + phone

	typingMu.Lock()
	state, ok := typingStates[key]
	if !ok || state == nil {
		state = &typingState{}
		typingStates[key] = state
	}
	prevTyping := state.typing
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}

	now := time.Now()
	action := ""
	minStartInterval := 3 * time.Second

	if isOn {
		// Solo enviar START si no estamos ya en typing y respetando un cooldown
		if !state.typing {
			if state.lastStartSent.IsZero() || now.Sub(state.lastStartSent) >= minStartInterval {
				action = "start"
				state.typing = true
				state.lastStartSent = now
			}
		}
		// Si estamos en typing (ya sea recién activado o de antes), refrescamos el auto-stop
		if state.typing {
			timeout := 8 * time.Second
			state.timer = time.AfterFunc(timeout, func() {
				handler.handleTypingTimeout(instanceID, phone, token, key)
			})
		}
	} else {
		// OFF: solo enviamos STOP si realmente estábamos en typing
		if state.typing {
			action = "stop"
			state.typing = false
			state.lastStopSent = now
		}
	}

	if !state.typing && state.timer == nil {
		delete(typingStates, key)
	}

	logrus.WithFields(logrus.Fields{
		"instance_id":   instanceID,
		"phone":         phone,
		"is_on":         isOn,
		"prev_typing":   prevTyping,
		"new_typing":    state.typing,
		"action":        action,
		"has_timer":     state.timer != nil,
		"typing_states": len(typingStates),
	}).Debug("[CHATWOOT] typing state updated")

	typingMu.Unlock()
	return action
}

func (handler *Instance) handleTypingTimeout(instanceID, phone, token, key string) {
	typingMu.Lock()
	state, ok := typingStates[key]
	if !ok || state == nil || !state.typing {
		typingMu.Unlock()
		return
	}
	state.typing = false
	state.timer = nil
	state.lastStopSent = time.Now()
	typingMu.Unlock()

	req := domainSend.ChatPresenceRequest{
		BaseRequest: domainSend.BaseRequest{
			Token: token,
		},
		Phone:  phone,
		Action: "stop",
	}
	_, err := handler.SendService.SendChatPresence(context.Background(), req)
	if err != nil {
		logrus.WithError(err).WithField("instance_id", instanceID).Error("[CHATWOOT] failed to send auto-stop chat presence to WhatsApp")
	}
}

func (handler *Instance) handlePresenceActivity(instanceID string, token string) {
	if token == "" || handler.SendService == nil {
		return
	}

	now := time.Now()
	minInterval := 30 * time.Second
	idleTimeout := 20 * time.Second

	presenceMu.Lock()
	state := presenceStates[instanceID]
	if state == nil {
		state = &presenceState{}
		presenceStates[instanceID] = state
	}

	// limpiamos cualquier timer previo
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}

	if !state.online || state.lastAvailableSent.IsZero() || now.Sub(state.lastAvailableSent) >= minInterval {
		req := domainSend.PresenceRequest{Type: "available", Token: token}
		resp, err := handler.SendService.SendPresence(context.Background(), req)
		if err != nil {
			logrus.WithError(err).Warn("[CHATWOOT] failed to send available presence to WhatsApp")
		} else {
			logrus.WithFields(logrus.Fields{
				"instance_id": instanceID,
				"status":      resp.Status,
			}).Info("[CHATWOOT] available presence sent to WhatsApp")
		}
		state.online = true
		state.lastAvailableSent = now
	}

	state.timer = time.AfterFunc(idleTimeout, func() {
		handler.handlePresenceTimeout(instanceID, token)
	})

	presenceMu.Unlock()
}

func (handler *Instance) handlePresenceTimeout(instanceID string, token string) {
	presenceMu.Lock()
	state := presenceStates[instanceID]
	if state == nil || !state.online {
		presenceMu.Unlock()
		return
	}
	state.online = false
	state.timer = nil
	delete(presenceStates, instanceID)
	presenceMu.Unlock()

	if token == "" || handler.SendService == nil {
		return
	}

	req := domainSend.PresenceRequest{Type: "unavailable", Token: token}
	resp, err := handler.SendService.SendPresence(context.Background(), req)
	if err != nil {
		logrus.WithError(err).Warn("[CHATWOOT] failed to send unavailable presence to WhatsApp")
		return
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": instanceID,
		"status":      resp.Status,
	}).Info("[CHATWOOT] unavailable presence sent to WhatsApp")
}

func InitRestInstance(app fiber.Router, service domainInstance.IInstanceUsecase, sendService domainSend.ISendUsecase) Instance {
	rest := Instance{Service: service, SendService: sendService}
	app.Post("/instances", rest.CreateInstance)
	app.Get("/instances", rest.ListInstances)
	app.Delete("/instances/:id", rest.DeleteInstance)
	app.Put("/instances/:id/webhook", rest.UpdateInstanceWebhookConfig)
	app.Put("/instances/:id/chatwoot", rest.UpdateInstanceChatwootConfig)
	app.Post("/instances/:id/chatwoot/webhook", rest.ReceiveChatwootWebhook)
	app.Put("/instances/:id/bot", rest.UpdateInstanceBotConfig)
	app.Put("/instances/:id/gemini", rest.UpdateInstanceGeminiConfig)
	app.Post("/instances/:id/gemini/memory/clear", rest.ClearInstanceGeminiMemory)
	app.Get("/settings/gemini", rest.GetGeminiSettings)
	app.Put("/settings/gemini", rest.UpdateGeminiSettings)
	return rest
}

func (handler *Instance) CreateInstance(c *fiber.Ctx) error {
	var request domainInstance.CreateInstanceRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	instance, err := handler.Service.Create(c.UserContext(), request)
	utils.PanicIfNeeded(err)

	response := map[string]any{
		"id":     instance.ID,
		"name":   instance.Name,
		"token":  instance.Token,
		"status": instance.Status,
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance created",
		Results: response,
	})
}

func (handler *Instance) ListInstances(c *fiber.Ctx) error {
	instances, err := handler.Service.List(c.UserContext())
	utils.PanicIfNeeded(err)

	var results []map[string]any
	for _, instance := range instances {
		results = append(results, map[string]any{
			"id":                           instance.ID,
			"name":                         instance.Name,
			"status":                       instance.Status,
			"token":                        instance.Token,
			"webhook_urls":                 instance.WebhookURLs,
			"webhook_secret":               instance.WebhookSecret,
			"webhook_insecure_skip_verify": instance.WebhookInsecureSkipVerify,
			"chatwoot_base_url":            instance.ChatwootBaseURL,
			"chatwoot_account_token":       instance.ChatwootAccountToken,
			"chatwoot_bot_token":           instance.ChatwootBotToken,
			"chatwoot_account_id":          instance.ChatwootAccountID,
			"chatwoot_inbox_id":            instance.ChatwootInboxID,
			"chatwoot_inbox_identifier":    instance.ChatwootInboxIdentifier,
			"chatwoot_enabled":             instance.ChatwootEnabled,
			"bot_id":                       instance.BotID,
			"gemini_enabled":               instance.GeminiEnabled,
			"gemini_api_key":               instance.GeminiAPIKey,
			"gemini_model":                 instance.GeminiModel,
			"gemini_system_prompt":         instance.GeminiSystemPrompt,
			"gemini_knowledge_base":        instance.GeminiKnowledgeBase,
			"gemini_timezone":              instance.GeminiTimezone,
			"gemini_audio_enabled":         instance.GeminiAudioEnabled,
			"gemini_image_enabled":         instance.GeminiImageEnabled,
			"gemini_memory_enabled":        instance.GeminiMemoryEnabled,
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instances fetched",
		Results: results,
	})
}

func (handler *Instance) DeleteInstance(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := handler.Service.Delete(c.UserContext(), id); err != nil {
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
		Message: "Instance deleted",
		Results: nil,
	})
}

func (handler *Instance) UpdateInstanceWebhookConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	var request struct {
		URLs               []string `json:"urls"`
		Secret             string   `json:"secret"`
		InsecureSkipVerify bool     `json:"insecure"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	inst, err := handler.Service.UpdateWebhookConfig(c.UserContext(), id, request.URLs, request.Secret, request.InsecureSkipVerify)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	response := map[string]any{
		"id":                           inst.ID,
		"name":                         inst.Name,
		"status":                       inst.Status,
		"token":                        inst.Token,
		"webhook_urls":                 inst.WebhookURLs,
		"webhook_secret":               inst.WebhookSecret,
		"webhook_insecure_skip_verify": inst.WebhookInsecureSkipVerify,
		"chatwoot_base_url":            inst.ChatwootBaseURL,
		"chatwoot_account_token":       inst.ChatwootAccountToken,
		"chatwoot_bot_token":           inst.ChatwootBotToken,
		"chatwoot_account_id":          inst.ChatwootAccountID,
		"chatwoot_inbox_id":            inst.ChatwootInboxID,
		"chatwoot_inbox_identifier":    inst.ChatwootInboxIdentifier,
		"chatwoot_credential_id":       inst.ChatwootCredentialID,
		"chatwoot_enabled":             inst.ChatwootEnabled,
		"bot_id":                       inst.BotID,
		"gemini_enabled":               inst.GeminiEnabled,
		"gemini_api_key":               inst.GeminiAPIKey,
		"gemini_model":                 inst.GeminiModel,
		"gemini_system_prompt":         inst.GeminiSystemPrompt,
		"gemini_knowledge_base":        inst.GeminiKnowledgeBase,
		"gemini_timezone":              inst.GeminiTimezone,
		"gemini_audio_enabled":         inst.GeminiAudioEnabled,
		"gemini_image_enabled":         inst.GeminiImageEnabled,
		"gemini_memory_enabled":        inst.GeminiMemoryEnabled,
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance webhook config updated",
		Results: response,
	})
}

func (handler *Instance) UpdateInstanceChatwootConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	var request struct {
		BaseURL         string `json:"base_url"`
		AccountID       string `json:"account_id"`
		InboxID         string `json:"inbox_id"`
		InboxIdentifier string `json:"inbox_identifier"`
		AccountToken    string `json:"account_token"`
		BotToken        string `json:"bot_token"`
		CredentialID    string `json:"credential_id"`
		Enabled         bool   `json:"enabled"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	inst, err := handler.Service.UpdateChatwootConfig(c.UserContext(), id, request.BaseURL, request.AccountID, request.InboxID, request.InboxIdentifier, request.AccountToken, request.BotToken, request.CredentialID, request.Enabled)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	response := map[string]any{
		"id":                           inst.ID,
		"name":                         inst.Name,
		"status":                       inst.Status,
		"token":                        inst.Token,
		"webhook_urls":                 inst.WebhookURLs,
		"webhook_secret":               inst.WebhookSecret,
		"webhook_insecure_skip_verify": inst.WebhookInsecureSkipVerify,
		"chatwoot_base_url":            inst.ChatwootBaseURL,
		"chatwoot_account_token":       inst.ChatwootAccountToken,
		"chatwoot_bot_token":           inst.ChatwootBotToken,
		"chatwoot_account_id":          inst.ChatwootAccountID,
		"chatwoot_inbox_id":            inst.ChatwootInboxID,
		"chatwoot_inbox_identifier":    inst.ChatwootInboxIdentifier,
		"gemini_enabled":               inst.GeminiEnabled,
		"bot_id":                       inst.BotID,
		"gemini_api_key":               inst.GeminiAPIKey,
		"gemini_model":                 inst.GeminiModel,
		"gemini_system_prompt":         inst.GeminiSystemPrompt,
		"gemini_knowledge_base":        inst.GeminiKnowledgeBase,
		"gemini_timezone":              inst.GeminiTimezone,
		"gemini_audio_enabled":         inst.GeminiAudioEnabled,
		"gemini_image_enabled":         inst.GeminiImageEnabled,
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance chatwoot config updated",
		Results: response,
	})
}

func (handler *Instance) UpdateInstanceGeminiConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	var request struct {
		Enabled       bool   `json:"enabled"`
		APIKey        string `json:"api_key"`
		Model         string `json:"model"`
		SystemPrompt  string `json:"system_prompt"`
		KnowledgeBase string `json:"knowledge_base"`
		Timezone      string `json:"timezone"`
		AudioEnabled  bool   `json:"audio_enabled"`
		ImageEnabled  bool   `json:"image_enabled"`
		MemoryEnabled bool   `json:"memory_enabled"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	inst, err := handler.Service.UpdateGeminiConfig(c.UserContext(), id, request.Enabled, request.APIKey, request.Model, request.SystemPrompt, request.KnowledgeBase, request.Timezone, request.AudioEnabled, request.ImageEnabled, request.MemoryEnabled)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	response := map[string]any{
		"id":                           inst.ID,
		"name":                         inst.Name,
		"status":                       inst.Status,
		"token":                        inst.Token,
		"webhook_urls":                 inst.WebhookURLs,
		"webhook_secret":               inst.WebhookSecret,
		"webhook_insecure_skip_verify": inst.WebhookInsecureSkipVerify,
		"chatwoot_base_url":            inst.ChatwootBaseURL,
		"chatwoot_account_token":       inst.ChatwootAccountToken,
		"chatwoot_bot_token":           inst.ChatwootBotToken,
		"chatwoot_account_id":          inst.ChatwootAccountID,
		"chatwoot_inbox_id":            inst.ChatwootInboxID,
		"chatwoot_inbox_identifier":    inst.ChatwootInboxIdentifier,
		"gemini_enabled":               inst.GeminiEnabled,
		"gemini_api_key":               inst.GeminiAPIKey,
		"gemini_model":                 inst.GeminiModel,
		"gemini_system_prompt":         inst.GeminiSystemPrompt,
		"gemini_knowledge_base":        inst.GeminiKnowledgeBase,
		"gemini_timezone":              inst.GeminiTimezone,
		"gemini_audio_enabled":         inst.GeminiAudioEnabled,
		"gemini_image_enabled":         inst.GeminiImageEnabled,
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance gemini config updated",
		Results: response,
	})
}

func (handler *Instance) UpdateInstanceBotConfig(c *fiber.Ctx) error {
	id := c.Params("id")
	var request struct {
		BotID *string `json:"bot_id"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	if request.BotID == nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "bot_id: cannot be null.",
			Results: nil,
		})
	}

	inst, err := handler.Service.UpdateBotConfig(c.UserContext(), id, *request.BotID)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}

	response := map[string]any{
		"id":                           inst.ID,
		"name":                         inst.Name,
		"status":                       inst.Status,
		"token":                        inst.Token,
		"webhook_urls":                 inst.WebhookURLs,
		"webhook_secret":               inst.WebhookSecret,
		"webhook_insecure_skip_verify": inst.WebhookInsecureSkipVerify,
		"chatwoot_base_url":            inst.ChatwootBaseURL,
		"chatwoot_account_token":       inst.ChatwootAccountToken,
		"chatwoot_bot_token":           inst.ChatwootBotToken,
		"chatwoot_account_id":          inst.ChatwootAccountID,
		"chatwoot_inbox_id":            inst.ChatwootInboxID,
		"chatwoot_inbox_identifier":    inst.ChatwootInboxIdentifier,
		"chatwoot_enabled":             inst.ChatwootEnabled,
		"bot_id":                       inst.BotID,
		"gemini_enabled":               inst.GeminiEnabled,
		"gemini_api_key":               inst.GeminiAPIKey,
		"gemini_model":                 inst.GeminiModel,
		"gemini_system_prompt":         inst.GeminiSystemPrompt,
		"gemini_knowledge_base":        inst.GeminiKnowledgeBase,
		"gemini_timezone":              inst.GeminiTimezone,
		"gemini_audio_enabled":         inst.GeminiAudioEnabled,
		"gemini_image_enabled":         inst.GeminiImageEnabled,
		"gemini_memory_enabled":        inst.GeminiMemoryEnabled,
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance bot config updated",
		Results: response,
	})
}

func (handler *Instance) ReceiveChatwootWebhook(c *fiber.Ctx) error {
	id := c.Params("id")

	instances, err := handler.Service.List(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	var targetInst domainInstance.Instance
	for _, inst := range instances {
		if inst.ID == id {
			targetInst = inst
			break
		}
	}

	if targetInst.ID == "" {
		return c.Status(404).JSON(utils.ResponseData{
			Status:  404,
			Code:    "NOT_FOUND",
			Message: "Instance not found",
			Results: nil,
		})
	}

	var payload map[string]any
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}
	flag := viper.GetString("capture_chatwoot_webhooks")
	if flag == "" {
		flag = viper.GetString("capture_chatwoot_payloads")
	}
	if flag == "1" {
		if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
			logrus.WithFields(logrus.Fields{
				"instance_id": id,
				"path":        c.Path(),
				"method":      c.Method(),
			}).Info("[CHATWOOT_WEBHOOK_CAPTURE] payload")
			logrus.Info(string(data))
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"instance_id": id,
			"path":        c.Path(),
			"method":      c.Method(),
			"payload":     payload,
		}).Info("[CHATWOOT] webhook received")
	}

	eventVal, ok := payload["event"]
	event, _ := eventVal.(string)
	if !ok || event == "" {
		logrus.WithField("instance_id", id).Warn("[CHATWOOT] webhook without event, ignoring")
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: missing event",
			Results: nil,
		})
	}

	if event == "conversation_typing_on" || event == "conversation_typing_off" {
		convVal, ok := payload["conversation"]
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: missing conversation for typing event",
				Results: nil,
			})
		}

		conv, ok := convVal.(map[string]any)
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: invalid conversation payload for typing event",
				Results: nil,
			})
		}

		metaVal, ok := conv["meta"]
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: missing meta for typing event",
				Results: nil,
			})
		}

		meta, ok := metaVal.(map[string]any)
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: invalid meta payload for typing event",
				Results: nil,
			})
		}

		senderMetaVal, ok := meta["sender"]
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: missing sender meta for typing event",
				Results: nil,
			})
		}

		senderMeta, ok := senderMetaVal.(map[string]any)
		if !ok {
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot webhook ignored: invalid sender meta for typing event",
				Results: nil,
			})
		}

		phone := ""
		if v, ok := senderMeta["phone_number"].(string); ok {
			phone = v
		} else if v, ok := senderMeta["identifier"].(string); ok {
			phone = v
		}
		if phone == "" {
			logrus.WithField("instance_id", id).Warn("[CHATWOOT] typing webhook missing phone number, ignoring")
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "IGNORED",
				Message: "Chatwoot typing webhook ignored: missing phone number",
				Results: nil,
			})
		}

		isOn := event == "conversation_typing_on"
		logrus.WithFields(logrus.Fields{
			"instance_id": id,
			"event":       event,
			"phone":       phone,
		}).Debug("[CHATWOOT] typing webhook resolved")
		// Cualquier actividad de escritura cuenta como actividad de presencia global
		handler.handlePresenceActivity(id, targetInst.Token)
		action := handler.handleTypingState(id, phone, targetInst.Token, isOn)
		if action == "" {
			logrus.WithFields(logrus.Fields{
				"instance_id": id,
				"event":       event,
				"phone":       phone,
			}).Debug("[CHATWOOT] typing event produced no state change")
			return c.JSON(utils.ResponseData{
				Status:  200,
				Code:    "SUCCESS",
				Message: "Chatwoot typing event ignored: no state change",
				Results: nil,
			})
		}

		chatPresenceReq := domainSend.ChatPresenceRequest{
			BaseRequest: domainSend.BaseRequest{
				Token: targetInst.Token,
			},
			Phone:  phone,
			Action: action,
		}

		resp, err := handler.SendService.SendChatPresence(c.UserContext(), chatPresenceReq)
		if err != nil {
			logrus.WithError(err).WithField("instance_id", id).Error("[CHATWOOT] failed to send chat presence to WhatsApp")
			return c.Status(500).JSON(utils.ResponseData{
				Status:  500,
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
				Results: nil,
			})
		}

		logrus.WithFields(logrus.Fields{
			"instance_id": id,
			"event":       event,
			"phone":       phone,
			"action":      action,
			"response":    resp.Status,
		}).Info("[CHATWOOT] typing event forwarded to WhatsApp")

		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "SUCCESS",
			Message: "Chatwoot typing event forwarded to WhatsApp",
			Results: resp,
		})
	}

	if event != "message_created" {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: event " + event,
			Results: nil,
		})
	}

	convVal, ok := payload["conversation"]
	if !ok {
		logrus.WithField("instance_id", id).Warn("[CHATWOOT] webhook without conversation, ignoring")
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: missing conversation",
			Results: nil,
		})
	}

	conv, ok := convVal.(map[string]any)
	if !ok {
		logrus.WithField("instance_id", id).Warn("[CHATWOOT] webhook conversation has unexpected type, ignoring")
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: invalid conversation payload",
			Results: nil,
		})
	}

	msgsVal, ok := conv["messages"]
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: no messages in conversation",
			Results: nil,
		})
	}

	msgsSlice, ok := msgsVal.([]interface{})
	if !ok || len(msgsSlice) == 0 {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: empty messages list",
			Results: nil,
		})
	}

	lastMsgVal, ok := msgsSlice[len(msgsSlice)-1].(map[string]any)
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: invalid message payload",
			Results: nil,
		})
	}

	var firstAttachment map[string]any
	if attachmentsVal, hasAttachments := lastMsgVal["attachments"]; hasAttachments {
		if attachmentsSlice, ok := attachmentsVal.([]interface{}); ok && len(attachmentsSlice) > 0 {
			if att, ok := attachmentsSlice[0].(map[string]any); ok {
				firstAttachment = att
			}
		}
	}

	contentVal, _ := lastMsgVal["content"].(string)
	message := contentVal
	if message == "" && firstAttachment == nil {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: empty message content and no attachments",
			Results: nil,
		})
	}

	senderTypeVal, _ := lastMsgVal["sender_type"].(string)
	msgTypeVal, hasMsgType := lastMsgVal["message_type"]
	msgType := 0
	if hasMsgType {
		if f, ok := msgTypeVal.(float64); ok {
			msgType = int(f)
		}
	}
	fromBot := false
	if rawAttrs, ok := lastMsgVal["content_attributes"]; ok {
		if attrs, ok2 := rawAttrs.(map[string]any); ok2 {
			if v, ok3 := attrs["from_bot"].(bool); ok3 && v {
				fromBot = true
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"instance_id":  id,
		"sender_type":  senderTypeVal,
		"message_type": msgType,
		"has_msg_type": hasMsgType,
		"from_bot":     fromBot,
	}).Debug("[CHATWOOT] Message validation check")

	allowedSender := senderTypeVal == "User"
	if fromBot {
		allowedSender = false
	}
	if !allowedSender || msgType != 1 {
		logrus.WithFields(logrus.Fields{
			"instance_id":  id,
			"sender_type":  senderTypeVal,
			"message_type": msgType,
			"from_bot":     fromBot,
		}).Info("[CHATWOOT] webhook ignored: not an agent outbound message")
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: not an agent outbound message",
			Results: nil,
		})
	}

	metaVal, ok := conv["meta"]
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: missing meta",
			Results: nil,
		})
	}

	meta, ok := metaVal.(map[string]any)
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: invalid meta payload",
			Results: nil,
		})
	}

	senderMetaVal, ok := meta["sender"]
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: missing sender meta",
			Results: nil,
		})
	}

	senderMeta, ok := senderMetaVal.(map[string]any)
	if !ok {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: invalid sender meta",
			Results: nil,
		})
	}

	phone := ""
	if v, ok := senderMeta["phone_number"].(string); ok {
		phone = v
	} else if v, ok := senderMeta["identifier"].(string); ok {
		phone = v
	}
	if phone == "" {
		logrus.WithField("instance_id", id).Warn("[CHATWOOT] webhook missing phone number, ignoring")
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: missing phone number",
			Results: nil,
		})
	}

	if firstAttachment != nil {
		fileTypeVal, _ := firstAttachment["file_type"].(string)
		dataURLVal, _ := firstAttachment["data_url"].(string)
		if dataURLVal == "" {
			logrus.WithField("instance_id", id).Warn("[CHATWOOT] attachment without data_url, falling back to text")
		} else {
			switch fileTypeVal {
			case "image":
				urlCopy := dataURLVal
				imgReq := domainSend.ImageRequest{
					BaseRequest: domainSend.BaseRequest{
						Phone: phone,
						Token: targetInst.Token,
					},
					Caption:  message,
					ImageURL: &urlCopy,
					Compress: true,
				}
				resp, err := handler.SendService.SendImage(c.UserContext(), imgReq)
				if err != nil {
					logrus.WithError(err).WithField("instance_id", id).Error("[CHATWOOT] failed to send image to WhatsApp")
					return c.Status(500).JSON(utils.ResponseData{
						Status:  500,
						Code:    "INTERNAL_ERROR",
						Message: err.Error(),
						Results: nil,
					})
				}
				handler.handlePresenceActivity(id, targetInst.Token)
				return c.JSON(utils.ResponseData{
					Status:  200,
					Code:    "SUCCESS",
					Message: "Chatwoot image forwarded to WhatsApp",
					Results: resp,
				})
			case "audio":
				urlCopy := dataURLVal
				audioReq := domainSend.AudioRequest{
					BaseRequest: domainSend.BaseRequest{
						Phone: phone,
						Token: targetInst.Token,
					},
					AudioURL: &urlCopy,
				}
				resp, err := handler.SendService.SendAudio(c.UserContext(), audioReq)
				if err != nil {
					logrus.WithError(err).WithField("instance_id", id).Error("[CHATWOOT] failed to send audio to WhatsApp")
					return c.Status(500).JSON(utils.ResponseData{
						Status:  500,
						Code:    "INTERNAL_ERROR",
						Message: err.Error(),
						Results: nil,
					})
				}
				handler.handlePresenceActivity(id, targetInst.Token)
				return c.JSON(utils.ResponseData{
					Status:  200,
					Code:    "SUCCESS",
					Message: "Chatwoot audio forwarded to WhatsApp",
					Results: resp,
				})
			case "video":
				urlCopy := dataURLVal
				videoReq := domainSend.VideoRequest{
					BaseRequest: domainSend.BaseRequest{
						Phone: phone,
						Token: targetInst.Token,
					},
					Caption:  message,
					VideoURL: &urlCopy,
				}
				resp, err := handler.SendService.SendVideo(c.UserContext(), videoReq)
				if err != nil {
					logrus.WithError(err).WithField("instance_id", id).Error("[CHATWOOT] failed to send video to WhatsApp")
					return c.Status(500).JSON(utils.ResponseData{
						Status:  500,
						Code:    "INTERNAL_ERROR",
						Message: err.Error(),
						Results: nil,
					})
				}
				handler.handlePresenceActivity(id, targetInst.Token)
				return c.JSON(utils.ResponseData{
					Status:  200,
					Code:    "SUCCESS",
					Message: "Chatwoot video forwarded to WhatsApp",
					Results: resp,
				})
			default:
				logrus.WithFields(logrus.Fields{
					"instance_id": id,
					"file_type":   fileTypeVal,
				}).Info("[CHATWOOT] unsupported attachment type, falling back to text if available")
			}
		}
	}

	if message == "" {
		return c.JSON(utils.ResponseData{
			Status:  200,
			Code:    "IGNORED",
			Message: "Chatwoot webhook ignored: empty message content and no attachments",
			Results: nil,
		})
	}

	req := domainSend.MessageRequest{
		BaseRequest: domainSend.BaseRequest{
			Phone: phone,
			Token: targetInst.Token,
		},
		Message: message,
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": id,
		"phone":       phone,
		"token":       targetInst.Token,
		"message":     message,
	}).Info("[CHATWOOT] Attempting to send text message to WhatsApp")

	sendResp, err := handler.SendService.SendText(c.UserContext(), req)
	if err != nil {
		logrus.WithError(err).WithField("instance_id", id).Error("[CHATWOOT] failed to send message to WhatsApp")
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	logrus.WithFields(logrus.Fields{
		"instance_id": id,
		"message_id":  sendResp.MessageID,
	}).Info("[CHATWOOT] Text message sent to WhatsApp successfully")

	// Enviar/renovar presencia global al enviar mensaje
	handler.handlePresenceActivity(id, targetInst.Token)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Chatwoot message forwarded to WhatsApp",
		Results: sendResp,
	})
}

func (handler *Instance) ClearInstanceGeminiMemory(c *fiber.Ctx) error {
	id := c.Params("id")
	if strings.TrimSpace(id) == "" {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: "id: cannot be blank.",
			Results: nil,
		})
	}

	instances, err := handler.Service.List(c.UserContext())
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}

	found := false
	for _, inst := range instances {
		if inst.ID == id {
			found = true
			break
		}
	}
	if !found {
		return c.Status(404).JSON(utils.ResponseData{
			Status:  404,
			Code:    "NOT_FOUND",
			Message: "Instance not found",
			Results: nil,
		})
	}

	integrationGemini.ClearInstanceMemory(id)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Instance gemini memory cleared",
		Results: nil,
	})
}

func (handler *Instance) GetGeminiSettings(c *fiber.Ctx) error {
	_ = handler
	response := map[string]any{
		"global_system_prompt": config.GeminiGlobalSystemPrompt,
		"timezone":             config.GeminiTimezone,
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Gemini settings fetched",
		Results: response,
	})
}

func (handler *Instance) UpdateGeminiSettings(c *fiber.Ctx) error {
	_ = handler
	var request struct {
		GlobalSystemPrompt string `json:"global_system_prompt"`
		Timezone           string `json:"timezone"`
	}
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(utils.ResponseData{
			Status:  400,
			Code:    "BAD_REQUEST",
			Message: err.Error(),
			Results: nil,
		})
	}
	if err := config.SaveGeminiGlobalSystemPrompt(request.GlobalSystemPrompt); err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}
	if err := config.SaveGeminiTimezone(request.Timezone); err != nil {
		return c.Status(500).JSON(utils.ResponseData{
			Status:  500,
			Code:    "INTERNAL_ERROR",
			Message: err.Error(),
			Results: nil,
		})
	}
	response := map[string]any{
		"global_system_prompt": config.GeminiGlobalSystemPrompt,
		"timezone":             config.GeminiTimezone,
	}
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Gemini settings updated",
		Results: response,
	})
}

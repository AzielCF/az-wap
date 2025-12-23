package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AzielCF/az-wap/config"
	"github.com/gofiber/fiber/v2"
)

func TestGeminiSettingsEndpoints_IncludeSafetySettings(t *testing.T) {
	origStorages := config.PathStorages
	t.Cleanup(func() { config.PathStorages = origStorages })
	config.PathStorages = t.TempDir()

	h := &Instance{}
	app := fiber.New()
	app.Get("/settings/gemini", h.GetGeminiSettings)
	app.Put("/settings/gemini", h.UpdateGeminiSettings)
	app.Get("/settings/ai", h.GetGeminiSettings)
	app.Put("/settings/ai", h.UpdateGeminiSettings)

	paths := []string{"/settings/gemini", "/settings/ai"}
	for _, p := range paths {
		payload := map[string]any{
			"global_system_prompt": "x",
			"timezone":             "UTC",
			"debounce_ms":          1500,
			"wait_contact_idle_ms": 9000,
			"typing_enabled":       true,
		}
		b, _ := json.Marshal(payload)
		updReq := httptest.NewRequest(http.MethodPut, p, bytes.NewReader(b))
		updReq.Header.Set("Content-Type", "application/json")

		updResp, err := app.Test(updReq)
		if err != nil {
			t.Fatalf("update app.Test() error (%s): %v", p, err)
		}
		updResp.Body.Close()
		if updResp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected update status %d (%s)", updResp.StatusCode, p)
		}

		getReq := httptest.NewRequest(http.MethodGet, p, nil)
		getResp, err := app.Test(getReq)
		if err != nil {
			t.Fatalf("get app.Test() error (%s): %v", p, err)
		}
		if getResp.StatusCode != http.StatusOK {
			getResp.Body.Close()
			t.Fatalf("unexpected get status %d (%s)", getResp.StatusCode, p)
		}

		var envelope struct {
			Code    string                 `json:"code"`
			Results map[string]interface{} `json:"results"`
		}
		if err := json.NewDecoder(getResp.Body).Decode(&envelope); err != nil {
			getResp.Body.Close()
			t.Fatalf("decode error (%s): %v", p, err)
		}
		getResp.Body.Close()
		if envelope.Code != "SUCCESS" {
			t.Fatalf("unexpected code %q (%s)", envelope.Code, p)
		}

		if _, ok := envelope.Results["debounce_ms"]; !ok {
			t.Fatalf("expected debounce_ms in results (%s)", p)
		}
		if _, ok := envelope.Results["wait_contact_idle_ms"]; !ok {
			t.Fatalf("expected wait_contact_idle_ms in results (%s)", p)
		}
		if _, ok := envelope.Results["typing_enabled"]; !ok {
			t.Fatalf("expected typing_enabled in results (%s)", p)
		}
	}
}

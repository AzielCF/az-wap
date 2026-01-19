package bot

type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ModelPricing define los costos por 1M tokens en USD
type ModelPricing struct {
	InputPerMToken   float64 `json:"input_per_m_token"`   // USD por 1M tokens de entrada
	OutputPerMToken  float64 `json:"output_per_m_token"`  // USD por 1M tokens de salida (incluye thinking)
	CacheInputPerMT  float64 `json:"cache_input_per_mt"`  // USD por 1M tokens de cache input
	CacheStoragePerH float64 `json:"cache_storage_per_h"` // USD por 1M tokens por hora de almacenamiento
	AudioInputPerMT  float64 `json:"audio_input_per_mt"`  // USD por 1M tokens de audio (si aplica)
}

const (
	DefaultGeminiModel     = "gemini-2.0-flash"
	DefaultGeminiLiteModel = "gemini-2.0-flash-lite"
)

var GeminiModels = []ModelInfo{
	// Gemini 3 Series (Next Gen)
	{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Description: "Most intelligent model for multimodal understanding, agentic and vibe-coding."},
	{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash (Preview)", Description: "Most intelligent model built for speed with superior search and grounding."},

	// Gemini 2.5 Series (Advanced Thinking)
	{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "State-of-the-art thinking model for complex reasoning, code, math, and STEM."},
	{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Best price-performance. Ideal for large scale processing and agentic use cases."},
	{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash-Lite", Description: "Fastest flash model optimized for cost-efficiency and high throughput."},

	// Gemini 2.0 Series (Workhorse)
	{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Description: "Next-gen workhorse model, superior speed and native tool use (1M context)."},
	{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash-Lite", Description: "Second generation small workhorse model optimized for low latency."},
	{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash (Experimental)", Description: "Experimental version of the second generation workhorse model."},

	// specialized / Legacy Aliases
	{ID: "gemini-1.5-pro-latest", Name: "Gemini 1.5 Pro (Latest)", Description: "Legacy Pro model with long context window."},
	{ID: "gemini-1.5-flash-latest", Name: "Gemini 1.5 Flash (Latest)", Description: "Legacy Flash model for versatile tasks."},
}

// GeminiModelPrices contiene los precios oficiales de Google (Paid Tier, por 1M tokens en USD)
var GeminiModelPrices = map[string]ModelPricing{
	// Gemini 3 Series
	"gemini-3-pro-preview": {
		InputPerMToken:   2.00,
		OutputPerMToken:  12.00,
		CacheInputPerMT:  0.20,
		CacheStoragePerH: 4.50,
	},
	"gemini-3-flash-preview": {
		InputPerMToken:   0.50,
		OutputPerMToken:  3.00,
		CacheInputPerMT:  0.05,
		CacheStoragePerH: 1.00,
		AudioInputPerMT:  1.00,
	},

	// Gemini 2.5 Series
	"gemini-2.5-pro": {
		InputPerMToken:   1.25,
		OutputPerMToken:  10.00,
		CacheInputPerMT:  0.125,
		CacheStoragePerH: 4.50,
	},
	"gemini-2.5-flash": {
		InputPerMToken:   0.30,
		OutputPerMToken:  2.50,
		CacheInputPerMT:  0.03,
		CacheStoragePerH: 1.00,
		AudioInputPerMT:  1.00,
	},
	"gemini-2.5-flash-lite": {
		InputPerMToken:   0.10,
		OutputPerMToken:  0.40,
		CacheInputPerMT:  0.01,
		CacheStoragePerH: 1.00,
		AudioInputPerMT:  0.30,
	},

	// Gemini 2.0 Series
	"gemini-2.0-flash": {
		InputPerMToken:   0.10,
		OutputPerMToken:  0.40,
		CacheInputPerMT:  0.025,
		CacheStoragePerH: 1.00,
		AudioInputPerMT:  0.70,
	},
	"gemini-2.0-flash-lite": {
		InputPerMToken:  0.075,
		OutputPerMToken: 0.30,
		// No soporta Context Caching
	},

	// Legacy/Fallback (precios estimados similares a 2.0 flash)
	"gemini-1.5-pro-latest": {
		InputPerMToken:   1.25,
		OutputPerMToken:  5.00,
		CacheInputPerMT:  0.3125,
		CacheStoragePerH: 4.50,
	},
	"gemini-1.5-flash-latest": {
		InputPerMToken:   0.075,
		OutputPerMToken:  0.30,
		CacheInputPerMT:  0.01875,
		CacheStoragePerH: 1.00,
	},
}

var ProviderModels = map[Provider][]ModelInfo{
	ProviderGemini: GeminiModels,
}

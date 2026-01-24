package bot

type ModelInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	AvgCostIn    float64 `json:"avg_cost_in,omitempty"`
	AvgCostOut   float64 `json:"avg_cost_out,omitempty"`
	IsMultimodal bool    `json:"is_multimodal"`
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
	DefaultOpenAIModel     = "gpt-4o"
	DefaultOpenAIMiniModel = "gpt-4o-mini"
)

var OpenAIModels = []ModelInfo{
	// GPT-5 Series (Next Gen Multimodal)
	{ID: "gpt-5.2", Name: "GPT-5.2", Description: "Next generation flagship model.", IsMultimodal: true},
	{ID: "gpt-5.1", Name: "GPT-5.1", Description: "Balanced high-intelligence model.", IsMultimodal: true},
	{ID: "gpt-5", Name: "GPT-5", Description: "Base GPT-5 model.", IsMultimodal: true},
	{ID: "gpt-5-mini", Name: "GPT-5 Mini", Description: "Efficient small model of the 5th generation.", IsMultimodal: true},
	{ID: "gpt-5-nano", Name: "GPT-5 Nano", Description: "Ultra-efficient nano model.", IsMultimodal: true},

	// GPT-4.1 Series
	{ID: "gpt-4.1", Name: "GPT-4.1", Description: "Updated GPT-4 generation.", IsMultimodal: true},
	{ID: "gpt-4.1-mini", Name: "GPT-4.1 Mini", Description: "Mini version of GPT-4.1.", IsMultimodal: true},
	{ID: "gpt-4.1-nano", Name: "GPT-4.1 Nano", Description: "Nano version of GPT-4.1.", IsMultimodal: true},

	// GPT-4o Series (Current Workhorse)
	{ID: "gpt-4o", Name: "GPT-4o", Description: "Current state-of-the-art multimodal model.", IsMultimodal: true},
	{ID: "gpt-4o-mini", Name: "GPT-4o mini", Description: "Fast, affordable small model.", IsMultimodal: true},

	// O-Series (Reasoning)
	{ID: "o3", Name: "OpenAI o3", Description: "Advanced reasoning model.", IsMultimodal: false},
	{ID: "o4-mini", Name: "OpenAI o4-mini", Description: "Next-gen small reasoning model.", IsMultimodal: false},
	{ID: "o3-mini", Name: "OpenAI o3-mini", Description: "Efficient reasoning model.", IsMultimodal: false},
	{ID: "o1-mini", Name: "OpenAI o1-mini", Description: "First gen efficient reasoning model.", IsMultimodal: false},
	{ID: "o1", Name: "OpenAI o1", Description: "First gen reasoning model.", IsMultimodal: false},

	// Legacy / Extra Cheap
	{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Legacy cheap workhorse.", IsMultimodal: false},
}

var OpenAIModelPrices = map[string]ModelPricing{
	// GPT-5 Series
	"gpt-5.2":    {InputPerMToken: 1.75, CacheInputPerMT: 0.175, OutputPerMToken: 14.00},
	"gpt-5.1":    {InputPerMToken: 1.25, CacheInputPerMT: 0.125, OutputPerMToken: 10.00},
	"gpt-5":      {InputPerMToken: 1.25, CacheInputPerMT: 0.125, OutputPerMToken: 10.00},
	"gpt-5-mini": {InputPerMToken: 0.25, CacheInputPerMT: 0.025, OutputPerMToken: 2.00},
	"gpt-5-nano": {InputPerMToken: 0.05, CacheInputPerMT: 0.005, OutputPerMToken: 0.40},

	// GPT-4.1 Series
	"gpt-4.1":      {InputPerMToken: 2.00, CacheInputPerMT: 0.50, OutputPerMToken: 8.00},
	"gpt-4.1-mini": {InputPerMToken: 0.40, CacheInputPerMT: 0.10, OutputPerMToken: 1.60},
	"gpt-4.1-nano": {InputPerMToken: 0.10, CacheInputPerMT: 0.025, OutputPerMToken: 0.40},

	// GPT-4o Series
	"gpt-4o":      {InputPerMToken: 2.50, CacheInputPerMT: 1.25, OutputPerMToken: 10.00},
	"gpt-4o-mini": {InputPerMToken: 0.15, CacheInputPerMT: 0.075, OutputPerMToken: 0.60},

	// O-Series (Reasoning)
	"o3":                    {InputPerMToken: 2.00, CacheInputPerMT: 0.50, OutputPerMToken: 8.00},
	"o4-mini":               {InputPerMToken: 1.10, CacheInputPerMT: 0.275, OutputPerMToken: 4.40},
	"o3-mini":               {InputPerMToken: 1.10, CacheInputPerMT: 0.55, OutputPerMToken: 4.40},
	"o1-mini":               {InputPerMToken: 1.10, CacheInputPerMT: 0.55, OutputPerMToken: 4.40},
	"o1":                    {InputPerMToken: 15.00, CacheInputPerMT: 7.50, OutputPerMToken: 60.00},
	"o1-pro":                {InputPerMToken: 150.00, OutputPerMToken: 600.00},
	"o3-pro":                {InputPerMToken: 20.00, OutputPerMToken: 80.00},
	"o3-deep-research":      {InputPerMToken: 10.00, CacheInputPerMT: 2.50, OutputPerMToken: 40.00},
	"o4-mini-deep-research": {InputPerMToken: 2.00, CacheInputPerMT: 0.50, OutputPerMToken: 8.00},

	// Legacy / Others
	"gpt-3.5-turbo": {InputPerMToken: 0.50, OutputPerMToken: 1.50},
	"gpt-4-turbo":   {InputPerMToken: 10.00, OutputPerMToken: 30.00},
}

var GeminiModels = []ModelInfo{
	// Gemini 3 Series (Next Gen)
	{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Description: "Most intelligent model for multimodal understanding, agentic and vibe-coding.", IsMultimodal: true},
	{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash (Preview)", Description: "Most intelligent model built for speed with superior search and grounding.", IsMultimodal: true},

	// Gemini 2.5 Series (Advanced Thinking)
	{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "State-of-the-art thinking model for complex reasoning, code, math, and STEM.", IsMultimodal: true},
	{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Best price-performance. Ideal for large scale processing and agentic use cases.", IsMultimodal: true},
	{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash-Lite", Description: "Fastest flash model optimized for cost-efficiency and high throughput.", IsMultimodal: true},

	// Gemini 2.0 Series (Workhorse)
	{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Description: "Next-gen workhorse model, superior speed and native tool use (1M context).", IsMultimodal: true},
	{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash-Lite", Description: "Second generation small workhorse model optimized for low latency.", IsMultimodal: true},
	{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash (Experimental)", Description: "Experimental version of the second generation workhorse model.", IsMultimodal: true},

	// specialized / Legacy Aliases
	{ID: "gemini-1.5-pro-latest", Name: "Gemini 1.5 Pro (Latest)", Description: "Legacy Pro model with long context window.", IsMultimodal: true},
	{ID: "gemini-1.5-flash-latest", Name: "Gemini 1.5 Flash (Latest)", Description: "Legacy Flash model for versatile tasks.", IsMultimodal: true},
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
	ProviderOpenAI: OpenAIModels,
}

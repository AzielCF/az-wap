package onlyclients

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AzielCF/az-wap/botengine/domain"
	domainMCP "github.com/AzielCF/az-wap/botengine/domain/mcp"
)

// =================================================================================
// LAYER A: DATA SOURCE (Provider)
// Responsabilidad: Obtener tasas crudas. No sabe de conversión ni de usuarios.
// =================================================================================

type ExchangeRates struct {
	Base      string             // Moneda base (ej: "USD")
	Rates     map[string]float64 // Mapa de tasas (ej: "EUR": 0.92)
	Timestamp time.Time          // Cuándo se obtuvieron
}

type RateProvider interface {
	FetchRates(ctx context.Context) (*ExchangeRates, error)
}

// MoneyConvertProvider implementación concreta para moneyconvert.net
type MoneyConvertProvider struct {
	client *http.Client
}

func NewMoneyConvertProvider() *MoneyConvertProvider {
	return &MoneyConvertProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Estructura interna para parsear la respuesta JSON específica de esta API
type moneyConvertResponse struct {
	Rates map[string]float64 `json:"rates"`
	Base  string             `json:"base"`
	Ts    string             `json:"ts"` // Timestamp string
}

func (p *MoneyConvertProvider) FetchRates(ctx context.Context) (*ExchangeRates, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://cdn.moneyconvert.net/api/latest.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	var raw moneyConvertResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode provider response: %v", err)
	}

	// Normalización de datos
	rates := raw.Rates
	if rates == nil {
		rates = make(map[string]float64)
	}
	// Asegurar consistencia de base (esta API suele usar USD implícito o explícito)
	if _, ok := rates["USD"]; !ok {
		rates["USD"] = 1.0
	}

	// Parseo de timestamp (best effort, fallback a Now)
	ts := time.Now()
	// (Esta API devuelve string, simplemente usamos Now como referencia de "fetch time"
	// ya que el string de ellos no es estándar RFC3339 a veces)

	return &ExchangeRates{
		Base:      "USD", // Esta API siempre es USD base
		Rates:     rates,
		Timestamp: ts,
	}, nil
}

// =================================================================================
// LAYER B: CONVERSION CORE (Engine)
// Responsabilidad: Matemática financiera, validación, caché.
// =================================================================================

type ConversionEngine struct {
	provider  RateProvider
	cache     *ExchangeRates
	lastFetch time.Time
	mu        sync.RWMutex
}

func NewConversionEngine(provider RateProvider) *ConversionEngine {
	return &ConversionEngine{
		provider: provider,
	}
}

// ConversionRequest define qué queremos convertir
type ConversionRequest struct {
	From   string
	To     string
	Amount float64
}

// ConversionResult encapsula el resultado matemático
type ConversionResult struct {
	FromAmount float64
	ToAmount   float64
	Rate       float64
	Timestamp  time.Time
}

func (e *ConversionEngine) Convert(ctx context.Context, req ConversionRequest) (*ConversionResult, error) {
	// 1. Validaciones de Negocio
	if req.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	from := strings.ToUpper(strings.TrimSpace(req.From))
	to := strings.ToUpper(strings.TrimSpace(req.To))
	if from == "" || to == "" {
		return nil, fmt.Errorf("currency codes required")
	}

	// 2. Obtención de Tasas (con Caché)
	rates, err := e.getRates(ctx)
	if err != nil {
		return nil, err
	}

	// 3. Resolución de Tasas Base
	// Matemáticamente: (Amount / RateFrom) * RateTo
	// Asumiendo que Rates[] es "cuántos X obtengo por 1 Base"

	rateFrom, okFrom := rates.Rates[from]
	rateTo, okTo := rates.Rates[to]

	// Manejo de moneda base implícita si no viene en el mapa
	if !okFrom && from == rates.Base {
		rateFrom, okFrom = 1.0, true
	}
	if !okTo && to == rates.Base {
		rateTo, okTo = 1.0, true
	}

	if !okFrom {
		return nil, fmt.Errorf("unsupported currency: %s", from)
	}
	if !okTo {
		return nil, fmt.Errorf("unsupported currency: %s", to)
	}

	// 4. Cálculo
	// Valor en Base = Amount / rateFrom
	// Valor Final = Valor en Base * rateTo
	valInBase := req.Amount / rateFrom
	valFinal := valInBase * rateTo

	// Redondeo Financiero (4 decimales para precisión técnica interna)
	valFinal = math.Round(valFinal*10000) / 10000
	effectiveRate := valFinal / req.Amount

	return &ConversionResult{
		FromAmount: req.Amount,
		ToAmount:   valFinal,
		Rate:       effectiveRate,
		Timestamp:  rates.Timestamp,
	}, nil
}

// getRates maneja el caché transparente para el consumidor
func (e *ConversionEngine) getRates(ctx context.Context) (*ExchangeRates, error) {
	e.mu.RLock()
	if e.cache != nil && time.Since(e.lastFetch) < 1*time.Hour { // Cache extendido a 1h según sugerencia "reference rates"
		defer e.mu.RUnlock()
		return e.cache, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	// Doble check
	if e.cache != nil && time.Since(e.lastFetch) < 1*time.Hour {
		return e.cache, nil
	}

	rates, err := e.provider.FetchRates(ctx)
	if err != nil {
		return nil, err
	}

	e.cache = rates
	e.lastFetch = time.Now()
	return e.cache, nil
}

// =================================================================================
// LAYER C: TOOL / ADAPTER (Orchestrator Interface)
// Responsabilidad: Hablar con el LLM, parsear basura, formatear salida bonita.
// =================================================================================

type ExchangeRateTools struct {
	engine *ConversionEngine
}

func NewExchangeRateTools() *ExchangeRateTools {
	// Inyección de dependencias interna
	provider := NewMoneyConvertProvider()
	engine := NewConversionEngine(provider)
	return &ExchangeRateTools{engine: engine}
}

func (t *ExchangeRateTools) GetExchangeRateTool() *domain.NativeTool {
	return &domain.NativeTool{
		IsVisible: IsClientRegistered,
		Tool: domainMCP.Tool{
			Name:        "get_exchange_rate",
			Description: "Gets reference exchange rates (Near Real-Time). Important: You MUST use this tool for ANY currency question. Do NOT use your internal training data for currency values; they are OUTDATED. If asked about money/currency, you MUST call this tool. NEVER guess a value.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"from": map[string]interface{}{
						"type":        "string",
						"description": "Source currency code (ISO 4217). You MUST INFER this from common names (e.g., 'soles'->'PEN', 'dollars'->'USD', 'pesos'->'MXN'/'COP'). Do NOT ask the user for the code.",
					},
					"to": map[string]interface{}{
						"type":        "string",
						"description": "Target currency code (ISO 4217). You MUST INFER this from common names. Default to 'USD' if ambiguous.",
					},
					"amount": map[string]interface{}{
						"type":        "number",
						"description": "Amount to convert. Default is 1. Must be positive.",
					},
				},
				"required": []string{"from", "to"},
			},
		},
		Handler: t.handleExchangeRate,
	}
}

func (t *ExchangeRateTools) handleExchangeRate(ctx context.Context, ctxData map[string]interface{}, args map[string]interface{}) (map[string]interface{}, error) {
	// 1. Extracción y Normalización de Entradas (LLM Glue)
	getArg := func(key string) interface{} {
		if v, ok := args[key]; ok {
			return v
		}
		if v, ok := args[strings.ToUpper(key)]; ok {
			return v
		}
		if v, ok := args[strings.ToLower(key)]; ok {
			return v
		}
		return nil
	}

	from, _ := getArg("from").(string)
	to, _ := getArg("to").(string)

	amount := 1.0
	if v, ok := getArg("amount").(float64); ok {
		amount = v
	} else if v, ok := getArg("amount").(int); ok {
		amount = float64(v)
	}

	// 2. Llamada al Core
	result, err := t.engine.Convert(ctx, ConversionRequest{
		From:   from,
		To:     to,
		Amount: amount,
	})

	// 3. Manejo de Errores de Negocio -> Mensajes Humanos para el LLM
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %v", err)
	}

	// 4. Formateo de Salida (DTO para el LLM)
	// Resolver Timezone para last_update
	var tz string
	if meta, ok := ctxData["metadata"].(map[string]interface{}); ok {
		if t, ok := meta["bot_timezone"].(string); ok {
			tz = t
		}
	}
	if tz == "" {
		tz = "UTC"
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
		tz = "UTC"
	}
	localTime := result.Timestamp.In(loc)
	timeStr := localTime.Format("2006-01-02 15:04:05") + " (" + tz + ")"

	return map[string]interface{}{
		"from":             from,
		"to":               to,
		"original_amount":  result.FromAmount,
		"converted_amount": result.ToAmount,
		"exchange_rate":    result.Rate,
		"formatted":        fmt.Sprintf("%.2f %s = %.2f %s", result.FromAmount, from, result.ToAmount, to),
		"data_source":      "MoneyConvert (Reference Rates)",
		"last_update":      timeStr,
	}, nil
}

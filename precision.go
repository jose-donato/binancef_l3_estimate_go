package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ExchangeInfo represents Binance exchange info response
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

// SymbolInfo represents information about a trading symbol
type SymbolInfo struct {
	Symbol  string   `json:"symbol"`
	Filters []Filter `json:"filters"`
}

// Filter represents a symbol filter (price, lot size, etc.)
type Filter struct {
	FilterType string `json:"filterType"`
	TickSize   string `json:"tickSize,omitempty"`
	StepSize   string `json:"stepSize,omitempty"`
}

// PrecisionInfo holds precision data for a symbol
type PrecisionInfo struct {
	Symbol        string `json:"symbol"`
	PricePrecision int   `json:"price_precision"`
	QtyPrecision   int   `json:"qty_precision"`
	TickSize       string `json:"tick_size"`
	StepSize       string `json:"step_size"`
	LastUpdated    int64  `json:"last_updated"`
}

// PrecisionManager manages precision information for symbols
type PrecisionManager struct {
	precisions map[string]*PrecisionInfo
	mu         sync.RWMutex
	client     *http.Client
}

// NewPrecisionManager creates a new precision manager
func NewPrecisionManager() *PrecisionManager {
	return &PrecisionManager{
		precisions: make(map[string]*PrecisionInfo),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// calculatePrecision calculates decimal places from a step size string
func calculatePrecision(stepSize string) int {
	if stepSize == "" {
		return 2 // Default precision
	}

	stepFloat, err := strconv.ParseFloat(stepSize, 64)
	if err != nil || stepFloat <= 0 {
		return 2 // Default precision
	}

	if stepFloat >= 1.0 {
		return 0
	}

	// Calculate precision from step size
	precision := int(math.Ceil(-math.Log10(stepFloat)))
	if precision < 0 {
		precision = 0
	}
	if precision > 10 { // Reasonable upper limit
		precision = 10
	}

	return precision
}

// FetchPrecisionInfo fetches precision information for a symbol from Binance
func (pm *PrecisionManager) FetchPrecisionInfo(symbol string) (*PrecisionInfo, error) {
	pm.mu.RLock()
	if info, exists := pm.precisions[symbol]; exists {
		// Check if info is recent (cache for 1 hour)
		if time.Now().Unix()-info.LastUpdated < 3600 {
			pm.mu.RUnlock()
			return info, nil
		}
	}
	pm.mu.RUnlock()

	// Fetch from API
	url := "https://fapi.binance.com/fapi/v1/exchangeInfo"
	resp, err := pm.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("exchange info API returned status %d", resp.StatusCode)
	}

	var exchangeInfo ExchangeInfo
	if err := json.NewDecoder(resp.Body).Decode(&exchangeInfo); err != nil {
		return nil, fmt.Errorf("failed to decode exchange info: %w", err)
	}

	upperSymbol := strings.ToUpper(symbol)
	
	// Find the symbol in the response
	for _, symbolInfo := range exchangeInfo.Symbols {
		if symbolInfo.Symbol == upperSymbol {
			precisionInfo := &PrecisionInfo{
				Symbol:         symbol,
				PricePrecision: 2, // Default
				QtyPrecision:   2, // Default
				TickSize:       "0.01",
				StepSize:       "0.01",
				LastUpdated:    time.Now().Unix(),
			}

			// Parse filters
			for _, filter := range symbolInfo.Filters {
				switch filter.FilterType {
				case "PRICE_FILTER":
					if filter.TickSize != "" {
						precisionInfo.TickSize = filter.TickSize
						precisionInfo.PricePrecision = calculatePrecision(filter.TickSize)
					}
				case "LOT_SIZE":
					if filter.StepSize != "" {
						precisionInfo.StepSize = filter.StepSize
						precisionInfo.QtyPrecision = calculatePrecision(filter.StepSize)
					}
				}
			}

			// Cache the result
			pm.mu.Lock()
			pm.precisions[symbol] = precisionInfo
			pm.mu.Unlock()

			log.Printf("Fetched precision for %s: price=%d, qty=%d, tick=%s, step=%s",
				strings.ToUpper(symbol), 
				precisionInfo.PricePrecision, 
				precisionInfo.QtyPrecision,
				precisionInfo.TickSize,
				precisionInfo.StepSize)

			return precisionInfo, nil
		}
	}

	return nil, fmt.Errorf("symbol %s not found in exchange info", upperSymbol)
}

// GetPrecisionInfo gets cached precision info or fetches it if not available
func (pm *PrecisionManager) GetPrecisionInfo(symbol string) *PrecisionInfo {
	info, err := pm.FetchPrecisionInfo(symbol)
	if err != nil {
		log.Printf("Failed to fetch precision for %s: %v, using defaults", symbol, err)
		return &PrecisionInfo{
			Symbol:         symbol,
			PricePrecision: 2,
			QtyPrecision:   2,
			TickSize:       "0.01",
			StepSize:       "0.01",
			LastUpdated:    time.Now().Unix(),
		}
	}
	return info
}

// FormatPrice formats a price with the correct precision for the symbol
func (pm *PrecisionManager) FormatPrice(symbol string, price float64) string {
	info := pm.GetPrecisionInfo(symbol)
	format := fmt.Sprintf("%%.%df", info.PricePrecision)
	return fmt.Sprintf(format, price)
}

// FormatQuantity formats a quantity with the correct precision for the symbol
func (pm *PrecisionManager) FormatQuantity(symbol string, qty float64) string {
	info := pm.GetPrecisionInfo(symbol)
	format := fmt.Sprintf("%%.%df", info.QtyPrecision)
	return fmt.Sprintf(format, qty)
}

// GetAllPrecisionInfo returns all cached precision info
func (pm *PrecisionManager) GetAllPrecisionInfo() map[string]*PrecisionInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	result := make(map[string]*PrecisionInfo)
	for k, v := range pm.precisions {
		result[k] = v
	}
	return result
}

// ClearCache clears the precision cache
func (pm *PrecisionManager) ClearCache() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.precisions = make(map[string]*PrecisionInfo)
}

// Global precision manager instance
var precisionManager *PrecisionManager

// InitializePrecisionManager initializes the global precision manager
func InitializePrecisionManager() {
	precisionManager = NewPrecisionManager()
}
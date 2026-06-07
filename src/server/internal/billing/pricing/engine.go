package pricing

import (
	"fmt"
	"sync"
)

// UsageDimension 计费维度
type UsageDimension string

const (
	DimInputTokens  UsageDimension = "input_tokens"
	DimOutputTokens UsageDimension = "output_tokens"
	DimImageCount   UsageDimension = "image_count"
	DimAudioSeconds UsageDimension = "audio_seconds"
)

// ModelPricing 模型定价（每单位价格）
type ModelPricing struct {
	ModelID        string
	InputPerToken  float64
	OutputPerToken float64
	ImagePerCount  float64
	AudioPerSecond float64
}

// Engine 动态定价引擎
type Engine struct {
	mu      sync.RWMutex
	prices  map[string]*ModelPricing
}

// NewEngine 创建定价引擎
func NewEngine() *Engine {
	return &Engine{
		prices: make(map[string]*ModelPricing),
	}
}

// NewEngineWithDefaults 创建带默认定价的引擎
func NewEngineWithDefaults() *Engine {
	engine := NewEngine()
	defaults := []*ModelPricing{
		{ModelID: "gpt-4o", InputPerToken: 0.0000025, OutputPerToken: 0.00001, ImagePerCount: 0.004, AudioPerSecond: 0.0001},
		{ModelID: "gpt-4o-mini", InputPerToken: 0.00000015, OutputPerToken: 0.0000006, ImagePerCount: 0.0004, AudioPerSecond: 0.00001},
		{ModelID: "gpt-4-turbo", InputPerToken: 0.00001, OutputPerToken: 0.00003, ImagePerCount: 0.01, AudioPerSecond: 0.0001},
		{ModelID: "gpt-3.5-turbo", InputPerToken: 0.0000005, OutputPerToken: 0.0000015, ImagePerCount: 0, AudioPerSecond: 0},
	}
	for _, p := range defaults {
		engine.prices[p.ModelID] = p
	}
	return engine
}

// SetModelPrice 设置模型价格
func (e *Engine) SetModelPrice(pricing *ModelPricing) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prices[pricing.ModelID] = pricing
}

// GetModelPrice 获取模型定价
func (e *Engine) GetModelPrice(modelID string) (*ModelPricing, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pricing, ok := e.prices[modelID]
	if !ok {
		return nil, fmt.Errorf("pricing: price not found for model=%s", modelID)
	}
	return pricing, nil
}

// CalculateCost 计算费用
func (e *Engine) CalculateCost(modelID string, inputTokens, outputTokens, imageCount int, audioSeconds float64) (float64, error) {
	pricing, err := e.GetModelPrice(modelID)
	if err != nil {
		return 0, err
	}

	var total float64
	if inputTokens > 0 {
		total += pricing.InputPerToken * float64(inputTokens)
	}
	if outputTokens > 0 {
		total += pricing.OutputPerToken * float64(outputTokens)
	}
	if imageCount > 0 {
		total += pricing.ImagePerCount * float64(imageCount)
	}
	if audioSeconds > 0 {
		total += pricing.AudioPerSecond * audioSeconds
	}
	return total, nil
}

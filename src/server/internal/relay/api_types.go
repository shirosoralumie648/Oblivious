package relay

import "oblivious/server/internal/relay/types"

// Re-export types for convenience within relay package
type (
	APIType          = types.APIType
	HandlerStrategy  = types.HandlerStrategy
	UsageDimension   = types.UsageDimension
	Usage            = types.Usage
	ProviderError    = types.ProviderError
	ProviderResponse = types.ProviderResponse
)

const (
	StrategyNative      = types.StrategyNative
	StrategyPassthrough = types.StrategyPassthrough
	StrategyFileProxy   = types.StrategyFileProxy
)

const (
	DimPromptTokens     = types.DimPromptTokens
	DimCompletionTokens = types.DimCompletionTokens
	DimTotalTokens      = types.DimTotalTokens
	DimImageCount       = types.DimImageCount
	DimVideoCount       = types.DimVideoCount
	DimAudioSeconds     = types.DimAudioSeconds
	DimStorageBytes     = types.DimStorageBytes
	DimTrainingTokens   = types.DimTrainingTokens
)

func NewOKResponse(content []byte, usage *Usage) *ProviderResponse {
	return types.NewOKResponse(content, usage)
}

func NewErrorResponse(statusCode int, err *ProviderError) *ProviderResponse {
	return types.NewErrorResponse(statusCode, err)
}

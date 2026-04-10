package relay

import (
	"github.com/pkoukk/tiktoken-go"

	"oblivious/server/internal/relay/types"
)

type TokenEstimator struct {
	encoder *tiktoken.Tiktoken
}

func NewTokenEstimator(model string) *TokenEstimator {
	encodingName := modelToEncoding(model)
	encoder, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		encoder, _ = tiktoken.GetEncoding("cl100k_base")
	}
	return &TokenEstimator{encoder: encoder}
}

func modelToEncoding(model string) string {
	switch model {
	case "gpt-4o", "gpt-4o-mini", "chatgpt-4o-latest":
		return "o200k_base"
	case "gpt-4-turbo", "gpt-4":
		return "o200k_base"
	case "gpt-3.5-turbo", "gpt-3.5-turbo-16k":
		return "cl100k_base"
	case "text-embedding-3-large":
		return "cl100k_base"
	case "text-embedding-3-small":
		return "cl100k_base"
	case "text-embedding-ada-002":
		return "cl100k_base"
	default:
		return "o200k_base"
	}
}

func (e *TokenEstimator) Estimate(text string) int {
	if text == "" {
		return 0
	}
	tokens := e.encoder.Encode(text, nil, nil)
	return len(tokens)
}

func (e *TokenEstimator) EstimateMessages(messages []types.Message) int {
	total := 0
	for _, m := range messages {
		total += e.Estimate(m.Role) + e.Estimate(m.Content)
	}
	return total
}

package relay

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestTokenEstimator_Estimate(t *testing.T) {
	est := NewTokenEstimator("gpt-4o")

	tokens := est.Estimate("Hello, world!")
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
}

func TestTokenEstimator_EstimateEmpty(t *testing.T) {
	est := NewTokenEstimator("gpt-4o")
	tokens := est.Estimate("")
	if tokens != 0 {
		t.Fatalf("expected 0 for empty string, got %d", tokens)
	}
}

func TestTokenEstimator_EstimateLong(t *testing.T) {
	est := NewTokenEstimator("gpt-4o")
	longText := ""
	for i := 0; i < 1000; i++ {
		longText += "Hello world. "
	}
	tokens := est.Estimate(longText)
	if tokens < 100 {
		t.Fatalf("expected substantial tokens for 1000 repetitions, got %d", tokens)
	}
}

func TestTokenEstimator_ModelMapping(t *testing.T) {
	// o200k_base is used by default when model not explicitly mapped
	est := NewTokenEstimator("unknown-model")
	tokens := est.Estimate("test")
	if tokens <= 0 {
		t.Fatalf("expected positive token count even for unknown model, got %d", tokens)
	}
}

func TestTokenEstimator_EstimateMessages(t *testing.T) {
	est := NewTokenEstimator("gpt-4o")
	messages := []types.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	tokens := est.EstimateMessages(messages)
	if tokens < 2 {
		t.Fatalf("expected at least 2 tokens for simple messages, got %d", tokens)
	}
}

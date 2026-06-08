package embedding

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServiceRequiresExplicitBaseURLWithoutInjectedFunction(t *testing.T) {
	service := NewService(Config{})

	if strings.Contains(service.baseURL, "api.openai.com") {
		t.Fatalf("embedding service must not default to a direct provider URL, got %q", service.baseURL)
	}

	_, err := service.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected missing base URL error")
	}
	if !strings.Contains(err.Error(), "base URL is required") {
		t.Fatalf("expected missing base URL error, got %v", err)
	}
}

func TestServiceAllowsInjectedEmbedFunctionWithoutBaseURL(t *testing.T) {
	service := NewService(Config{}, WithEmbedFunc(func(_ context.Context, model, input string) ([]float32, error) {
		if model != defaultModel {
			return nil, errors.New("unexpected model")
		}
		if input != "hello" {
			return nil, errors.New("unexpected input")
		}
		return []float32{0.1, 0.2}, nil
	}))

	embedding, err := service.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(embedding) != 2 || embedding[0] != 0.1 || embedding[1] != 0.2 {
		t.Fatalf("unexpected embedding: %+v", embedding)
	}
}

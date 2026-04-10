package relay

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestRetry_DoesNotRetryOnSuccess(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) (*types.ProviderResponse, error) {
		callCount++
		return &types.ProviderResponse{StatusCode: http.StatusOK}, nil
	}

	resp, err := Retry(fn, 3, "chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetry_RetriesOnFailure(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) (*types.ProviderResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary error")
		}
		return &types.ProviderResponse{StatusCode: http.StatusOK}, nil
	}

	resp, err := Retry(fn, 3, "chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) (*types.ProviderResponse, error) {
		callCount++
		return nil, errors.New("permanent error")
	}

	_, err := Retry(fn, 3, "chat")
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
}

func TestRetry_Retries429(t *testing.T) {
	callCount := 0
	fn := func(ctx context.Context) (*types.ProviderResponse, error) {
		callCount++
		if callCount == 1 {
			return &types.ProviderResponse{StatusCode: http.StatusTooManyRequests}, errors.New("rate limited")
		}
		return &types.ProviderResponse{StatusCode: http.StatusOK}, nil
	}

	resp, err := Retry(fn, 3, "chat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

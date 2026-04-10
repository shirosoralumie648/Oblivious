package relay

import (
	"context"
	"errors"
	"net/http"
	"time"

	"oblivious/server/internal/relay/types"
)

var (
	ErrMaxAttemptsReached = errors.New("max retry attempts reached")
)

var retryableCodes = map[int]bool{
	http.StatusTooManyRequests:       true,
	http.StatusBadGateway:             true,
	http.StatusServiceUnavailable:     true,
	http.StatusGatewayTimeout:         true,
}

func Retry(fn func(ctx context.Context) (*types.ProviderResponse, error), maxAttempts int, apiType string) (*types.ProviderResponse, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := fn(context.Background())
		if err == nil && resp != nil {
			if !retryableCodes[resp.StatusCode] {
				return resp, nil
			}
			lastErr = errors.New("retryable error")
		} else if err != nil {
			lastErr = err
		}

		if attempt < maxAttempts {
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			time.Sleep(backoff)
		}
	}
	return nil, lastErr
}

func IsRetryable(statusCode int) bool {
	return retryableCodes[statusCode]
}

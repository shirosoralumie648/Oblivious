package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type HealthCheckStrategy string

const (
	HealthCheckModelsAPI HealthCheckStrategy = "models_api"
	HealthCheckRealtime  HealthCheckStrategy = "realtime_probe"
	HealthCheckDisabled  HealthCheckStrategy = "disabled"
)

type HealthChecker struct {
	strategy   HealthCheckStrategy
	timeout    time.Duration
	httpClient *http.Client
}

func NewHealthChecker(strategy HealthCheckStrategy, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		strategy: strategy,
		timeout:  timeout,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				MaxIdleConns:      1,
			},
		},
	}
}

func (hc *HealthChecker) Check(ctx context.Context, baseURL, apiKey string) (bool, time.Duration) {
	if hc.strategy == HealthCheckDisabled {
		return true, 0
	}

	if hc.strategy == HealthCheckModelsAPI {
		return hc.checkModelsAPI(ctx, baseURL, apiKey)
	}

	if hc.strategy == HealthCheckRealtime {
		return hc.checkRealtimeProbe(ctx, baseURL, apiKey)
	}

	return false, 0
}

func (hc *HealthChecker) checkModelsAPI(ctx context.Context, baseURL, apiKey string) (bool, time.Duration) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return false, 0
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := hc.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return false, latency
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, latency
}

func (hc *HealthChecker) checkRealtimeProbe(ctx context.Context, baseURL, apiKey string) (bool, time.Duration) {
	body := map[string]any{
		"model":      "gpt-4o-mini",
		"max_tokens": 5,
		"messages": []map[string]string{{
			"role":    "user",
			"content": "hi",
		}},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return false, 0
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return false, 0
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := hc.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return false, latency
	}
	defer resp.Body.Close()

	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices, latency
}

func (hc *HealthChecker) RecordProbeResult(cb *CircuitBreaker, healthy bool) {
	if healthy {
		cb.RecordSuccess()
	} else {
		cb.RecordFailure()
	}
}

func (hc *HealthChecker) Strategy() HealthCheckStrategy {
	return hc.strategy
}

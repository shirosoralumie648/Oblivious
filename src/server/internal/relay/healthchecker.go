package relay

import (
	"context"
	"net/http"
	"time"
)

type HealthCheckStrategy string

const (
	HealthCheckModelsAPI  HealthCheckStrategy = "models_api"
	HealthCheckRealtime   HealthCheckStrategy = "realtime_probe"
	HealthCheckDisabled   HealthCheckStrategy = "disabled"
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
				MaxIdleConns:       1,
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

	return true, 0
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

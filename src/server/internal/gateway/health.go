package gateway

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// HealthAggregator checks the health of downstream services and aggregates
// their statuses into a single response.
type HealthAggregator struct {
	targets map[ServiceTarget]string
	timeout time.Duration
}

// NewHealthAggregator creates a HealthAggregator. targets maps service names
// to their health check URLs (e.g., "http://localhost:8081/healthz").
func NewHealthAggregator(targets map[ServiceTarget]string, timeout time.Duration) *HealthAggregator {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HealthAggregator{
		targets: targets,
		timeout: timeout,
	}
}

// Check probes all registered downstream services and returns an aggregated
// health report. If any service is unhealthy, the overall status is "degraded".
func (h *HealthAggregator) Check(ctx context.Context) AggregatedHealth {
	if len(h.targets) == 0 {
		return AggregatedHealth{
			Status:   "ok",
			Services: []HealthStatus{},
		}
	}

	statuses := make([]HealthStatus, 0, len(h.targets))
	var mu sync.Mutex
	var wg sync.WaitGroup

	checkCtx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	for target, url := range h.targets {
		wg.Add(1)
		go func(target ServiceTarget, url string) {
			defer wg.Done()
			status := h.checkOne(checkCtx, target, url)
			mu.Lock()
			statuses = append(statuses, status)
			mu.Unlock()
		}(target, url)
	}

	wg.Wait()

	overallStatus := "ok"
	for _, s := range statuses {
		if s.Status != "ok" {
			overallStatus = "degraded"
			break
		}
	}

	return AggregatedHealth{
		Status:   overallStatus,
		Services: statuses,
	}
}

// checkOne performs a health check against a single downstream service.
func (h *HealthAggregator) checkOne(ctx context.Context, target ServiceTarget, url string) HealthStatus {
	startedAt := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthStatus{
			Service: target,
			Status:  "unreachable",
			Error:   err.Error(),
		}
	}

	client := &http.Client{Timeout: h.timeout}
	resp, err := client.Do(req)
	latency := time.Since(startedAt)

	if err != nil {
		return HealthStatus{
			Service: target,
			Status:  "unhealthy",
			Latency: latency.Truncate(time.Millisecond).String(),
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return HealthStatus{
			Service: target,
			Status:  "ok",
			Latency: latency.Truncate(time.Millisecond).String(),
		}
	}

	return HealthStatus{
		Service: target,
		Status:  "unhealthy",
		Latency: latency.Truncate(time.Millisecond).String(),
		Error:   "unexpected status code: " + resp.Status,
	}
}

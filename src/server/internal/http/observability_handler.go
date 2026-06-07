package http

import (
	stdhttp "net/http"

	"oblivious/server/internal/metrics"
	"oblivious/server/internal/relay"
)

type observabilityHandler struct {
	pool *relay.ChannelPool
}

func newObservabilityHandler(pool *relay.ChannelPool) observabilityHandler {
	return observabilityHandler{pool: pool}
}

func (h observabilityHandler) getLogs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"logs":    []any{},
		"message": "log retrieval not yet implemented",
	})
}

func (h observabilityHandler) getMetrics(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	channels := h.pool.ListChannels()
	channelMetrics := make([]map[string]any, 0, len(channels))
	for _, ch := range channels {
		stats, _ := h.pool.GetStats(ch.ID)
		channelMetrics = append(channelMetrics, map[string]any{
			"channelId": ch.ID,
			"name":      ch.Name,
			"enabled":   ch.Enabled,
			"stats":     stats,
		})
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"channels": channelMetrics,
	})
}

func (h observabilityHandler) getChannelMetrics(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	ch, found := h.pool.GetChannel(channelID)
	if !found {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return
	}

	stats, _ := h.pool.GetStats(channelID)
	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"channelId": ch.ID,
		"name":      ch.Name,
		"enabled":   ch.Enabled,
		"stats":     stats,
	})
}

func (h observabilityHandler) listAlerts(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	// Collect alerts from unhealthy channels
	channels := h.pool.ListChannels()
	alerts := make([]map[string]any, 0)
	for _, ch := range channels {
		stats, found := h.pool.GetStats(ch.ID)
		if !found {
			continue
		}
		if stats.CBState == "open" {
			alerts = append(alerts, map[string]any{
				"channelId": ch.ID,
				"type":      "circuit_breaker_open",
				"message":   "channel " + ch.Name + " circuit breaker is open",
				"failures":  stats.CBFailures,
			})
		}
		if stats.FailureCount > 0 && float64(stats.FailureCount)/float64(stats.TotalRequests) > 0.5 {
			alerts = append(alerts, map[string]any{
				"channelId": ch.ID,
				"type":      "high_failure_rate",
				"message":   "channel " + ch.Name + " has high failure rate",
				"failures":  stats.FailureCount,
				"total":     stats.TotalRequests,
			})
		}
	}

	writeSuccess(w, stdhttp.StatusOK, alerts)
}

func (h observabilityHandler) getAlertConfig(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"failureRateThreshold": 0.5,
		"circuitBreakerAlert":  true,
		"rateLimitAlert":       true,
	})
}

func (h observabilityHandler) getDashboard(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	channels := h.pool.ListChannels()
	stats := h.pool.GetAllStats()

	totalRequests := int64(0)
	totalSuccess := int64(0)
	totalFailure := int64(0)
	for _, s := range stats {
		totalRequests += s.TotalRequests
		totalSuccess += s.SuccessCount
		totalFailure += s.FailureCount
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"channels_total":  len(channels),
		"total_requests":  totalRequests,
		"total_success":   totalSuccess,
		"total_failure":   totalFailure,
		"channel_stats":   stats,
	})
}

// RecordRequest is a helper that delegates to the metrics package.
func (h observabilityHandler) RecordRequest(channelID, model, apiType, status string) {
	metrics.RecordRequest(channelID, model, apiType, status)
}

package http

import (
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/relay"
)

type gatewayHandler struct {
	pool *relay.ChannelPool
}

func newGatewayHandler(pool *relay.ChannelPool) gatewayHandler {
	return gatewayHandler{pool: pool}
}

func (h gatewayHandler) health(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	channels := h.pool.ListChannels()
	healthy := 0
	for _, ch := range channels {
		if ch.Enabled {
			healthy++
		}
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"status":         "ok",
		"channels_total": len(channels),
		"channels_active": healthy,
	})
}

func (h gatewayHandler) listChannels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	channels := h.pool.ListChannels()
	writeSuccess(w, stdhttp.StatusOK, channels)
}

func (h gatewayHandler) getChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
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

	writeSuccess(w, stdhttp.StatusOK, ch)
}

func (h gatewayHandler) getChannelStats(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	stats, found := h.pool.GetStats(channelID)
	if !found {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel stats not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, stats)
}

func (h gatewayHandler) getAllStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	stats := h.pool.GetAllStats()
	writeSuccess(w, stdhttp.StatusOK, stats)
}

func (h gatewayHandler) getRoutes(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	channels := h.pool.ListChannels()
	routes := make(map[string][]string)
	for _, ch := range channels {
		for _, model := range ch.Models {
			routes[model] = append(routes[model], ch.ID)
		}
	}

	writeSuccess(w, stdhttp.StatusOK, routes)
}

func (h gatewayHandler) proxyChat(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/gateway/proxy/")
	if trimmedPath == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "proxy path is required")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"status":  "accepted",
		"message": "proxy request forwarded",
		"path":    trimmedPath,
	})
}

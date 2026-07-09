package http

import (
	stdhttp "net/http"
	"os"
	"strings"

	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/types"
)

type gatewayHandler struct {
	pool         *relay.ChannelPool
	relayHandler stdhttp.Handler
}

func newGatewayHandler(pool *relay.ChannelPool, relayHandler stdhttp.Handler) gatewayHandler {
	return gatewayHandler{pool: pool, relayHandler: relayHandler}
}

func (h gatewayHandler) health(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	channels := h.listChannelsSafe()
	healthy := 0
	for _, ch := range channels {
		if ch.Enabled {
			healthy++
		}
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"status":          "ok",
		"channels_total":  len(channels),
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

	channels := h.listChannelsSafe()
	writeSuccess(w, stdhttp.StatusOK, channels)
}

func (h gatewayHandler) getChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = session

	if h.pool == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "relay_unavailable", "relay is not configured")
		return
	}

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

	if h.pool == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "relay_unavailable", "relay is not configured")
		return
	}

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

	if h.pool == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "relay_unavailable", "relay is not configured")
		return
	}

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

	channels := h.listChannelsSafe()
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
	if h.relayHandler == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "relay_unavailable", "relay proxy is not configured")
		return
	}

	proxyRequest := r.Clone(r.Context())
	proxyRequest.URL.Path = "/v1/" + trimmedPath
	proxyRequest.RequestURI = ""
	proxyRequest.Header = r.Header.Clone()
	proxyRequest.Header.Del("Authorization")
	proxyRequest.Header.Set(types.HeaderInternalAuth, gatewayInternalAuthToken())
	proxyRequest.Header.Set(types.HeaderInternalUserID, session.User.ID)
	proxyRequest.Header.Set(types.HeaderInternalOrganization, session.OrganizationID)
	proxyRequest.Header.Set(types.HeaderInternalFeatureType, "gateway_proxy")
	if requestID := requestIDFromContext(r.Context()); requestID != "" {
		proxyRequest.Header.Set(types.HeaderRequestID, requestID)
	}

	h.relayHandler.ServeHTTP(w, proxyRequest)
}

func (h gatewayHandler) listChannelsSafe() []*types.Channel {
	if h.pool == nil {
		return nil
	}
	return h.pool.ListChannels()
}

func gatewayInternalAuthToken() string {
	if token := strings.TrimSpace(os.Getenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN")); token != "" {
		return token
	}
	return types.SharedInternalToken
}

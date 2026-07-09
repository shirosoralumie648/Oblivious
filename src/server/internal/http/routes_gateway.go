package http

import (
	stdhttp "net/http"
	"strings"
)

func registerGatewayRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, gatewayHandler gatewayHandler) {
	mux.Handle("/api/v1/gateway/proxy/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if strings.TrimPrefix(r.URL.Path, "/api/v1/gateway/proxy/") == "" {
			gatewayHandler.proxyChat(w, r)
			return
		}
		switch r.Method {
		case stdhttp.MethodGet, stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodPatch, stdhttp.MethodDelete:
			gatewayHandler.proxyChat(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
}

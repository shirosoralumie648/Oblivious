package http

import (
	stdhttp "net/http"
	"strings"
)

func registerObservabilityAlertRoutes(mux *stdhttp.ServeMux, authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler observabilityAlertHandler) {
	mux.Handle("/api/v1/admin/observability/alert-routing", newObservabilityAlertRouter(authMiddleware, handler))
	mux.Handle("/api/v1/admin/observability/alert-providers", newObservabilityAlertRouter(authMiddleware, handler))
	mux.Handle("/api/v1/admin/observability/alert-providers/", newObservabilityAlertRouter(authMiddleware, handler))
	mux.Handle("/api/v1/admin/observability/recovery-actions", newObservabilityAlertRouter(authMiddleware, handler))
	mux.Handle("/api/v1/admin/observability/alerts", newObservabilityAlertRouter(authMiddleware, handler))
	mux.Handle("/api/v1/admin/observability/alerts/", newObservabilityAlertRouter(authMiddleware, handler))
}

func newObservabilityAlertRouter(authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler observabilityAlertHandler) stdhttp.Handler {
	return authMiddleware.requireAdmin(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/admin/observability/alert-routing" {
			switch r.Method {
			case stdhttp.MethodGet:
				handler.getAlertRoutingRules(w, r)
			case stdhttp.MethodPut:
				handler.updateAlertRoutingRules(w, r)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if r.URL.Path == "/api/v1/admin/observability/alert-providers" {
			switch r.Method {
			case stdhttp.MethodGet:
				handler.listAlertProviderConfigs(w, r)
			case stdhttp.MethodPost:
				handler.createAlertProviderConfig(w, r)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/observability/alert-providers/") {
			trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/observability/alert-providers"), "/")
			parts := strings.Split(trimmedPath, "/")
			if len(parts) == 1 && parts[0] != "" {
				if r.Method == stdhttp.MethodPut {
					handler.updateAlertProviderConfig(w, r, parts[0])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
				return
			}
			if len(parts) == 2 && parts[0] != "" && parts[1] == "test" {
				if r.Method == stdhttp.MethodPost {
					handler.testAlertProviderConfig(w, r, parts[0])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
				return
			}
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		if r.URL.Path == "/api/v1/admin/observability/recovery-actions" {
			if r.Method == stdhttp.MethodGet {
				handler.listRecoveryActions(w, r)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/observability/alerts"), "/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			if r.Method == stdhttp.MethodGet {
				handler.listAlerts(w, r)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		alertKey := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				handler.getAlert(w, r, alertKey)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 {
			switch parts[1] {
			case "deliveries":
				if r.Method == stdhttp.MethodGet {
					handler.listDeliveryAttempts(w, r, alertKey)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "acknowledge":
				if r.Method == stdhttp.MethodPost {
					handler.acknowledgeAlert(w, r, alertKey)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "resolve":
				if r.Method == stdhttp.MethodPost {
					handler.resolveAlert(w, r, alertKey)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	}))
}

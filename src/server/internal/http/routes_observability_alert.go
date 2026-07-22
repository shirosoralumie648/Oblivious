package http

import (
	stdhttp "net/http"
	"strings"
)

func observabilityAlertRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/admin/observability/alert-providers", "listAdminObservabilityAlertProviders", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:6e412ce69dfa596b31212d5c0ec446f42d3ef970f3448061ca2a03a0d3173a79"},
		{"POST", "/api/v1/admin/observability/alert-providers", "createAdminObservabilityAlertProvider", "cookie+csrf", true, "observability.audit", "application/json", "ref", "#/components/schemas/AdminObservabilityAlertProviderRequest", "201", "application/json", "inline", "sha256:1c2ffe6f7a00b3f000b03aaebf5664a6b5de2045c16afc4f550550bea6b49fe7"},
		{"PUT", "/api/v1/admin/observability/alert-providers/{providerId}", "updateAdminObservabilityAlertProvider", "cookie+csrf", true, "observability.audit", "application/json", "ref", "#/components/schemas/AdminObservabilityAlertProviderRequest", "200", "application/json", "inline", "sha256:1c2ffe6f7a00b3f000b03aaebf5664a6b5de2045c16afc4f550550bea6b49fe7"},
		{"POST", "/api/v1/admin/observability/alert-providers/{providerId}/test", "testAdminObservabilityAlertProvider", "cookie+csrf", true, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:d15f02ddbd4d3f7242c6d90ce689ef4ebca5858a241ba0901dd60a0d31d6e200"},
		{"GET", "/api/v1/admin/observability/alert-routing", "getAdminObservabilityAlertRouting", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:9ca6150bc630913afcad4c6f0c8e1799275afdfb9a8a204aa10b2ddb0426f7c0"},
		{"PUT", "/api/v1/admin/observability/alert-routing", "updateAdminObservabilityAlertRouting", "cookie+csrf", true, "observability.audit", "application/json", "ref", "#/components/schemas/UpdateAdminObservabilityAlertRoutingRulesRequest", "200", "application/json", "inline", "sha256:9ca6150bc630913afcad4c6f0c8e1799275afdfb9a8a204aa10b2ddb0426f7c0"},
		{"GET", "/api/v1/admin/observability/alerts", "listAdminObservabilityAlerts", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:04237b318bd3aeecf6890227cb3cd21120627267f8d775c8d7f15e6017b5780c"},
		{"GET", "/api/v1/admin/observability/alerts/{alertKey}", "getAdminObservabilityAlert", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:3aa36f0cb385fabb4e6fbb645facd8f3dfdd1dbfc3224781a91eb7fa52d760f2"},
		{"POST", "/api/v1/admin/observability/alerts/{alertKey}/acknowledge", "acknowledgeAdminObservabilityAlert", "cookie+csrf", true, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:3aa36f0cb385fabb4e6fbb645facd8f3dfdd1dbfc3224781a91eb7fa52d760f2"},
		{"GET", "/api/v1/admin/observability/alerts/{alertKey}/deliveries", "listAdminObservabilityAlertDeliveries", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:c63888c3a3a24576af8a617efcb25427c3ac4d74818a360c2ad8e9b8d0cf64ee"},
		{"POST", "/api/v1/admin/observability/alerts/{alertKey}/resolve", "resolveAdminObservabilityAlert", "cookie+csrf", true, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:3aa36f0cb385fabb4e6fbb645facd8f3dfdd1dbfc3224781a91eb7fa52d760f2"},
		{"GET", "/api/v1/admin/observability/latency-slo-proof", "getAdminObservabilityLatencySLOProof", "cookie", false, "observability.slo", "", "none", "", "200", "application/json", "inline", "sha256:0142f248d6992fb78b8d5e03994727fd4286f0b4c7f69d9eb857921df869fe5d"},
		{"GET", "/api/v1/admin/observability/recovery-actions", "listAdminObservabilityRecoveryActions", "cookie", false, "observability.audit", "", "none", "", "200", "application/json", "inline", "sha256:639610c20e90b0aeadaf663f376bbec4fc51cdb87163045501b8a9fea7494de3"},
	})
}

func observabilityAlertRouteHandler(handler observabilityAlertHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/admin/observability/alert-routing" {
			if r.Method == stdhttp.MethodGet {
				handler.getAlertRoutingRules(w, r)
			} else {
				handler.updateAlertRoutingRules(w, r)
			}
			return
		}
		if r.URL.Path == "/api/v1/admin/observability/alert-providers" {
			if r.Method == stdhttp.MethodGet {
				handler.listAlertProviderConfigs(w, r)
			} else {
				handler.createAlertProviderConfig(w, r)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/observability/alert-providers/") {
			parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/observability/alert-providers"), "/"), "/")
			if len(parts) == 1 && parts[0] != "" {
				handler.updateAlertProviderConfig(w, r, parts[0])
				return
			}
			if len(parts) == 2 && parts[0] != "" && parts[1] == "test" {
				handler.testAlertProviderConfig(w, r, parts[0])
				return
			}
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		if r.URL.Path == "/api/v1/admin/observability/recovery-actions" {
			handler.listRecoveryActions(w, r)
			return
		}
		if r.URL.Path == "/api/v1/admin/observability/latency-slo-proof" {
			handler.getLatencySLOProof(w, r)
			return
		}

		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/observability/alerts"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			handler.listAlerts(w, r)
			return
		}
		alertKey := parts[0]
		if len(parts) == 1 {
			handler.getAlert(w, r, alertKey)
			return
		}
		if len(parts) != 2 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "deliveries":
			handler.listDeliveryAttempts(w, r, alertKey)
		case "acknowledge":
			handler.acknowledgeAlert(w, r, alertKey)
		case "resolve":
			handler.resolveAlert(w, r, alertKey)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerObservabilityAlertRouteSurfaces(registrar *RouteSurfaceRegistrar, handler observabilityAlertHandler) error {
	operations := observabilityAlertRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthAdmin, observabilityAlertRouteHandler(handler)))
}

func registerObservabilityAlertRoutes(mux *stdhttp.ServeMux, authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler observabilityAlertHandler) {
	if err := registerObservabilityAlertRouteSurfaces(mustRouteSurfaceAdminAdapterRegistrar(mux, authMiddleware), handler); err != nil {
		panic(err)
	}
}

func newObservabilityAlertRouter(authMiddleware interface {
	requireAdmin(stdhttp.Handler) stdhttp.Handler
}, handler observabilityAlertHandler) stdhttp.Handler {
	return authMiddleware.requireAdmin(observabilityAlertRouteHandler(handler))
}

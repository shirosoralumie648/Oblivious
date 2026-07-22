package http

import (
	stdhttp "net/http"
	"strings"
)

func consoleRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/console/access", "getAccess", "cookie", false, "billing.payment_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:7bca0203d52e920cdb20af3a3cffd18de1b67614c3d810ae2412ba77df0278ba"},
		{"GET", "/api/v1/console/api-tokens", "listConsoleAPITokens", "cookie", false, "billing.payment_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:f5efe0ea6efd1121ea71546b248a953730fa6dc39e9c8fd7463671197d8a8c3b"},
		{"POST", "/api/v1/console/api-tokens", "createConsoleAPIToken", "cookie+csrf", true, "billing.payment_lifecycle", "application/json", "ref", "#/components/schemas/CreateRelayAPITokenRequest", "201", "application/json", "inline", "sha256:abd3a5393dac91fb59e8fca29ce95ad44c2c90449394dc7dc660608ae242b45c"},
		{"DELETE", "/api/v1/console/api-tokens/{tokenId}", "revokeConsoleAPIToken", "cookie+csrf", true, "billing.payment_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:5b0791615289a039978295bbddc67def85ec0a11530a3202d29f67653135995c"},
		{"GET", "/api/v1/console/api-tokens/{tokenId}/usage", "listConsoleAPITokenUsage", "cookie", false, "billing.ledger_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:1a05cdbeb19e2a80c0cf01921b9e17bf61d0e3e468a621699eaeef7917ffbcac"},
		{"GET", "/api/v1/console/billing", "getBilling", "cookie", false, "billing.payment_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:80297f2cb7e77b14f9cc5b3a24ab1e9a502182c8436d071a0834ac321422c4ff"},
		{"GET", "/api/v1/console/invoices", "listConsoleBillingInvoices", "cookie", false, "billing.ledger_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:f545ca2d5692769acbb9d7066d42dad0102d4cebf07031eb1ed19a95bc73d646"},
		{"GET", "/api/v1/console/models", "getConsoleModels", "cookie", false, "billing.payment_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:dfe83a6f41021b65379e6eecf0c74b63315b46a8202959a7a07ba51e6de91337"},
		{"GET", "/api/v1/console/usage", "getUsage", "cookie", false, "billing.ledger_lifecycle", "", "none", "", "200", "application/json", "inline", "sha256:800312895564e6b11d6e3f22b60ef34601f52e31dc3c71592435b4a2a170d049"},
	})
}

func consoleRouteHandler(consoleHandler consoleHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/api/v1/console/usage":
			consoleHandler.getUsage(w, r)
		case "/api/v1/console/access":
			consoleHandler.getAccess(w, r)
		case "/api/v1/console/models":
			consoleHandler.getModels(w, r)
		case "/api/v1/console/billing":
			consoleHandler.getBilling(w, r)
		case "/api/v1/console/invoices":
			consoleHandler.listBillingInvoices(w, r)
		case "/api/v1/console/api-tokens":
			if r.Method == stdhttp.MethodGet {
				consoleHandler.listAPITokens(w, r)
			} else {
				consoleHandler.createAPIToken(w, r)
			}
		default:
			trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/console/api-tokens/"), "/")
			if trimmedPath == "" {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			if strings.HasSuffix(trimmedPath, "/usage") {
				consoleHandler.listAPITokenUsage(w, r)
				return
			}
			consoleHandler.revokeAPIToken(w, r)
		}
	})
}

func registerConsoleRouteSurfaces(registrar *RouteSurfaceRegistrar, consoleHandler consoleHandler) error {
	operations := consoleRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, consoleRouteHandler(consoleHandler)))
}

func registerConsoleRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, consoleHandler consoleHandler) {
	if err := registerConsoleRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), consoleHandler); err != nil {
		panic(err)
	}
}

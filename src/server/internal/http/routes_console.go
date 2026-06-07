package http

import (
	stdhttp "net/http"
	"strings"
)

func registerConsoleRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, consoleHandler consoleHandler) {
	mux.Handle("/api/v1/console/usage", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		consoleHandler.getUsage(w, r)
	})))
	mux.Handle("/api/v1/console/access", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		consoleHandler.getAccess(w, r)
	})))
	mux.Handle("/api/v1/console/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		consoleHandler.getModels(w, r)
	})))
	mux.Handle("/api/v1/console/billing", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		consoleHandler.getBilling(w, r)
	})))
	mux.Handle("/api/v1/console/invoices", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		consoleHandler.listBillingInvoices(w, r)
	})))
	mux.Handle("/api/v1/console/api-tokens", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			consoleHandler.listAPITokens(w, r)
		case stdhttp.MethodPost:
			consoleHandler.createAPIToken(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/console/api-tokens/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/console/api-tokens/"), "/")
		if trimmedPath == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		if strings.HasSuffix(trimmedPath, "/usage") {
			if r.Method == stdhttp.MethodGet {
				consoleHandler.listAPITokenUsage(w, r)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if r.Method == stdhttp.MethodDelete {
			consoleHandler.revokeAPIToken(w, r)
			return
		}
		writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})))
}

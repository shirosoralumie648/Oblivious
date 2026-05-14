package http

import stdhttp "net/http"

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
}

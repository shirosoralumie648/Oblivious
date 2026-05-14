package http

import stdhttp "net/http"

func registerPreferenceRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, preferencesHandler preferencesHandler) {
	mux.Handle("/api/v1/app/me/preferences", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			preferencesHandler.get(w, r)
		case stdhttp.MethodPut:
			preferencesHandler.update(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
}

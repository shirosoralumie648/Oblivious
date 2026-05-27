package http

import stdhttp "net/http"

func registerAuthRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, authHandler authHandler) {
	mux.HandleFunc("/api/v1/auth/login", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.login(w, r)
	})
	mux.HandleFunc("/api/v1/auth/register", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.register(w, r)
	})
	mux.HandleFunc("/api/v1/auth/password-reset/request", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.requestPasswordReset(w, r)
	})
	mux.HandleFunc("/api/v1/auth/password-reset/confirm", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.confirmPasswordReset(w, r)
	})
	mux.Handle("/api/v1/auth/me", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.me(w, r)
	})))
	mux.Handle("/api/v1/auth/logout", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		authHandler.logout(w, r)
	})))
}

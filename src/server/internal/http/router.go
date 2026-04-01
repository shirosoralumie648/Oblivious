package http

import stdhttp "net/http"

func NewRouter() stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeJSON(w, stdhttp.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/v1/auth/login", placeholderHandler(stdhttp.MethodPost, "login"))
	mux.HandleFunc("/api/v1/auth/register", placeholderHandler(stdhttp.MethodPost, "register"))
	mux.HandleFunc("/api/v1/auth/me", placeholderHandler(stdhttp.MethodGet, "me"))
	mux.HandleFunc("/api/v1/auth/logout", placeholderHandler(stdhttp.MethodPost, "logout"))

	return applyMiddleware(mux, withRecover, withRequestID, withLogging)
}

func placeholderHandler(method, route string) stdhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != method {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		writeSuccess(w, stdhttp.StatusOK, map[string]any{
			"placeholder": true,
			"route":       route,
		})
	}
}

package http

import (
	"encoding/json"
	stdhttp "net/http"
)

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	OK    bool          `json:"ok"`
	Data  any           `json:"data"`
	Error *ErrorPayload `json:"error"`
}

func writeJSON(w stdhttp.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSuccess(w stdhttp.ResponseWriter, status int, data any) {
	writeJSON(w, status, Envelope{
		OK:    true,
		Data:  data,
		Error: nil,
	})
}

func writeError(w stdhttp.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Envelope{
		OK:   false,
		Data: nil,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

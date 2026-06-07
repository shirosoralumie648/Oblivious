package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serverrelay "oblivious/server/internal/relay"
)

func TestCombineHandlersRelayAliasesRouteToOpenAICompatiblePaths(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		aliasPath  string
		targetPath string
		body       string
	}{
		{
			name:       "chat completions",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/chat/completions?trace=1",
			targetPath: "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[]}`,
		},
		{
			name:       "embeddings",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/embeddings",
			targetPath: "/v1/embeddings",
			body:       `{"model":"text-embedding-3-small","input":"ping"}`,
		},
		{
			name:       "responses",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/responses",
			targetPath: "/v1/responses",
			body:       `{"model":"gpt-4o","input":"ping"}`,
		},
		{
			name:       "image generations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/generations",
			targetPath: "/v1/images/generations",
			body:       `{"model":"gpt-image-1","prompt":"ping"}`,
		},
		{
			name:       "image edits",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/edits",
			targetPath: "/v1/images/edits",
			body:       `image-edit-bytes`,
		},
		{
			name:       "image variations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/variations",
			targetPath: "/v1/images/variations",
			body:       `image-variation-bytes`,
		},
		{
			name:       "audio speech",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/speech",
			targetPath: "/v1/audio/speech",
			body:       `{"model":"tts-1","input":"ping","voice":"alloy"}`,
		},
		{
			name:       "audio transcriptions",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/transcriptions",
			targetPath: "/v1/audio/transcriptions",
			body:       `audio-bytes`,
		},
		{
			name:       "audio translations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/translations",
			targetPath: "/v1/audio/translations",
			body:       `audio-bytes`,
		},
		{
			name:       "models",
			method:     stdhttp.MethodGet,
			aliasPath:  "/api/v1/relay/models?limit=20",
			targetPath: "/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relayCalled := false
			main := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				t.Fatalf("alias route reached main handler as %s %s", r.Method, r.URL.String())
			})
			relay := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				relayCalled = true
				if r.Method != tt.method {
					t.Fatalf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.targetPath {
					t.Fatalf("relay path = %q, want %q", r.URL.Path, tt.targetPath)
				}
				if r.URL.RawQuery != httptest.NewRequest(tt.method, tt.aliasPath, nil).URL.RawQuery {
					t.Fatalf("relay query = %q, want query from %q", r.URL.RawQuery, tt.aliasPath)
				}
				if r.Header.Get("X-Relay-Alias-Test") != "preserved" {
					t.Fatalf("expected relay alias request header to be preserved")
				}
				w.WriteHeader(stdhttp.StatusAccepted)
			})
			handler := combineHandlers(main, relay)
			request := httptest.NewRequest(tt.method, tt.aliasPath, strings.NewReader(tt.body))
			request.Header.Set("X-Relay-Alias-Test", "preserved")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusAccepted {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, stdhttp.StatusAccepted, recorder.Body.String())
			}
			if !relayCalled {
				t.Fatal("expected relay handler to be called")
			}
		})
	}
}

func TestCombineHandlersRelayAliasesReachProductionRelayPolicy(t *testing.T) {
	relayInstance, err := serverrelay.NewRelay(&serverrelay.Config{Production: true})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	handler := combineHandlers(stdhttp.NotFoundHandler(), relayInstance.Engine())

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "chat completions", method: stdhttp.MethodPost, path: "/api/v1/relay/chat/completions", body: `{"model":"gpt-4o","messages":[]}`},
		{name: "embeddings", method: stdhttp.MethodPost, path: "/api/v1/relay/embeddings", body: `{"model":"text-embedding-3-small","input":"ping"}`},
		{name: "responses", method: stdhttp.MethodPost, path: "/api/v1/relay/responses", body: `{"model":"gpt-4o","input":"ping"}`},
		{name: "image generations", method: stdhttp.MethodPost, path: "/api/v1/relay/images/generations", body: `{"model":"gpt-image-1","prompt":"ping"}`},
		{name: "image edits", method: stdhttp.MethodPost, path: "/api/v1/relay/images/edits", body: `image-edit-bytes`},
		{name: "image variations", method: stdhttp.MethodPost, path: "/api/v1/relay/images/variations", body: `image-variation-bytes`},
		{name: "audio speech", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/speech", body: `{"model":"tts-1","input":"ping","voice":"alloy"}`},
		{name: "audio transcriptions", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/transcriptions", body: `audio-bytes`},
		{name: "audio translations", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/translations", body: `audio-bytes`},
		{name: "models", method: stdhttp.MethodGet, path: "/api/v1/relay/models"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if strings.HasPrefix(strings.TrimSpace(tt.body), "{") {
				request.Header.Set("Content-Type", "application/json")
			}

			handler.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("status = %d, want %d from relay policy; body=%s", recorder.Code, stdhttp.StatusUnauthorized, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "relay_identity_required") {
				t.Fatalf("expected relay policy response, got %s", recorder.Body.String())
			}
		})
	}
}

func TestCombineHandlersDoesNotBroadenRelayAliasesToUnsupportedSurfaces(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{method: stdhttp.MethodPost, path: "/api/v1/relay/files"},
		{method: stdhttp.MethodPost, path: "/api/v1/relay/fine_tuning/jobs"},
		{method: stdhttp.MethodPost, path: "/api/v1/relay/assistants"},
		{method: stdhttp.MethodGet, path: "/api/v1/relay/realtime"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			mainCalled := false
			main := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				mainCalled = true
				w.WriteHeader(stdhttp.StatusNotFound)
			})
			relay := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				t.Fatalf("unsupported alias was routed to relay as %s %s", r.Method, r.URL.String())
			})
			handler := combineHandlers(main, relay)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			handler.ServeHTTP(recorder, request)

			if !mainCalled {
				t.Fatal("expected unsupported alias to remain with main handler")
			}
			if recorder.Code != stdhttp.StatusNotFound {
				t.Fatalf("status = %d, want %d", recorder.Code, stdhttp.StatusNotFound)
			}
		})
	}
}

package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

func TestProductionDisabledRoutesFailClosedBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &countingHandler{}
	threadsHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles:   filesHandler,
		types.APITypeThreads: threadsHandler,
	}, RouteRegistrationOptions{Production: true})

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler *countingHandler
	}{
		{name: "file upload", method: http.MethodPost, path: "/v1/files", handler: filesHandler},
		{name: "thread create", method: http.MethodPost, path: "/v1/threads", handler: threadsHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "endpoint_disabled_in_production") {
				t.Fatalf("expected endpoint_disabled_in_production response, got %s", rec.Body.String())
			}
			if tc.handler.calls != 0 {
				t.Fatalf("disabled route reached handler %d times", tc.handler.calls)
			}
		})
	}
}

func TestProductionSupportedRoutesStillReachHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if chatHandler.calls != 1 {
		t.Fatalf("supported route reached handler %d times, want 1", chatHandler.calls)
	}
}

func TestDevelopmentDisabledRoutesRemainCallableForLocalCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles: filesHandler,
	}, RouteRegistrationOptions{Production: false})

	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if filesHandler.calls != 1 {
		t.Fatalf("development route reached handler %d times, want 1", filesHandler.calls)
	}
}

type countingHandler struct {
	calls int
}

func (h *countingHandler) Handle(c *gin.Context) error {
	h.calls++
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
	return nil
}

func (h *countingHandler) HandleStream(c *gin.Context) error {
	return errors.New("stream should not be called in these tests")
}

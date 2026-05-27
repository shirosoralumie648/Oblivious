package handler

import (
	"context"
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
	addTrustedRelayHeaders(req)
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

func TestProductionSupportedRoutesRequireTrustedIdentityBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	audit := &recordingRouteAuditSink{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, AuditSink: audit})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_identity_required") {
		t.Fatalf("expected relay_identity_required response, got %s", rec.Body.String())
	}
	if chatHandler.calls != 0 {
		t.Fatalf("unauthenticated route reached handler %d times", chatHandler.calls)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0].Result != RouteAuditResultRejected || audit.events[0].FailureReason == "" {
		t.Fatalf("expected rejected audit event with failure reason, got %+v", audit.events[0])
	}
}

func TestProductionSupportedRoutesAttachTrustedIdentityAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &identityRecordingHandler{}
	audit := &recordingRouteAuditSink{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, AuditSink: audit})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	addTrustedRelayHeaders(req)
	req.Header.Set(types.HeaderRequestID, "req_123")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if chatHandler.userID != "user_1" || chatHandler.organizationID != "org_1" || chatHandler.requestID != "req_123" {
		t.Fatalf("handler saw identity user=%q org=%q request=%q", chatHandler.userID, chatHandler.organizationID, chatHandler.requestID)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	event := audit.events[0]
	if event.Result != RouteAuditResultAllowed || event.UserID != "user_1" || event.OrganizationID != "org_1" || event.RequestID != "req_123" {
		t.Fatalf("unexpected audit event: %+v", event)
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

type identityRecordingHandler struct {
	calls          int
	userID         string
	organizationID string
	requestID      string
}

func (h *identityRecordingHandler) Handle(c *gin.Context) error {
	h.calls++
	h.userID, _ = types.TrustedUserIDFromContext(c.Request.Context())
	h.organizationID, _ = types.TrustedOrganizationIDFromContext(c.Request.Context())
	h.requestID, _ = types.TrustedRequestIDFromContext(c.Request.Context())
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
	return nil
}

func (h *identityRecordingHandler) HandleStream(c *gin.Context) error {
	return errors.New("stream should not be called in these tests")
}

type recordingRouteAuditSink struct {
	events []RouteAuditEvent
}

func (s *recordingRouteAuditSink) RecordRelayRouteDecision(ctx context.Context, event RouteAuditEvent) {
	s.events = append(s.events, event)
}

func addTrustedRelayHeaders(req *http.Request) {
	req.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	req.Header.Set(types.HeaderInternalUserID, "user_1")
	req.Header.Set(types.HeaderInternalOrganization, "org_1")
}

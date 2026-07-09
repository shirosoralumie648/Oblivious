package http

import (
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestGatewayProxyRequiresSession(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		GatewayRelayHandler: stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			t.Fatal("relay handler should not be called without a session")
		}),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous gateway proxy request, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestGatewayProxyRequiresRelayHandler(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore: stubAuthStore{session: session},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(csrfHeaderName, routeSurfaceCSRFToken(session))
	request.AddCookie(routeSurfaceSignedSessionCookie(t, session))

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusServiceUnavailable {
		t.Fatalf("expected 503 when relay proxy is not configured, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "relay_unavailable") {
		t.Fatalf("expected relay_unavailable error, got body %s", recorder.Body.String())
	}
}

func TestGatewayProxyMutationsRejectCookieWithoutCSRF(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore: stubAuthStore{session: session},
		GatewayRelayHandler: stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			t.Fatal("gateway proxy mutation without csrf must not reach relay")
		}),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(routeSurfaceSignedSessionCookie(t, session))

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected 403 for gateway proxy mutation without csrf, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "csrf_required") {
		t.Fatalf("expected csrf_required response, got %s", recorder.Body.String())
	}
}

func TestGatewayProxyForwardsSessionRequestToRelayEngine(t *testing.T) {
	t.Setenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN", "test-internal-token")

	session := routeSurfaceUserSession()
	var gotPath string
	var gotQuery string
	var gotAuth string
	var gotInternalAuth string
	var gotUserID string
	var gotOrgID string
	var gotFeature string
	var gotRequestID string
	var gotBody string
	relayHandler := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotInternalAuth = r.Header.Get(types.HeaderInternalAuth)
		gotUserID = r.Header.Get(types.HeaderInternalUserID)
		gotOrgID = r.Header.Get(types.HeaderInternalOrganization)
		gotFeature = r.Header.Get(types.HeaderInternalFeatureType)
		gotRequestID = r.Header.Get(types.HeaderRequestID)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read proxied body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stdhttp.StatusAccepted)
		_, _ = w.Write([]byte(`{"data":{"proxied":true}}`))
	})
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore:           stubAuthStore{session: session},
		GatewayRelayHandler: relayHandler,
	})

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions?trace=1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer browser-token")
	request.Header.Set(csrfHeaderName, routeSurfaceCSRFToken(session))
	request.AddCookie(routeSurfaceSignedSessionCookie(t, session))

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusAccepted {
		t.Fatalf("expected proxied relay status 202, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected proxied path /v1/chat/completions, got %q", gotPath)
	}
	if gotQuery != "trace=1" {
		t.Fatalf("expected proxied query trace=1, got %q", gotQuery)
	}
	if gotAuth != "" {
		t.Fatalf("expected browser Authorization header to be stripped, got %q", gotAuth)
	}
	if gotInternalAuth != "test-internal-token" {
		t.Fatalf("expected configured internal auth token, got %q", gotInternalAuth)
	}
	if gotUserID != session.User.ID {
		t.Fatalf("expected proxied user id %q, got %q", session.User.ID, gotUserID)
	}
	if gotOrgID != session.OrganizationID {
		t.Fatalf("expected proxied organization id %q, got %q", session.OrganizationID, gotOrgID)
	}
	if gotFeature != "gateway_proxy" {
		t.Fatalf("expected feature gateway_proxy, got %q", gotFeature)
	}
	if gotRequestID == "" {
		t.Fatal("expected request id header to be forwarded")
	}
	if gotBody != body {
		t.Fatalf("expected proxied body %s, got %s", body, gotBody)
	}
	if !strings.Contains(recorder.Body.String(), `"proxied":true`) {
		t.Fatalf("expected relay response body to pass through, got %s", recorder.Body.String())
	}
}

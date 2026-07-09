package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"oblivious/server/internal/auth"
)

func TestWebSocketHandshakeAllowsConfiguredOrigin(t *testing.T) {
	cfg := testConfig()
	cfg.CORSAllowedOrigins = []string{"https://console.example.test"}
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(cfg, nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	server := httptest.NewServer(router)
	defer server.Close()

	conn, resp, err := websocket.DefaultDialer.Dial(webSocketTestURL(server), webSocketTestHeaders(t, session, "https://console.example.test"))
	if err != nil {
		t.Fatalf("expected websocket handshake to succeed, got %v with response %s", err, webSocketResponseStatus(resp))
	}
	defer conn.Close()

	if resp == nil || resp.StatusCode != stdhttp.StatusSwitchingProtocols {
		t.Fatalf("expected 101 switching protocols, got %s", webSocketResponseStatus(resp))
	}
}

func TestWebSocketHandshakeRejectsDisallowedOrigin(t *testing.T) {
	cfg := testConfig()
	cfg.CORSAllowedOrigins = []string{"https://console.example.test"}
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(cfg, nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	server := httptest.NewServer(router)
	defer server.Close()

	conn, resp, err := websocket.DefaultDialer.Dial(webSocketTestURL(server), webSocketTestHeaders(t, session, "https://evil.example.test"))
	if conn != nil {
		defer conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket handshake with disallowed Origin to fail")
	}
	if resp == nil || resp.StatusCode != stdhttp.StatusForbidden {
		t.Fatalf("expected 403 forbidden for disallowed Origin, got %s", webSocketResponseStatus(resp))
	}
}

func webSocketTestURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws"
}

func webSocketTestHeaders(t *testing.T, session auth.Session, origin string) stdhttp.Header {
	t.Helper()

	headers := stdhttp.Header{}
	headers.Set("Cookie", routeSurfaceSignedSessionCookie(t, session).String())
	if origin != "" {
		headers.Set("Origin", origin)
	}
	return headers
}

func webSocketResponseStatus(resp *stdhttp.Response) string {
	if resp == nil {
		return "<nil>"
	}
	return resp.Status
}

package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginPolicyAllowsServerAndSameHostClients(t *testing.T) {
	policy := NewOriginPolicy(nil)

	withoutOrigin := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	if !policy.Allow(withoutOrigin) {
		t.Fatal("expected websocket request without Origin to be allowed")
	}

	sameHost := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	sameHost.Header.Set("Origin", "https://api.example.test")
	if !policy.Allow(sameHost) {
		t.Fatal("expected same-host websocket Origin to be allowed")
	}
}

func TestOriginPolicyAllowsConfiguredOrigins(t *testing.T) {
	policy := NewOriginPolicy([]string{" https://console.example.test/ "})
	request := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	request.Header.Set("Origin", "https://console.example.test")

	if !policy.Allow(request) {
		t.Fatal("expected configured websocket Origin to be allowed")
	}
}

func TestOriginPolicyRejectsCrossSiteOriginsByDefault(t *testing.T) {
	policy := NewOriginPolicy(nil)
	request := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	request.Header.Set("Origin", "https://evil.example.test")

	if policy.Allow(request) {
		t.Fatal("expected cross-site websocket Origin to be rejected")
	}
}

func TestOriginPolicyRejectsWildcardAndMalformedOrigins(t *testing.T) {
	policy := NewOriginPolicy([]string{"*"})

	wildcardConfigured := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	wildcardConfigured.Header.Set("Origin", "https://evil.example.test")
	if policy.Allow(wildcardConfigured) {
		t.Fatal("expected wildcard configuration not to allow arbitrary websocket Origins")
	}

	malformed := httptest.NewRequest(http.MethodGet, "http://api.example.test/api/v1/ws", nil)
	malformed.Header.Set("Origin", "not an origin")
	if policy.Allow(malformed) {
		t.Fatal("expected malformed websocket Origin to be rejected")
	}
}

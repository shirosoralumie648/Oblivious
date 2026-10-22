package http

import (
	stdhttp "net/http"
	"testing"
)

func TestRequestClientIP(t *testing.T) {
	req, _ := stdhttp.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.2")

	ip := requestClientIP(req)
	if ip != "198.51.100.2" {
		t.Errorf("expected 198.51.100.2, got %s", ip)
	}

	// Test single IP
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	ip = requestClientIP(req)
	if ip != "203.0.113.10" {
		t.Errorf("expected 203.0.113.10, got %s", ip)
	}

	// Test empty parts
	req.Header.Set("X-Forwarded-For", " , ")
	ip = requestClientIP(req)
	if ip != "" {
		t.Errorf("expected empty string, got %s", ip)
	}
}

package channel

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestValidateProviderUpstreamURLRejectsUnsafeProductionURLs(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	tests := []struct {
		name      string
		rawURL    string
		wantError string
	}{
		{
			name:      "metadata service",
			rawURL:    "http://169.254.169.254/latest/meta-data",
			wantError: "provider upstream URL must not target local or private network addresses",
		},
		{
			name:      "loopback IPv4",
			rawURL:    "https://127.0.0.1/v1/chat/completions",
			wantError: "provider upstream URL must not target local or private network addresses",
		},
		{
			name:      "private IPv4",
			rawURL:    "https://10.0.0.8/v1/chat/completions",
			wantError: "provider upstream URL must not target local or private network addresses",
		},
		{
			name:      "loopback IPv6",
			rawURL:    "https://[::1]/v1/chat/completions",
			wantError: "provider upstream URL must not target local or private network addresses",
		},
		{
			name:      "embedded credentials",
			rawURL:    "https://user:pass@api.example.test/v1/chat/completions",
			wantError: "provider upstream URL must not include credentials",
		},
		{
			name:      "plain HTTP public host",
			rawURL:    "http://api.example.test/v1/chat/completions",
			wantError: "provider upstream URL must use https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProviderUpstreamURL(tt.rawURL)
			if err == nil {
				t.Fatalf("expected unsafe upstream URL error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestValidateProviderUpstreamURLAllowsLocalDevelopmentURLs(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	if err := validateProviderUpstreamURL("http://127.0.0.1:8080/v1/chat/completions"); err != nil {
		t.Fatalf("expected development local URL to be allowed, got %v", err)
	}
}

func TestProductionProviderHTTPClientRejectsUnsafeResolvedAddress(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	dialed := false
	client := newProviderHTTPClient(0, func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = true
		return nil, nil
	})
	req, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/v1/models", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected unsafe resolved address error")
	}
	if !strings.Contains(err.Error(), "provider upstream URL must not target local or private network addresses") {
		t.Fatalf("expected unsafe resolved address error, got %v", err)
	}
	if dialed {
		t.Fatal("unsafe upstream should be rejected before dial")
	}
}

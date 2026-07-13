package outboundpolicy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
)

type resolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f resolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestStrictPolicyRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	validator := New(StrictConfig())
	tests := []string{
		"http://api.example.com/v1",
		"https://user:pass@api.example.com/v1",
		"https://127.0.0.1/v1",
		"https://10.0.0.8/v1",
		"https://169.254.169.254/latest/meta-data",
		"https://[::1]/v1",
		"https://[::ffff:127.0.0.1]/v1",
	}
	for _, rawURL := range tests {
		rawURL := rawURL
		t.Run(rawURL, func(t *testing.T) {
			if _, err := validator.ValidateURL(context.Background(), rawURL); !errors.Is(err, ErrBlocked) {
				t.Fatalf("ValidateURL(%q) error = %v, want ErrBlocked", rawURL, err)
			}
		})
	}
}

func TestStrictPolicyRejectsAnyUnsafeDNSAnswer(t *testing.T) {
	t.Parallel()

	cfg := StrictConfig()
	cfg.Resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("1.1.1.1")},
			{IP: net.ParseIP("10.0.0.8")},
		}, nil
	})
	validator := New(cfg)

	if _, err := validator.ValidateURL(context.Background(), "https://api.example.com/v1"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("ValidateURL error = %v, want ErrBlocked", err)
	}
}

func TestAllowedHostOnlyExemptsTheNamedTestHost(t *testing.T) {
	t.Parallel()

	cfg := StrictConfig()
	cfg.RequireHTTPS = false
	cfg.AllowedHosts = []string{"127.0.0.1"}
	cfg.Resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("1.1.1.1")}}, nil
	})
	validator := New(cfg)

	if _, err := validator.ValidateURL(context.Background(), "http://127.0.0.1:8080/test"); err != nil {
		t.Fatalf("allowed test host rejected: %v", err)
	}
	if _, err := validator.ValidateURL(context.Background(), "http://169.254.169.254/latest/meta-data"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("metadata target error = %v, want ErrBlocked", err)
	}
}

func TestRedirectPolicyRejectsCrossOriginAndPrivateTargets(t *testing.T) {
	t.Parallel()

	cfg := StrictConfig()
	cfg.RequireHTTPS = false
	cfg.AllowedHosts = []string{"127.0.0.1"}
	validator := New(cfg)
	via := []*http.Request{{URL: mustURL(t, "http://127.0.0.1:8080/start")}}

	privateRequest := &http.Request{URL: mustURL(t, "http://169.254.169.254/latest/meta-data")}
	if err := validator.checkRedirect(privateRequest, via); !errors.Is(err, ErrBlocked) {
		t.Fatalf("private redirect error = %v, want ErrBlocked", err)
	}

	publicRequest := &http.Request{URL: mustURL(t, "http://example.com/next")}
	if err := validator.checkRedirect(publicRequest, via); !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin redirect error = %v, want ErrCrossOriginRedirect", err)
	}
}

func TestBlockedIPHandlesMappedAndReservedAddresses(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"0.0.0.0",
		"100.64.0.1",
		"127.0.0.1",
		"169.254.1.1",
		"192.0.2.1",
		"198.18.0.1",
		"198.51.100.1",
		"203.0.113.1",
		"::",
		"::1",
		"::ffff:127.0.0.1",
		"fc00::1",
		"fe80::1",
		"2001:db8::1",
	} {
		addr := netip.MustParseAddr(raw)
		if !blockedIP(addr) {
			t.Fatalf("blockedIP(%s) = false, want true", raw)
		}
	}
}

func TestGuardedDialContextDialsTheValidatedAddress(t *testing.T) {
	t.Parallel()

	cfg := StrictConfig()
	cfg.Resolver = resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("1.1.1.1")}}, nil
	})
	validator := New(cfg)
	dialedAddress := ""
	dial := validator.guardedDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		dialedAddress = address
		return nil, errors.New("dial stopped by test")
	})

	_, _ = dial(context.Background(), "tcp", "api.example.com:443")
	if dialedAddress != "1.1.1.1:443" {
		t.Fatalf("dialed address = %q, want 1.1.1.1:443", dialedAddress)
	}
}

func TestValidateUserHeaderNameRejectsRoutingHeaders(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Host", "Proxy-Authorization", "Forwarded", "X-Forwarded-Host"} {
		if err := ValidateUserHeaderName(name); !errors.Is(err, ErrBlocked) {
			t.Fatalf("ValidateUserHeaderName(%q) error = %v, want ErrBlocked", name, err)
		}
	}
	if err := ValidateUserHeaderName("Authorization"); err != nil {
		t.Fatalf("Authorization should be allowed for same-origin guarded requests: %v", err)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL %q: %v", raw, err)
	}
	return parsed
}

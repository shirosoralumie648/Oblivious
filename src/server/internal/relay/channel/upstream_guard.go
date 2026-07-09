package channel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type providerDialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

func validateProviderUpstreamURL(raw string) error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("provider upstream URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid provider upstream URL: %w", err)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("provider upstream URL must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("provider upstream URL must not include credentials")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("provider upstream URL must include a host")
	}
	if ip := net.ParseIP(host); ip != nil && isUnsafeProviderUpstreamIP(ip) {
		return fmt.Errorf("provider upstream URL must not target local or private network addresses")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("provider upstream URL must use https")
	}
	return nil
}

func newProviderHTTPClient(timeout time.Duration, dialContext providerDialContextFunc) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: timeout}
		dialContext = dialer.DialContext
	}

	if providerUpstreamGuardEnabled() {
		transport.DialContext = guardedProviderDialContext(dialContext)
		return &http.Client{
			Timeout: timeout,
			Transport: providerUpstreamTransport{
				base: transport,
			},
		}
	}

	transport.DialContext = dialContext
	return &http.Client{Timeout: timeout, Transport: transport}
}

type providerUpstreamTransport struct {
	base http.RoundTripper
}

func (t providerUpstreamTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("provider upstream URL is required")
	}
	if err := validateProviderUpstreamURL(req.URL.String()); err != nil {
		return nil, err
	}
	if _, err := resolveProviderUpstreamIPs(req.Context(), req.URL.Hostname()); err != nil {
		return nil, err
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func guardedProviderDialContext(dialContext providerDialContextFunc) providerDialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			if _, resolveErr := resolveProviderUpstreamIPs(ctx, address); resolveErr != nil {
				return nil, resolveErr
			}
			return dialContext(ctx, network, address)
		}

		resolvedIPs, err := resolveProviderUpstreamIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		if net.ParseIP(host) != nil {
			return dialContext(ctx, network, address)
		}

		var lastErr error
		for _, ip := range resolvedIPs {
			conn, err := dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("provider upstream host %q resolved to no dialable addresses", host)
	}
}

func resolveProviderUpstreamIPs(ctx context.Context, host string) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("provider upstream URL must include a host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeProviderUpstreamIP(ip) {
			return nil, fmt.Errorf("provider upstream URL must not target local or private network addresses")
		}
		return []net.IP{ip}, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve provider upstream host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve provider upstream host %q: no addresses", host)
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if isUnsafeProviderUpstreamIP(addr.IP) {
			return nil, fmt.Errorf("provider upstream URL must not target local or private network addresses")
		}
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

func providerUpstreamGuardEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func isUnsafeProviderUpstreamIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

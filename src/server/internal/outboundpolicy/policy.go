package outboundpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var (
	ErrBlocked             = errors.New("outbound request blocked")
	ErrCrossOriginRedirect = errors.New("cross-origin outbound redirect blocked")
	ErrTooManyRedirects    = errors.New("too many outbound redirects")
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type Config struct {
	RequireHTTPS        bool
	AllowPrivateNetwork bool
	AllowedHosts        []string
	MaxRedirects        int
	Resolver            Resolver
	DialContext         DialContextFunc
	BaseTransport       *http.Transport
}

type Validator struct {
	config       Config
	allowedHosts map[string]struct{}
}

func StrictConfig() Config {
	return Config{
		RequireHTTPS: true,
		MaxRedirects: 3,
	}
}

func New(config Config) *Validator {
	if config.MaxRedirects <= 0 {
		config.MaxRedirects = 3
	}
	if config.Resolver == nil {
		config.Resolver = net.DefaultResolver
	}
	allowedHosts := make(map[string]struct{}, len(config.AllowedHosts))
	for _, host := range config.AllowedHosts {
		host = normalizeHost(host)
		if host != "" {
			allowedHosts[host] = struct{}{}
		}
	}
	return &Validator{config: config, allowedHosts: allowedHosts}
}

func (v *Validator) ValidateURLSyntax(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, blockedf("URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, blockedf("invalid URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, blockedf("URL must use http or https")
	}
	if v.config.RequireHTTPS && parsed.Scheme != "https" {
		return nil, blockedf("URL must use https")
	}
	if parsed.User != nil {
		return nil, blockedf("URL must not include credentials")
	}
	if normalizeHost(parsed.Hostname()) == "" {
		return nil, blockedf("URL must include a host")
	}
	if ip, err := netip.ParseAddr(parsed.Hostname()); err == nil && !v.hostAllowed(parsed.Hostname()) && !v.config.AllowPrivateNetwork && blockedIP(ip) {
		return nil, blockedf("URL targets a local, private, reserved, or non-routable address")
	}
	return parsed, nil
}

func (v *Validator) ValidateURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := v.ValidateURLSyntax(raw)
	if err != nil {
		return nil, err
	}
	if _, err := v.resolveAllowedIPs(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func blockedf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrBlocked, fmt.Sprintf(format, args...))
}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func blockedIP(addr netip.Addr) bool {
	if addr.Is4In6() {
		addr = addr.Unmap()
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (v *Validator) HTTPClient(timeout time.Duration) *http.Client {
	base := v.config.BaseTransport
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		base = base.Clone()
	}
	dialContext := v.config.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: timeout}
		dialContext = dialer.DialContext
	}
	base.DialContext = v.guardedDialContext(dialContext)

	return &http.Client{
		Timeout:       timeout,
		Transport:     validatingTransport{validator: v, base: base},
		CheckRedirect: v.checkRedirect,
	}
}

type validatingTransport struct {
	validator *Validator
	base      http.RoundTripper
}

func (t validatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, blockedf("request URL is required")
	}
	if _, err := t.validator.ValidateURL(req.Context(), req.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func (v *Validator) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= v.config.MaxRedirects {
		return ErrTooManyRedirects
	}
	if req == nil || req.URL == nil {
		return blockedf("redirect URL is required")
	}
	if _, err := v.ValidateURLSyntax(req.URL.String()); err != nil {
		return err
	}
	if len(via) > 0 && !sameOrigin(via[len(via)-1].URL, req.URL) {
		return ErrCrossOriginRedirect
	}
	if _, err := v.resolveAllowedIPs(req.Context(), req.URL.Hostname()); err != nil {
		return err
	}
	return nil
}

func (v *Validator) guardedDialContext(base DialContextFunc) DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, blockedf("invalid dial address %q", address)
		}
		ips, err := v.resolveAllowedIPs(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := base(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, blockedf("host %q resolved to no dialable addresses", host)
	}
}

func (v *Validator) resolveAllowedIPs(ctx context.Context, host string) ([]netip.Addr, error) {
	host = normalizeHost(host)
	if host == "" {
		return nil, blockedf("host is required")
	}
	if parsed, err := netip.ParseAddr(host); err == nil {
		if parsed.Is4In6() {
			parsed = parsed.Unmap()
		}
		if !v.hostAllowed(host) && !v.config.AllowPrivateNetwork && blockedIP(parsed) {
			return nil, blockedf("host %q resolves to a local, private, reserved, or non-routable address", host)
		}
		return []netip.Addr{parsed}, nil
	}

	resolved, err := v.config.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, blockedf("resolve host %q: %v", host, err)
	}
	if len(resolved) == 0 {
		return nil, blockedf("resolve host %q: no addresses", host)
	}
	ips := make([]netip.Addr, 0, len(resolved))
	for _, item := range resolved {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return nil, blockedf("resolve host %q: invalid address", host)
		}
		if addr.Is4In6() {
			addr = addr.Unmap()
		}
		if !v.hostAllowed(host) && !v.config.AllowPrivateNetwork && blockedIP(addr) {
			return nil, blockedf("host %q resolves to a local, private, reserved, or non-routable address", host)
		}
		ips = append(ips, addr)
	}
	return ips, nil
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

func (v *Validator) hostAllowed(host string) bool {
	_, ok := v.allowedHosts[normalizeHost(host)]
	return ok
}

func effectivePort(target *url.URL) string {
	if target == nil {
		return ""
	}
	if port := target.Port(); port != "" {
		return port
	}
	switch strings.ToLower(target.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func sameOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		normalizeHost(left.Hostname()) == normalizeHost(right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

var forbiddenUserHeaders = map[string]struct{}{
	"Connection":          {},
	"Forwarded":           {},
	"Host":                {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Proxy-Connection":    {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func ValidateUserHeaderName(name string) error {
	canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
	if canonical == "" {
		return blockedf("request header name is required")
	}
	if _, blocked := forbiddenUserHeaders[canonical]; blocked {
		return blockedf("request header %q controls routing or transport behavior", canonical)
	}
	if strings.HasPrefix(canonical, "X-Forwarded-") {
		return blockedf("request header %q controls forwarded routing identity", canonical)
	}
	return nil
}

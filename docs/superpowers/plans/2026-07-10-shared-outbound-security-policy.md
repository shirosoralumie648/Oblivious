# Shared Outbound Security Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one reusable outbound HTTP security policy and apply it to Agent custom API tools, Workflow HTTP nodes, remote MCP calls, Marketplace payout dispatch, and the existing Relay provider guard.

**Architecture:** Add a standard-library-only `internal/outboundpolicy` package that owns URL syntax validation, DNS/IP classification, DNS-pinned dialing, redirect validation, routing-header validation, and bounded response reads. Untrusted customer/admin-configured endpoints use a strict external profile in every environment; tests explicitly allow only their local `httptest` host. Relay keeps its current development behavior through a compatibility profile while reusing the same implementation. Marketplace production startup also wires the already-configured webhook payout provider into `RouterOptions`, closing the current unused-provider gap.

**Tech Stack:** Go 1.25 standard library (`net/http`, `net`, `net/netip`, `net/url`, `io`), existing Go unit tests, Bash quality gates, GitHub Actions.

---

## Scope And Non-Goals

This plan closes the first item in `docs/audit/2026-07-10-module-capability-gap-matrix.md`:

- Agent custom API requests use the shared policy.
- Workflow HTTP nodes use the shared policy.
- Remote MCP JSON-RPC requests use the shared policy.
- Marketplace webhook payout dispatch uses the shared policy.
- Relay provider upstream protection delegates to the shared implementation.
- Redirects are bounded, revalidated, and limited to the same origin.
- DNS answers are checked before request execution and again at dial time.
- Response bodies are bounded for every migrated consumer.
- Production server construction injects the configured webhook payout provider.
- CI rejects reintroduction of direct default HTTP clients in these paths.

The following work remains outside this plan:

- Enabling the disabled builtin `http_request` tool.
- Adding MCP OAuth discovery, SSE, or streamable HTTP transports that do not exist in the current production client.
- Adding MCP request correlation, latency, error, and result-size telemetry; that belongs to the later Observability Persistence and SLO plan, while this plan enforces the response-size boundary.
- Applying the policy to provider-controlled Relay URLs beyond the current compatibility migration.
- Applying the policy to publishing-channel webhooks, observability alert sinks, web search providers, RPA browser navigation, or archive sinks.
- Deployment-level egress firewall, Kubernetes NetworkPolicy, service mesh, or DNS proxy enforcement.
- Full target-environment payout, MCP, Agent, and Workflow evidence collection.
- Marketplace payout retry-attempt accounting, webhook reconciliation, refund, and chargeback target proof; that belongs to the Payment and Marketplace Reconciliation plan.

## File Structure

**Create:**

- `src/server/internal/outboundpolicy/policy.go` — URL, DNS/IP, redirect, dial, and header policy.
- `src/server/internal/outboundpolicy/body.go` — bounded response body reader.
- `src/server/internal/outboundpolicy/policy_test.go` — syntax, IP, DNS, dial, and redirect tests.
- `src/server/internal/outboundpolicy/body_test.go` — response size tests.
- `src/server/internal/http/marketplace_payout_provider.go` — config-to-provider production builder.
- `src/server/internal/http/marketplace_payout_provider_test.go` — builder behavior tests.
- `scripts/verify-outbound-security.sh` — static regression gate.
- `docs/architecture/outbound-http-security.md` — policy contract and extension guide.
- `docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md` — repository-local completion evidence.

**Modify:**

- `src/server/internal/relay/channel/upstream_guard.go`
- `src/server/internal/relay/channel/upstream_guard_test.go`
- `src/server/internal/agent/executor.go`
- `src/server/internal/agent/service.go`
- `src/server/internal/agent/executor_test.go`
- `src/server/internal/agent/service_test.go`
- `src/server/internal/workflow/node_executor.go`
- `src/server/internal/workflow/types.go`
- `src/server/internal/workflow/types_test.go`
- `src/server/internal/workflow/http_node_executor_test.go`
- `src/server/internal/http/workflow_secret_response_test.go`
- `src/server/internal/mcp/client.go`
- `src/server/internal/mcp/client_test.go`
- `src/server/internal/marketplace/payout_provider.go`
- `src/server/internal/marketplace/payout_provider_test.go`
- `src/server/internal/config/config.go`
- `src/server/internal/config/config_test.go`
- `src/server/internal/http/server.go`
- `scripts/check.sh`
- `.github/workflows/ci.yml`
- `docs/audit/2026-07-10-module-capability-gap-matrix.md`

---

### Task 1: Build The Shared Outbound Policy

**Files:**

- Create: `src/server/internal/outboundpolicy/policy.go`
- Create: `src/server/internal/outboundpolicy/body.go`
- Create: `src/server/internal/outboundpolicy/policy_test.go`
- Create: `src/server/internal/outboundpolicy/body_test.go`

- [ ] **Step 1: Write failing URL, DNS, redirect, and body-limit tests**

Create `policy_test.go` with package-local fakes so the policy can be tested without external network access:

```go
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
```

Create `body_test.go`:

```go
package outboundpolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestReadBodyLimitedRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	_, err := ReadBodyLimited(strings.NewReader("12345"), 4)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("ReadBodyLimited error = %v, want ErrResponseTooLarge", err)
	}
}

func TestReadBodyLimitedReturnsBodyWithinLimit(t *testing.T) {
	t.Parallel()

	body, err := ReadBodyLimited(strings.NewReader("1234"), 4)
	if err != nil {
		t.Fatalf("ReadBodyLimited returned error: %v", err)
	}
	if string(body) != "1234" {
		t.Fatalf("ReadBodyLimited body = %q, want 1234", string(body))
	}
}
```

- [ ] **Step 2: Run tests and verify they fail because the package implementation is absent**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/outboundpolicy -count=1 -v
```

Expected: FAIL with undefined identifiers such as `New`, `StrictConfig`, `ErrBlocked`, and `ReadBodyLimited`.

- [ ] **Step 3: Implement the policy types and syntax validation**

Create `policy.go` with these public contracts:

```go
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
```

Add explicit blocked prefixes and normalize IPv4-mapped IPv6 before comparison:

```go
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
```

- [ ] **Step 4: Implement DNS validation, DNS-pinned dialing, and redirects**

Add these methods to `policy.go`:

```go
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
	if _, err := v.ValidateURL(req.Context(), req.URL.String()); err != nil {
		return err
	}
	if len(via) > 0 && !sameOrigin(via[len(via)-1].URL, req.URL) {
		return ErrCrossOriginRedirect
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
```

Add the remaining helpers to `policy.go`:

```go
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
```

Add the package-local URL helper to `policy_test.go`:

```go
func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL %q: %v", raw, err)
	}
	return parsed
}
```

- [ ] **Step 5: Implement bounded body reads**

Create `body.go`:

```go
package outboundpolicy

import (
	"errors"
	"fmt"
	"io"
)

var ErrResponseTooLarge = errors.New("outbound response body too large")

func ReadBodyLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit must be positive", ErrResponseTooLarge)
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: limit=%d", ErrResponseTooLarge, limit)
	}
	return body, nil
}
```

- [ ] **Step 6: Run the package tests**

Run:

```bash
cd src/server
gofmt -w internal/outboundpolicy/*.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/outboundpolicy -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit the shared package**

```bash
git add src/server/internal/outboundpolicy
git commit -m "feat: add shared outbound HTTP policy"
```

---

### Task 2: Move Relay Provider Guard Onto The Shared Policy

**Files:**

- Modify: `src/server/internal/relay/channel/upstream_guard.go`
- Modify: `src/server/internal/relay/channel/upstream_guard_test.go`

- [ ] **Step 1: Add a failing compatibility test**

Extend `upstream_guard_test.go`:

```go
func TestProductionProviderHTTPClientInstallsSharedRedirectPolicy(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	client := newProviderHTTPClient(time.Second, nil)
	if client.CheckRedirect == nil {
		t.Fatal("provider HTTP client must install the shared redirect policy")
	}

	redirectRequest := &http.Request{
		URL: mustParseProviderURL(t, "https://169.254.169.254/latest/meta-data"),
	}
	via := []*http.Request{{
		URL: mustParseProviderURL(t, "https://api.example.com/v1/models"),
	}}
	err := client.CheckRedirect(redirectRequest, via)
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("redirect error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func mustParseProviderURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse URL %q: %v", raw, err)
	}
	return parsed
}
```

- [ ] **Step 2: Run the focused test and verify failure before migration**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/relay/channel -run 'ProviderHTTPClient|ProviderUpstreamURL' -count=1 -v
```

Expected: FAIL because Relay does not yet import or delegate to `outboundpolicy`.

- [ ] **Step 3: Replace the duplicate Relay implementation with compatibility wrappers**

Keep the existing private function names so adapters do not need broad changes:

```go
func providerOutboundConfig() outboundpolicy.Config {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return outboundpolicy.StrictConfig()
	}
	return outboundpolicy.Config{
		RequireHTTPS:        false,
		AllowPrivateNetwork: true,
		MaxRedirects:        3,
	}
}

func validateProviderUpstreamURL(raw string) error {
	_, err := outboundpolicy.New(providerOutboundConfig()).ValidateURLSyntax(raw)
	if err != nil {
		return fmt.Errorf("provider upstream URL rejected: %w", err)
	}
	return nil
}

func newProviderHTTPClient(timeout time.Duration, dialContext providerDialContextFunc) *http.Client {
	config := providerOutboundConfig()
	config.DialContext = outboundpolicy.DialContextFunc(dialContext)
	return outboundpolicy.New(config).HTTPClient(timeout)
}
```

Delete the duplicate private transport, DNS resolver, dial guard, and unsafe-IP classifier from `upstream_guard.go`.

Update test error assertions to use `errors.Is(err, outboundpolicy.ErrBlocked)` instead of matching the old provider-specific string.

- [ ] **Step 4: Run shared and Relay tests**

Run:

```bash
cd src/server
gofmt -w internal/relay/channel/upstream_guard.go internal/relay/channel/upstream_guard_test.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/outboundpolicy ./internal/relay/channel -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit the Relay migration**

```bash
git add src/server/internal/relay/channel/upstream_guard.go \
  src/server/internal/relay/channel/upstream_guard_test.go
git commit -m "refactor: share relay outbound guard"
```

---

### Task 3: Secure Agent Custom API Tools

**Files:**

- Modify: `src/server/internal/agent/executor.go`
- Modify: `src/server/internal/agent/service.go`
- Modify: `src/server/internal/agent/executor_test.go`
- Modify: `src/server/internal/agent/service_test.go`

- [ ] **Step 1: Update the existing success test to use explicit local-host permission**

Add a test helper in `executor_test.go`:

```go
func agentTestOutboundValidator(t *testing.T, rawURL string) *outboundpolicy.Validator {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	config := outboundpolicy.StrictConfig()
	config.RequireHTTPS = false
	config.AllowedHosts = []string{parsed.Hostname()}
	return outboundpolicy.New(config)
}
```

Change the existing executor construction:

```go
executor := NewToolExecutor(
	nil,
	WithToolOutboundValidator(agentTestOutboundValidator(t, server.URL)),
)
```

Make the same explicit injection in the service-level custom API test around the existing `customToolServer`:

```go
service.SetToolOutboundValidator(agentTestOutboundValidator(t, customToolServer.URL))
```

- [ ] **Step 2: Add failing security and size tests**

Add:

```go
func TestToolExecutorRejectsUnsafeCustomAPIURL(t *testing.T) {
	t.Parallel()

	executor := NewToolExecutor(nil)
	result, err := executor.Execute(context.Background(), &Agent{
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:     "metadata",
			Type:     "custom",
			ServerID: "http://169.254.169.254/latest/meta-data",
			Enabled:  true,
		}},
	}, &ToolCall{Name: "metadata"})
	if err != nil {
		t.Fatalf("Execute returned transport error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "outbound request blocked") {
		t.Fatalf("unsafe custom API result = %+v", result)
	}
}

func TestToolExecutorRejectsOversizedCustomAPIResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", customAPIResponseLimit+1)))
	}))
	t.Cleanup(server.Close)

	executor := NewToolExecutor(
		nil,
		WithToolOutboundValidator(agentTestOutboundValidator(t, server.URL)),
	)
	result, err := executor.Execute(context.Background(), &Agent{
		OrganizationID: "org_1",
		Tools: []Tool{{
			Name:     "oversized",
			Type:     "custom",
			ServerID: server.URL,
			Enabled:  true,
		}},
	}, &ToolCall{Name: "oversized"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || !result.IsError || !strings.Contains(result.Content, "outbound response body too large") {
		t.Fatalf("oversized custom API result = %+v", result)
	}
}

func TestServiceCreateAgentRejectsUnsafeCustomAPIEndpoint(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeStore{}, &fakeGateway{})
	session := auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}
	_, err := service.CreateAgent(context.Background(), session, &CreateAgentRequest{
		Name: "unsafe custom tool",
		Tools: []Tool{{
			Name:     "metadata",
			Type:     "custom",
			Runtime:  "api",
			ServerID: "http://169.254.169.254/latest/meta-data",
			Enabled:  true,
		}},
	})
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("CreateAgent error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func TestServiceUpdateAgentRejectsUnsafeCustomAPIEndpoint(t *testing.T) {
	t.Parallel()

	store := &fakeStore{agent: &Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "existing",
	}}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}
	_, err := service.UpdateAgent(context.Background(), session, "agent_1", &UpdateAgentRequest{
		Tools: []Tool{{
			Name:     "metadata",
			Type:     "custom",
			Runtime:  "api",
			ServerID: "http://169.254.169.254/latest/meta-data",
			Enabled:  true,
		}},
	})
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("UpdateAgent error = %v, want outboundpolicy.ErrBlocked", err)
	}
}
```

- [ ] **Step 3: Run tests and verify the new cases fail**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/agent \
  -run 'TestToolExecutorExecutesCustomAPITool|TestToolExecutorRejectsUnsafeCustomAPIURL|TestToolExecutorRejectsOversizedCustomAPIResponse|TestService.*CustomTool|TestService(Create|Update)AgentRejectsUnsafeCustomAPIEndpoint' \
  -count=1 -v
```

Expected: FAIL because `ToolExecutor` still uses `http.DefaultClient`, responses are unbounded, and Agent writes do not validate custom API endpoint syntax.

- [ ] **Step 4: Add outbound policy injection and bounded reads**

Modify `ToolExecutor`:

```go
const customAPIResponseLimit int64 = 1 << 20

type ToolExecutorOption func(*ToolExecutor)

type ToolExecutor struct {
	mcpClient                 *mcp.Client
	builtinTools              map[string]mcp.BuiltinTool
	webSearchProvider         mcp.WebSearchProvider
	customPythonSandboxRunner CustomPythonSandboxRunner
	outboundValidator         *outboundpolicy.Validator
	outboundClient            *http.Client
}

func WithToolOutboundValidator(validator *outboundpolicy.Validator) ToolExecutorOption {
	return func(executor *ToolExecutor) {
		if validator != nil {
			executor.outboundValidator = validator
		}
	}
}

func NewToolExecutor(mcpClient *mcp.Client, options ...ToolExecutorOption) *ToolExecutor {
	executor := &ToolExecutor{
		mcpClient:         mcpClient,
		builtinTools:      mcp.BuiltinTools,
		outboundValidator: outboundpolicy.New(outboundpolicy.StrictConfig()),
	}
	for _, option := range options {
		if option != nil {
			option(executor)
		}
	}
	executor.outboundClient = executor.outboundValidator.HTTPClient(30 * time.Second)
	return executor
}
```

Persist the validator on `Service` so every executor rebuilt by `initRunner`, `SetMCPClient`, `SetMemory`, and `SetPlanStepExecutor(nil)` receives the same policy:

Add this field to the existing `Service` struct:

```go
toolOutboundValidator *outboundpolicy.Validator
```

Then add:

```go
func (s *Service) SetToolOutboundValidator(validator *outboundpolicy.Validator) {
	s.toolOutboundValidator = validator
	s.initRunner()
}

func (s *Service) newToolExecutor() *ToolExecutor {
	options := []ToolExecutorOption{}
	if s.toolOutboundValidator != nil {
		options = append(options, WithToolOutboundValidator(s.toolOutboundValidator))
	}
	executor := NewToolExecutor(s.mcpClient, options...)
	executor.SetWebSearchProvider(s.webSearchProvider)
	executor.SetCustomPythonSandboxRunner(s.pythonSandbox)
	return executor
}
```

Use `s.newToolExecutor()` in `initRunner` and `SetPlanStepExecutor(nil)` rather than constructing a fresh default executor inline.

Validate custom API syntax before persistence while keeping execution-time DNS and redirect validation:

```go
func validateCustomToolEndpoints(tools []Tool, validator *outboundpolicy.Validator) error {
	if validator == nil {
		validator = outboundpolicy.New(outboundpolicy.StrictConfig())
	}
	for _, tool := range tools {
		if tool.Type != "custom" || normalizeCustomToolRuntime(tool.Runtime) == "python" {
			continue
		}
		if _, err := validator.ValidateURLSyntax(tool.ServerID); err != nil {
			return fmt.Errorf("validate custom tool %q endpoint: %w", tool.Name, err)
		}
	}
	return nil
}
```

Call it from `CreateAgent` and from `UpdateAgent` when `req.Tools != nil`. Existing stored Agents remain protected because `executeCustomAPI` always performs request-time validation.

Replace:

```go
resp, err := http.DefaultClient.Do(req)
```

with:

```go
resp, err := e.outboundClient.Do(req)
```

Replace unbounded response reading with:

```go
respBody, err := outboundpolicy.ReadBodyLimited(resp.Body, customAPIResponseLimit)
if err != nil {
	return &ExecuteResult{Content: err.Error(), IsError: true}, nil
}
```

Keep the existing `ExecuteResult` error behavior so Agent runs record a tool error rather than crashing the runner.

- [ ] **Step 5: Run Agent tests**

Run:

```bash
cd src/server
gofmt -w internal/agent/executor.go internal/agent/service.go internal/agent/executor_test.go internal/agent/service_test.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/agent \
  -run 'TestToolExecutor|TestService.*CustomTool' \
  -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Commit the Agent migration**

```bash
git add src/server/internal/agent/executor.go \
  src/server/internal/agent/service.go \
  src/server/internal/agent/executor_test.go \
  src/server/internal/agent/service_test.go
git commit -m "feat: secure agent custom API requests"
```

---

### Task 4: Secure Workflow HTTP Nodes

**Files:**

- Modify: `src/server/internal/workflow/node_executor.go`
- Modify: `src/server/internal/workflow/types.go`
- Create: `src/server/internal/workflow/types_test.go`
- Modify: `src/server/internal/workflow/http_node_executor_test.go`
- Modify: `src/server/internal/http/workflow_secret_response_test.go`

- [ ] **Step 1: Inject an explicit test validator into existing HTTP-node tests**

Add:

```go
func workflowTestHTTPExecutor(t *testing.T, rawURL string) *HTTPNodeExecutor {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	config := outboundpolicy.StrictConfig()
	config.RequireHTTPS = false
	config.AllowedHosts = []string{parsed.Hostname()}
	return NewHTTPNodeExecutor(outboundpolicy.New(config))
}
```

After constructing each service that calls a local `httptest` upstream:

```go
service.RegisterNodeExecutors(workflowTestHTTPExecutor(t, upstream.URL))
```

- [ ] **Step 2: Add failing unsafe URL, redirect, header, and body-limit tests**

Add direct executor tests:

```go
func TestHTTPNodeExecutorRejectsPrivateTarget(t *testing.T) {
	t.Parallel()

	executor := NewHTTPNodeExecutor(nil)
	_, err := executor.Execute(context.Background(), NodeExecutorInput{
		Input: map[string]any{"url": "http://169.254.169.254/latest/meta-data"},
	})
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("Execute error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func TestHTTPNodeExecutorRejectsRoutingHeaders(t *testing.T) {
	t.Parallel()

	executor := NewHTTPNodeExecutor(nil)
	_, err := executor.Execute(context.Background(), NodeExecutorInput{
		Input: map[string]any{
			"url": "https://api.example.com/tickets",
			"headers": map[string]any{
				"Host": "169.254.169.254",
			},
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Execute error = %v, want ErrInvalidInput", err)
	}
}

func TestHTTPNodeExecutorRejectsOversizedResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", workflowHTTPResponseLimit+1)))
	}))
	t.Cleanup(upstream.Close)

	executor := workflowTestHTTPExecutor(t, upstream.URL)
	_, err := executor.Execute(context.Background(), NodeExecutorInput{
		Input: map[string]any{"url": upstream.URL},
	})
	if !errors.Is(err, outboundpolicy.ErrResponseTooLarge) {
		t.Fatalf("Execute error = %v, want ErrResponseTooLarge", err)
	}
}

func TestHTTPNodeExecutorRejectsPrivateRedirect(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	t.Cleanup(upstream.Close)

	executor := workflowTestHTTPExecutor(t, upstream.URL)
	_, err := executor.Execute(context.Background(), NodeExecutorInput{
		Input: map[string]any{"url": upstream.URL},
	})
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("Execute redirect error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func TestWorkflowSecretDefinitionKeysIncludeHTTPAuthorizers(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Set-Cookie",
		"X-API-Key",
		"api_key",
		"access_token",
		"refresh_token",
	} {
		if !IsWorkflowSecretDefinitionKey(key) {
			t.Fatalf("IsWorkflowSecretDefinitionKey(%q) = false, want true", key)
		}
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/workflow \
  -run 'HTTPNodeExecutor|RunReadyNodeExecutesHTTPNode|RunReadyNodeRecordsHTTPNodeFailure|TestNodeExecutesHTTPNode|WorkflowSecretDefinitionKeys' \
  -count=1 -v
```

Expected: FAIL because `NewHTTPNodeExecutor` accepts a raw `*http.Client`, routing headers are unrestricted, responses are unbounded, and HTTP credential header keys are not classified as protected workflow secrets.

- [ ] **Step 4: Replace the raw client with the shared validator**

Implement:

```go
const workflowHTTPResponseLimit int64 = 1 << 20

type HTTPNodeExecutor struct {
	validator *outboundpolicy.Validator
	client    *http.Client
}

func NewHTTPNodeExecutor(validator *outboundpolicy.Validator) *HTTPNodeExecutor {
	if validator == nil {
		validator = outboundpolicy.New(outboundpolicy.StrictConfig())
	}
	return &HTTPNodeExecutor{
		validator: validator,
		client:    validator.HTTPClient(30 * time.Second),
	}
}
```

Before setting each user-provided header:

```go
if err := outboundpolicy.ValidateUserHeaderName(key); err != nil {
	return nil, fmt.Errorf("%w: invalid http node header %q: %v", ErrInvalidInput, key, err)
}
req.Header.Set(key, value)
```

Replace `io.ReadAll(resp.Body)` with:

```go
responseBody, err := outboundpolicy.ReadBodyLimited(resp.Body, workflowHTTPResponseLimit)
if err != nil {
	return nil, err
}
```

Remove the fallback to `http.DefaultClient`.

Restrict persisted response headers to the operational allowlist:

```go
var workflowHTTPResponseHeaderAllowlist = map[string]struct{}{
	"Content-Type": {},
	"ETag":         {},
	"Retry-After":  {},
	"X-Request-Id": {},
}

func workflowHTTPHeaders(headers http.Header) map[string]any {
	out := map[string]any{}
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if _, allowed := workflowHTTPResponseHeaderAllowlist[canonical]; !allowed {
			continue
		}
		if len(values) == 1 {
			out[canonical] = values[0]
			continue
		}
		out[canonical] = append([]string(nil), values...)
	}
	return out
}
```

Extend `IsWorkflowSecretDefinitionKey` so existing secretbox persistence and response redaction protect HTTP credentials:

```go
func IsWorkflowSecretDefinitionKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").
		Replace(strings.ToLower(strings.TrimSpace(key)))
	switch normalized {
	case "secret",
		"webhooksecret",
		"authorization",
		"proxyauthorization",
		"cookie",
		"setcookie",
		"apikey",
		"xapikey",
		"accesstoken",
		"refreshtoken":
		return true
	default:
		return false
	}
}
```

Extend the SQL-backed workflow secret response test with an HTTP node containing `Authorization`, `Cookie`, and `X-API-Key`; assert raw values are absent from `workflow_node_executions` and API/debug responses while `secretbox.Open` restores them for execution.

- [ ] **Step 5: Run Workflow tests**

Run:

```bash
cd src/server
gofmt -w internal/workflow/node_executor.go \
  internal/workflow/types.go \
  internal/workflow/types_test.go \
  internal/workflow/http_node_executor_test.go \
  internal/http/workflow_secret_response_test.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/workflow \
  -run 'HTTPNodeExecutor|RunReadyNodeExecutesHTTPNode|RunReadyNodeRecordsHTTPNodeFailure|TestNodeExecutesHTTPNode' \
  -count=1 -v
```

Expected: PASS.

When `TEST_DATABASE_URL` is available, also run:

```bash
cd src/server
OBLIVIOUS_REQUIRE_TEST_DATABASE=true \
TEST_DATABASE_URL="$TEST_DATABASE_URL" \
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/http \
  -run '^TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers$' \
  -count=1 -v
```

Expected: PASS with no skip.

- [ ] **Step 6: Commit the Workflow migration**

```bash
git add src/server/internal/workflow/node_executor.go \
  src/server/internal/workflow/types.go \
  src/server/internal/workflow/types_test.go \
  src/server/internal/workflow/http_node_executor_test.go \
  src/server/internal/http/workflow_secret_response_test.go
git commit -m "feat: secure workflow HTTP nodes"
```

---

### Task 5: Secure Remote MCP JSON-RPC Requests

**Files:**

- Modify: `src/server/internal/mcp/client.go`
- Modify: `src/server/internal/mcp/client_test.go`

- [ ] **Step 1: Add an explicit test validator to the persisted-server test**

Add:

```go
func mcpTestOutboundValidator(t *testing.T, rawURL string) *outboundpolicy.Validator {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	config := outboundpolicy.StrictConfig()
	config.RequireHTTPS = false
	config.AllowedHosts = []string{parsed.Hostname()}
	return outboundpolicy.New(config)
}
```

Change:

```go
client := NewClient(store)
```

to:

```go
client := NewClient(store, WithOutboundValidator(mcpTestOutboundValidator(t, mcpServer.URL)))
```

- [ ] **Step 2: Add failing private-target, redirect, and body-limit tests**

Add:

```go
func TestClientRejectsPrivateRemoteServer(t *testing.T) {
	t.Parallel()

	store := newMemoryMCPStore(&Server{
		ID:             "mcp_private",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "private",
		URL:            "http://169.254.169.254/latest/meta-data",
	})
	client := NewClient(store)

	err := client.Connect(context.Background(), "mcp_private", "org_1")
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("Connect error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func TestClientRejectsOversizedRemoteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", mcpResponseLimit+1)))
	}))
	t.Cleanup(server.Close)

	store := newMemoryMCPStore(&Server{
		ID:             "mcp_oversized",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "oversized",
		URL:            server.URL,
	})
	client := NewClient(store, WithOutboundValidator(mcpTestOutboundValidator(t, server.URL)))

	err := client.Connect(context.Background(), "mcp_oversized", "org_1")
	if !errors.Is(err, outboundpolicy.ErrResponseTooLarge) {
		t.Fatalf("Connect error = %v, want ErrResponseTooLarge", err)
	}
}

func TestClientRejectsPrivateRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	store := newMemoryMCPStore(&Server{
		ID:             "mcp_redirect",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "redirect",
		URL:            server.URL,
	})
	client := NewClient(store, WithOutboundValidator(mcpTestOutboundValidator(t, server.URL)))

	err := client.Connect(context.Background(), "mcp_redirect", "org_1")
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("Connect redirect error = %v, want outboundpolicy.ErrBlocked", err)
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/mcp \
  -run 'TestClientRehydratesPersistedServerOnConnectListToolsAndCallTool|TestClientRejectsPrivateRemoteServer|TestClientRejectsOversizedRemoteResponse' \
  -count=1 -v
```

Expected: FAIL because `Client` uses a plain `*http.Client` and unbounded `io.ReadAll`.

- [ ] **Step 4: Add MCP client options and strict defaults**

Implement:

```go
const mcpResponseLimit int64 = 4 << 20

type ClientOption func(*Client)

type Client struct {
	mu                sync.RWMutex
	servers           map[string]*serverConnection
	outboundValidator *outboundpolicy.Validator
	httpClient        *http.Client
	store             Store
}

func WithOutboundValidator(validator *outboundpolicy.Validator) ClientOption {
	return func(client *Client) {
		if validator != nil {
			client.outboundValidator = validator
		}
	}
}

func NewClient(store Store, options ...ClientOption) *Client {
	client := &Client{
		servers:           make(map[string]*serverConnection),
		outboundValidator: outboundpolicy.New(outboundpolicy.StrictConfig()),
		store:             store,
	}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	client.httpClient = client.outboundValidator.HTTPClient(30 * time.Second)
	return client
}
```

In `AddServer`, fail fast on invalid syntax without requiring DNS availability:

```go
if _, err := c.outboundValidator.ValidateURLSyntax(server.URL); err != nil {
	return nil, fmt.Errorf("validate MCP server URL: %w", err)
}
```

In `sendRequest`, replace unbounded reads:

```go
body, err := outboundpolicy.ReadBodyLimited(resp.Body, mcpResponseLimit)
if err != nil {
	return nil, err
}
return body, nil
```

The shared `http.Client` performs full DNS/IP and redirect validation during `Connect`, `ListTools`, and `CallTool`. Authorization remains set only from the decrypted server token, and cross-origin redirects are rejected before that header can be forwarded.

- [ ] **Step 5: Run MCP tests**

Run:

```bash
cd src/server
gofmt -w internal/mcp/client.go internal/mcp/client_test.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/mcp \
  -run 'TestClient|TestSQLStore' \
  -count=1 -v
```

Expected: PASS. DB-backed tests may skip when `TEST_DATABASE_URL` is absent; the changed network tests must not skip.

- [ ] **Step 6: Commit the MCP migration**

```bash
git add src/server/internal/mcp/client.go src/server/internal/mcp/client_test.go
git commit -m "feat: secure remote MCP requests"
```

---

### Task 6: Secure And Wire Marketplace Payout Dispatch

**Files:**

- Modify: `src/server/internal/marketplace/payout_provider.go`
- Modify: `src/server/internal/marketplace/payout_provider_test.go`
- Modify: `src/server/internal/config/config.go`
- Modify: `src/server/internal/config/config_test.go`
- Create: `src/server/internal/http/marketplace_payout_provider.go`
- Create: `src/server/internal/http/marketplace_payout_provider_test.go`
- Modify: `src/server/internal/http/server.go`

- [ ] **Step 1: Update payout success tests to allow only their local test host**

Add:

```go
func payoutTestOutboundValidator(t *testing.T, rawURL string) *outboundpolicy.Validator {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test URL: %v", err)
	}
	config := outboundpolicy.StrictConfig()
	config.RequireHTTPS = false
	config.AllowedHosts = []string{parsed.Hostname()}
	return outboundpolicy.New(config)
}
```

Construct each test provider with:

```go
provider := NewWebhookPayoutProvider(
	server.URL,
	secret,
	WithWebhookPayoutOutboundValidator(payoutTestOutboundValidator(t, server.URL)),
)
```

Inside the existing signed-request test handler, add the idempotency assertion before writing the response:

```go
if got, want := r.Header.Get("Idempotency-Key"), "payout_1"; got != want {
	t.Fatalf("Idempotency-Key = %q, want %q", got, want)
}
```

- [ ] **Step 2: Add failing payout policy tests**

Add:

```go
func TestWebhookPayoutProviderRejectsPrivateEndpoint(t *testing.T) {
	t.Parallel()

	provider := NewWebhookPayoutProvider(
		"http://169.254.169.254/payouts",
		"payout-secret",
	)
	_, err := provider.CreatePayout(context.Background(), validMarketplacePayoutDispatchRequest())
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("CreatePayout error = %v, want outboundpolicy.ErrBlocked", err)
	}
}

func TestWebhookPayoutProviderRejectsPrivateRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	provider := NewWebhookPayoutProvider(
		server.URL,
		"payout-secret",
		WithWebhookPayoutOutboundValidator(payoutTestOutboundValidator(t, server.URL)),
	)
	_, err := provider.CreatePayout(context.Background(), validMarketplacePayoutDispatchRequest())
	if !errors.Is(err, outboundpolicy.ErrBlocked) {
		t.Fatalf("CreatePayout error = %v, want outboundpolicy.ErrBlocked", err)
	}
}
```

Refactor the repeated valid request into:

```go
func validMarketplacePayoutDispatchRequest() MarketplacePayoutDispatchRequest {
	return MarketplacePayoutDispatchRequest{
		PayoutID:                "payout_1",
		PublisherOrganizationID: "org_publisher",
		PublisherUserID:         "user_publisher",
		Amount:                  42.50,
		Currency:                "usd",
		SettlementIDs:           []string{"settlement_1"},
	}
}
```

- [ ] **Step 3: Run payout tests and verify failure**

Run:

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/marketplace \
  -run 'TestWebhookPayoutProvider' \
  -count=1 -v
```

Expected: FAIL because payout dispatch has only syntax validation and a plain `*http.Client`.

- [ ] **Step 4: Replace payout client and validation**

Implement:

```go
type WebhookPayoutProvider struct {
	endpointURL      string
	secret           string
	outboundValidator *outboundpolicy.Validator
	client           *http.Client
}

func WithWebhookPayoutOutboundValidator(validator *outboundpolicy.Validator) WebhookPayoutProviderOption {
	return func(provider *WebhookPayoutProvider) {
		if validator != nil {
			provider.outboundValidator = validator
		}
	}
}

func NewWebhookPayoutProvider(endpointURL, secret string, opts ...WebhookPayoutProviderOption) *WebhookPayoutProvider {
	provider := &WebhookPayoutProvider{
		endpointURL:       strings.TrimSpace(endpointURL),
		secret:            strings.TrimSpace(secret),
		outboundValidator: outboundpolicy.New(outboundpolicy.StrictConfig()),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	provider.client = provider.outboundValidator.HTTPClient(15 * time.Second)
	return provider
}
```

Replace `validateWebhookPayoutEndpoint` with:

```go
if _, err := p.outboundValidator.ValidateURLSyntax(p.endpointURL); err != nil {
	return MarketplacePayoutDispatchResult{}, fmt.Errorf("validate webhook payout endpoint: %w", err)
}
```

Keep the existing one-megabyte response limit, but use `outboundpolicy.ReadBodyLimited`.

Add:

```go
httpRequest.Header.Set("Idempotency-Key", payload.PayoutID)
```

Retain `X-Oblivious-Payout-ID` and the HMAC signature.

- [ ] **Step 5: Reuse syntax validation in production config loading**

In `validateMarketplacePayoutConfig`, replace the shallow `validateHTTPURL` call for `MARKETPLACE_PAYOUT_WEBHOOK_URL` with:

```go
validator := outboundpolicy.New(outboundpolicy.StrictConfig())
if _, err := validator.ValidateURLSyntax(webhookURL); err != nil {
	return fmt.Errorf("MARKETPLACE_PAYOUT_WEBHOOK_URL is invalid: %w", err)
}
return nil
```

Add this table test to `config_test.go`:

```go
func TestValidateMarketplacePayoutConfigRejectsUnsafeWebhookURLs(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"http://payments.example.com/payouts",
		"https://user:pass@payments.example.com/payouts",
		"https://127.0.0.1/payouts",
		"https://10.0.0.8/payouts",
		"https://169.254.169.254/latest/meta-data",
		"ftp://payments.example.com/payouts",
	} {
		rawURL := rawURL
		t.Run(rawURL, func(t *testing.T) {
			err := validateMarketplacePayoutConfig(
				"production",
				"webhook",
				rawURL,
				"payout-secret",
			)
			if !errors.Is(err, outboundpolicy.ErrBlocked) {
				t.Fatalf("validateMarketplacePayoutConfig(%q) error = %v, want ErrBlocked", rawURL, err)
			}
		})
	}
}
```

These are syntax/literal-IP checks only; hostname DNS validation remains request-time behavior.

- [ ] **Step 6: Add the production provider builder**

Create `src/server/internal/http/marketplace_payout_provider.go`:

```go
package http

import (
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/marketplace"
)

func buildMarketplacePayoutProvider(cfg config.Config) marketplace.MarketplacePayoutProvider {
	switch strings.ToLower(strings.TrimSpace(cfg.MarketplacePayoutProvider)) {
	case "webhook":
		return marketplace.NewWebhookPayoutProvider(
			cfg.MarketplacePayoutWebhookURL,
			cfg.MarketplacePayoutWebhookSecret,
		)
	default:
		return nil
	}
}
```

Create tests:

```go
func TestBuildMarketplacePayoutProviderReturnsConfiguredWebhookProvider(t *testing.T) {
	t.Parallel()

	provider := buildMarketplacePayoutProvider(config.Config{
		MarketplacePayoutProvider:      "webhook",
		MarketplacePayoutWebhookURL:    "https://payments.example.com/payouts",
		MarketplacePayoutWebhookSecret: "secret",
	})
	if provider == nil || provider.Name() != "webhook" {
		t.Fatalf("provider = %#v, want webhook provider", provider)
	}
}

func TestBuildMarketplacePayoutProviderReturnsNilForLocalMode(t *testing.T) {
	t.Parallel()

	if provider := buildMarketplacePayoutProvider(config.Config{MarketplacePayoutProvider: "local"}); provider != nil {
		t.Fatalf("provider = %#v, want nil", provider)
	}
}
```

In `server.go`, add to the existing `RouterOptions`:

```go
MarketplacePayoutProvider: buildMarketplacePayoutProvider(cfg),
```

- [ ] **Step 7: Run config, payout, and HTTP tests**

Run:

```bash
cd src/server
gofmt -w internal/marketplace/payout_provider.go \
  internal/marketplace/payout_provider_test.go \
  internal/config/config.go \
  internal/config/config_test.go \
  internal/http/marketplace_payout_provider.go \
  internal/http/marketplace_payout_provider_test.go \
  internal/http/server.go
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/config ./internal/marketplace ./internal/http \
  -run 'MarketplacePayout|WebhookPayout|BuildMarketplacePayoutProvider|LoadRejects.*Payout' \
  -count=1 -v
```

Expected: PASS.

- [ ] **Step 8: Commit payout security and production wiring**

```bash
git add src/server/internal/marketplace/payout_provider.go \
  src/server/internal/marketplace/payout_provider_test.go \
  src/server/internal/config/config.go \
  src/server/internal/config/config_test.go \
  src/server/internal/http/marketplace_payout_provider.go \
  src/server/internal/http/marketplace_payout_provider_test.go \
  src/server/internal/http/server.go
git commit -m "feat: secure and wire marketplace payouts"
```

---

### Task 7: Add Static Security Gates And Documentation

**Files:**

- Create: `scripts/verify-outbound-security.sh`
- Modify: `scripts/check.sh`
- Modify: `.github/workflows/ci.yml`
- Create: `docs/architecture/outbound-http-security.md`
- Create: `docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md`
- Modify: `docs/audit/2026-07-10-module-capability-gap-matrix.md`

- [ ] **Step 1: Write the failing static gate**

Create `scripts/verify-outbound-security.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

fail() {
  echo "[outbound-security] $*" >&2
  exit 1
}

policy_file="$repo_root/src/server/internal/outboundpolicy/policy.go"
[[ -f "$policy_file" ]] || fail "shared outbound policy is missing"

targets=(
  "$repo_root/src/server/internal/agent/executor.go"
  "$repo_root/src/server/internal/workflow/node_executor.go"
  "$repo_root/src/server/internal/mcp/client.go"
  "$repo_root/src/server/internal/marketplace/payout_provider.go"
)

for target in "${targets[@]}"; do
  grep -Fq -- "internal/outboundpolicy" "$target" ||
    fail "$(basename "$target") does not import outboundpolicy"
done

violations=$(grep -En \
  'http\.DefaultClient\.Do|client = http\.DefaultClient|&http\.Client\{Timeout:' \
  "${targets[@]}" || true)
if [[ -n "$violations" ]]; then
  echo "$violations" >&2
  fail "direct default HTTP client patterns remain in guarded consumers"
fi

grep -Fq -- "MarketplacePayoutProvider: buildMarketplacePayoutProvider(cfg)" \
  "$repo_root/src/server/internal/http/server.go" ||
  fail "production server does not wire the configured marketplace payout provider"

grep -Fq -- "scripts/verify-outbound-security.sh" "$repo_root/scripts/check.sh" ||
  fail "check.sh does not execute the outbound security gate"

echo "[outbound-security] shared outbound policy is wired into guarded consumers."
```

Make it executable:

```bash
chmod +x scripts/verify-outbound-security.sh
```

- [ ] **Step 2: Run the gate and verify it fails before check.sh wiring**

Run:

```bash
bash scripts/verify-outbound-security.sh
```

Expected: FAIL until every target imports `outboundpolicy` and `server.go` wires the payout provider.

- [ ] **Step 3: Wire the gate into check.sh and CI**

Update the usage line:

```bash
Usage: bash scripts/check.sh [all|docs|web|server|relay-security|outbound-security|security]
```

Add:

```bash
run_outbound_security_checks() {
  echo "[check] Verifying outbound HTTP security boundary."
  bash "$repo_root/scripts/verify-outbound-security.sh"
}
```

Run it in:

- `all`, after Relay security;
- `outbound-security`, as its own target;
- `security`, before dependency security.

Add a CI release-gates step:

```yaml
- name: Verify outbound HTTP security boundary
  run: bash scripts/check.sh outbound-security
```

- [ ] **Step 4: Write the architecture contract**

Create `docs/architecture/outbound-http-security.md` with this content:

```markdown
# Outbound HTTP Security

## Contract

All customer- or administrator-configured HTTP destinations must use
`internal/outboundpolicy`. A consumer must not create its own default
`http.Client`, follow redirects without policy validation, or read a response
body without a fixed limit.

The strict external profile requires HTTPS, rejects URL userinfo, rejects
literal local/private/reserved/non-routable IP addresses, rejects a hostname
when any DNS answer is unsafe, validates again at dial time, dials the approved
IP rather than the unresolved hostname, permits at most three redirects, and
permits redirects only within the same scheme, host, and effective port.

## Threat Model

The policy covers literal private and metadata IPs, DNS names resolving to
private IPs, mixed safe/unsafe DNS answers, DNS rebinding between validation and
dial, IPv4-mapped IPv6, redirect pivots, cross-origin credential forwarding,
routing-header overrides, and unbounded response bodies.

## Profiles

- Agent custom API, Workflow HTTP, remote MCP, and Marketplace payout use the
  strict profile in every environment.
- Unit tests may allow only the exact `httptest` hostname they own; they must not
  enable all private networks.
- Relay delegates to the same policy. Outside production it preserves local
  provider development by allowing HTTP and private networks; production uses
  the strict profile.

## Consumer Limits

- Agent custom API response: 1 MiB.
- Workflow HTTP response: 1 MiB.
- Remote MCP JSON-RPC response: 4 MiB.
- Marketplace payout response: 1 MiB.

Workflow rejects routing and hop-by-hop request headers. Credential-bearing
request keys are protected by the existing workflow secretbox path, and only
`Content-Type`, `ETag`, `Retry-After`, and `X-Request-ID` are persisted from
HTTP responses.

## Extending The Policy

Before enabling another outbound consumer, add policy injection, an exact
timeout and response limit, literal-IP tests, fake-resolver tests, redirect
tests, sensitive-header tests, a static gate assertion, and repository-local
evidence. Publishing webhooks, observability sinks, web search providers, RPA,
and archive clients remain open consumers after this slice.

## Deployment Boundary

Application validation supplements but does not replace Kubernetes
NetworkPolicy, cloud egress controls, DNS policy, proxy governance, target
secret audit, and live endpoint evidence.
```

- [ ] **Step 5: Write implementation evidence and update the gap matrix**

Create `docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md`:

```markdown
# Shared Outbound Security Policy Evidence

Date: 2026-07-10

## Repository-Local Scope

This slice adds one outbound HTTP policy and applies it to Relay provider
compatibility wrappers, Agent custom API tools, Workflow HTTP nodes, remote MCP
JSON-RPC, and Marketplace payout dispatch. Production server construction now
injects the configured webhook payout provider.

## Security Properties

- HTTPS and userinfo policy is enforced by profile.
- Literal and DNS-resolved local/private/reserved addresses are rejected.
- Dialing uses the validated address set.
- Redirects are revalidated, bounded, and same-origin only.
- Workflow routing headers are rejected and credential keys use secretbox.
- Consumer response bodies have fixed size limits.
- Marketplace payout sends both the signed payout ID and `Idempotency-Key`.

## Verification

- `go test ./internal/outboundpolicy ./internal/relay/channel`: PASS.
- focused Agent, Workflow, MCP, Marketplace, Config, and HTTP tests: PASS.
- `bash scripts/check.sh outbound-security`: PASS.
- `bash scripts/check.sh server`: PASS.
- `bash scripts/check.sh docs`: PASS.
- `git diff --check`: PASS.

## Boundary

This is repository-local policy and wiring evidence. It does not prove target
Kubernetes egress policy, live DNS behavior, live MCP/provider connectivity,
payment reconciliation, or the final no-skip commercial verifier. Publishing
webhooks, observability sinks, web search, RPA, and archive clients remain open
outbound-policy consumers.
```

Before `## 21. 推荐执行顺序` in the gap matrix, add:

```markdown
### Shared Outbound Security Policy 本地切片状态

- Relay、Agent custom API、Workflow HTTP、remote MCP JSON-RPC 和 Marketplace
  payout 已统一到 `internal/outboundpolicy`；完成证据以对应实施提交和
  `docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md` 为准。
- 本地完成不提高模块目标环境评分；Kubernetes egress、live DNS、真实 MCP、
  payout reconciliation 和最终 no-skip verifier 仍是开放证据。
- 下一批待迁移消费者是 publishing webhook、observability alert sink、web
  search provider、RPA 和 archive sink。
```

- [ ] **Step 6: Run static and documentation checks**

Run:

```bash
bash scripts/check.sh outbound-security
bash scripts/check.sh docs
git diff --check
```

Expected: PASS.

- [ ] **Step 7: Commit gates and documentation**

```bash
git add scripts/verify-outbound-security.sh \
  scripts/check.sh \
  .github/workflows/ci.yml \
  docs/architecture/outbound-http-security.md \
  docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md \
  docs/audit/2026-07-10-module-capability-gap-matrix.md
git commit -m "chore: gate shared outbound security"
```

---

### Task 8: Run The Integrated Verification Bundle

**Files:**

- Verify all files changed in Tasks 1-7.
- Do not stage or modify:
  - `scripts/run-target-release-evidence.sh`
  - `scripts/run-target-release-evidence-fixtures.sh`

- [ ] **Step 1: Run all focused package tests**

```bash
cd src/server
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test \
  ./internal/outboundpolicy \
  ./internal/relay/channel \
  ./internal/agent \
  ./internal/workflow \
  ./internal/mcp \
  ./internal/marketplace \
  ./internal/config \
  ./internal/http \
  -run 'Outbound|ProviderUpstream|CustomAPI|HTTPNode|ClientRehydrates|ClientRejects|WebhookPayout|MarketplacePayout|BuildMarketplacePayoutProvider|LoadRejects.*Payout' \
  -count=1 -v
```

Expected: PASS with no new skips in the changed network-policy tests.

- [ ] **Step 2: Run compile and security gates**

From the repository root:

```bash
bash scripts/check.sh relay-security
bash scripts/check.sh outbound-security
bash scripts/check.sh security
bash scripts/check.sh server
```

Expected: PASS.

- [ ] **Step 3: Run documentation and diff checks**

```bash
bash scripts/check.sh docs
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Inspect the final worktree and commit history**

```bash
git status --short
git log --oneline -8
```

Expected:

- only the two pre-existing target-release runner files remain untracked;
- no implementation or documentation changes remain unstaged;
- Tasks 1-7 appear as separate focused commits.

- [ ] **Step 5: Record final verification output**

Append the exact commands, timestamps, PASS results, and remaining target-environment boundary to:

```text
docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md
```

If the evidence document changes after its earlier commit, create one final evidence-only commit:

```bash
git add docs/audit/agent-evidence/2026-07-10-shared-outbound-security-policy.md
git commit -m "docs: record outbound security verification"
```

## Completion Criteria

The plan is complete only when all of the following are true:

- `internal/outboundpolicy` has unit tests for syntax, literal IPs, mixed DNS answers, DNS-pinned dialing, redirects, mapped IPv6, routing headers, and response limits.
- Relay uses the shared implementation and preserves development versus production behavior.
- Agent custom API tools cannot reach unsafe destinations or return unbounded responses.
- Workflow HTTP nodes cannot reach unsafe destinations, override routing headers, pivot through redirects, or return unbounded responses.
- Remote MCP requests cannot reach unsafe destinations, leak bearer tokens through cross-origin redirects, or return unbounded responses.
- Marketplace payout dispatch cannot reach unsafe destinations, uses bounded responses, sends an idempotency key, and is actually injected by production server construction.
- `scripts/verify-outbound-security.sh` passes and runs in CI.
- Focused tests, server compile checks, docs checks, and `git diff --check` pass.
- The two pre-existing untracked target-release runner files remain untouched.
- Evidence explicitly distinguishes repository-local policy proof from target deployment egress proof.

# Relay Provider Upstream Runtime SSRF Guard

Date: 2026-07-04

## Scope

This slice hardens the runtime Relay provider adapter path so production-mode provider calls fail before dialing unsafe upstream URLs, even if a channel row, request override, or DNS resolution result points at local/private infrastructure.

## Runtime Changes

- Added `validateProviderUpstreamURL` in `src/server/internal/relay/channel/upstream_guard.go`.
- Added `newProviderHTTPClient`, `providerUpstreamTransport`, and guarded `DialContext` wiring so production provider calls validate both the request URL and the resolved dial address before network I/O.
- Applied the guard to OpenAI-compatible `DoRequest`, `ListModels`, and `CheckBalance`.
- Applied the guard to Claude `DoRequest` and `ListModels`.
- Applied the guard to Gemini `DoRequest` and `ListModels`.
- Applied the guard to Vertex `DoRequest`.
- Applied the guard to Bedrock `DoRequest`.

## Production Behavior

When `APP_ENV=production`, explicit provider upstream URLs now reject:

- empty URLs
- missing hosts
- embedded credentials
- non-HTTPS URLs
- literal IP hosts that are loopback, private, link-local, multicast, or unspecified
- DNS-resolved addresses that are loopback, private, link-local, multicast, or unspecified

Development and test environments remain permissive so local `httptest` and localhost provider workflows continue to work.

## RED Evidence

Command:

```bash
cd src/server && go test ./internal/relay/channel -run TestOpenAIAdapter_DoRequestRejectsUnsafeOverrideURL -count=1 -v
```

Observed failure before the runtime guard:

```text
expected unsafe provider upstream URL error, got Post "http://169.254.169.254/latest/meta-data": dial tcp 169.254.169.254:80: connect: no route to host
```

This proved the adapter attempted to dial the metadata-service URL instead of rejecting it before network I/O.

Additional RED command for the transport-level guard:

```bash
cd src/server && go test ./internal/relay/channel -run TestProductionProviderHTTPClientRejectsUnsafeResolvedAddress -count=1 -v
```

Observed failure before the guarded provider HTTP client existed:

```text
internal/relay/channel/upstream_guard_test.go:75:12: undefined: newProviderHTTPClient
FAIL	oblivious/server/internal/relay/channel [build failed]
```

This proved runtime provider calls did not yet have a shared production-only transport hook for resolving and rejecting unsafe dial targets.

## GREEN Evidence

Focused command:

```bash
cd src/server && go test ./internal/relay/channel -run 'TestProductionProviderHTTPClientRejectsUnsafeResolvedAddress|TestOpenAIAdapter_DoRequestRejectsUnsafeOverrideURL|TestValidateProviderUpstreamURLRejectsUnsafeProductionURLs|TestValidateProviderUpstreamURLAllowsLocalDevelopmentURLs' -count=1 -v
```

Result:

```text
PASS
ok  	oblivious/server/internal/relay/channel	0.004s
```

Full relay/channel command:

```bash
cd src/server && go test ./internal/relay/channel -count=1 -v
```

Result:

```text
PASS
ok  	oblivious/server/internal/relay/channel	0.006s
```

Cross-package command:

```bash
cd src/server && go test ./internal/relay/channel ./internal/admin ./internal/http -count=1 -timeout 300s
```

Result:

```text
ok  	oblivious/server/internal/relay/channel	0.009s
ok  	oblivious/server/internal/admin	0.011s
ok  	oblivious/server/internal/http	1.590s
```

## Remaining Boundary

This slice blocks unsafe literal-IP provider upstreams and unsafe DNS-resolved dial targets in production before request execution. It still relies on the process resolver and per-request/per-dial validation rather than a deployment-level egress firewall; hardened target environments should keep network-policy and allowlist controls in place before claiming full SSRF closure for arbitrary custom upstream hosts.

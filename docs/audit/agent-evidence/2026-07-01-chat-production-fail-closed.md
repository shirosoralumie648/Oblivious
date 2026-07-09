# Agent Evidence: Chat Production Fail Closed

Date: 2026-07-01

Agent: Codex

Commit: working tree

## Runtime Claim

Production Chat, Agent, and Workflow LLM gateway construction no longer routes to the local demo reply generator when Relay is disabled. If a production config is manually constructed with `RelayEnabled=false`, the runtime uses an unavailable gateway that returns `chat.ErrModelGatewayUnavailable` instead of producing `"Assistant reply..."` text.

Non-production Relay-disabled configs still keep the local demo generator for development only.

2026-07-02 follow-up: the standalone `cmd/agent` runtime now follows the same production fail-closed rule. This closes the separate microservice entrypoint gap where `buildAgentGateway` could previously construct a local demo gateway when Relay was disabled.

Do not count docs, mocks, fakes, local-only fallbacks, fixtures, generated OpenAPI, or tests as runtime evidence.

## Reference Inputs

```text
docs/audit/reference-to-oblivious-product-delta-v3.md:20 - P0 requires fail-closed live model response with no production demo reply fallback.
docs/audit/product-roadmap-v2-from-reference.md:31 - P0 private beta loop must run real metered AI product flows without demo or fake paths.
docs/audit/current-implementation-depth.md:14 - Prior current-state audit identified local demo fallback as a Chat incompleteness.
```

## Oblivious Files Changed

```text
src/server/internal/http/router.go
src/server/internal/http/server.go
src/server/internal/http/chat_gateway_config_test.go
src/server/cmd/agent/main.go
src/server/cmd/agent/main_test.go
docs/audit/agent-evidence/2026-07-01-chat-production-fail-closed.md
```

## Contract Changes

None. This hardens runtime construction behavior without changing API, database, environment variable, billing, request-log, or tenant/RBAC contracts.

## Verification Commands

```text
command: bash -lc 'export PATH="/c/Program Files/Go/bin:$PATH"; cd src/server && go test ./internal/http -run "TestNewConfiguredChatGateways|TestGatewayProxy" -count=1 -v'
result: PASS

command: bash -lc 'export PATH="/c/Program Files/Go/bin:$PATH"; cd src/server && go test ./internal/http -run "TestNewConfiguredChatGateways|TestRouteSurfaceRuntimeAPIRoutesAreDocumentedInManifestWithoutDatabase|TestRouteSurfaceDeclaredRouteRegistrarsAreMountedWithoutDatabase|TestRouteSurfaceManifestSecurityGuardsWithoutDatabase" -count=1 -v'
result: PASS

command: bash -lc 'export PATH="/c/Program Files/Go/bin:$PATH"; cd src/server && go test ./cmd/agent -run "Test(BuildAgentGateway|AgentRelayBaseURL|SelectAgentDatabaseURL)" -count=1'
result: PASS
```

## Runtime Evidence IDs

Not applicable. The hardened behavior is a deterministic process-local gateway construction path and is verified by direct runtime unit tests rather than persisted request, usage, billing, or workflow records.

## Failure Evidence

`TestNewConfiguredChatGatewaysProductionRelayDisabledFailsClosed` constructs a production config with `RelayEnabled=false` and verifies:

```text
chat reply generator: returns chat.ErrModelGatewayUnavailable and no demo text
agent gateway GenerateReply: returns chat.ErrModelGatewayUnavailable and no demo text
agent gateway GenerateReplyStream: returns chat.ErrModelGatewayUnavailable and no demo text
```

`TestBuildAgentGatewayProductionRelayDisabledFailsClosed` verifies the same behavior for the standalone Agent service entrypoint:

```text
standalone agent gateway GenerateReply: returns chat.ErrModelGatewayUnavailable and no demo text
standalone agent gateway GenerateReplyStream: returns chat.ErrModelGatewayUnavailable and no demo text
```

## Unsupported / Deferred Surfaces

```text
Realtime Relay WebSocket origin/auth/billing hardening remains a separate P0 security item.
Secretbox production plaintext denial is now covered by `docs/audit/agent-evidence/2026-07-02-secretbox-production-plaintext-deny.md`; target secret rotation and audit proof remain separate release evidence.
Provider-authoritative Chat token usage and cost reconciliation remains broader Relay billing evidence work.
```

## Known Residual Risk

This does not prove a deployed production Relay provider is healthy or that every provider account has valid credentials. It only proves production runtime construction cannot fall back to local demo assistant text when Relay is disabled or bypassed at the config-loader layer.

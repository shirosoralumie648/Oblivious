# Agent Evidence: Relay chat true streaming

Date: 2026-07-01

Agent: main

Commit: pending

## Runtime Claim

Relay chat streaming now proxies upstream SSE bytes as they arrive instead of waiting for the full provider response body before writing to the client.

For stream requests, the Chat handler marks the route context as trusted streaming, installs a provider chunk callback, writes SSE headers, flushes each upstream chunk through Gin's response writer, and still captures the complete stream body for provider usage parsing after upstream completion.

`Router.RouteWithBilling` writes a `pending` usage record before invoking the provider callback for trusted streaming requests. If that pending write fails, Relay refunds owned quota/API-token preauthorization and returns `usage_recording_failed` before any provider bytes can be sent.

## Reference Inputs

```text
docs/audit/product-roadmap-v2-from-reference.md - P0 requires Chat + Relay usage/billing evidence.
src/server/internal/relay/handler/chat.go - commercial chat handler path.
src/server/internal/relay/handler/common.go - provider adapter HTTP execution path.
src/server/internal/relay/router.go - billing, quota, settlement, and usage evidence flow.
src/server/internal/relay/channel/openai_adapter.go - streaming usage parsing from SSE data lines.
```

## Oblivious Files Changed

```text
src/server/internal/relay/handler/chat.go
src/server/internal/relay/handler/common.go
src/server/internal/relay/handler/chat_test.go
src/server/internal/relay/router.go
src/server/internal/relay/router_test.go
src/server/internal/relay/types/types.go
src/server/internal/relay/usage.go
docs/audit/agent-evidence/2026-07-01-relay-chat-true-streaming.md
```

## Contract Changes

Trusted streaming route context:

```text
types.WithTrustedStreaming(ctx, true)
```

Relay usage status:

```text
pending
success
error
```

## Verification Commands

```text
command: git diff --check -- src/server/internal/relay/handler/chat.go src/server/internal/relay/handler/common.go src/server/internal/relay/handler/chat_test.go src/server/internal/relay/router.go src/server/internal/relay/router_test.go src/server/internal/relay/types/types.go src/server/internal/relay/usage.go docs/audit/agent-evidence/2026-07-01-relay-chat-true-streaming.md
result: passed; Git reported LF-to-CRLF warnings only.

command: go test ./internal/relay/... -run 'TestChatStreamingProxiesSelectedProviderSSEThroughBillingRoute|TestChatStreamingFlushesFirstProviderChunkBeforeUpstreamCompletes|TestRouterRouteWithBillingRecordsPendingUsageBeforeStreamingProvider|TestRouterRouteWithBillingFailsClosedWhenUsageRecordingFails' -count=1 -v
result: blocked; Go is not on PATH. Error: /usr/bin/bash: line 1: go: command not found.

command: gofmt -w src/server/internal/relay/handler/chat.go src/server/internal/relay/handler/common.go src/server/internal/relay/handler/chat_test.go src/server/internal/relay/router.go src/server/internal/relay/router_test.go src/server/internal/relay/types/types.go src/server/internal/relay/usage.go
result: blocked; gofmt is not on PATH. Error: /usr/bin/bash: line 1: gofmt: command not found.
```

## Runtime Evidence IDs

```text
request_id: req_stream
organization_id: org_stream
user_id: user_stream
api_key_id: tok_stream
channel_id: ch-live-stream
```

## Failure Evidence

`TestRouterRouteWithBillingRecordsPendingUsageBeforeStreamingProvider` verifies that the pending usage record exists before the provider callback can emit any stream bytes.

`TestRouterRouteWithBillingFailsClosedWhenUsageRecordingFails` remains the non-stream fail-closed guard for post-provider usage recording failure.

## Unsupported / Deferred Surfaces

Historical note: this evidence originally listed standalone `cmd/relay` production startup as deferred. A later deployment hardening pass completed DB-backed channel loading, trusted auth, quota, pricing, rate limits, usage recording, and request-log wiring, and `scripts/verify_deployment_operations_contract.py` now guards that contract.

## Known Residual Risk

For streaming responses, a final usage write can fail after bytes have already been sent to the client. The route now relies on the pre-provider `pending` usage record and continues settlement, but a complete commercial ledger should make usage records updateable from `pending` to a terminal final status instead of writing separate pending and final records.

This change has not been compiled in this environment because Go and gofmt are unavailable on PATH.

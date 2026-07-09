# Agent Evidence: Relay usage recording fail-closed

Date: 2026-07-01

Agent: main

Commit: pending

## Runtime Claim

`Router.RouteWithBilling` no longer returns a successful provider response when durable usage recording fails on the post-provider path.

After a provider response is available and before quota/API-token settlement, Relay now attempts to persist the usage record. If that write fails, Relay refunds any quota and API-token preauthorization it owns for the attempt, records runtime metrics with a `500` failure, and returns `usage_recording_failed`.

This prevents the P0 commercial break where a provider call could be settled and returned successfully while missing durable usage evidence.

## Reference Inputs

```text
docs/audit/product-roadmap-v2-from-reference.md - P0 requires request log and usage ledger evidence for every billable call.
src/server/internal/relay/router.go - RouteWithBilling provider, quota, settlement, and usage flow.
src/server/internal/usage/store.go - durable usage_records writer.
src/server/internal/http/server.go - monolith wires usage.NewSQLRecorder into Relay.
```

## Oblivious Files Changed

```text
src/server/internal/relay/router.go
src/server/internal/relay/router_test.go
src/server/internal/relay/relay_test.go
docs/audit/agent-evidence/2026-07-01-relay-usage-recording-fail-closed.md
```

## Contract Changes

Successful billed Relay responses require durable usage recording before settlement.

If usage recording fails, the Relay response becomes an internal metering failure:

```text
status: 500
error_code: usage_recording_failed
```

## Verification Commands

```text
command: git diff --check -- src/server/internal/relay/router.go src/server/internal/relay/router_test.go src/server/internal/relay/relay_test.go
result: passed; Git reported LF-to-CRLF warnings only.

command: go test ./internal/relay -run 'TestRouterRouteWithBillingFailsClosedWhenUsageRecordingFails|TestRouterRouteWithBillingAppliesTrustedUserGroupPricingMultiplier' -count=1 -v
result: blocked; go is not on PATH. Error: /usr/bin/bash: line 1: go: command not found.
```

## Runtime Evidence IDs

The new test uses:

```text
request_id: req_metered
organization_id: org_metered
user_id: user_metered
api_key_id: tok_metered
```

## Failure Evidence

`TestRouterRouteWithBillingFailsClosedWhenUsageRecordingFails` injects `usage store unavailable` from the usage logger after a successful provider response.

Expected behavior:

- no provider response is returned to the caller;
- `RouterError.Code == 500`;
- `RouterError.ErrorCode == "usage_recording_failed"`;
- quota settlement is skipped;
- API-token settlement is skipped;
- quota preauthorization refund is attempted;
- API-token preauthorization refund is attempted.

## Unsupported / Deferred Surfaces

True upstream SSE pass-through is tracked by `docs/audit/agent-evidence/2026-07-01-relay-chat-true-streaming.md`.

Historical note: this evidence originally listed standalone `cmd/relay` production wiring as deferred. A later deployment hardening pass completed that wiring, and `scripts/verify_deployment_operations_contract.py` now guards the `cmd/relay` DB, migration, quota, API-token, usage, and rate-limit contract.

## Known Residual Risk

This change records usage before settlement. If a later settlement step fails after usage is recorded, the flow still relies on the existing settlement failure path and may leave both a success-attempt record and a settlement-error record. A fuller commercial ledger should add pending/final usage states or an updateable usage lifecycle.

The change has not been compiled in this environment because Go and gofmt are unavailable on PATH.

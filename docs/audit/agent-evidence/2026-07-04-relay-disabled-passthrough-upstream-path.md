# Relay Disabled Passthrough Upstream Path

Date: 2026-07-04

## Runtime Claim

Relay passthrough handlers for currently production-disabled Assistants/Threads/Runs and Fine-tuning routes now construct exact OpenAI-compatible upstream paths when used outside the production route-policy gate. This removes malformed duplicated paths such as `/v1/assistants/v1/assistants` and `/v1/fine_tuning/jobs/v1/fine_tuning/jobs`.

The same path builder now preserves provider base paths that already include a version prefix, such as `/proxy/v2`, by trimming only the endpoint suffix and the local `/v1` request prefix. These surfaces remain `DisabledInProduction`; this slice fixes local/development passthrough correctness and prevents future enablement work from inheriting a broken upstream path.

## Reference Inputs

- `reference/CLIProxyAPI/README_CN.md` - OpenAI-compatible proxy surface used as route-compatibility reference.
- `reference/CliRelay/README_CN.md` - Relay passthrough compatibility reference.
- `docs/release/relay-route-table.md` - Current Oblivious commercial boundary for disabled passthrough routes.

## Oblivious Files Changed

```text
src/server/internal/relay/handler/assistants.go
src/server/internal/relay/handler/assistants_test.go
src/server/internal/relay/handler/fine_tuning.go
src/server/internal/relay/handler/fine_tuning_test.go
src/server/internal/relay/handler_new/assistants.go
src/server/internal/relay/handler_new/fine_tuning.go
docs/audit/agent-evidence/2026-07-04-relay-disabled-passthrough-upstream-path.md
```

## Contract Changes

None. No route is newly enabled for production and no API/schema/config/database contract changed.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run 'Test(FineTuningPassthroughUsesExactOpenAIPath|AssistantsPassthroughUsesExactOpenAIPath|Production.*RoutesFailClosedBeforeHandler)' -count=1 -v
result: PASS; focused RED tests now pass and production-disabled route guard tests still pass.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run 'Test(FineTuningPassthroughUsesExactOpenAIPath|AssistantsPassthroughUsesExactOpenAIPath|FineTuningPassthroughPreservesVersionedUpstreamBasePath|AssistantsPassthroughPreservesVersionedUpstreamBasePath)' -count=1 -v
result: PASS; exact-path tests and versioned upstream base-path tests pass.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -count=1
result: PASS; ok oblivious/server/internal/relay/handler 0.030s
```

## Failure Evidence

Before the fix, the new focused tests failed with duplicated upstream paths:

```text
upstream path[0] = "/v1/assistants/v1/assistants", want "/v1/assistants"
upstream path[0] = "/v1/fine_tuning/jobs/v1/fine_tuning/jobs", want "/v1/fine_tuning/jobs"
```

The versioned upstream regression tests also failed before the second path-builder fix:

```text
upstream path = "/proxy/v2/assistants/v1/threads/thread_123/runs/run_123", want /proxy/v2/threads/thread_123/runs/run_123
upstream path = "/proxy/v2/fine_tuning/jobs/v1/fine_tuning/jobs/ftjob_123/events", want /proxy/v2/fine_tuning/jobs/ftjob_123/events
```

## Unsupported / Deferred Surfaces

- `POST /v1/fine_tuning/jobs`
- `GET /v1/fine_tuning/jobs`
- `GET /v1/fine_tuning/jobs/:id`
- `POST /v1/fine_tuning/jobs/:id/cancel`
- `GET /v1/fine_tuning/jobs/:id/events`
- `POST /v1/assistants`
- `GET /v1/assistants`
- `GET /v1/assistants/:id`
- `POST /v1/threads`
- `GET /v1/threads/:id`
- `POST /v1/threads/:id/runs`
- `GET /v1/threads/:id/runs/:rid`
- `POST /v1/threads/:id/runs/:rid/submit`

## Known Residual Risk

This is not commercial enablement evidence for Assistants, Threads, Runs, or Fine-tuning. Production remains fail-closed until lifecycle billing, usage capture, tenant audit, provider reconciliation, and target-runtime evidence are implemented.

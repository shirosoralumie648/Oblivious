# Relay Batch Request Model Routing

Date: 2026-07-04

## Runtime Claim

Active Relay Batch submit now requires the request body's `model` field, carries that model through billing routing and batch polling registration, and rejects missing-model requests before billing or upstream dispatch. Active Batch list/get passthrough now routes to the adapter's `/v1/batches` endpoints without duplicating `/v1/batch`, while preserving upstream base-path prefixes such as `/proxy`. This closes the old active-handler hardcoded/default-model and duplicate-path defects without changing the production policy: Batch routes remain disabled for commercial release until durable async polling, settlement/refund, audit, and usage-capture lifecycle proof exists.

## Reference Inputs

```text
reference/CLIProxyAPI/README.md - OpenAI-compatible proxy surface used as route-compatibility target.
reference/CliRelay/README.md - Relay-style OpenAI-compatible gateway reference; used for Batch surface parity framing, not copied as lifecycle proof.
docs/audit/stub-hardcoded-todo-report.md - Live local P0 entry identifying active Batch hardcoded model.
docs/release/relay-route-table.md - Local route-policy authority for keeping Batch production-disabled.
```

## Oblivious Files Changed

```text
src/server/internal/relay/handler/batch.go
src/server/internal/relay/handler/batch_test.go
docs/audit/stub-hardcoded-todo-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/agent-evidence/2026-07-04-relay-batch-request-model-routing.md
```

## Contract Changes

None for production release policy. Non-production direct handler calls now fail with `400` and `batch_model_required` when submit `model` is missing. Batch list/get passthrough preserves the public `/v1/batches` and `/v1/batches/:id` contract while correcting upstream URL construction.

## Verification Commands

```text
command: cd src/server && go test ./internal/relay/handler -run 'TestBatchSubmitUsesRequestModelForBillingAndPolling' -count=1
result: RED before implementation; failed with billing model = "gpt-4o", want request model "gpt-4.1-mini".

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchSubmitUsesRequestModelForBillingAndPolling|TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID|TestBatchSubmitRoutesThroughBillingRouter' -count=1
result: GREEN after implementation; `ok oblivious/server/internal/relay/handler 0.011s` and later focused run with `TestBatchPoliciesDeclareCommercialReleaseBlockers` also passed.

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchSubmitRejectsMissingModelBeforeBillingOrUpstream' -count=1
result: RED before missing-model fail-fast implementation; failed with status 500 and `relay: no available channel` because the handler entered the default billing path.

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchSubmitRejectsMissingModelBeforeBillingOrUpstream|TestBatchSubmitUsesRequestModelForBillingAndPolling|TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID|TestBatchSubmitRoutesThroughBillingRouter|TestBatchPoliciesDeclareCommercialReleaseBlockers|TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath|TestRealtimeRejectsMissingModelBeforeUpstreamDial|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler' -count=1
result: GREEN after implementation; ok oblivious/server/internal/relay/handler 0.013s.

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchListAndGetUseBatchesEndpointWithoutDuplicateBatchPath' -count=1
result: RED before passthrough URL implementation; failed with upstream paths `/v1/batch/v1/batches` and `/v1/batch/v1/batches/batch_123`.

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchListPreservesUpstreamBasePathPrefix' -count=1
result: RED before base-path preservation implementation; failed with upstream path `/v1/batches`, want `/proxy/v1/batches`.

command: cd src/server && go test ./internal/relay/handler -run 'TestBatchListPreservesUpstreamBasePathPrefix|TestBatchListAndGetUseBatchesEndpointWithoutDuplicateBatchPath|TestBatchSubmitRejectsMissingModelBeforeBillingOrUpstream|TestBatchSubmitUsesRequestModelForBillingAndPolling|TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID|TestBatchSubmitRoutesThroughBillingRouter|TestBatchPoliciesDeclareCommercialReleaseBlockers|TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath|TestRealtimeRejectsMissingModelBeforeUpstreamDial|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler' -count=1
result: GREEN after passthrough URL implementation; ok oblivious/server/internal/relay/handler 0.017s.
```

## Runtime Evidence IDs

```text
request_id: req_batch_model
batch_id: batch_model_specific
model: gpt-4.1-mini
api_type: batch
error_code: batch_model_required
upstream_list_path: /v1/batches
upstream_get_path: /v1/batches/batch_123
upstream_prefixed_list_path: /proxy/v1/batches
```

## Failure Evidence

The request-model regression test failed before implementation because `RouteWithBilling` received `gpt-4o` instead of the request model. The missing-model regression test then failed because the handler entered billing with the old default path instead of returning `batch_model_required`. The list/get passthrough regression tests then failed because active passthrough produced duplicate `/v1/batch/v1/batches` paths and initially dropped upstream base-path prefixes. Together these prove the tests catch active default-model and upstream-path defects rather than only asserting existing behavior.

## Unsupported / Deferred Surfaces

- Batch production enablement remains deferred.
- `GET /v1/batches` and `GET /v1/batches/:id` now route to the correct upstream endpoint shape, but still do not prove a durable commercial polling lifecycle.
- `src/server/internal/relay/handler_new/batch.go` remains a stale alternate handler and must not be counted as commercial runtime evidence.

## Known Residual Risk

Batch must remain disabled in production until prebill, durable async polling, idempotent completion settlement/refund, audit retention, usage capture, provider/result reconciliation, and target release evidence are implemented and verified.

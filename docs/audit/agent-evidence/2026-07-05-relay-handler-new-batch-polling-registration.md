# Relay Handler New Batch Polling Registration Evidence

Date: 2026-07-05

## Summary

`handler_new` Batch submit now has the same local polling registration seam used by the active Batch handler. When a polling registrar is injected, successful upstream Batch creation must produce a registration task that contains the upstream batch id, request id, trusted identity context, model, API type, billing session id, and preauthorized amounts. If the upstream response does not include a batch id, the handler fails closed with `batch_polling_registration_failed` instead of returning a locally successful response that cannot be followed by async settlement.

## Code Changes

- `src/server/internal/relay/handler_new/batch.go`
  - Added `BatchPollingRegistration`, `BatchPollingRegistrar`, and `WithPollingRegistrar`.
  - Calls `registerBatchPolling` after successful routed and direct upstream Batch submit responses.
  - Extracts upstream batch id from response JSON.
  - Carries trusted user, organization, API token, feature type, request id, model, API type, billing session, and preauthorization amounts into the registration task.
  - Fails closed with `batch_polling_registration_failed` when an injected registrar cannot receive a valid upstream batch id.
- `src/server/internal/relay/handler_new/batch_test.go`
  - Adds `TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID`.
  - Adds `TestBatchSubmitFailsClosedWhenPollingRegistrationCannotExtractBatchID`.

## Verification

```text
go test ./internal/relay/handler_new -run 'TestBatchSubmitRoutesThroughBillingRouter|TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID|TestBatchSubmitFailsClosedWhenPollingRegistrationCannotExtractBatchID' -count=1 -v
```

Result: PASS

```text
go test ./internal/relay/handler_new -count=1
```

Result: PASS

## Remaining Batch Gaps

- Production Batch routes remain disabled by default.
- Commercial readiness still requires durable target polling jobs, completion settlement/refund finalizers, request-log/audit joins, authoritative usage capture, operator evidence, and target release artifacts proving the lifecycle.

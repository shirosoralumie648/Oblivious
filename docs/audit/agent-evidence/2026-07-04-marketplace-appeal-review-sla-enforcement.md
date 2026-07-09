# Marketplace Appeal Review SLA Enforcement Evidence

Date: 2026-07-04

## Scope

Close the Marketplace moderation operations gap where `appeal_pending` review items were visible in the admin queue but were not part of SLA metadata and alert enforcement.

## Implementation

- Extended `AddReviewSLA` so both `pending_review` and `appeal_pending` agents receive SLA metadata.
- Extended `EnforceReviewSLAs` to scan both review queues:
  - `pending_review`
  - `appeal_pending`
- SLA alert payloads now include `reviewStatus` so operators can distinguish initial reviews from appeal reviews in alert routing and incident timelines.
- Kept duplicate alert suppression keyed by agent/status/SLA status, so repeated enforcement does not spam the same alert.

## RED Evidence

Before implementation:

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestServiceEnforceReviewSLAsIncludesAppealPendingQueue' -count=1 -v

=== RUN   TestServiceEnforceReviewSLAsIncludesAppealPendingQueue
    service_test.go:812: expected pending and appeal queues to be scanned and alerted, got {Scanned:2 Alerted:1}
--- FAIL: TestServiceEnforceReviewSLAsIncludesAppealPendingQueue
FAIL
```

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestServiceEnforceReviewSLAs(AlertsDueSoonAndOverdueOnce|IncludesAppealPendingQueue)' -count=1 -v

=== RUN   TestServiceEnforceReviewSLAsAlertsDueSoonAndOverdueOnce
--- PASS: TestServiceEnforceReviewSLAsAlertsDueSoonAndOverdueOnce
=== RUN   TestServiceEnforceReviewSLAsIncludesAppealPendingQueue
--- PASS: TestServiceEnforceReviewSLAsIncludesAppealPendingQueue
PASS
ok oblivious/server/internal/marketplace 0.007s
```

## Remaining Boundary

- This is repo-local service-level proof. Full target-runtime proof still requires a deployed database and alert sink environment exercising the scheduled/admin SLA enforcement path against real `appeal_pending` rows.

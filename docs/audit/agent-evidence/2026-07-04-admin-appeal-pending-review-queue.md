# Admin Appeal Pending Review Queue

Date: 2026-07-04

## Scope

Closed the next repository-local Marketplace governance gap after `appeal_pending` was introduced: Admin review operators can now query and see the appeal queue instead of only seeing initial `pending_review` submissions.

## Changed Files

- `src/server/internal/marketplace/store.go`
- `src/server/internal/admin/store.go`
- `src/server/internal/admin/service.go`
- `src/server/internal/http/admin_handler.go`
- `src/server/internal/http/admin_marketplace_handler_test.go`
- `src/server/internal/marketplace/service_test.go`
- `src/web/src/types/admin.ts`
- `src/web/src/components/shared/StatusBadge.tsx`
- `src/web/src/routes/admin/AdminReviewsPage.tsx`
- `src/web/src/routes/admin/AdminReviewsPage.test.tsx`
- `docs/api/openapi.yaml`

## Runtime Fixes

- Added `SQLStore.ListReviewQueue(ctx, status, limit, offset)` so Marketplace review queues can be loaded by supported queue state.
- Kept `ListPendingReviews` compatible by delegating it to `ListReviewQueue(..., pending_review, ...)`.
- Threaded the review queue status through Admin store, service, and HTTP handler.
- Changed `/api/v1/admin/reviews?status=appeal_pending` to return `appeal_pending` agents instead of an empty response.
- Preserved existing behavior where blank status and `pending` alias map to `pending_review`.
- Added Admin UI support for `appeal_pending` in the status type, shared `StatusBadge`, status label, and review status filter.
- Updated `MarketplacePublishedAgent.status` OpenAPI enum to include `appeal_pending`.

## RED Evidence

Before implementation, the focused Admin handler test failed because the handler rejected `status=appeal_pending` as an unsupported review queue filter and returned an empty response:

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestAdminHandlerListsAppealPendingMarketplaceReviews' -count=1 -v

expected appeal pending review in response, got {"ok":true,"data":{"reviews":[],"total":0},"error":null}
```

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http ./internal/admin ./internal/marketplace -run 'TestAdminHandlerListsAppealPendingMarketplaceReviews|TestAdminHandlerListsReviewsWithMarketplaceReviewSLA|TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue|TestServiceListPendingReviewsAddsReviewSLAForStandardAndVIPPublishers' -count=1 -v
```

Observed result:

```text
--- PASS: TestAdminHandlerListsReviewsWithMarketplaceReviewSLA (0.00s)
--- PASS: TestAdminHandlerListsAppealPendingMarketplaceReviews (0.00s)
PASS
ok  	oblivious/server/internal/http	0.027s
PASS
ok  	oblivious/server/internal/admin	0.013s [no tests to run]
--- PASS: TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue (0.00s)
--- PASS: TestServiceListPendingReviewsAddsReviewSLAForStandardAndVIPPublishers (0.00s)
PASS
ok  	oblivious/server/internal/marketplace	0.013s
```

```text
cd src/web && npm test -- --run src/routes/admin/AdminReviewsPage.test.tsx -t "(loads appeal pending agents from the review queue filter|renders pending agents with owner)"
```

Observed result:

```text
Test Files  1 passed (1)
Tests  2 passed | 11 skipped (13)
```

```text
bash scripts/verify-openapi-contract.sh
```

Observed result:

```text
[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented.
```

## Remaining Boundary

This makes the Admin review queue aware of appeal submissions, but it does not complete Marketplace governance for commercial release. Remaining work still includes reviewer assignment, appeal accept/reject decision history, appeal SLA escalation, abuse/ranking controls, and target-runtime policy operations evidence.

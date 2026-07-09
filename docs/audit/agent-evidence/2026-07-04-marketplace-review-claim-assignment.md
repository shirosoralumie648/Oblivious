# Marketplace Review Claim Assignment Evidence

Date: 2026-07-04

## Scope

Add a first-class admin review claim path so Marketplace moderation queues expose who is actively reviewing an agent before approve/reject/needs-changes decisions.

## Implementation

- Added `GovernanceService.AssignReview`:
  - requires reviewer actor and agent ID
  - only allows `pending_review` or `appeal_pending`
  - rejects claims by a different reviewer when the latest assignment already belongs to someone else
  - allows the same reviewer to refresh their claim
  - records `marketplace_governance_events.action = review_assign`
  - stores `reviewerUserId` in governance event metadata
- Extended review queue loading:
  - `ListReviewQueue` now reads latest `review_assign` event per agent
  - review queue responses include `reviewerUserId`
  - no new table column is required; the assignment is append-only governance evidence
- Added admin review claim route:
  - `POST /api/v1/admin/reviews/{agentId}/claim`
  - returns `{ "status": "claimed" }`
  - requires admin session plus CSRF
- Updated Admin Review Queue UI:
  - shows `Reviewer: <id>` or `Reviewer: Unassigned`
  - adds a claim action per review row
- Updated OpenAPI, route manifest, and contract verifier for the new route and `claimed` response status.

## RED Evidence

Before implementation:

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernance(AssignReviewSQLRecordsReviewerAssignment|AssignReviewRejectsApprovedAgent|MigrationAllowsRuntimeReviewActions)' -count=1 -v

service.AssignReview undefined
FAIL oblivious/server/internal/marketplace [build failed]
```

```text
npm test -- --run src/features/admin/api.test.ts -t "claims marketplace reviews through the admin review endpoint"

TypeError: api.claimReview is not a function
```

```text
npm test -- --run src/routes/admin/AdminReviewsPage.test.tsx -t "renders assigned reviewer and claims pending reviews"

Unable to find an element with the text: Reviewer: reviewer_user
```

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace ./internal/admin ./internal/http -run 'TestGovernance(AssignReviewSQLRecordsReviewerAssignment|AssignReviewRejectsApprovedAgent|MigrationAllowsRuntimeReviewActions)|TestAdminHandler(ClaimsMarketplaceReview|ListsAssignedMarketplaceReviewReviewer)|TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase' -count=1 -v

--- PASS: TestGovernanceMigrationAllowsRuntimeReviewActions
--- PASS: TestGovernanceAssignReviewSQLRecordsReviewerAssignment
--- PASS: TestGovernanceAssignReviewRejectsApprovedAgent
--- PASS: TestAdminHandlerListsAssignedMarketplaceReviewReviewer
--- PASS: TestAdminHandlerClaimsMarketplaceReview
--- PASS: TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase/claim_review
PASS
ok oblivious/server/internal/marketplace 0.013s
ok oblivious/server/internal/http 0.030s
```

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace ./internal/admin ./internal/http -run 'TestGovernanceAssignReview(RejectsClaimByAnotherReviewer|AllowsSameReviewerToRefreshClaim|SQLRecordsReviewerAssignment|RejectsApprovedAgent)|TestGovernanceMigrationAllowsRuntimeReviewActions|TestAdminHandler(ClaimsMarketplaceReview|ListsAssignedMarketplaceReviewReviewer)|TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase' -count=1 -v

--- PASS: TestGovernanceAssignReviewRejectsClaimByAnotherReviewer
--- PASS: TestGovernanceAssignReviewAllowsSameReviewerToRefreshClaim
--- PASS: TestGovernanceAssignReviewSQLRecordsReviewerAssignment
--- PASS: TestGovernanceAssignReviewRejectsApprovedAgent
--- PASS: TestAdminHandlerListsAssignedMarketplaceReviewReviewer
--- PASS: TestAdminHandlerClaimsMarketplaceReview
--- PASS: TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase/claim_review
PASS
ok oblivious/server/internal/marketplace 0.020s
ok oblivious/server/internal/http 0.033s
```

```text
npm test -- --run src/features/admin/api.test.ts src/routes/admin/AdminReviewsPage.test.tsx -t "(claims marketplace reviews through the admin review endpoint|renders assigned reviewer and claims pending reviews)"

Test Files 2 passed
Tests 2 passed | 41 skipped
```

## Remaining Boundary

- This is repo-local governance evidence. Full target-runtime verification still requires a deployed database environment and exercising the complete admin review workflow end to end.
- The assignment model intentionally uses append-only governance events rather than a mutable column; current conflict protection rejects claims by a different reviewer based on the latest `review_assign` event.

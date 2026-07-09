# Marketplace Appeal Reject State Machine Evidence

Date: 2026-07-04

## Scope

Close the remaining Marketplace appeal decision gap by adding a first-class admin rejection path for agents in `appeal_pending`.

## Implementation

- Added `GovernanceService.RejectAppealAgent` with a state-constrained transition:
  - requires admin actor, agent ID, and reason
  - requires current `published_agents.status = appeal_pending`
  - updates `published_agents.status` back to `takedown`
  - preserves the rejection reason in `review_reason`
  - writes `marketplace_governance_events.action = appeal_reject` with `from_status = appeal_pending` and `to_status = takedown`
- Added admin HTTP route:
  - `POST /api/v1/admin/marketplace/agents/{agentId}/reject-appeal`
  - returns `{ "status": "takedown" }`
  - requires admin session plus CSRF like other governance mutations
- Added Admin UI support in the Marketplace Governance panel:
  - action option `Reject appeal`
  - frontend API method `rejectMarketplaceAgentAppeal`
  - success copy `Marketplace agent appeal rejected.`
- Updated OpenAPI and route-surface contract coverage for the new endpoint.
- Updated migration action constraint to include `appeal_reject`.

## RED Evidence

Before implementation:

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernance(RejectAppealSQLRequiresPendingAppealDecision|RejectAppealRejectsTakedownWithoutPendingAppeal|MigrationAllowsRuntimeReviewActions)' -count=1 -v

internal/marketplace/governance_test.go:204:20: service.RejectAppealAgent undefined
internal/marketplace/governance_test.go:232:16: service.RejectAppealAgent undefined
FAIL oblivious/server/internal/marketplace [build failed]
```

```text
npm test -- --run src/features/admin/api.test.ts -t "supports admin marketplace takedown and reinstate routes"

TypeError: api.rejectMarketplaceAgentAppeal is not a function
```

```text
npm test -- --run src/routes/admin/AdminReviewsPage.test.tsx -t "rejects marketplace agent appeals from the governance panel"

expected rejectMarketplaceAgentAppeal to be called with arguments, calls: 0
```

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernance(RejectAppealSQLRequiresPendingAppealDecision|RejectAppealRejectsTakedownWithoutPendingAppeal|MigrationAllowsRuntimeReviewActions)' -count=1 -v

--- PASS: TestGovernanceMigrationAllowsRuntimeReviewActions
--- PASS: TestGovernanceRejectAppealSQLRequiresPendingAppealDecision
--- PASS: TestGovernanceRejectAppealRejectsTakedownWithoutPendingAppeal
PASS
ok oblivious/server/internal/marketplace 0.012s
```

```text
npm test -- --run src/features/admin/api.test.ts -t "supports admin marketplace takedown and reinstate routes"

Test Files 1 passed
Tests 1 passed | 26 skipped
```

```text
npm test -- --run src/routes/admin/AdminReviewsPage.test.tsx -t "rejects marketplace agent appeals from the governance panel"

Test Files 1 passed
Tests 1 passed | 13 skipped
```

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestMarketplaceGovernanceRejectsPendingAppeal|TestRouteSurfaceAdminMarketplaceGovernanceMutationsRejectAdminCookieWithoutCSRFWithoutDatabase' -count=1 -v

--- SKIP: TestMarketplaceGovernanceRejectsPendingAppeal
    TEST_DATABASE_URL is required for integration tests
--- PASS: TestRouteSurfaceAdminMarketplaceGovernanceMutationsRejectAdminCookieWithoutCSRFWithoutDatabase
PASS
ok oblivious/server/internal/http 0.020s
```

```text
bash scripts/verify-openapi-contract.sh

[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented.
```

```text
git diff --check

passed with no output
```

## Remaining Boundary

- Full HTTP lifecycle coverage still needs a configured `TEST_DATABASE_URL`; the route protection test proves the new route is registered and CSRF-protected without a database.
- This closes the appeal accept/reject decision state machine locally, but target-runtime deployment evidence is still separate from repo-local tests.

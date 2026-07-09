# Marketplace Appeal Pending Queue

Date: 2026-07-04

## Scope

Closed a repository-local Marketplace governance gap where publisher appeals were recorded as audit events but did not move a taken-down agent into an explicit operator-review queue state.

## Changed Files

- `src/server/internal/marketplace/types.go`
- `src/server/internal/marketplace/governance.go`
- `src/server/internal/marketplace/governance_test.go`
- `src/server/internal/http/marketplace_handler.go`
- `src/web/src/features/marketplace/api.ts`
- `src/web/src/features/marketplace/api.test.ts`
- `src/web/src/routes/marketplace/MarketplaceAgentDetailPage.tsx`
- `src/web/src/routes/marketplace/MarketplacePage.test.tsx`
- `docs/api/openapi.yaml`
- `scripts/verify_openapi_contract.py`

## Runtime Fixes

- Added `appeal_pending` as a first-class Marketplace agent status.
- Changed `AppealAgent` so only publisher-owned agents in `takedown` state can be appealed.
- Changed appeal handling to update `published_agents.status` from `takedown` to `appeal_pending` inside the same transaction that writes the `marketplace_governance_events` audit event.
- Preserved existing approved version history during appeal so historical installs and versions are not accidentally rewritten.
- Changed the user-facing appeal response from the ambiguous `appealed` status to `appeal_pending`.
- Updated the Marketplace frontend API type, page tests, and success copy to reflect that appeals enter an operator-review queue.
- Updated OpenAPI and the OpenAPI contract verifier to require `appeal_pending` in `MarketplaceActionStatusResponse.status`.

## RED Evidence

Before implementation, the focused sqlmock test failed because `AppealAgent` inserted the `appeal` event directly and never executed the required `UPDATE published_agents ... status = appeal_pending` step:

```text
go test ./internal/marketplace -run 'TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue' -count=1 -v

AppealAgent returned error: appeal agent: insert event: ExecQuery: could not match actual sql:
"INSERT INTO marketplace_governance_events ..."
with expected regexp "UPDATE published_agents"
```

The first integration-style test attempt was skipped without `TEST_DATABASE_URL`, so the durable RED proof for this slice is the sqlmock unit test above.

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernance(AppealSQLMovesTakedownIntoPendingAppealQueue|MigrationAllowsRuntimeReviewActions)' -count=1 -v
```

Observed result:

```text
--- PASS: TestGovernanceMigrationAllowsRuntimeReviewActions (0.01s)
--- PASS: TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue (0.00s)
PASS
ok  	oblivious/server/internal/marketplace	0.013s
```

```text
cd src/web && npm test -- --run src/features/marketplace/api.test.ts src/routes/marketplace/MarketplacePage.test.tsx -t "(supports marketplace user governance routes|submits marketplace governance appeals)"
```

Observed result:

```text
Test Files  2 passed (2)
Tests  2 passed | 38 skipped (40)
```

```text
bash scripts/verify-openapi-contract.sh
```

Observed result:

```text
[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented.
```

## Remaining Boundary

This makes appeals durable and review-queue visible in repository-owned code paths, but it does not complete Marketplace governance for commercial release. Remaining work still includes a fuller admin appeal queue surface, reviewer assignment/SLA evidence, richer appeal decision history, ranking-abuse controls, and target-runtime policy operations evidence.

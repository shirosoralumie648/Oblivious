# Marketplace Appeal Reinstate State Machine

Date: 2026-07-04

## Scope

Closed a repository-local Marketplace governance gap where admin reinstatement used a generic status update and could restore a taken-down agent without a publisher appeal entering the `appeal_pending` review queue first.

## Changed Files

- `src/server/internal/marketplace/governance.go`
- `src/server/internal/marketplace/governance_test.go`

## Runtime Fixes

- Changed `ReinstateAgent` from a generic governance status update to a state-constrained transition.
- Reinstatement now requires the agent to be in `appeal_pending` state.
- Reinstatement updates `published_agents.status` from `appeal_pending` to `approved` inside the same transaction that writes the `marketplace_governance_events` row.
- The reinstate audit event now records `from_status = appeal_pending` and `to_status = approved`.
- Direct reinstatement from `takedown` is rejected, which prevents admins from bypassing the publisher appeal queue.

## RED Evidence

Before implementation, the focused sqlmock test failed because `ReinstateAgent` used the generic update helper without a `WHERE status = appeal_pending` guard:

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernanceReinstateSQLRequiresPendingAppealDecision' -count=1 -v

ReinstateAgent returned error: reinstate agent: update status: ExecQuery '
    UPDATE published_agents
    SET status = $2, review_reason = NULLIF($3, ''), updated_at = $4
    WHERE id = $1
', arguments do not match: expected 5, but got 4 arguments
```

## GREEN Evidence

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace -run 'TestGovernance(ReinstateSQLRequiresPendingAppealDecision|ReinstateRejectsTakedownWithoutAppeal|AppealSQLMovesTakedownIntoPendingAppealQueue|ReinstateAfterPendingAppealRecordEvents)' -count=1 -v
```

Observed result:

```text
--- PASS: TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue (0.00s)
--- PASS: TestGovernanceReinstateSQLRequiresPendingAppealDecision (0.00s)
--- PASS: TestGovernanceReinstateRejectsTakedownWithoutAppeal (0.00s)
--- SKIP: TestGovernanceReinstateAfterPendingAppealRecordEvents (0.00s)
PASS
ok  	oblivious/server/internal/marketplace	0.007s
```

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/marketplace ./internal/http -run 'TestGovernance(ReinstateSQLRequiresPendingAppealDecision|ReinstateRejectsTakedownWithoutAppeal|AppealSQLMovesTakedownIntoPendingAppealQueue|ReinstateAfterPendingAppealRecordEvents)|TestMarketplaceGovernanceTakedownAppealAndReinstate' -count=1 -v
```

Observed result:

```text
--- PASS: TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue (0.00s)
--- PASS: TestGovernanceReinstateSQLRequiresPendingAppealDecision (0.00s)
--- PASS: TestGovernanceReinstateRejectsTakedownWithoutAppeal (0.00s)
--- SKIP: TestGovernanceReinstateAfterPendingAppealRecordEvents (0.00s)
PASS
ok  	oblivious/server/internal/marketplace	0.009s
--- SKIP: TestMarketplaceGovernanceTakedownAppealAndReinstate (0.00s)
PASS
ok  	oblivious/server/internal/http	0.025s
```

```text
bash scripts/verify-openapi-contract.sh
```

Observed result:

```text
[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented.
```

## Remaining Boundary

This closes the local accept path state constraint for appeal decisions. Marketplace governance is still not commercially complete: appeal rejection needs a first-class decision event, reviewer assignment/SLA escalation are still missing, and target-runtime policy operations evidence is still required before commercial release.

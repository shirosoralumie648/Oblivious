# Marketplace Governance Commercial DB Evidence Profile

Date: 2026-07-04

## Scope

Close the release-evidence gap where recently added Marketplace governance operations were covered by focused tests but were not all required by the `marketplace-governance-review` commercial DB evidence profile.

## Implementation

- Expanded `scripts/verify-commercial-db-evidence.sh marketplace-governance-review` to require DB-backed coverage for:
  - `appeal_pending` appeal queue transitions
  - admin appeal reinstatement and appeal rejection state machines
  - append-only review assignment via `review_assign`
  - claim conflict protection and same-reviewer refresh
  - `appeal_pending` review SLA enforcement
  - admin review claim/list/SLA/appeal queue route coverage
  - CSRF coverage for admin review mutation routes
- Replaced one-off governance profile assertions in `scripts/verify-commercial-db-evidence-profiles.sh` with a required-token list so future additions can be audited consistently.
- Updated the profile to use current live test names instead of stale historical names such as `TestGovernanceAppealAndReinstateRecordEvents`.

## RED Evidence

After adding the new required governance tokens to the profile guard, the existing DB evidence profile failed because the new appeal rejection path was not required:

```text
bash scripts/verify-commercial-db-evidence-profiles.sh

[commercial-db-evidence-profiles] marketplace-governance-review must include TestGovernanceRejectAppealRecordsTakedownDecision
```

The failure confirmed the guard could catch missing Marketplace governance release evidence.

## GREEN Evidence

```text
bash scripts/verify-commercial-db-evidence-profiles.sh

[commercial-db-evidence-profiles] commercial DB evidence profile list is synchronized.
```

```text
bash scripts/verify-commercial-db-evidence-profiles-fixtures.sh

[commercial-db-evidence-profiles-fixtures] commercial DB evidence profile fixture behavior is guarded.
```

```text
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-governance-review

[commercial-db-evidence] PASS  marketplace governance and automated review persistence
[commercial-db-evidence] PASS  marketplace review SLA enforcement persistence
[commercial-db-evidence] PASS  marketplace governance and review HTTP routes
[commercial-db-evidence] SUMMARY
[commercial-db-evidence] database: disposable pgvector PostgreSQL
[commercial-db-evidence] skipped tests: none
```

Representative tests now included in the commercial DB evidence profile:

```text
TestGovernanceAppealSQLMovesTakedownIntoPendingAppealQueue
TestGovernanceRejectAppealSQLRequiresPendingAppealDecision
TestGovernanceAssignReviewSQLRecordsReviewerAssignment
TestGovernanceAssignReviewRejectsClaimByAnotherReviewer
TestServiceEnforceReviewSLAsIncludesAppealPendingQueue
TestAdminHandlerListsAppealPendingMarketplaceReviews
TestAdminHandlerClaimsMarketplaceReview
TestMarketplaceGovernanceRejectsPendingAppeal
TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase
```

## Remaining Boundary

- This closes a repository-local commercial evidence guard. It does not by itself prove the full Marketplace governance workflow in a deployed target environment.
- Final commercial release still needs target-runtime evidence with real deployment configuration, scheduled/admin SLA execution, durable alert delivery, and operator workflow proof against production-like credentials and data.

# Relay Batch Commercial Release Boundary

Date: 2026-07-04

## Scope

This slice tightens the Batch commercial-release boundary without enabling Batch as a production commercial runtime. `POST /v1/batch`, `GET /v1/batches`, and `GET /v1/batches/:id` remain production-disabled until the async lifecycle has explicit prebill, polling, settlement, refund, audit, and usage-capture proof.

## Changed Files

- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/policy_test.go`
- `scripts/verify_target_release_evidence.py`
- `scripts/assemble_target_release_evidence.py`
- `scripts/target_release_fixture_mutations.py`
- `scripts/verify-target-release-evidence-fixtures.sh`
- `scripts/assemble-target-release-evidence-fixtures.sh`
- `scripts/assemble-target-release-evidence.sh`
- `scripts/verify-target-release-evidence.sh`
- `scripts/verify-fusion-evidence-pack.sh`
- `scripts/verify-quality-gates.sh`
- `docs/release/rc-checklist.md`
- `docs/release/fusion-spec-evidence-pack.md`
- `docs/release/relay-route-table.md`

## Verification

- RED: `cd src/server && go test ./internal/relay/handler -run 'TestBatchPoliciesDeclareCommercialReleaseBlockers' -count=1`
  - Failed because `POST /v1/batch` did not mention `prebill` in its disabled reason.
- RED: `bash scripts/verify-target-release-evidence-fixtures.sh`
  - Failed after adding Batch fixture mutations because the manifest template/verifier did not yet define `relayBatch`.
- GREEN: `cd src/server && go test ./internal/relay/handler -run 'TestBatchPoliciesDeclareCommercialReleaseBlockers|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestInitialCommercialPolicyClassifiesCurrentSurface|TestRoutePoliciesDeclareBillingSettlementPolicy' -count=1`
- GREEN: `bash scripts/verify-target-release-evidence-fixtures.sh`
- GREEN: `bash scripts/assemble-target-release-evidence-fixtures.sh`
- GREEN: `python -m py_compile scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py`
- GREEN: `bash scripts/verify-fusion-evidence-pack.sh`
- GREEN: `bash scripts/verify-quality-gates.sh`

## Remaining Boundary

Batch remains disabled for production. A future enablement slice must wire durable async polling, idempotent settlement/refund, provider/result reconciliation, and request-log/usage capture evidence before changing the route policy from `DisabledInProduction`.

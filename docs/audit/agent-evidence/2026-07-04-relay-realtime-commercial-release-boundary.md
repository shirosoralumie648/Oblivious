# Relay Realtime Commercial Release Boundary

Date: 2026-07-04

## Scope

This slice tightened the Realtime commercial-release boundary without enabling the Realtime runtime. Later 2026-07-05 slices added repository-local origin, usage-capture, and API-token query-model authorization proof. `GET /v1/realtime` remains disabled in production until the commercial lifecycle has production prebill, abort settlement, request-log linkage, and target-runtime proof.

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
- `docs/audit/current-implementation-depth.md`
- `docs/audit/oblivious-gap-matrix.md`
- `docs/audit/implementation-roadmap.md`
- `docs/audit/product-roadmap-v2-from-reference.md`
- `docs/audit/reference-deep-rescan-v3.md`
- `docs/audit/reference-to-oblivious-product-delta-v3.md`

## Verification

- RED: `cd src/server && go test ./internal/relay/handler -run 'TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestInitialCommercialPolicyClassifiesCurrentSurface|TestRoutePoliciesDeclareBillingSettlementPolicy' -count=1`
  - Failed because the Realtime disabled reason did not mention all commercial blockers.
- RED: `bash scripts/verify-target-release-evidence-fixtures.sh`
  - Failed because `missing-relay-realtime-proof` unexpectedly passed before the manifest verifier required `relayRealtime`.
- GREEN: `cd src/server && go test ./internal/relay/handler -run 'TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestInitialCommercialPolicyClassifiesCurrentSurface|TestRoutePoliciesDeclareBillingSettlementPolicy' -count=1`
- GREEN: `bash scripts/verify-target-release-evidence-fixtures.sh`
- GREEN: `bash scripts/assemble-target-release-evidence-fixtures.sh`
- GREEN: `python -m py_compile scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py`

## Remaining Boundary

Realtime is still not a commercial runtime capability. Final readiness must keep `relayRealtime.mode=disabled_until_commercial_lifecycle` until the runtime has target evidence for production prebill, abort settlement, request-log linkage, and target-runtime proof, or the release intentionally excludes Realtime with this disabled lifecycle proof.

# Relay Target Proof Closure

Date: 2026-07-08

## Summary

Relay Realtime and Relay Batch target evidence no longer self-certify lifecycle boundary proof inside the target release assembler. The assembler now requires explicit Relay Realtime and Relay Batch proof JSON, carries the selected lifecycle mode into the manifest, and writes matching artifact-level `proofs` metadata for the referenced `relay-realtime-proof` and `relay-batch-proof` artifacts.

## Evidence Chain

- `scripts/assemble-target-release-evidence.sh` requires `--relay-realtime-proof-file` and `--relay-batch-proof-file` or the matching `OBLIVIOUS_TARGET_RELAY_*_PROOF_FILE` environment values.
- `scripts/assemble_target_release_evidence.py` validates disabled lifecycle proof for production-disabled policy plus Realtime auth/origin/prebill/abort/usage blockers and Batch prebill/polling/settlement/refund/audit/usage blockers. Enabled Batch proof also requires `summary.requestLogAuditRecords` from allowed `relay.route_decision` request-log evidence to cover settlement plus refund records.
- `scripts/verify_target_release_evidence.py` requires both manifest proof fields and artifact-level proof metadata to match the active Relay lifecycle mode.
- `scripts/assemble-target-release-evidence-fixtures.sh` rejects missing Relay proof inputs, failed Realtime blocker proof, and incomplete Batch blocker summaries.
- `scripts/verify-target-release-evidence-fixtures.sh` rejects `relayRealtime.evidenceRef` and `relayBatch.evidenceRef` artifacts that omit required Relay proof metadata.

## Verification

Expected green commands for this evidence slice:

```bash
bash scripts/collect-relay-realtime-evidence-fixtures.sh
bash scripts/collect-relay-batch-evidence-fixtures.sh
bash scripts/assemble-target-release-evidence-fixtures.sh
bash scripts/verify-target-release-evidence-fixtures.sh
bash scripts/collect-target-release-artifacts-fixtures.sh
bash scripts/verify-commercial-preflight-fixtures.sh
bash scripts/verify-quality-gates.sh
bash scripts/check.sh docs
```

## Boundary

This is repository-local evidence-chain hardening. Final commercial readiness still requires target Relay proof artifacts, target request/usage traffic, downloaded artifact bodies with matching SHA-256 values, and a no-skip strict verifier run against the target environment.

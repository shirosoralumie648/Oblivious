# Marketplace Target Proof Closure

Date: 2026-07-08

Scope: Release readiness evidence spine for target Marketplace payout and governance proof.

## Change

- `scripts/assemble-target-release-evidence.sh` now requires `--marketplace-payout-proof-file` or `OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_PROOF_FILE`.
- `scripts/assemble-target-release-evidence.sh` now requires `--marketplace-governance-proof-file` or `OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_PROOF_FILE`.
- `scripts/assemble_target_release_evidence.py` now derives the manifest `marketplacePayouts`, `marketplaceGovernance`, and matching artifact-level `proofs` metadata from supplied target proof JSON instead of self-certifying those fields as `pass`.
- Marketplace payout proof must include outbound dispatch, inbound webhook lifecycle, settlement ledger, reconciliation, refund/chargeback handling, and summary consistency for dispatch, webhook, ledger, reconciliation, and refund/chargeback handling counts.
- Marketplace governance proof must include review queue, appeal queue, appeal decision lifecycle, review assignment, review SLA enforcement, abuse-report lifecycle, and summary consistency for review assignment, appeal decision, SLA, and abuse-report counts.

## Verification

Expected fixture coverage:

- Missing Marketplace payout proof input is rejected by the assembler.
- Missing Marketplace governance proof input is rejected by the assembler.
- Failed payout refund/chargeback proof is rejected by the assembler.
- Governance appeal decision summary drift is rejected by the assembler.

## Boundary

This is repository-local evidence-chain hardening. It does not replace target Marketplace traffic, live provider payout rails, downloaded artifact bodies, refund/chargeback operations, governance reviewer operations, or the final no-skip commercial verifier run.

# Target Marketplace Payout And Governance Release Gates

Date: 2026-07-04

## Scope

Tighten the final target-release evidence manifest so Marketplace payout and governance readiness cannot be claimed from generic provider, Kubernetes, or local DB evidence.

## Implementation

- Added `marketplaceGovernance` as a required target evidence section:
  - `reviewQueue`
  - `appealQueue`
  - `appealDecisionLifecycle`
  - `reviewAssignment`
  - `reviewSLAEnforcement`
  - `abuseReportLifecycle`
- Added `marketplace-governance-proof` as a first-class artifact kind.
- Added `OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI` and matching SHA-256 input to the target evidence assembler.
- Added `marketplacePayouts.refundChargebackHandling` to the target evidence manifest and verifier.
- Updated release checklist, fusion evidence pack, and quality-gate assertions so docs cannot drift behind the verifier.

## RED Evidence

Before verifier support, the new fixture proved the manifest still passed without Marketplace governance target evidence:

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] missing-marketplace-governance-proof unexpectedly passed
```

Before payout refund/chargeback support, the new fixture proved the manifest still passed without explicit refund/chargeback handling evidence:

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] missing-marketplace-payout-refund-chargeback-proof unexpectedly passed
```

## GREEN Evidence

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] rejected missing-marketplace-governance-proof
[target-release-evidence-fixtures] rejected failed-marketplace-governance-review-assignment
[target-release-evidence-fixtures] rejected marketplace-governance-kind-mismatch
[target-release-evidence-fixtures] rejected missing-marketplace-payout-refund-chargeback-proof
[target-release-evidence-fixtures] target release evidence verifier behavior is guarded.
```

```text
bash scripts/assemble-target-release-evidence-fixtures.sh

[assemble-target-release-evidence-fixtures] assembled and validated target evidence manifest
```

```text
python -m py_compile scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py
git diff --check
```

## Remaining Boundary

- This is a target-release manifest gate, not the target proof itself.
- Final commercial readiness still requires collecting real target artifacts for payout provider dispatch, inbound payout webhooks, refund/chargeback handling, reconciliation, Marketplace governance operator workflow, SLA alert delivery, and abuse-report lifecycle.

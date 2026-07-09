# Target Artifact Proof Metadata Validation

Date: 2026-07-05

## Scope

Tighten the target-release evidence manifest so Marketplace payout and governance final readiness require both section-level pass fields and artifact-level proof metadata.

## Implementation

- Added verifier support for artifact-level `proofs` metadata on evidence artifacts.
- Required `marketplace-payout-proof` artifacts to declare:
  - `proofs.outboundDispatch`
  - `proofs.inboundWebhookLifecycle`
  - `proofs.settlementLedger`
  - `proofs.reconciliation`
  - `proofs.refundChargebackHandling`
- Required `marketplace-governance-proof` artifacts to declare:
  - `proofs.reviewQueue`
  - `proofs.appealQueue`
  - `proofs.appealDecisionLifecycle`
  - `proofs.reviewAssignment`
  - `proofs.reviewSLAEnforcement`
  - `proofs.abuseReportLifecycle`
- Updated the target evidence assembler and fixtures to emit the same proof metadata.
- Updated release checklist, fusion evidence pack, and quality-gate assertions so docs and release gates stay aligned with verifier behavior.

## RED Evidence

Before verifier support, the new fixture proved the manifest could still pass when the Marketplace payout artifact omitted `proofs.refundChargebackHandling`:

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] marketplace-payout-artifact-missing-refund-chargeback-proof unexpectedly passed
```

## GREEN Evidence

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] rejected marketplace-payout-artifact-missing-refund-chargeback-proof
[target-release-evidence-fixtures] rejected marketplace-governance-artifact-missing-review-sla-proof
[target-release-evidence-fixtures] target release evidence verifier behavior is guarded.
```

## Remaining Boundary

- This validates manifest metadata and release gates; it does not fetch or inspect remote artifact body content.
- Final commercial readiness still requires real target artifacts for payout dispatch, inbound webhook lifecycle, settlement ledger, reconciliation, refund/chargeback handling, review queues, appeal lifecycle, review assignment, SLA enforcement, and abuse-report lifecycle.

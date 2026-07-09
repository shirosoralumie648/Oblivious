# Deployment, Kubernetes, And Provider Live Target Proof Closure

Date: 2026-07-08

## Summary

Deployment, Kubernetes, and provider live rail target evidence no longer self-certify release proof fields inside the target release assembler. The assembler now requires explicit deployment, Kubernetes, and Stripe/Alipay/WeChat Pay live-rail proof JSON, derives the manifest fields from those files, and writes matching artifact-level `proofs` metadata for `deployment-log`, `kubernetes-validation`, and provider-specific `provider-live-rail` artifacts.

## Evidence Chain

- `scripts/assemble-target-release-evidence.sh` requires `--deployment-proof-file`, `--kubernetes-proof-file`, and provider-specific `--*-provider-live-rail-proof-file` inputs or matching environment values.
- `scripts/assemble_target_release_evidence.py` validates deployment proof for `targetEnvironment=production`, deploy validation, backup/restore, migration replay, and concrete operation references before writing `deployment`.
- `scripts/assemble_target_release_evidence.py` validates Kubernetes proof for `targetEnvironment=production`, `clusterRef`, `namespace`, validation, rollout, failover, `secretFileClass=external-filled`, and concrete operation references before writing `kubernetes`.
- `scripts/assemble_target_release_evidence.py` validates provider live rail proof provider identity, `mode=live`, `providerEnvironment=live`, checkout, refund, payout, reconciliation, concrete provider operation references, and positive summary counts before writing `providers[]`.
- `scripts/verify_target_release_evidence.py` requires deployment, Kubernetes, and provider live rail evidence refs to expose matching artifact-level proof metadata, production target class, and concrete artifact-body references.
- `scripts/assemble-target-release-evidence-fixtures.sh` rejects missing deployment proof input, missing Kubernetes proof input, missing Stripe provider live rail proof input, invalid Kubernetes secret class, and provider proof identity mismatch.
- `scripts/verify-target-release-evidence-fixtures.sh` rejects deployment, Kubernetes, and provider live rail artifacts that omit required proof metadata.

## Verification

Expected green commands for this evidence slice:

```bash
bash scripts/collect-deployment-evidence-fixtures.sh
bash scripts/collect-kubernetes-evidence-fixtures.sh
bash scripts/collect-provider-live-rail-evidence-fixtures.sh
bash scripts/assemble-target-release-evidence-fixtures.sh
bash scripts/verify-target-release-evidence-fixtures.sh
bash scripts/collect-target-release-artifacts-fixtures.sh
bash scripts/verify-commercial-preflight-fixtures.sh
bash scripts/verify-quality-gates.sh
bash scripts/check.sh docs
```

## Boundary

This is repository-local evidence-chain hardening. Final commercial readiness still requires target deployment proof, target Kubernetes proof, live provider rail proof for Stripe, Alipay, and WeChat Pay, downloaded artifact bodies with matching SHA-256 values, and a no-skip strict verifier run against the target environment.

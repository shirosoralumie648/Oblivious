# Provider Runtime And Microservice Database Target Proof Closure

Date: 2026-07-08

## Summary

Provider runtime config and microservice database target evidence no longer self-certify release proof fields inside the target release assembler. The assembler now requires explicit provider runtime config and microservice database proof JSON, derives the manifest sections from those files, and writes matching artifact-level `proofs` metadata for `provider-runtime-config` and `microservice-database-proof` artifacts.

## Evidence Chain

- `scripts/assemble-target-release-evidence.sh` requires `--provider-runtime-config-proof-file` and `--microservice-database-proof-file` or the matching `OBLIVIOUS_TARGET_*_PROOF_FILE` environment values.
- `scripts/assemble_target_release_evidence.py` validates provider coverage for Stripe, Alipay, WeChat Pay, provider env vars, checkout base URLs, webhook routes, and webhook verification before writing `providerRuntimeConfig`.
- `scripts/assemble_target_release_evidence.py` validates `mode=microservices`, `serviceUrlClass=external-filled`, all eleven service database proofs, and migration readiness before writing `microserviceDatabases`.
- `scripts/verify_target_release_evidence.py` requires both manifest proof fields and artifact-level proof metadata for provider runtime config and microservice database evidence refs.
- `scripts/assemble-target-release-evidence-fixtures.sh` rejects missing provider runtime config proof input, missing microservice database proof input, failed provider runtime config proof, and incomplete microservice database proof.
- `scripts/verify-target-release-evidence-fixtures.sh` rejects provider runtime config and microservice database artifacts that omit required proof metadata.

## Verification

Expected green commands for this evidence slice:

```bash
bash scripts/collect-provider-runtime-config-evidence-fixtures.sh
bash scripts/collect-microservice-database-evidence-fixtures.sh
bash scripts/assemble-target-release-evidence-fixtures.sh
bash scripts/verify-target-release-evidence-fixtures.sh
bash scripts/collect-target-release-artifacts-fixtures.sh
bash scripts/verify-commercial-preflight-fixtures.sh
bash scripts/verify-quality-gates.sh
bash scripts/check.sh docs
```

## Boundary

This is repository-local evidence-chain hardening. Final commercial readiness still requires target provider runtime config proof, external-filled microservice database proof, downloaded artifact bodies with matching SHA-256 values, and a no-skip strict verifier run against the target environment.

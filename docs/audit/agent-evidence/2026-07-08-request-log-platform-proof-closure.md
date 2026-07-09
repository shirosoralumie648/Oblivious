# 2026-07-08 Request-Log Platform Proof Closure

## Scope

Close the request-log observability evidence gap where release assembly or artifact collection could imply ClickHouse platform readiness without reading an explicit target proof.

## Implementation

- `scripts/assemble-target-release-evidence.sh` now requires request-log platform proof, request-log coverage proof, and latency SLO proof files before assembling a target manifest.
- `scripts/assemble_target_release_evidence.py` derives `requestLogObservability` section fields and the request-log artifact `proofs` metadata from those proof files instead of hard-coding request-log proof fields to `pass`.
- `scripts/collect-request-log-observability-evidence.sh` and `scripts/collect_request_log_observability_evidence.py` now require `--platform-proof-file` or `--platform-proof-url`; failed `clickHouseDeployment`, `clickHouseMigration`, `requestLogsTable`, or `ingestQuerySmoke` values stop collection.
- `scripts/collect-target-release-artifacts.sh` and `scripts/collect_target_release_artifacts.py` now require `--request-log-platform-proof-file` or `--request-log-platform-proof-url` in addition to request-log coverage source and SLO proof.
- `scripts/verify_target_release_evidence.py` validates request-log artifact-level manifest proofs and requires downloaded request-log artifact bodies to include `platformProofSource`.

## Fixture Coverage

- Assembler fixtures reject failed request-log platform proof and failed request-log coverage proof.
- Request-log collector fixtures reject failed ClickHouse platform proof, missing request-log coverage, failed latency SLO proof, and secret-like coverage query parameters.
- Aggregate artifact collector fixtures reject missing request-log platform proof source.
- Target evidence verifier fixtures reject missing request-log artifact usage-join proof, failed downloaded ClickHouse platform proof, missing request-log platform proof source, and missing request-log coverage.

## Boundary

This is repository-local evidence-chain hardening. It does not create real target ClickHouse deployment evidence, target `request_logs` traffic, provider live rails, or the final no-skip commercial verifier run.

# Strict, Workflow, And Secret Target Proof Closure

Date: 2026-07-08

## Scope

This closure hardens the target release evidence assembler for proof families that previously could be represented by manifest fields alone:

- `strictVerifier`
- `workflowTelemetry`
- `secretAudit`

The assembler now requires explicit raw proof JSON for each family and derives the manifest fields from those inputs.

## Contract Changes

- `scripts/assemble-target-release-evidence.sh` requires `--strict-verifier-proof-file`, `--secret-audit-proof-file`, and `--workflow-telemetry-proof-file`, or the matching `OBLIVIOUS_TARGET_*_PROOF_FILE` environment variables.
- `scripts/assemble_target_release_evidence.py` validates the canonical strict verifier command, `result=pass`, empty `skippedChecks`, ISO-8601 strict verifier timestamps, empty secret-audit findings, required secret-audit scope coverage for Kubernetes/providers/runtime, workflow telemetry success-rate bounds, ISO-8601 telemetry windows, and telemetry count consistency.
- The assembler cross-checks strict verifier start/end timestamps and workflow telemetry rate/window against the exported `OBLIVIOUS_TARGET_*` environment values.
- Fixture coverage rejects missing strict verifier, secret audit, and workflow telemetry proof inputs, plus invalid skipped strict checks, secret-audit findings, and workflow telemetry success-rate/count drift.

## Verification

Expected green commands:

- `python -m py_compile scripts\assemble_target_release_evidence.py scripts\collect_strict_verifier_evidence.py scripts\collect_workflow_telemetry_evidence.py scripts\collect_secret_audit_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash -n scripts/assemble-target-release-evidence.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/verify-quality-gates.sh`
- `bash scripts/collect-strict-verifier-evidence-fixtures.sh`
- `bash scripts/collect-workflow-telemetry-evidence-fixtures.sh`
- `bash scripts/collect-secret-audit-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`
- `bash scripts/collect-target-release-artifacts-fixtures.sh`
- `bash scripts/verify-commercial-preflight-fixtures.sh`
- `bash scripts/verify-quality-gates.sh`
- `bash scripts/check.sh docs`

## Boundary

This is repository-local evidence-chain hardening only. It does not provide a real target strict verifier log, real target workflow telemetry, a real target secret audit, downloaded artifact bodies, or a final no-skip commercial verifier run.

# Request Log Artifact Body Proof Validation

Date: 2026-07-05

## Scope

Tighten the target-release evidence manifest so ClickHouse request-log observability final readiness requires both section-level pass fields and downloaded artifact-body proof metadata.

## Implementation

- Added downloaded artifact-body validation for `request-log-observability` artifacts.
- Required downloaded request-log observability artifact bodies to declare:
  - `proofs.clickHouseDeployment`
  - `proofs.clickHouseMigration`
  - `proofs.requestLogsTable`
  - `proofs.ingestQuerySmoke`
  - `proofs.requestUsageJoin`
  - `proofs.latencySLOTrigger`
  - `proofs.latencySLOAlertDelivery`
  - `proofs.latencySLORecoveryAction`
- Updated the target evidence assembler and fixtures to emit the same request-log proof metadata.
- Updated the target evidence template, RC checklist, fusion evidence pack, and quality-gate assertions so docs and release gates stay aligned with verifier behavior.

## RED Evidence

Before verifier support, a downloaded `request-log-observability` artifact body could omit `proofs.requestUsageJoin` while the manifest still passed when the SHA-256 was updated to match the weakened body:

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence] PASS target evidence manifest
[target-release-evidence-fixtures] artifact-bundle-request-log-missing-usage-join-proof unexpectedly passed
```

## GREEN Evidence

```text
bash scripts/verify-target-release-evidence-fixtures.sh

[target-release-evidence-fixtures] rejected artifact-bundle-request-log-missing-usage-join-proof
[target-release-evidence-fixtures] target release evidence verifier behavior is guarded.
```

```text
bash scripts/assemble-target-release-evidence-fixtures.sh

[assemble-target-release-evidence-fixtures] assembled and validated target evidence manifest
[assemble-target-release-evidence-fixtures] rejected missing required artifact URI
[assemble-target-release-evidence-fixtures] rejected missing required artifact SHA-256
[assemble-target-release-evidence-fixtures] rejected missing required latency SLO proof
[assemble-target-release-evidence-fixtures] rejected invalid environment class
```

```text
python -m py_compile scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py

exit 0
```

```text
bash scripts/verify-quality-gates.sh

[quality-gates] quality gate assets look complete.
```

```text
git diff --check -- scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py scripts/verify-target-release-evidence-fixtures.sh scripts/verify-quality-gates.sh docs/release/rc-checklist.md docs/release/fusion-spec-evidence-pack.md

exit 0
```

## Remaining Boundary

- This validates target evidence manifest and downloaded artifact-body consistency for request-log observability.
- It still does not create or fetch real target ClickHouse artifacts.
- Final commercial readiness still requires target ClickHouse deployment, migration, `request_logs` ingest/query smoke, and a deployment-data request-log to usage/billing join sample by `request_id`.

# Request Log Observability Evidence Collector

Date: 2026-07-05

## Scope

Add a target-release evidence collector that turns deployed Admin request-log coverage and latency SLO proof into a `request-log-observability` artifact body for strict commercial readiness validation.

## Implementation

- Added `scripts/collect-request-log-observability-evidence.sh`.
- Added `scripts/collect_request_log_observability_evidence.py`.
- Added `scripts/collect-request-log-observability-evidence-fixtures.sh`.
- Wired the collector fixture into `scripts/check.sh docs`.
- Wired the assembler fixture to build a downloaded artifact bundle, replace the request-log artifact body with collector output, refresh artifact SHA-256 values, and re-run strict validation with `OBLIVIOUS_TARGET_ARTIFACT_DIR`.
- Added quality-gate assertions so the collector, implementation, and fixture are release-owned assets.
- Updated `docs/release/rc-checklist.md` to document the target evidence collection flow:
  - export `GET /api/v1/admin/billing/reconciliation/usage-request-logs` from the target Admin API
  - provide target latency SLO proof JSON
  - generate the downloaded `request-log-observability` artifact body
  - validate the manifest with `OBLIVIOUS_TARGET_ARTIFACT_DIR`

## RED Evidence

Before the collector existed, the new fixture failed immediately:

```text
bash scripts/collect-request-log-observability-evidence-fixtures.sh

bash: scripts/collect-request-log-observability-evidence.sh: No such file or directory
```

## GREEN Evidence

```text
bash scripts/collect-request-log-observability-evidence-fixtures.sh

[collect-request-log-observability-evidence-fixtures] generated request-log observability artifact body
[collect-request-log-observability-evidence-fixtures] rejected missing request-log coverage
[collect-request-log-observability-evidence-fixtures] rejected failed latency SLO proof
```

```text
bash scripts/assemble-target-release-evidence-fixtures.sh

[assemble-target-release-evidence-fixtures] assembled and validated target evidence manifest
[assemble-target-release-evidence-fixtures] assembled and validated collector artifact bundle
[assemble-target-release-evidence-fixtures] rejected missing required artifact URI
[assemble-target-release-evidence-fixtures] rejected missing required artifact SHA-256
[assemble-target-release-evidence-fixtures] rejected missing required latency SLO proof
[assemble-target-release-evidence-fixtures] rejected invalid environment class
```

```text
python -m py_compile scripts/collect_request_log_observability_evidence.py

exit 0
```

```text
bash scripts/verify-quality-gates.sh

[quality-gates] quality gate assets look complete.
```

```text
git diff --check -- scripts/collect-request-log-observability-evidence.sh scripts/collect_request_log_observability_evidence.py scripts/collect-request-log-observability-evidence-fixtures.sh scripts/assemble-target-release-evidence-fixtures.sh scripts/check.sh scripts/verify-quality-gates.sh docs/release/rc-checklist.md docs/release/fusion-spec-evidence-pack.md

exit 0
```

## Remaining Boundary

- This creates a repository-owned collector for target request-log evidence artifacts.
- It still requires a real target run to export Admin coverage and latency SLO proof JSON.
- Final commercial readiness still requires target ClickHouse deployment, migration, ingest/query smoke, and live usage/request-log joins from deployment data.

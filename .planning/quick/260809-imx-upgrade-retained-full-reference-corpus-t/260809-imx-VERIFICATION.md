---
quick_id: 260809-imx
status: passed
verified_at: 2026-08-09T06:24:40Z
implementation_commit: ec0e11e
scope: corpus-materialization-and-persistent-clean-launch
---

# Quick Task 260809-imx Verification

## Verdict

**PASS for the declared quick-task boundary.** The full v3 raw corpus is verified and the resumable candidate cleaner is demonstrably running. **RUNNING, not complete, for the 91,273-unit clean.** No feature catalog or full-recall claim is authorized yet.

## Must-Have Results

| Requirement | Result | Evidence |
| --- | --- | --- |
| Preserve old evidence | PASS | Source manifest/index hashes remained `b17004...` / `ef2e39...`; prior sample directories were not modified |
| Materialize all retained records | PASS | 190,299 target lines and exact per-kind manifest counts |
| Enrich linked issues | PASS | 12,827 issues, 14,321/14,321 PR contexts resolved and content-joined |
| Preserve non-issue records | PASS | 101,379/101,379 records exactly equal to source |
| Report both scopes | PASS | Unfiltered 178,377 records / 179,669 units; candidate 90,444 / 91,273 |
| Reject stale progress | PASS | Dedicated current/stale contract tests; status exposes only current progress object |
| Probe requested model | PASS | Fresh `gpt-5.4-mini`, low-effort unit completed successfully |
| Persistent resumable run | PASS | Named tmux alive; atomic progress running; no errors at verification snapshot |
| Aggregate only after clean | PASS | Runner uses `set -euo pipefail` and sequential clean, aggregate, status; catalog absent during incomplete clean |

## Automated Checks

| Command | Result |
| --- | --- |
| `PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 python3 -m unittest discover -s tests -p 'test_reference_intel.py' -v` | PASS, 17/17 |
| `python3 -m py_compile scripts/reference_intel/pipeline.py tests/test_reference_intel.py` | PASS |
| `bash -n scripts/reference-intel.sh` | PASS |
| `bash -n <external runner>` | PASS |
| `jq empty scripts/reference_intel/feature_record.schema.json` | PASS |
| `git diff --check` | PASS |
| Source/target streaming pair audit | PASS |
| Linked PR identity/content audit | PASS, 14,321/14,321 |

## Live Snapshot

At `2026-08-09T06:23:44Z`, tmux pane PID `472247` reported `pane_dead=0`. `clean/progress.json` had current clean v2 / prompt v3 / `gpt-5.4-mini` / low identities, `running=true`, position 41, 40 successful calls this run, 0 failures, and an ETA of 1,557,464 seconds. The target had 44 clean item files and 0 error files; `catalog/` was absent.

The changing progress file is operational state, not a fixed completion artifact. Later positions may be higher; a dead tmux, `running=false` with incomplete counts, or any error checkpoint supersedes this launch-time PASS and requires diagnosis.

## Residual Risk / Next Gate

- Candidate-only filtering excludes 88,396 unfiltered units and therefore cannot prove complete recall.
- The probe and early live calls do not prove the model will complete the multi-week run without provider errors, rate limits, or semantic drift.
- Long-history feature-key stability, deduplication quality, review rate, and false-positive rate remain unverified until strict aggregation completes.
- The 150 open/weak issues with merged-PR context intentionally remain outside candidate issue evidence; their PR records are still in candidate scope.
- Full upstream metadata is historical evidence and does not prove behavior remains present at each upstream current HEAD.

## Evidence Boundary

This verification closes only quick task `260809-imx`. It does not close Phase 31.2, prove an Oblivious feature, or establish target/live commercial release readiness.

---
quick_id: 260809-ko0
status: passed
verified_at: 2026-08-09T07:14:00Z
implementation_commit: f13dc6c
scope: persistent-unfiltered-clean-launch
---

# Quick Task 260809-ko0 Verification

## Verdict

**PASS for the declared launch boundary.** The candidate-only writer was replaced without checkpoint loss, and the 179,669-unit unfiltered cleaner is demonstrably running with one writer and zero errors. **RUNNING, not complete, for the clean and catalog.**

## Must-Have Results

| Requirement | Result | Evidence |
| --- | --- | --- |
| Preserve candidate checkpoints | PASS | 116-item baseline retained; sampled checkpoint SHA-256 unchanged |
| Avoid concurrent writers | PASS | Old process tree exited before relaunch; snapshot has exactly 1 cleaner writer and 0 candidate runners |
| Cover unfiltered scope | PASS | Progress reports `implementation_candidates_only=false`, 178,377 records, and 179,669 units |
| Cover 88,396-unit delta | PASS | Full scope minus candidate scope is 88,396; replacement scans full scope and reuses current identities |
| Keep requested model contract | PASS | `gpt-5.4-mini`, low effort, confidence 0.80 |
| Fail closed before aggregation | PASS | Runner has `set -euo pipefail`, error cap 1, strict sequential stages, and no allowance flags |
| Demonstrate forward progress | PASS | Stable progress checkpoint reached 30/30 successful new calls with 0 failures |
| Withhold incomplete catalog | PASS | `catalog/` absent while clean is running |
| Preserve old runner | PASS | `run-full-v3.sh` remains present; replacement has a separate path and log |

## Automated Checks

| Command / Gate | Result |
| --- | --- |
| `PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 python3 -m unittest discover -s tests -p 'test_reference_intel.py' -v` | PASS, 17/17 |
| `python3 -m py_compile scripts/reference_intel/pipeline.py tests/test_reference_intel.py` | PASS |
| `bash -n scripts/reference-intel.sh` | PASS |
| `bash -n <unfiltered external runner>` | PASS |
| Forbidden runner allowance/filter flag scan | PASS, zero matches |
| `jq empty scripts/reference_intel/feature_record.schema.json` | PASS |
| `git diff --check` | PASS |

## Live Snapshot

At `2026-08-09T07:14:00Z`, tmux pane PID `509848` reported `pane_dead=0`. `clean/progress.json` reported clean v2 / prompt v3 / `gpt-5.4-mini` / low, `running=true`, `implementation_candidates_only=false`, 178,377 records / 179,669 units, position 30, 30 successful calls, 0 failures, and ETA 4,554,146 seconds. The workdir held 146 item files and 0 error files; `catalog/` was absent.

Progress is live operational state. A later dead pane, `running=false` with incomplete counts, any error checkpoint, or scope/identity drift supersedes this launch-time PASS and requires diagnosis.

## Residual Risk / Next Gate

- The early ETA is volatile and currently multi-week; persistent launch is not completion.
- The active-run `clean/manifest.json` remains the last completed command manifest and therefore still shows the earlier candidate scope. During the run, current full-scope truth is `clean/progress.json`; the manifest is rebuilt when clean exits.
- Any first new model error stops this strict run. Recovery requires cause analysis and an explicit retry-errors invocation.
- Long-history deduplication, review rate, feature-key stability, and false-positive/false-negative rates remain unverified until aggregation and semantic audit.
- Upstream issue text is untrusted. The model process uses an empty inherited environment, read-only sandbox, ephemeral session, and isolated cwd, but an interrupted candidate call still attempted an outbound `git clone`; outbound tool behavior remains a residual operational/security risk for the long run.
- Full upstream metadata is historical evidence and does not prove behavior remains present at each upstream current HEAD.

## Evidence Boundary

This verification closes only quick task `260809-ko0`. It does not close the running clean, assert a complete feature inventory, prove an Oblivious feature, or establish target/live commercial release readiness.

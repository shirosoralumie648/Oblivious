---
quick_id: 260809-imx
status: planned
description: Upgrade retained full reference corpus to v3 evidence, recalculate scope, and launch resumable GPT-5.4 mini cleaning
must_haves:
  truths:
    - The retained v1 full directory and failed cross-repository evidence directories remain unchanged.
    - A new corpus is materialized locally from all retained raw records, with linked merged-PR content added to issue evidence and hashes recomputed.
    - Status reports both evidence-eligible candidate scope and the unfiltered model scope, and never presents stale clean contracts as current progress.
    - Long-running cleaning exposes durable progress and ETA, remains resumable per unit, and aggregates only after a complete error-free clean.
    - Starting the persistent cleaner does not imply that all upstream features have already been extracted or that Oblivious implements them.
  artifacts:
    - scripts/reference_intel/pipeline.py
    - tests/test_reference_intel.py
    - docs/audit/reference-intelligence-pipeline.md
    - .planning/quick/260809-imx-upgrade-retained-full-reference-corpus-t/260809-imx-SUMMARY.md
    - .planning/quick/260809-imx-upgrade-retained-full-reference-corpus-t/260809-imx-VERIFICATION.md
  key_links:
    - materialize-corpus reads the retained SQLite/raw indexes and writes a new external workdir through a same-filesystem staging directory.
    - Issue enrichment resolves every closing PR through immutable record IDs and fails closed when a linked PR cannot be found.
    - The cleaner writes atomic progress checkpoints whose schema, prompt, model, effort, and source scope match the clean contract.
    - The named tmux command runs clean, strict aggregate, and final status in sequence, so aggregate cannot run after a failed or incomplete clean.
---

# Quick Task 260809-imx: Upgrade And Launch Full Reference Cleaning

## Scope Boundary

This task prepares and launches the next durable full-history cleaning job after the bounded v3 sample passed. Completion means the new corpus is verified and the named resumable job is demonstrably running with durable checkpoints. It does not wait for the multi-week model run to finish and does not claim a completed feature inventory.

## Task 1: Add deterministic full-corpus materialization and observable progress

**Files:** `scripts/reference_intel/pipeline.py`, `tests/test_reference_intel.py`

**Action:** Add a streaming `materialize-corpus` command that preserves all retained raw records in a new workdir while enriching linked issues from the retained PR index. Extend status with candidate and unfiltered clean scopes plus clean-contract progress. Make the candidate filter high-recall for merged PRs while preserving deterministic non-feature exclusions.

**Verify:** Focused tests cover complete record preservation, issue enrichment, source immutability, scope counts, neutral merged-PR inclusion, stale contract reporting, and progress checkpoints.

**Done:** The new corpus can be built without GitHub refetch or in-place mutation, and operators can distinguish scope and live progress.

## Task 2: Materialize and audit the full v3 corpus

**Files:** External workdir `/home/shirosora/.cache/oblivious-reference-intel/full-20260809-v3`

**Action:** Materialize from `/home/shirosora/.cache/oblivious-reference-intel/full-20260809`, rebuild JSONL/SQLite indexes, verify exact record-kind counts, verify non-issue hashes are unchanged, verify every strong issue has complete linked-PR identity/content, and calculate candidate versus unfiltered units.

**Verify:** Corpus manifest, hashes, record counts, linked-PR audit, and `status` all agree; old evidence directory hashes remain unchanged.

**Done:** A clean v3 raw corpus exists outside git with no model calls made during materialization.

## Task 3: Probe and launch resumable GPT-5.4 mini cleaning

**Files:** External logs/checkpoints under the v3 workdir; repository audit documentation and quick-task artifacts

**Action:** Run a fresh bounded model probe. If it passes, launch a named tmux session using `gpt-5.4-mini`, effort `low`, default confidence `0.80`, evidence-eligible candidate scope, strict clean failure handling, strict aggregation, and final status capture. Record the exact session, command, workdir, initial progress, scope, ETA, and evidence boundary.

**Verify:** The tmux process is alive, the log identifies the current unit/model/effort, at least one current clean v2 checkpoint appears, status/progress is readable, and no aggregate catalog exists while cleaning is incomplete.

**Done:** The long-running job is safely launched and resumable; any provider or data failure remains fail-closed and visible.

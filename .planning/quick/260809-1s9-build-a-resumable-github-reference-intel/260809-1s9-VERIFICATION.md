---
quick_id: 260809-1s9
status: gaps_found
verified_at: 2026-08-08T21:49:55Z
implementation_commit: ccf3ce3
---

# Quick Task 260809-1s9 Verification

## Must-Have Results

| Must Have | Result | Evidence |
| --- | --- | --- |
| Every reference repository is discovered from local Git origin | Passed | `pipeline.py discover --repo-root .` returned 30 repositories and 0 skipped; no hand-maintained list is used. |
| Raw records retain immutable provenance and are not model-rewritten | Passed | External raw manifest contains 190,299 records with per-record `content_sha256`; SQLite/raw indexes are built before clean; tests cover raw hash join and provenance preservation. |
| Only implementation-bearing evidence can enter the inventory | Passed locally | Evidence levels, merged-PR/linked-issue rules, excerpt grounding, status/confidence gates, and exclusion/review outputs are covered by code and 9 focused tests. No accepted live claims exist yet. |
| Model failures resume per unit and cannot become claims | Passed locally | Atomic `clean/errors/<unit-hash>.json`; new low-effort timeout includes model + effort; clean index has 0 records after the smoke. |
| Luna cleaning produces all implemented feature claims | Gap / blocked | Real Luna smoke timed out: `gpt-5.6-luna`, newapi, explicit low effort, 35s pipeline timeout; 0/94,097 candidate units succeeded and 94,096 remain pending. |

## Commands

- `PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 python3 -m unittest discover -s tests -p 'test_reference_intel.py' -v` — passed, 9/9。
- `python3 -m py_compile scripts/reference_intel/pipeline.py tests/test_reference_intel.py` — passed。
- `bash -n scripts/reference-intel.sh` and `python3 -m json.tool scripts/reference_intel/feature_record.schema.json` — passed。
- `python3 scripts/reference_intel/pipeline.py discover --repo-root .` — passed, 30/30。
- `bash scripts/reference-intel.sh status --workdir /home/shirosora/.cache/oblivious-reference-intel/full-20260809` — collection failures 0; clean `successful_units=0`, `error_units=1`, `pending_units=94096`; catalog empty。
- `bash scripts/check.sh docs` — blocked by the repository-wide clean-head gate (`source_worktree_dirty`) caused by pre-existing unrelated dirty paths; not treated as a pipeline test failure。

## External Provider Evidence

- Luna probe: `timeout 30s codex exec --model gpt-5.6-luna -c 'model_reasoning_effort="low"' ...` exited 124, emitted only SessionStart, and wrote no output file。
- Control probe: identical read-only invocation with `gpt-5.6-sol` exited 0 and returned `OK` under the same `newapi` provider。
- Durable pipeline evidence: `/home/shirosora/.cache/oblivious-reference-intel/full-20260809/clean/errors/8b9467c58dec9bd73cb8b42cf5b6c98556893964b9b74c75201297c32f3381bf.json` records the low-effort 35s timeout; the older 240s error lacks the effort field and is therefore not considered current。

## Residual Risk / Next Boundary

The raw corpus is complete but the model stage is not. After Luna availability is restored, retry the low-effort checkpoint in a named tmux session, observe a small successful sample, then run the requested full scope. `--implementation-candidates-only` is a recall-saving option; the raw corpus also supports the unfiltered 181,878-unit run when full recall is required. Only after clean completion should `aggregate` create `catalog/features.{json,jsonl,md}` and a review queue.

## Evidence Boundary

This verification proves repository-local pipeline behavior and external provider diagnostics only. It does not prove that Oblivious implements any cataloged upstream feature, nor target/live deployment, billing, or release readiness.

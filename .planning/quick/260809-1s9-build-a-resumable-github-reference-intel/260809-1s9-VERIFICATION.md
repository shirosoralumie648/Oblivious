---
quick_id: 260809-1s9
status: gaps_found
verified_at: 2026-08-09T03:21:48Z
implementation_commit: ccf3ce3
---

# Quick Task 260809-1s9 Verification

## Must-Have Results

| Must Have | Result | Evidence |
| --- | --- | --- |
| Every reference repository is discovered from local Git origin | Passed | `pipeline.py discover --repo-root .` returned 30 repositories and 0 skipped; no hand-maintained list is used. |
| Raw records retain immutable provenance and are not model-rewritten | Passed | External raw manifest contains 190,299 records with per-record `content_sha256`; SQLite/raw indexes are built before clean; tests cover raw hash join and provenance preservation. |
| Only implementation-bearing evidence can enter the inventory | Passed for smoke | Evidence levels, merged-PR/linked-issue rules, excerpt grounding, status/confidence gates, and exclusion/review outputs are covered by code and 9 focused tests. The mini smoke produced 17 grounded accepted claims from 20 units. |
| Model failures resume per unit and cannot become claims | Passed locally | Atomic `clean/errors/<unit-hash>.json`; model + effort isolate checkpoints; the current mini clean index contains only 20 successful mini results and 0 current errors. |
| Luna cleaning produces all implemented feature claims | Gap / blocked | Real Luna smoke timed out: `gpt-5.6-luna`, newapi, explicit low effort, 35s pipeline timeout; 0/94,097 candidate units succeeded and 94,096 remain pending. |
| User-requested GPT-5.4 mini fallback works | Passed across repositories, full run gated | Minimal probe returned `OK`; the cross-repository sample completed 54/54 units across 27 repositories with 0 errors. Remaining candidate scope is 94,077 units. |
| Cross-repository sample validates unattended semantic quality | Gap / fail closed | 22/54 records require review, and 51/154 provisional accepted claims come from those records. Current aggregation does not make those claims non-publishable or directly filterable. |

## Commands

- `PYTEST_DISABLE_PLUGIN_AUTOLOAD=1 python3 -m unittest discover -s tests -p 'test_reference_intel.py' -v` — passed, 9/9。
- `python3 -m py_compile scripts/reference_intel/pipeline.py tests/test_reference_intel.py` — passed。
- `bash -n scripts/reference-intel.sh` and `python3 -m json.tool scripts/reference_intel/feature_record.schema.json` — passed。
- `python3 scripts/reference_intel/pipeline.py discover --repo-root .` — passed, 30/30。
- `bash scripts/reference-intel.sh status --workdir /home/shirosora/.cache/oblivious-reference-intel/full-20260809` — collection failures 0; mini clean `successful_units=20`, `error_units=0`, `pending_units=94077`; catalog not assembled while incomplete。
- `bash scripts/reference-intel.sh aggregate --workdir /home/shirosora/.cache/oblivious-reference-intel/cross-repo-20260809-gpt54mini` — 154 groups/claims、22 review entries、5 exclusions、0 clean errors。
- Cross-repository status — clean complete、54/54 successful、0 errors/pending；17 issue、19 merged PR、18 release records from 27 repositories。
- Deterministic audit — 0 model/effort/schema/prompt mismatches；0 accepted ungrounded excerpts；0 accepted claims below confidence 0.70。
- `bash scripts/check.sh docs` — blocked by the repository-wide clean-head gate (`source_worktree_dirty`) caused by pre-existing unrelated dirty paths; not treated as a pipeline test failure。

## External Provider Evidence

- Luna probe: `timeout 30s codex exec --model gpt-5.6-luna -c 'model_reasoning_effort="low"' ...` exited 124, emitted only SessionStart, and wrote no output file。
- Priority probe: adding `-c 'service_tier="priority"'` still exited 124 after 30s with no output file。
- Control probe: identical read-only invocation with `gpt-5.6-sol` exited 0 and returned `OK` under the same `newapi` provider。
- Durable pipeline evidence: `/home/shirosora/.cache/oblivious-reference-intel/full-20260809/clean/errors/8b9467c58dec9bd73cb8b42cf5b6c98556893964b9b74c75201297c32f3381bf.json` records the low-effort 35s timeout; the older 240s error lacks the effort field and is therefore not considered current。
- GPT-5.4 mini probe: `codex exec --model gpt-5.4-mini -c 'model_reasoning_effort="low"' ...` exited 0 in about 6 seconds and returned `OK`。
- GPT-5.4 mini pipeline smoke: 20/20 successful, 0 current errors, 17 grounded accepted claims, 6 review-marked units; observed serial ETA was about 16 days for 94,097 candidates。
- GPT-5.4 mini cross-repository sample: 54/54 successful, 0 errors, model `gpt-5.4-mini`, effort `low`, prompt `reference-feature-cleaner/v1`; first/last completed timestamps span about 22m36s。
- Sample quality report: `docs/audit/reference-intelligence-gpt54mini-cross-repo-sample.md`; external selection, clean manifest, and catalog digests are recorded there。

## Residual Risk / Next Boundary

The raw corpus is complete and GPT-5.4 mini is operational across repositories, but the full model stage is only 20/94,097 candidates complete. The sample exposed a semantic publication gap: review-marked records contribute 51/154 provisional accepted claims, while issue records do not contain the linked PR body/diff needed to verify the inferred final behavior. Starting the remaining serial run would be both a material time/call-volume decision and premature before this quality gate is repaired and re-sampled. `--implementation-candidates-only` remains a recall tradeoff; the raw corpus also supports the unfiltered 181,878-unit run when full recall is required.

## Evidence Boundary

This verification proves repository-local pipeline behavior and external provider diagnostics only. It does not prove that Oblivious implements any cataloged upstream feature, nor target/live deployment, billing, or release readiness.

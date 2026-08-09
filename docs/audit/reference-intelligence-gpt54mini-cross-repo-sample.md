# GPT-5.4 Mini Cross-Repository Reference Sample

Date: 2026-08-09

Status: **SAMPLE_ONLY / bounded cross-repository quality gate passed / full run not authorized**

This report evaluates the reference-intelligence cleaner on a bounded cross-repository sample. It does not prove that Oblivious implements any capability found in `reference/`, and it is not target/live or release evidence.

## Sample Contract

- External workdir: `/home/shirosora/.cache/oblivious-reference-intel/cross-repo-20260809-gpt54mini-v3`
- Candidate repositories: 28; sampled repositories: 27
- Records and model units: 54, with at most two records from distinct source kinds per repository
- Source mix: 17 issues, 19 merged pull requests, and 18 published releases
- Selection order: stable SHA-256 order, restricted to implementation candidates that fit one 12,000-character unit
- Explicit exclusion: `AIDotNet/lobe-chat`; its only candidate was a changelog that did not fit one unit
- Cleaner: `gpt-5.4-mini`, reasoning effort `low`, clean schema v2, prompt `reference-feature-cleaner/v3`, sequential execution
- Catalog policy: confidence `>= 0.80`; `needs_review` claims held outside `features.*`

The v3 sample uses exactly the same 54 record IDs as the first sample. Their sorted record-ID digest is `8cd95ed0e270ff7c17893f2efd80e46a512a0809a239a9e708ad6fbf75d99365`. Materialization enriched every sampled issue from the retained raw merged-PR records; all 17 issue records contain the linked PR record ID, content hash, number, URL, merge SHA, merge time, title, and body.

## Execution Result

The cleaner completed all 54 units between `2026-08-09T04:02:22Z` and `2026-08-09T04:22:07Z`:

| Check | Result |
| --- | --- |
| Successful units | 54/54 |
| Model errors / pending units | 0 / 0 |
| Stale clean results | 0 |
| Model / effort mismatches | 0 / 0 |
| Schema / prompt mismatches | 0 / 0 |
| Raw hash mismatches | 0 |
| Accepted ungrounded excerpts | 0 |
| Accepted / review overlap | 0 |

External artifact SHA-256 values:

| Artifact | SHA-256 |
| --- | --- |
| `sample-selection.json` | `cb775da497d09dd3a159a83861ee5a82d542e962c4a89bb22020658147866139` |
| `clean/manifest.json` | `a0dec91c478541283c526eadc8a48c5072350da29b99f6d97de190e588704c01` |
| `clean/records.jsonl` | `94da179843c3157a31578c02dedc47fc20db2534bcfe1d6cdca51189ac9cbdd8` |
| `catalog/features.json` | `3a2999ca79adcb8f47f51acdba912b6a98892f27f2b21e710e834b923d59aa6f` |
| `catalog/review-queue.jsonl` | `caa7fa9716b99343c78fecbed4551fe9947c3abf0ba6de5a0d7c0b62f9acd4c4` |
| `catalog/excluded-claims.jsonl` | `45b3c76486b44187904de5f6b24fbba350d987037b974d6b098331c0734b4bab` |

## Claim Partition

The 54 clean records produced 144 model claims. Aggregation partitioned every claim exactly once:

| Source | Units | Accepted | Review units | Review-held claims | Excluded |
| --- | ---: | ---: | ---: | ---: | ---: |
| Issue | 17 | 18 | 1 | 0 | 2 |
| Merged PR | 19 | 30 | 2 | 2 | 6 |
| Release | 18 | 72 | 4 | 8 | 6 |
| **Total** | **54** | **120** | **7** | **10** | **14** |

Review-unit rate fell from 22/54 (40.7%) in v1 to 7/54 (13.0%) in v3. This reduction does not come from silently accepting the old review set: all 10 otherwise eligible claims from review-marked units are held only in `review-queue.jsonl`, and all 14 rejected claims have `accepted_for_inventory=false` in `excluded-claims.jsonl`.

## Semantic Spot Checks

- LobeHub issue `#2025` is classified `unknown`, marked for review, and its request to disable animation is excluded rather than presented as implemented behavior.
- The Helicone Anthropic `base_url` issue is classified `documentation_only` and excluded.
- MaxKB `not_eq`, NextChat context retention, FastGPT multiline JSON body handling, new-api oversized upstream-error truncation, and copilot-api thinking-block handling are supported by the linked merged-PR text carried with each issue.
- The LiteLLM tests-only PR, Apache-2.0 license bullet, and LobeHub internal database relocation are deterministically excluded from the feature catalog.
- Ambiguous or ungrounded PR/release subclaims remain in review or exclusion output; none leak into the 120 accepted claims.

The spot checks cover the failure modes found in v1, but they are not a statistically labeled precision or recall estimate. This 54-record sample also produced 120 distinct groups from 120 accepted claims, so long-history key stability and cross-record deduplication remain untested.

## Verdict And Full-Run Boundary

- **PASS:** `gpt-5.4-mini` availability and sequential completion across repositories.
- **PASS:** clean completeness, schema/prompt/model identity, provenance, exact-excerpt grounding, and exclusive claim partitioning.
- **PASS FOR THIS BOUNDED SAMPLE:** targeted semantic checks across issue, PR, and release failure modes.
- **NOT AUTHORIZED / NOT STARTED:** full-history v3 model cleaning.

The retained full directory still reports the old clean v1 scope of 94,097 candidate units with 20 historical successes. Those checkpoints are stale under clean v2 / prompt v3. With the tightened prefilter, current code estimates 85,448 candidates from the old raw index, but the full issue records have not yet been enriched under the new linked-PR contract. Neither number is the final approved call volume.

Before a full run, first upgrade the full raw issue evidence, recompute the exact clean scope and cost, and explicitly choose between candidate-first cleaning and a higher-recall unfiltered run. Preserve both failed/partial evidence directories: v1 at `cross-repo-20260809-gpt54mini` and the interrupted v2 attempt at `cross-repo-20260809-gpt54mini-v2`.

## Evidence Boundary

The generated catalog records `evidence_boundary.class=upstream-metadata`, `reference_only=true`, `oblivious_implementation_proof=false`, and `current_upstream_head_verified=false`. It is a local sample-quality result only.

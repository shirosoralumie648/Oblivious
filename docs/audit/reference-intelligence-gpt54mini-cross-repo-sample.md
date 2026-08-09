# GPT-5.4 Mini Cross-Repository Reference Sample

Date: 2026-08-09

Status: **SAMPLE_ONLY / semantic auto-publication not approved**

This report evaluates the reference-intelligence cleaner on a bounded cross-repository sample. It does not prove that Oblivious implements any capability found in `reference/`, and it is not target/live or release evidence.

## Sample Contract

- External workdir: `/home/shirosora/.cache/oblivious-reference-intel/cross-repo-20260809-gpt54mini`
- Candidate repositories: 28
- Sampled repositories: 27
- Records and model units: 54, with at most two records from distinct source kinds per repository
- Source mix: 17 issues, 19 merged pull requests, and 18 published releases
- Selection order: stable SHA-256 order, restricted to implementation candidates that fit one 12,000-character unit
- Explicit exclusion: `AIDotNet/lobe-chat`; its only candidate was a changelog that did not fit one unit
- Cleaner: `gpt-5.4-mini`, reasoning effort `low`, prompt `reference-feature-cleaner/v1`, sequential execution

The sample-selection manifest has SHA-256 `ceebd92937072eca8a037e5671dd99a784c328cfe8fcd3721bc79401c73621c7`.

## Execution Result

The cleaner completed all 54 units between `2026-08-09T02:49:49Z` and `2026-08-09T03:12:25Z`:

| Check | Result |
| --- | --- |
| Successful units | 54/54 |
| Model errors | 0 |
| Pending units | 0 |
| Model / effort mismatches | 0 / 0 |
| Schema / prompt-version mismatches | 0 / 0 |
| Collection failures | 0 |

The clean manifest has SHA-256 `023faa588414b7b94e2615b3212f2e80ad98d0968258fbefe8e757210ad339cf`.

## Deterministic Gate Result

The 54 clean records contained 159 model-produced capability candidates:

- 154 passed the implemented/fixed, strong-or-medium evidence, exact excerpt grounding, and confidence `>= 0.70` gates.
- 5 were excluded: three were below the confidence threshold (two of those were also ungrounded), one was planned, and one was unknown/documentation-only.
- All 154 accepted claims had grounded excerpts and confidence `>= 0.70`.
- Record classification was 48 `implementation_bearing`, 4 `metadata_only`, and 2 `documentation_only`.
- Aggregation produced 154 groups, 22 review-queue entries, and zero clean errors.

Representative positive checks:

- A dependency-only MaxKB pull request was classified `metadata_only` and produced no capability.
- The AnythingLLM Codespaces port-forwarding pull request and CLIProxyAPI Keep-Alive release item produced narrow claims with exact source excerpts.
- Low-confidence build-error and ungrounded release/PR subclaims were excluded rather than entering the catalog.

The catalog has SHA-256 `4b757036e3fcdf5abea94c33d5910083a81b72a0c6e79516634cef91db74bf1a`.

## Semantic Quality Finding

The run validates model availability, structured output, provenance, and deterministic grounding, but it does not validate unattended semantic precision:

| Source | Units | Review-marked | Review rate | Accepted claims from review-marked units |
| --- | ---: | ---: | ---: | ---: |
| Issue | 17 | 12 | 70.6% | 11 |
| Merged PR | 19 | 2 | 10.5% | 5 |
| Release | 18 | 8 | 44.4% | 35 |
| **Total** | **54** | **22** | **40.7%** | **51** |

The current aggregate catalog still contains those 51 review-tainted claims. They are 33.1% of its 154 accepted claims. The remaining 103 claims are model-unflagged provisional claims, not a human-labeled precision result.

Manual inspection confirms why the distinction matters:

- Issue records contain the problem/request text and a linked merged-PR identity, but not the linked PR title, body, or diff. Claims such as NextChat context retention and new-api large-error handling therefore infer the final behavior from the issue description.
- Terse release bullets can be expanded beyond what they literally establish. For example, `agents with telemetry` became a broader proxy-observability claim. The model marked this record for review, but aggregation still accepted it.
- Broad release notes can yield coarse implementation or internal-maintenance claims that are grounded textually but are not yet normalized to a stable product-capability boundary.
- The sample has no merged duplicate groups, so it does not yet validate long-history key stability or cross-record deduplication.

## Verdict And Next Gate

- **PASS:** `gpt-5.4-mini` availability and sequential completion across repositories.
- **PASS:** schema, provenance, model identity, confidence threshold, and exact-excerpt grounding.
- **FAIL CLOSED:** unattended semantic catalog publication. Review-marked claims are not separated from accepted catalog features, and issue evidence lacks linked-PR content.
- **NOT STARTED:** the remaining 94,077-unit full candidate clean.

Before authorizing the full run, the pipeline should make review-required claims non-publishable or explicitly filterable, enrich linked-issue evidence with the merged PR content, and repeat a cross-repository sample with a human-labeled precision rubric. The existing raw corpus and checkpoints must remain intact.

## Evidence Boundary

The generated catalog is `upstream-metadata`, `reference_only=true`, `oblivious_implementation_proof=false`, and `current_upstream_head_verified=false`. This report records a local sample-quality gate only.

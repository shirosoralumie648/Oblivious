# RAG Indexing Target Proof Closure

Date: 2026-07-08

Scope: Release readiness evidence spine for target RAG indexing proof.

## Change

- `scripts/assemble-target-release-evidence.sh` now requires `--rag-proof-file` or `OBLIVIOUS_TARGET_RAG_PROOF_FILE` before it can assemble a target release evidence manifest.
- `scripts/assemble_target_release_evidence.py` now derives the manifest `ragIndexing` section and the `rag-indexing-proof` artifact metadata from the supplied RAG proof JSON instead of self-certifying those fields as `pass`.
- RAG target proof JSON must prove durable queue migration, worker deployment, enqueue/drain, raw parser replay, retrieval, stale-vector filtering, positive worker/raw-parser/retrieval/stale-vector counts, and `summary.workerCompletedJobs == summary.drainedJobs`.
- `scripts/verify_target_release_evidence.py` now requires the `ragIndexing.evidenceRef` artifact to carry artifact-level `proofs` metadata for all required RAG proof keys.
- `scripts/collect_rag_indexing_evidence.py` now rejects worker/drain count drift before writing a RAG artifact body.

## Verification

Expected fixture coverage:

- Missing RAG proof input is rejected by the assembler.
- Failed RAG raw parser replay proof is rejected by the assembler.
- RAG worker/drain mismatch is rejected by the assembler and collector.
- Missing `rag-indexing-proof` artifact-level `proofs.rawParserReplay` is rejected by the target manifest verifier.

## Boundary

This is repository-local evidence-chain hardening. It does not replace target worker rollout, target traffic, downloaded artifact bodies, target secret audit, target provider rails, or the final no-skip commercial verifier run.

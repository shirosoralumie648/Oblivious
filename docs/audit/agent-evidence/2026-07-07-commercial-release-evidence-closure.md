# 2026-07-07 Commercial Release Evidence Closure

## Scope

This record covers repository-local hardening added after the Phase 30 baseline. It does not renew final commercial readiness by itself. A renewed claim still requires real target/live artifacts outside git and a no-skip `scripts/verify-commercial-completion.sh` run with deploy, Kubernetes, backup/restore, target evidence, and artifact body validation enabled.

## Evidence Added

- `scripts/verify-commercial-preflight.mjs` now runs `scripts/verify-target-release-evidence.sh` during target evidence preflight. Missing manifest commit fields and semantic artifact body failures are rejected before the heavy docs, security, frontend, DB, deploy, Kubernetes, and backup/restore gates.
- RAG target evidence now carries `summary.workerCompletedJobs` and `summary.rawParserReplayCount` from collection through artifact assembly, target artifact collection, manifest fixture generation, and final target verifier checks.
- Target artifact body verification now rejects RAG worker completion undercounts, collection sources collected before the body `recordedAt`, collection source URLs that point at a different artifact/proof family, Marketplace payout evidence with zero refund/chargeback cases despite a pass proof, and microservice database bodies missing `mode=microservices` plus `serviceUrlClass=external-filled`.
- Chat HTTP send and stream paths now preserve the request id from middleware into `chat.RelayRequestMetadata`, so Chat, Relay usage, and request-log observability evidence can join on the same request id.
- `docs/release/commercial-gates.md` now distinguishes the historical Phase 30 baseline from a current commercial-release claim that needs target/live evidence and a no-skip verifier run.

## Verification

- `bash -n scripts/verify-commercial-preflight-fixtures.sh scripts/collect-rag-indexing-evidence-fixtures.sh scripts/verify-target-release-evidence-fixtures.sh scripts/verify-quality-gates.sh scripts/check.sh`
- `node --check scripts/verify-commercial-preflight.mjs`
- `python -m py_compile scripts\collect_rag_indexing_evidence.py scripts\verify_target_release_evidence.py scripts\target_release_fixture_mutations.py`
- `bash scripts/verify-commercial-preflight-fixtures.sh`
- `bash scripts/collect-rag-indexing-evidence-fixtures.sh`
- `bash scripts/verify-target-release-evidence-fixtures.sh`
- `bash scripts/assemble-target-release-evidence-fixtures.sh`
- `bash scripts/collect-target-release-artifacts-fixtures.sh`
- `bash scripts/verify-quality-gates.sh`
- `bash scripts/check.sh docs`
- `git diff --check`

## Open Release Boundary

Go formatting and Go package tests were not rerun in this environment because `go` and `gofmt` were not available on `PATH`. Final commercial readiness remains open until the target environment supplies real `OBLIVIOUS_TARGET_EVIDENCE_FILE`, `OBLIVIOUS_TARGET_ARTIFACT_DIR`, external `OBLIVIOUS_K8S_SECRET_FILE`, live provider rail proof, target gRPC smoke proof, microservice database proof, and a no-skip strict verifier pass.

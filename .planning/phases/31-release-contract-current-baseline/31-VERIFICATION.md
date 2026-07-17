---
phase: 31-release-contract-current-baseline
status: gaps_found
verified: 2026-07-17
requirements:
  - RELS-01
evidence_class: repository-local
environment: detached-clean-worktree
release_commit: e64ead7c858202b8f48a05784871a4e58d1925eb
automated_checks: 18
failures: 4
skipped_checks: 1
next_action: Reconcile the Phase 31 contract, canonical fixtures, and embedded-provider mutation test without relying on an uncommitted Phase 31.1 overlay; repair structural verifier patterns; rerun Stage A, full regression, and Stage B from the final clean HEAD.
next_command: $gsd-plan-phase 31 --gaps
---

# Phase 31 Verification - Release Contract Current Baseline

## Verdict

`gaps_found`. All six plans have summaries and the Phase 31 implementation surface exists, but the exact committed snapshot does not pass its own Stage A or full Go regression contract. Phase 31 remains implementation-complete by plan count only; it is not phase-complete and must not advance to Phase 31.1.

`RELS-01` remains pending. This phase is limited to repository-local contract/build-identity foundations and does not prove dynamic readiness, surface parity, target deployment, or commercial release readiness.

## Goal Score

| Validation ID | Result | Evidence |
|---|---|---|
| V31-F01 | PASS | Strict authored contract/schema/semantic tests pass. |
| V31-F02 | FAIL | Canonical fixtures contain three timing fields rejected by the committed profile schema. |
| V31-F03 | PASS | Explicit profile, operation, literal argv, path, and environment tests pass. |
| V31-F04 | PARTIAL | Git identity behavior passes; embedded digest mutation proof fails before mutation. |
| V31-F05 | PARTIAL | Entry-point, report, and build fixtures pass independently; aggregate Stage A is red and final-HEAD Stage B is absent. |
| V31-F06 | PASS | Strict nested report and typed registry tests pass. |
| V31-F07 | PASS | Atomic writer and rollback tests pass. |
| V31-F08 | PASS | Non-vacuous Go selector fixtures pass. |
| V31-F09 | PASS | Evidence remains explicitly repository-local with target/live proof unclaimed. |

Fully verified: 6/9. Three behaviors are failed or partial and block completion.

## Fresh Automated Evidence

All commands in this section ran from detached clean worktree `/tmp/oblivious-phase31-review-e64ead7` at commit `e64ead7c858202b8f48a05784871a4e58d1925eb`.

| Command | Result |
|---|---|
| `bash scripts/verify-release-contract.sh --stage-a` | FAIL. `TestCanonicalBytesGolden`, `TestDigestEquivalentFormatting`, and `TestDigestSemanticMutation` reject `/profiles/0` as schema-invalid. |
| `go test ./... -count=1` | FAIL. The three digest tests fail, plus `TestEmbeddedProviderRecomputesContractDigest` reports `fixture contract mutation did not apply`. |
| `go vet ./internal/buildinfo ./internal/releasecontract ./internal/surfacereport ./cmd/release-contract ./cmd/server ./cmd/migrate ./cmd/grpc-smoke` | PASS. |
| `bash -n` over all seven Phase 31 scripts | PASS. |
| `bash scripts/verify-release-build-fixtures.sh` | PASS. |
| Race-enabled report/CLI/entrypoint packages | PASS except the separately run buildinfo package, which is blocked by the embedded mutation failure above. |
| `git diff --check e016fba..HEAD` | PASS. |

`shellcheck` was unavailable. `TEST_DATABASE_URL` was unset, so DB-backed integration proof was not run; Phase 31 has no database migration behavior and this skip does not substitute for any failed foundation check.

## Root Cause

The canonical digest fixtures were committed with Phase 31.1 timing fields before the committed Phase 31 schema, typed model, and authored contract defined those fields. The embedded-provider negative test also targets one of those absent fields. The four uncommitted Phase 31.1 changes in the main worktree add the missing schema/model/contract fields and can mask the defect, but they are not part of the trusted Phase 31 commit.

This is a cross-phase ownership error, not a flaky test or host issue. Clean HEAD reproduces it consistently.

## Stage B Evidence Boundary

A real Stage B run previously passed at the exact tuple below:

- commit: `cd711f23d94643b36a886e8225f5ea50610b37a5`
- tree: `8e38c550d6d47c0ddbc99383e91836f71e2e456b`
- contract digest: `sha256:ef38b7e0e64492e3d8167e47a9d9c96bcf06fcb1448bdb2f0744d54bf23cc53d`
- profile: `monolith`
- image: `oblivious-release:cd711f23d94643b36a886e8225f5ea50610b37a5`
- result: pass; skips: none; evidence class: `repository-local`

That tuple is historical evidence only. It cannot be transferred to `e64ead7` or to a future gap-fix/documentation commit because build identity binds the exact commit/tree/digest. A fresh Stage B run is mandatory after all gap fixes and verification artifacts are committed.

## Artifact And Key-Link Verification

All 21 declared artifact paths exist. The automated artifact checker passed 19/21 content patterns; the automated key-link checker passed 15/21 links.

Manual inspection confirmed the reported false negatives are present in implementation:

- authored profile operations reference `scripts/release-profile-operation.sh`;
- `provider.go` calls `releasecontract.Digest` and derives clean HEAD commit/tree;
- `build.go` calls `identities.Resolve`;
- the build fixture and Stage B script invoke `build-release-image.sh`;
- dirty-source rejection is delegated to the release-build fixture.

The remaining issue is metadata quality: over-escaped, invalid, or overly literal plan patterns prevent the canonical structural checker from proving those links automatically.

## Hook Results

| Hook | Status | Blocking evidence |
|---|---|---|
| Code review | `issues_found` | One critical clean-HEAD test-contract defect and one structural metadata warning. |
| Nyquist validation | `partial` | `nyquist_compliant: false`; V31-F02/F04/F05 are not fully green. |
| Security | `blocked` | `threats_open: 2` for canonical digest and embedded linker-tuple proof. |
| UI review | skipped | Phase 31 changed no UI files and has no UI-SPEC. |

## Gaps

### GAP-31-01 - Contract and fixture version split

Reconcile the schema, `DeploymentProfile`, authored contract, canonical vectors/golden digest, and embedded-provider mutation test as one coherent revision. The fix must not depend on dirty worktree state and must preserve the protected Phase 31.1 ownership boundary.

Acceptance evidence:

1. `bash scripts/verify-release-contract.sh --stage-a` passes at clean HEAD.
2. `go test ./... -count=1` passes at the same clean HEAD, with any DB skip reported separately.
3. The four protected files are only committed by their owner or after explicit ownership transfer.

### GAP-31-02 - Structural verifier pattern drift

Repair the plan `must_haves` patterns that are over-escaped, syntactically invalid, or tied to an exact command spelling rather than the implemented semantic link.

Acceptance evidence: all six `verify artifacts` and all six `verify key-links` results report fully passed/verified without manual waiver.

## Required Closure Sequence

1. Close GAP-31-01 and GAP-31-02 with atomic commits that preserve unrelated user changes.
2. Rerun code review, Nyquist, and security hooks; require `threats_open: 0`.
3. Rerun Stage A and the full repository regression gate from clean HEAD.
4. Commit final verification/tracking artifacts while keeping `RELS-01` pending.
5. Create a fresh detached worktree at that final commit and run `bash scripts/verify-release-contract.sh --clean-head --profile monolith` with zero skips.
6. Push only the exact final tuple that passed Stage B.

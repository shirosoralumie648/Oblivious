---
phase: 31-release-contract-current-baseline
plan: "06"
subsystem: release-contract-gates
tags: [surface-report, stage-a, clean-head, docker, release-evidence, repository-local]
requires:
  - phase: 31-04
    provides: "Typed SurfaceReportV1 registry and atomic writer"
  - phase: 31-05
    provides: "Binary, OCI, and packaged-contract identity wrapper"
provides:
  - "Trusted build-report producer and strict report validator CLI"
  - "Non-vacuous Stage A contract, identity, report, writer, and artifact fixture matrix"
  - "Exact clean-HEAD Stage B gate with real Docker artifact and report inspection"
affects: [phase-31.1, phase-31.2, phase-39]
tech-stack:
  added: []
  patterns: [trusted observation enrichment, identity-splice rejection, two-stage verification, exact clean-head evidence]
key-files:
  created:
    - scripts/verify-release-contract-fixtures.sh
    - scripts/verify-release-contract.sh
  modified:
    - src/server/cmd/release-contract/main.go
    - src/server/cmd/release-contract/main_test.go
    - src/server/internal/surfacereport/build.go
    - src/server/internal/surfacereport/report.go
    - scripts/build-release-image.sh
key-decisions:
  - "CLI inspection input contains observations only; trusted identity is resolved once internally and attached to every typed component detail."
  - "Report validation rechecks every component identity and both surface digests against the envelope to reject valid-looking identity splices."
  - "Stage B runs from an exact detached clean worktree and accepts only explicit resolver-confirmed monolith with zero skips."
patterns-established:
  - "Stage A remains runnable from a dirty development checkout by confining identity behavior to disposable clean fixtures."
  - "Stage B never accepts fixture roots, caller identity fields, mismatch allowances, or release identity environment overrides."
requirements-completed: []
coverage:
  - id: D1
    description: "Build-report CLI binds trusted identity and committed profile, persists atomically, and rejects schema/profile/identity substitution."
    verification:
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunReportBuildIdentityUsesTrustedResolversAndAtomicWriter"
        status: pass
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunVerifyReportRejectsSchemaAndIdentitySplice"
        status: pass
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunReportPreservesProducerFailure"
        status: pass
    human_judgment: false
  - id: D2
    description: "Stage A exercises all foundation positive and negative families with non-vacuous selectors and leaves repository status unchanged."
    verification:
      - kind: fixture
        ref: "scripts/verify-release-contract-fixtures.sh"
        status: pass
      - kind: fixture
        ref: "scripts/verify-release-build-fixtures.sh"
        status: pass
    human_judgment: false
  - id: D3
    description: "Real clean-HEAD Stage B compares binary, OCI, packaged contract, and typed report identity with zero skips."
    verification:
      - kind: repository-local-runtime
        ref: "clean commit cd711f23d94643b36a886e8225f5ea50610b37a5"
        status: pass
    human_judgment: false
duration: 64 min
completed: 2026-07-17
status: complete
---

# Phase 31 Plan 06: Foundation Report And Stage Gates Summary

**The release-contract foundation now emits one trusted build report and proves it through exhaustive disposable fixtures plus a real zero-skip clean-HEAD Docker gate.**

## Performance

- **Duration:** 64 min active execution across the session boundary
- **Completed:** 2026-07-17T09:50:49Z
- **Tasks:** 3
- **Files modified:** 2 created and 5 modified files

## Accomplishments

- Added observation-only report input, trusted identity enrichment, committed-profile resolution, atomic output, strict read-back, and producer-error preservation.
- Added a Stage A fixture entrypoint that lists and executes 43 concrete tests across contract/digest/operation, identity, report/writer, and CLI families, plus the dynamic artifact mutation suite.
- Added a two-mode verifier and passed real Stage B on clean commit `cd711f23d94643b36a886e8225f5ea50610b37a5` with all three image binaries, OCI labels, packaged contract, and typed report agreeing.

## Task Commits

1. **Task 1: Emit and validate the trusted build-identity report** - `3fda357` (`feat`)
2. **Task 2: Complete contract/report Stage A mutations and re-run artifact fixtures** - `c4b2acf` (`test`)
3. **Task 3: Install Stage A and exact post-commit clean HEAD gate** - `be07c37` (`feat`), corrected for linked worktrees in `cd711f2` (`fix`)

## Automated Evidence

- Report CLI selector - 4 concrete tests listed and passed; full CLI and surface-report packages passed.
- Stage A - 43 concrete foundation tests, zero-match helper self-tests, artifact mutation matrix, and repository-status preservation passed.
- Stage B environment - detached clean linked worktree, Docker 29.1.3, migration state not applicable.
- Stage B release commit - `cd711f23d94643b36a886e8225f5ea50610b37a5`.
- Stage B source tree - `8e38c550d6d47c0ddbc99383e91836f71e2e456b`.
- Stage B contract digest - `sha256:ef38b7e0e64492e3d8167e47a9d9c96bcf06fcb1448bdb2f0744d54bf23cc53d`.
- Stage B image tag - `oblivious-release:cd711f23d94643b36a886e8225f5ea50610b37a5`.
- Stage B report - ignored worktree output under the release-contract temporary directory, result pass, profile monolith, zero skipped checks.
- Residual risks - external target not inspected; supply-chain attestations deferred.

## Decisions Made

- Caller observation JSON cannot carry release identity. The CLI populates component identities from a cached trusted provider and the report constructor resolves the committed profile independently on the same paths.
- `verify-report` validates both type-local details and envelope-to-details identity/digest links, preventing a report assembled from individually valid but unrelated fragments.
- Stage B output remains repository-local E1/E2 and says only `RELS-01 foundation contribution`; it does not claim target, parity, readiness, signature, provenance, or commercial release completion.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added envelope-to-details identity-splice validation**
- **Found during:** Task 1 strict report verifier implementation
- **Issue:** The typed registry validated build details structurally but did not compare their component identities and consumer digest back to the envelope.
- **Fix:** Recompute and compare the trusted tuple plus source/consumer digests during shared report validation.
- **Verification:** The named identity-splice CLI test and full surface-report package pass.
- **Committed in:** `3fda357`

**2. [Rule 1 - Bug] Accepted standard linked-worktree Git metadata**
- **Found during:** First real Stage B attempt at `be07c37`
- **Issue:** Directory-only `.git` detection rejected valid linked worktrees, where `.git` is a pointer file.
- **Fix:** Use Git's own `rev-parse --is-inside-work-tree` check.
- **Verification:** Real Stage B passed from a detached linked worktree at `cd711f2`.
- **Committed in:** `cd711f2`

**3. [Rule 1 - Bug] Kept permission-sensitive fixtures on the system temporary filesystem**
- **Found during:** Stage A recheck after output-directory wiring
- **Issue:** Placing Go temporary fixtures on the mounted workspace lost executable-bit semantics and invalidated the non-executable operation-path test.
- **Fix:** Keep ephemeral test directories on system `/tmp`; reserve ignored output-dir for verifier artifacts.
- **Verification:** Full Stage A passed after the correction.
- **Committed in:** `cd711f2`

---

**Total deviations:** 3 auto-fixed (2 bugs, 1 missing critical validation). **Impact on plan:** All fixes enforce the approved trust boundaries without expanding release claims.

## Known Stubs

- No target/live deployment proof, DB migration run, profile parity, signature, SBOM, provenance, or registry immutability proof is claimed.
- `RELS-01` and `RELS-02` remain pending; Phase 31 contributes only the repository-local foundation.
- The main development checkout still contains four separately owned release-contract timing changes; Stage B used a clean detached worktree and did not alter them.

## User Setup Required

None for repository-local verification. Later target evidence requires environment-specific access and remains outside Phase 31.

## Next Phase Readiness

Phase 31 implementation plans are complete. Phase-level review and verification must pass before routing to Phase 31.1.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-17*

## Self-Check: PASSED

- All declared source, test, fixture, verifier, and summary artifacts exist.
- Task commits `3fda357`, `c4b2acf`, `be07c37`, and `cd711f2` resolve as commits.
- Stage A and real clean-HEAD Stage B have fresh repository-local automated evidence.

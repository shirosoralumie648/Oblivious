---
phase: 31-release-contract-current-baseline
plan: "04"
subsystem: surface-report
tags: [surface-report, typed-registry, build-identity, atomic-write, repository-local]
requires:
  - phase: 31-03
    provides: "Trusted BuildIdentityV1 and explicit-root identity provider"
provides:
  - "Closed nested SurfaceReportV1 envelope and strict decoder"
  - "Typed per-surface evidence.details registry with build-identity registration"
  - "Trusted build-identity report constructor and rollback-backed atomic writer"
affects: [31-06, phase-31.1, phase-31.2]
tech-stack:
  added: []
  patterns: [closed report envelope, typed details registration, resolver-owned identity, rollback-backed atomic replace]
key-files:
  created:
    - src/server/internal/surfacereport/report.go
    - src/server/internal/surfacereport/registry.go
    - src/server/internal/surfacereport/build.go
    - src/server/internal/surfacereport/writer.go
    - src/server/internal/surfacereport/report_test.go
    - src/server/internal/surfacereport/registry_test.go
    - src/server/internal/surfacereport/writer_test.go
  modified: []
key-decisions:
  - "SurfaceReportV1 admits only six nested top-level ownership blocks and strictly rejects unknown or misplaced fields."
  - "Build identity and committed deployment profile come only from providers sharing the same explicit authority paths."
  - "Any returned post-rename write error requires byte-verified restoration of the prior destination or prior absence."
patterns-established:
  - "Later producers extend DetailsRegistry with owned typed schemas but cannot replace the envelope or foundation build-identity registration."
  - "Passing reports require empty drift, errors, and skips plus explicit sorted residual risks in typed details."
requirements-completed: []
coverage:
  - id: D1
    description: "Nested report validation rejects flat, misplaced, unknown, nondeterministic, and unregistered evidence."
    verification:
      - kind: unit
        ref: "src/server/internal/surfacereport/report_test.go#TestSurfaceReportV1NestedValidation"
        status: pass
      - kind: unit
        ref: "src/server/internal/surfacereport/report_test.go#TestSurfaceReportV1RejectsFlatAndMisplacedFields"
        status: pass
      - kind: unit
        ref: "src/server/internal/surfacereport/registry_test.go#TestDetailsRegistryRejectsDuplicateUnknownAndWrongType"
        status: pass
    human_judgment: false
  - id: D2
    description: "Foundation build reports use trusted identity and committed profile resolvers and reject caller substitution or component mismatch."
    verification:
      - kind: unit
        ref: "src/server/internal/surfacereport/report_test.go#TestNewBuildIdentityReportUsesTrustedIdentityAndCommittedProfile"
        status: pass
      - kind: unit
        ref: "src/server/internal/surfacereport/report_test.go#TestNewBuildIdentityReportRejectsUnknownExcludedAndConditionalProfile"
        status: pass
      - kind: unit
        ref: "src/server/internal/surfacereport/report_test.go#TestBuildIdentityReportRejectsCallerIdentityAndMismatch"
        status: pass
    human_judgment: false
  - id: D3
    description: "Atomic writer preserves prior bytes or absence across pre-rename and post-rename injected failures."
    verification:
      - kind: unit
        ref: "src/server/internal/surfacereport/writer_test.go#TestAtomicWriterPreservesDestinationOnInjectedFailures"
        status: pass
      - kind: unit
        ref: "src/server/internal/surfacereport/writer_test.go#TestAtomicWriterRollsBackPostRenameDirectorySyncFailure"
        status: pass
    human_judgment: false
duration: 50 min
completed: 2026-07-16
status: complete
---

# Phase 31 Plan 04: Shared Surface Report And Atomic Writer Summary

**All producers now have one strict nested repository-local report protocol, a typed details extension point, and an atomic writer that restores prior evidence before returning failure.**

## Performance

- **Duration:** 50 min
- **Completed:** 2026-07-16T11:23:39Z
- **Tasks:** 3
- **Files modified:** 7 created files

## Accomplishments

- Added strict `SurfaceReportV1` decoding and validation for trusted identity, UTC evidence time, canonical digests, required collections, sorted arrays, and pass/fail invariants.
- Registered typed `build-identity` details and constructed reports only from a validated identity provider plus a committed authored-profile resolver sharing explicit authority paths.
- Added same-directory staging, synced rollback snapshots, atomic replace, parent sync, post-rename restoration/read-back, residue cleanup, and producer-error preservation.

## Task Commits

1. **Task 1: Define the nested envelope and typed details registry** - `a75d422` (`feat`)
2. **Task 2: Register and construct the trusted build-identity surface** - `418b297` (`feat`)
3. **Task 3: Write validated reports atomically and preserve producer failures** - `de53518` (`feat`)

## Automated Evidence

- Envelope/registry selector - 3 concrete tests listed and passed.
- Build-report selector - 4 concrete tests listed and passed.
- Atomic-writer selector - 4 concrete tests listed and passed.
- `go test ./internal/surfacereport -count=1` - passed.
- `git diff --check` - passed while preserving unrelated working-tree changes.

## Decisions Made

- `evidence.details` remains raw JSON only at the envelope boundary; every accepted value must strict-decode through the concrete type registered for its surface ID.
- The foundation report carries explicit residual risk but cannot claim readiness, parity, E3/E4, or full `RELS-01` completion.
- Rollback cleanup occurs only after the replacement and parent directory are durably synchronized.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected injected writer failures hidden by variable shadowing**
- **Found during:** Task 3 named failure matrix
- **Issue:** Short declarations inside write, rename, and parent-sync branches shadowed the outer error, allowing injected failures to continue as success.
- **Fix:** Assign injected failures to the shared operation error and re-run every pre/post-rename preservation case.
- **Files modified:** `src/server/internal/surfacereport/writer.go`
- **Verification:** All four named writer tests and the full package pass.
- **Committed in:** `de53518`

---

**Total deviations:** 1 auto-fixed bug. **Impact on plan:** The correction restored the specified fail-closed behavior without changing scope.

## Known Stubs

- Active binaries, OCI labels, and packaged contract are not yet wired; Plan 31-05 owns that integration.
- Report CLI commands and aggregate Stage A/clean-HEAD Stage B gates remain Plan 31-06 work.
- Current worktree contains separately owned release-contract timing changes, so no clean-HEAD, target, live, readiness, parity, or release-completion claim is made.

## User Setup Required

None - no external services or credentials are required for this repository-local foundation.

## Next Phase Readiness

Plan 31-05 can bind the trusted tuple into active artifacts; Plan 31-06 can consume this registry/report/writer API. `RELS-01` remains pending.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-16*

## Self-Check: PASSED

- All seven declared source/test artifacts and this summary exist.
- Task commits `a75d422`, `418b297`, and `de53518` resolve as commits.
- All three deliverables have fresh repository-local automated evidence.

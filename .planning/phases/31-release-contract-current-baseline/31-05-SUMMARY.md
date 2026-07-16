---
phase: 31-release-contract-current-baseline
plan: "05"
subsystem: release-build-identity
tags: [binary-inspection, docker, oci-labels, packaged-contract, repository-local]
requires:
  - phase: 31-03
    provides: "Embedded identity provider and side-effect-free inspection handler"
provides:
  - "Pre-side-effect identity inspection in server, migrate, and grpc-smoke"
  - "One linker tuple and matching OCI labels across the runtime image"
  - "Clean-Git release build wrapper and dynamic artifact mutation fixture"
affects: [31-06, phase-31.1, phase-31.2, phase-39]
tech-stack:
  added: []
  patterns: [inspection-before-effects, external Git derivation, post-build tuple comparison, packaged-authority validation]
key-files:
  created:
    - src/server/cmd/server/main_test.go
    - scripts/build-release-image.sh
    - scripts/verify-release-build-fixtures.sh
  modified:
    - src/server/cmd/server/main.go
    - src/server/cmd/migrate/main.go
    - src/server/cmd/migrate/main_test.go
    - src/server/cmd/grpc-smoke/main.go
    - src/server/cmd/grpc-smoke/main_test.go
    - Dockerfile.server
key-decisions:
  - "All active binary inspection paths return before their normal startup continuation and cannot reach config, DB, migration, flag parse, listener, dial, or RPC work."
  - "Git identity is derived outside Docker; the final image must independently agree across all three binary outputs, OCI labels, and canonical packaged-contract digest."
  - "The packaged contract includes its referenced allowlisted profile-operation script so strict semantic validation remains possible inside the runtime image."
patterns-established:
  - "Dirty tracked, staged, or untracked source fails before either Go or Docker build invocation."
  - "Stage A artifact mutations use disposable clean Git plus deterministic tool shims and never count as target/live proof."
requirements-completed: []
coverage:
  - id: D1
    description: "Server and migrate identity inspection precedes every mutable runtime boundary."
    verification:
      - kind: unit
        ref: "src/server/cmd/server/main_test.go#TestIdentityInspectionPrecedesRuntimeSideEffects"
        status: pass
      - kind: unit
        ref: "src/server/cmd/migrate/main_test.go#TestIdentityInspectionPrecedesRuntimeSideEffects"
        status: pass
    human_judgment: false
  - id: D2
    description: "grpc-smoke identity inspection precedes flag parsing, dialing, and RPC dispatch."
    verification:
      - kind: unit
        ref: "src/server/cmd/grpc-smoke/main_test.go#TestIdentityInspectionPrecedesRuntimeSideEffects"
        status: pass
    human_judgment: false
  - id: D3
    description: "Build wrapper rejects dirty source and every binary, label, missing-package, or packaged-digest mismatch."
    verification:
      - kind: fixture
        ref: "scripts/verify-release-build-fixtures.sh"
        status: pass
    human_judgment: false
duration: 31 min
completed: 2026-07-16
status: complete
---

# Phase 31 Plan 05: Binary, OCI, And Packaged Contract Identity Summary

**The runtime image now has one repository-derived identity tuple across all active binaries, OCI metadata, and packaged authority, with inspection proven before runtime effects.**

## Performance

- **Duration:** 31 min
- **Completed:** 2026-07-16T11:55:01Z
- **Tasks:** 3
- **Files modified:** 3 created and 6 modified files

## Accomplishments

- Wired shared identity inspection before server/migrate startup and grpc-smoke parsing/network behavior, with exact JSON/status and zero-effect tests.
- Injected identical linker fields into server, migrate, and grpc-smoke, added matching OCI labels, and packaged the canonical contract/schema plus its referenced operation script.
- Added a clean-Git build/inspection wrapper and dynamic fixture covering dirty tracked/staged/untracked source, three binary mismatches, three OCI label mismatches, packaged contract mutation, and packaged contract absence.

## Task Commits

1. **Task 1: Wire and dynamically prove server and migrate inspection ordering** - `113dca6` (`feat`)
2. **Task 2: Wire and dynamically prove grpc-smoke inspection ordering** - `e58487c` (`feat`)
3. **Task 3: Inject and inspect one identity across binaries, OCI labels, and packaged contract** - `8577649` (`feat`)

## Automated Evidence

- Server inspection selector - 1 concrete test listed and passed.
- Migrate inspection selector - 1 concrete test listed and passed.
- grpc-smoke inspection selector - 1 concrete test listed and passed; full package also passed.
- `bash -n scripts/build-release-image.sh scripts/verify-release-build-fixtures.sh` - passed.
- `bash scripts/verify-release-build-fixtures.sh` - baseline and all dynamic mutations passed.
- Related six-package Go regression - passed; DB-backed migrate cases remain environment-skipped without `TEST_DATABASE_URL`.
- `git diff --check` - passed while preserving unrelated working-tree changes.

## Decisions Made

- Final binary identity is inspected from the built runtime image, where the explicit packaged root is `/app`; no caller identity flags or environment values are accepted.
- Host Git and foundation CLI derive the tuple before Docker, while `.dockerignore` continues to exclude `.git`.
- Canonical packaged digest is recomputed through the same strict contract loader, not by raw byte hashing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected valid dirty=false parsing in the build wrapper**
- **Found during:** Task 3 baseline fixture
- **Issue:** `jq -e` treats boolean `false` as an unsuccessful result, so the required clean identity was rejected.
- **Fix:** Read the boolean normally and enforce the exact `false` value in the explicit tuple invariant.
- **Verification:** Baseline and the full artifact mutation fixture pass.
- **Committed in:** `8577649`

**2. [Rule 2 - Missing Critical] Packaged the contract's referenced operation script**
- **Found during:** Task 3 packaged digest read-back
- **Issue:** Strict contract validation correctly rejected a package containing contract/schema but missing its profile-bound operation path.
- **Fix:** Package and extract `scripts/release-profile-operation.sh` with the authority before canonical digest verification.
- **Verification:** Extracted-package validation and digest comparison pass.
- **Committed in:** `8577649`

---

**Total deviations:** 2 auto-fixed (1 bug, 1 missing critical dependency). **Impact on plan:** Both fixes are required for the specified fail-closed identity path and remain within release-build ownership.

## Known Stubs

- Real clean-HEAD Docker Stage B has not run; the current worktree contains separately owned release-contract timing changes.
- DB integration, target environment, signatures, SBOM/provenance, immutable registry digest, readiness, and parity are not proven here.
- The mounted repository uses `core.filemode=false`; scripts are executable in the working filesystem but Git records them as `100644`, so documented `bash scripts/...` invocation remains authoritative.

## User Setup Required

None for Stage A. Clean-HEAD Stage B requires a reachable Docker daemon and is owned by Plan 31-06.

## Next Phase Readiness

Plan 31-06 can add trusted report CLI commands, aggregate Stage A, and exact clean-HEAD Stage B using the completed artifact wrapper. `RELS-01` remains pending.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-16*

## Self-Check: PASSED

- All nine declared source/test/build artifacts and this summary exist.
- Task commits `113dca6`, `e58487c`, and `8577649` resolve as commits.
- All three deliverables have fresh repository-local automated evidence.

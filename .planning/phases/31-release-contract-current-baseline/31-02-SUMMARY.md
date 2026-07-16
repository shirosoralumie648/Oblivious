---
phase: 31-release-contract-current-baseline
plan: "02"
subsystem: release-contract
tags: [canonical-json, sha256, operation-dispatch, go-test, fail-closed]
requires:
  - phase: 31-01
    provides: "Strict AuthoredContractV1 loader, explicit profile resolver, and profile-bound OperationRef values"
provides:
  - "Non-vacuous list-before-run helper for focused Go evidence"
  - "Deterministic canonical contract bytes and sha256 digest"
  - "Explicit-profile literal-argv operation dispatcher with zero-effect rejection"
affects: [31-03, 31-05, 31-06, phase-31.1, phase-38]
tech-stack:
  added: []
  patterns: [list-before-run test selection, normalized typed canonical JSON, injected literal-argv runner]
key-files:
  created:
    - scripts/run-go-tests-matched.sh
    - scripts/run-go-tests-matched-fixtures.sh
    - src/server/internal/releasecontract/digest.go
    - src/server/internal/releasecontract/digest_test.go
    - src/server/internal/releasecontract/operation.go
    - src/server/internal/releasecontract/operation_test.go
    - src/server/internal/releasecontract/testdata/canonical-equivalent-a.json
    - src/server/internal/releasecontract/testdata/canonical-equivalent-b.json
    - src/server/internal/releasecontract/testdata/canonical-semantic-change.json
  modified: []
key-decisions:
  - "Canonical contract JSON sorts object keys and set-like typed collections, preserves semantic argv order, uses UTF-8, and has no trailing newline."
  - "Operation execution receives only literal argv plus PATH; release identity cannot enter through inherited environment."
  - "RELS-01 remains pending because this plan supplies repository-local foundation evidence only."
patterns-established:
  - "Every focused Go selector lists concrete Test names first and fails before -run on invalid or zero matches."
  - "Operation dispatch resolves an explicit committed profile and allowlisted real path before the injected Runner is called."
requirements-completed: []
coverage:
  - id: D1
    description: "Focused Go verification cannot pass when its selector is invalid, matches no concrete test, names a missing package, or runs a failing test."
    verification:
      - kind: integration
        ref: "bash scripts/run-go-tests-matched-fixtures.sh"
        status: pass
    human_judgment: false
  - id: D2
    description: "Validated typed contracts produce deterministic canonical bytes and sha256 digests across formatting and set ordering while semantic changes alter the digest."
    verification:
      - kind: unit
        ref: "src/server/internal/releasecontract/digest_test.go#TestCanonicalBytesGolden"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/digest_test.go#TestDigestEquivalentFormatting"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/digest_test.go#TestDigestSemanticMutation"
        status: pass
    human_judgment: false
  - id: D3
    description: "Profile-bound operations reject unsafe or excluded inputs before runner effects and pass shell metacharacters as literal argv."
    verification:
      - kind: unit
        ref: "src/server/internal/releasecontract/operation_test.go#TestDispatchCommittedOperationPassesLiteralArgv"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/operation_test.go#TestDispatchRejectsBeforeRunner"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/operation_test.go#TestResolveOperationPathRejectsEscapes"
        status: pass
    human_judgment: false
duration: 9 min
completed: 2026-07-16
status: complete
---

# Phase 31 Plan 02: Deterministic Contract Identity And Safe Operations Summary

**Canonical contract digests, non-vacuous focused Go evidence, and explicit-profile literal-argv operation dispatch now protect the release foundation from three false-green paths.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-07-16T09:51:35Z
- **Completed:** 2026-07-16T10:00:42Z
- **Tasks:** 3
- **Files modified:** 9 created files

## Accomplishments

- Added a repository-owned Go test helper that lists concrete tests first and preserves invalid-selector, missing-package, zero-match, and failing-test status.
- Added normalized compact UTF-8 canonical bytes with no trailing newline and fixed `sha256:2baeda420ab178420e0fcb64072ce3c91bc3b821ed8dd1eede6eac073864cbfa` golden evidence.
- Added an explicit committed-profile dispatcher with allowlisted real-path resolution, literal argv, minimal environment, typed errors, and zero-runner-call rejection tests.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add a non-vacuous targeted Go test helper** - `7855ac6` (`chore`)
2. **Task 2: Canonicalize validated contracts and lock the digest with golden vectors** - `0f4884f` (`feat`)
3. **Task 3: Resolve and dispatch profile-bound operations without a shell** - `d5a567a` (`feat`)

## Verification

- `bash -n scripts/run-go-tests-matched.sh scripts/run-go-tests-matched-fixtures.sh` - passed.
- `bash scripts/run-go-tests-matched-fixtures.sh` - passed all match/zero/invalid/package/failure cases.
- Matched digest selector - 3 concrete tests listed and passed.
- Matched operation selector - 3 concrete tests listed and passed.
- `cd src/server && go test ./internal/releasecontract -count=1` - passed.
- `git diff --check` - passed with preserved unrelated working-tree changes.

## Files Created/Modified

- `scripts/run-go-tests-matched.sh` - list-before-run focused Go test entrypoint.
- `scripts/run-go-tests-matched-fixtures.sh` - self-fixtures for false-green and exit propagation cases.
- `src/server/internal/releasecontract/digest.go` - versioned canonical bytes and digest API.
- `src/server/internal/releasecontract/digest_test.go` and `testdata/canonical-*.json` - exact canonical/digest vectors.
- `src/server/internal/releasecontract/operation.go` - profile-safe dispatcher, path resolver, and literal command runner.
- `src/server/internal/releasecontract/operation_test.go` - zero-effect rejection, path escape, literal argv, and environment tests.

## Decisions Made

- Canonicalization normalizes typed set-like collections but never reorders operation argv, which remains semantic.
- The default runner inherits no ambient environment except `PATH`; identity fields cannot be supplied through operation environment.
- Existing uncommitted profile timing work was treated as separate ownership and left out of every 31-02 commit.
- `RELS-01` is not marked complete; runtime readiness and target/live proof remain owned by later phases.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Made the missing-package fixture accept the current Go diagnostic**
- **Found during:** Task 1 acceptance verification
- **Issue:** Go 1.25 reports `directory not found`, while the first fixture assertion accepted only `does not exist` or `cannot find`.
- **Fix:** Added the equivalent `not found` diagnostic without weakening the required nonzero/listing-failure assertions.
- **Files modified:** `scripts/run-go-tests-matched-fixtures.sh`
- **Verification:** The complete fixture suite passed under explicit outer `set -e`.
- **Committed in:** `7855ac6`

**2. [Rule 3 - Blocking] Preserved executable modes in the Git index**
- **Found during:** Task 1 commit inspection
- **Issue:** The mounted worktree has `core.filemode=false`, so filesystem chmod initially staged both helpers as `100644`.
- **Fix:** Explicitly set both index entries to `100755` and amended only the Task 1 commit.
- **Files modified:** `scripts/run-go-tests-matched.sh`, `scripts/run-go-tests-matched-fixtures.sh`
- **Verification:** `git ls-files -s` reports mode `100755` for both scripts.
- **Committed in:** `7855ac6`

---

**Total deviations:** 2 auto-fixed (Rules 1 and 3). **Impact on plan:** Both fixes strengthened the intended evidence and execution contract without adding scope.

## Issues Encountered

- An initial outer multi-command shell did not use `set -e`, so a failing fixture was followed by a successful `git diff --check` and the aggregate shell status looked green. The fixture was rerun directly, the diagnostic mismatch was fixed, and every subsequent compound verification used explicit fail-fast mode.

## Known Stubs

- Successful repository-local operation dispatch does not prove migration, deployment, rollback, or deployment-profile parity; Phase 38 owns those target checks.
- Runtime readiness, continuous side-effect authorization, report aggregation, and E3/E4 evidence remain outside this plan.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 31-03 can consume deterministic `Digest` output and the profile-safe `Dispatcher` while deriving trusted clean-source `BuildIdentityV1`. The full `RELS-01` requirement remains pending.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-16*

## Self-Check: PASSED

- All nine declared implementation/test artifacts and this summary exist.
- Task commits `7855ac6`, `0f4884f`, and `d5a567a` resolve as commits.
- Both helper scripts are tracked as executable mode `100755`.
- Coverage classifies all three deliverables with fresh passing automated evidence.

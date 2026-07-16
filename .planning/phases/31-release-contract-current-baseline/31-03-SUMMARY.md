---
phase: 31-release-contract-current-baseline
plan: "03"
subsystem: build-identity
tags: [git, linker, build-identity, cli, repository-local]
requires:
  - phase: 31-02
    provides: "Canonical contract digest and safe profile operation dispatcher"
provides:
  - "Closed validated BuildIdentityV1 and package-private linker transport"
  - "Clean Git and recomputed embedded identity providers"
  - "Side-effect-free inspection handler and foundation release-contract CLI"
affects: [31-04, 31-05, 31-06, phase-31.1, phase-31.2]
tech-stack:
  added: []
  patterns: [explicit-root Git derivation, comparison-only CI identity, recomputed embedded identity, structured CLI failures]
key-files:
  created:
    - src/server/internal/buildinfo/identity.go
    - src/server/internal/buildinfo/linker.go
    - src/server/internal/buildinfo/provider.go
    - src/server/internal/buildinfo/identity_test.go
    - src/server/internal/buildinfo/provider_test.go
    - src/server/internal/buildinfo/inspect.go
    - src/server/internal/buildinfo/inspect_test.go
    - src/server/cmd/release-contract/main.go
    - src/server/cmd/release-contract/main_test.go
  modified: []
key-decisions:
  - "Only clean Git HEAD commit/tree plus recomputed canonical contract digest can construct GitProvider identity."
  - "GITHUB_SHA is comparison-only; other identity-like environment values are ignored."
  - "All CLI contract and identity paths require explicit repo, contract, and schema inputs."
patterns-established:
  - "Embedded linker values remain untrusted until exact shape and packaged-contract digest verification pass."
  - "Active binaries can call HandleInspection before startup side effects without giving the handler a continuation callback."
requirements-completed: []
coverage:
  - id: D1
    description: "BuildIdentityV1 and linker parsing accept only exact lowercase clean repository-local tuples with stable failure classes."
    verification:
      - kind: unit
        ref: "src/server/internal/buildinfo/identity_test.go#TestBuildIdentityV1Validation"
        status: pass
      - kind: unit
        ref: "src/server/internal/buildinfo/identity_test.go#TestParseLinkedIdentityRejectsInvalidValues"
        status: pass
    human_judgment: false
  - id: D2
    description: "Git and embedded providers derive or validate identity against explicit-root clean source and actual contract bytes."
    verification:
      - kind: integration
        ref: "src/server/internal/buildinfo/provider_test.go#TestGitProviderDerivesCleanIdentity"
        status: pass
      - kind: integration
        ref: "src/server/internal/buildinfo/provider_test.go#TestGitProviderRejectsDirtyAndSubstitution"
        status: pass
      - kind: integration
        ref: "src/server/internal/buildinfo/provider_test.go#TestEmbeddedProviderRecomputesContractDigest"
        status: pass
      - kind: integration
        ref: "src/server/internal/buildinfo/provider_test.go#TestIdentityProvidersUseExplicitRepoRoot"
        status: pass
    human_judgment: false
  - id: D3
    description: "Inspection and all five foundation CLI subcommands expose trusted primitives without identity override or external side effects in tests."
    verification:
      - kind: unit
        ref: "src/server/internal/buildinfo/inspect_test.go#TestHandleInspectionReturnsTrustedIdentityWithoutSideEffects"
        status: pass
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunValidateDigestAndIdentity"
        status: pass
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunRejectsIdentityOverrideFlags"
        status: pass
      - kind: unit
        ref: "src/server/cmd/release-contract/main_test.go#TestRunInspectHasNoExternalSideEffects"
        status: pass
    human_judgment: false
duration: 19 min
completed: 2026-07-16
status: complete
---

# Phase 31 Plan 03: Trusted Build Identity And Foundation CLI Summary

**Clean Git and recomputed packaged-contract identity now feed one validated BuildIdentityV1, with a side-effect-free inspector and explicit-root foundation CLI.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-07-16T10:11:58Z
- **Completed:** 2026-07-16T10:30:48Z
- **Tasks:** 3
- **Files modified:** 9 created files

## Accomplishments

- Added exact six-field identity validation and package-private linker transport with stable missing/mismatch/dirty/digest errors.
- Added bounded explicit-root Git derivation and embedded identity verification against actual contract bytes, covered by disposable clean/detached/dirty fixtures.
- Added `HandleInspection` plus validate/digest/identity/operation/inspect CLI commands with structured output and no identity override flags.

## Task Commits

1. **Task 1: Define and validate BuildIdentityV1 plus linker inputs** - `7f50b55` (`feat`)
2. **Task 2: Resolve trusted identity from clean Git or validated embedded inputs** - `659dbd2` (`feat`)
3. **Task 3: Expose foundation CLI primitives and the shared side-effect-free inspector** - `e1e8d20` (`feat`)

## Verification

- Identity/linker selector - 2 concrete tests listed and passed.
- Git/embedded provider selector - 4 concrete tests listed and passed.
- Inspection selector - 3 concrete tests listed and passed.
- CLI selector - 4 concrete tests listed and passed.
- `go test ./internal/buildinfo ./cmd/release-contract -count=1` - passed.
- `git diff --check` - passed while preserving unrelated working-tree changes.

## Decisions Made

- Git command output is bounded and failures surface only stable identity codes, not arbitrary repository or environment content.
- `GITHUB_SHA` can reject a mismatch but cannot supply identity; `RELEASE_COMMIT`, `SOURCE_TREE`, and `CONTRACT_DIGEST` have no authority.
- The CLI requires explicit `--repo --contract --schema` for every source-bound command and reports only `repository-local` evidence.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected common CLI flag binding to update caller-owned state**
- **Found during:** Task 3 required CLI selector
- **Issue:** Returning `commonOptions` by value left FlagSet pointers attached to an internal copy, so valid flags parsed but the caller still saw empty values.
- **Fix:** Bind flags directly into a caller-owned `*commonOptions`.
- **Files modified:** `src/server/cmd/release-contract/main.go`
- **Verification:** All four named CLI tests and the full command package pass.
- **Committed in:** `e1e8d20`

---

**Total deviations:** 1 auto-fixed (Rule 1). **Impact on plan:** The fix restored the specified CLI contract without changing scope.

## Issues Encountered

- The first CLI execution run correctly failed the new tests and exposed the value-copy binding defect; systematic root-cause tracing isolated and fixed it before commit.

## Known Stubs

- Current binaries and OCI images do not yet embed or expose this identity; Plan 31-05 owns that integration.
- The product worktree remained dirty due separately owned readiness timing changes, so only disposable Git fixtures were used for Stage A; no clean-HEAD claim is made.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plans 31-04 and 31-05 can consume one validated identity provider for report construction and binary/OCI/package binding. `RELS-01` remains pending.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-16*

## Self-Check: PASSED

- All nine declared source/test artifacts and this summary exist.
- Task commits `7f50b55`, `659dbd2`, and `e1e8d20` resolve as commits.
- Coverage classifies all three deliverables with fresh passing automated evidence.

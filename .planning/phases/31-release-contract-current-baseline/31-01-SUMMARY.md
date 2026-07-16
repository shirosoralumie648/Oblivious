---
phase: 31-release-contract-current-baseline
plan: "01"
subsystem: release-contract
tags: [json-schema, go, release-profile, strict-loader, fail-closed]
requires: []
provides:
  - "Closed Draft 2020-12 contract/v1 schema and concrete AuthoredContractV1 model"
  - "Authored capability catalog with monolith-only committed/default profile policy"
  - "Explicit-root schema-first loader and committed-profile resolver"
affects: [31-02, 31-03, phase-31.1, phase-31.2]
tech-stack:
  added: [github.com/santhosh-tekuri/jsonschema/v6@v6.0.2]
  patterns: [schema-first decode, duplicate-key rejection, explicit-root path authority, typed semantic closure]
key-files:
  created:
    - config/release/contract.schema.json
    - config/release/contract.v1.json
    - scripts/release-profile-operation.sh
    - src/server/internal/releasecontract/contract.go
    - src/server/internal/releasecontract/load.go
    - src/server/internal/releasecontract/contract_test.go
  modified:
    - src/server/go.mod
    - src/server/go.sum
key-decisions:
  - "contract.v1.json is the sole authored authority; source identity, dynamic availability, observations, and contract digest remain derived or deferred."
  - "monolith is the only committed/default profile; microservices, dual, and split remain excluded with profile_parity_unproven."
  - "RELS-01 remains pending because this plan contributes only the repository-local foundation."
patterns-established:
  - "Every object boundary is closed in JSON Schema and every typed decode rejects unknown fields and trailing JSON."
  - "Executable references are repo-relative, scripts-allowlisted, symlink-resolved, and validated against the explicit absolute repoRoot."
requirements-completed: []
coverage:
  - id: D1
    description: "Strict contract/v1 schema and concrete typed authority model reject unknown nested fields and authored identity/digest fields."
    verification:
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestContractSchemaRejectsUnknownAndAuthoredIdentityFields"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestAuthoredContractV1ModelsRequiredSections"
        status: pass
    human_judgment: false
  - id: D2
    description: "Authored capability/profile catalog is fully reference-closed, monolith-only committed/default, and operation dispatch is fixed-argv fail-closed."
    verification:
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestCheckedInContractProfilePolicyAndReferenceClosure"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestReleaseProfileOperationScriptRejectsExcludedProfiles"
        status: pass
      - kind: other
        ref: "bash -n scripts/release-profile-operation.sh"
        status: pass
    human_judgment: false
  - id: D3
    description: "Schema-first loaders and FileProfileResolver reject malformed or unsafe inputs and honor only an explicit committed profile beneath the supplied root."
    verification:
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestLoadBytesRejectsNegativeFamilies"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestLoadBytesUsesExplicitRepoRootAndIgnoresCWD"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestFileProfileResolverRequiresCommittedExplicitProfile"
        status: pass
      - kind: unit
        ref: "src/server/internal/releasecontract/contract_test.go#TestLoadRejectsContractAndSchemaOutsideExplicitRoot"
        status: pass
    human_judgment: false
duration: 47 min
completed: 2026-07-16
status: complete
---

# Phase 31 Plan 01: Release Contract Current Baseline Summary

**Strict authored release contract, monolith-only profile authority, and explicit-root fail-closed loader are now implemented as the repository-local Phase 31 foundation.**

## Performance

- **Duration:** 47 min
- **Started:** 2026-07-16T05:31:16Z
- **Completed:** 2026-07-16T06:18:45Z
- **Tasks:** 3
- **Files modified:** 8 tracked files plus 6 created files

## Accomplishments

- Added a closed Draft 2020-12 `contract/v1` schema and concrete Go model with no open-ended authority maps or authored identity fields.
- Authored capability, reason-code, catalog, surface, readiness, topology, dependency, and operation references with one committed/default `monolith` profile and explicitly excluded candidate profiles.
- Added bounded UTF-8, duplicate-key, schema-first, strict typed decode, semantic closure, executable path safety, and explicit committed-profile resolution.
- Added an executable fixed-argv profile operation wrapper that rejects excluded/unknown profiles and fail-closes monolith rollback.

## Task Commits

Each task was committed atomically:

1. **Task 1: Define the strict authored contract schema and typed model** - `c043cbc` (`feat`)
2. **Task 2: Author the capability/profile contract and profile operation entrypoint** - `7d58301` (`feat`)
3. **Task 3: Enforce schema-first strict loading and semantic closure** - `70a834d` (`feat`)

## Verification

- `bash -n scripts/release-profile-operation.sh` - passed.
- `cd src/server && go test ./internal/releasecontract -count=1` - passed (`ok`, 0.155s on final run).
- Named test listing for all four required tests - passed with exact matches.
- `cd src/server && go mod verify` - passed (`all modules verified`).
- `git diff --check` - passed.

## Files Created/Modified

- `config/release/contract.schema.json` - closed structural schema for `AuthoredContractV1`.
- `config/release/contract.v1.json` - sole authored capability/profile authority.
- `scripts/release-profile-operation.sh` - fixed profile/operation dispatch with fail-closed rollback.
- `src/server/internal/releasecontract/contract.go` - typed model, enums, error codes, semantic closure, and operation path validation.
- `src/server/internal/releasecontract/load.go` - bounded explicit-root schema-first loader and profile resolver.
- `src/server/internal/releasecontract/contract_test.go` - positive, mutation, path-safety, root-authority, and profile-resolution tests.
- `src/server/go.mod`, `src/server/go.sum` - pinned JSON Schema validator and checksums.

## Decisions Made

- Authored JSON contains commitments and references only; build commit/tree, dirty state, observations, and contract digest are outside this authority.
- `monolith` is the only committed/default deployment profile; repository microservice assets cannot promote excluded profiles.
- `RELS-01` is intentionally not marked complete: this plan delivers only its repository-local foundation and leaves runtime readiness/target evidence to later phases.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted the pinned JSON Schema v6 API**
- **Found during:** Task 1 verification
- **Issue:** `Compiler.AddResource` in `jsonschema/v6@v6.0.2` accepts a decoded JSON value rather than an `io.Reader`.
- **Fix:** Used the package's `UnmarshalJSON` helper before registering the schema resource.
- **Files modified:** `src/server/internal/releasecontract/contract_test.go`
- **Verification:** Package tests and `go mod verify` passed.
- **Committed in:** `c043cbc`

**2. [Rule 1 - Bug] Corrected schema NUL-path regex for Go's regexp implementation**
- **Found during:** Task 1 schema compilation
- **Issue:** Draft schema pattern `\\u0000` was rejected by the validator's regexp engine.
- **Fix:** Used the equivalent `\\x00` exclusion while retaining NUL rejection.
- **Files modified:** `config/release/contract.schema.json`
- **Verification:** Schema compiles and negative schema tests pass.
- **Committed in:** `c043cbc`

**3. [Rule 2 - Missing Critical] Made profile-parity reason applicable to excluded capability overrides**
- **Found during:** Task 2 semantic validation
- **Issue:** Excluded profile operation overrides used `profile_parity_unproven`, but the catalog only allowed profile-level applicability.
- **Fix:** Declared the stable reason for both `capability` and `profile` contexts.
- **Files modified:** `config/release/contract.v1.json`
- **Verification:** Checked-in contract reference-closure test passes.
- **Committed in:** `7d58301`

**4. [Rule 1 - Bug] Canonicalized set-like readiness reference ordering**
- **Found during:** Task 2 semantic validation
- **Issue:** Profile readiness IDs were not lexicographically ordered (`scheduler` preceded `sandbox-capacity`).
- **Fix:** Ordered all profile reference arrays deterministically.
- **Files modified:** `config/release/contract.v1.json`
- **Verification:** Semantic contract validation passes.
- **Committed in:** `7d58301`

**5. [Rule 3 - Blocking] Preserved executable mode in the Git index**
- **Found during:** Task 2 commit inspection
- **Issue:** The mounted worktree reports `core.filemode=false`, so chmod alone staged the operation script as `100644`.
- **Fix:** Explicitly indexed `scripts/release-profile-operation.sh` with mode `100755` and amended the task commit.
- **Files modified:** `scripts/release-profile-operation.sh`
- **Verification:** `git ls-files -s` reports mode `100755`; `bash -n` and operation tests pass.
- **Committed in:** `7d58301`

**Total deviations:** 5 auto-fixed (Rules 1, 2, and 3). **Impact:** All fixes were required for validator compatibility, deterministic closure, or executable fail-closed behavior; no product scope was added.

## Issues Encountered

- The NTFS/FUSE mount transiently returned `ENOSPC` for new directory entries while creating this summary despite 921G and 965M inodes reported free. Capacity, permissions, entry count, name collision, and atomic rename probes excluded a persistent repository fault; creation recovered without deleting evidence or changing the mount.

## Known Stubs

None in the files created or modified by this plan. Dynamic readiness, runtime side-effect enforcement, surface parity, target evidence, and supply-chain proof remain intentionally deferred by the Phase 31 boundary.

## User Setup Required

None - no external service configuration required for this repository-local foundation.

## Next Phase Readiness

Plan 31-02 can consume the typed contract and loader contracts directly. `RELS-01` remains pending until the later readiness and target-evidence phases close the full requirement; this summary claims only repository-local E1/E2 foundation progress.

---
*Phase: 31-release-contract-current-baseline*
*Completed: 2026-07-16*

## Self-Check: PASSED

- All six created implementation artifacts and this summary exist.
- Task commits `c043cbc`, `7d58301`, and `70a834d` resolve as commits.
- The operation script is tracked as executable mode `100755`.
- Coverage classification reports 3/3 deliverables automatically covered with passing evidence.

---
phase: 31
slug: release-contract-current-baseline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-15
updated: 2026-07-16
---

# Phase 31 - Foundation Validation Strategy

> Nyquist strategy for authored contract, trusted build identity, shared report protocol, excluded-profile operations and two-stage clean-commit proof.

## Scope And Current Planning State

This validation contract covers only the Phase 31 foundation contribution to `RELS-01`. It does not validate dynamic readiness/runtime enforcement (31.1), surface parity/aggregate gates (31.2), deployment-mode parity (38), or target/supply-chain proof (39).

No `31-*-PLAN.md` files exist at the time of this update. Therefore this document deliberately contains no task IDs, plan rows, wave assignments or claimed task coverage. The planner must populate the per-task verification map after creating 5-6 plans and must recalculate `nyquist_compliant`; until then it remains `false`.

## Plan Shape Constraints

- Target: **5-6 plans**.
- Each plan: **2-3 tasks**.
- Each plan: at most **9 modified files**, with 5-8 preferred.
- Plans in the same wave: **no overlapping writes**.
- Every task: one narrow automated verification plus `git diff --check`.
- Identity-bearing tasks must label their proof as Stage A pre-commit or Stage B post-commit; Stage A cannot substitute for Stage B.

## Test Infrastructure

| Capability | Planned entrypoint | Current state |
|---|---|---|
| Non-vacuous targeted Go tests | `scripts/run-go-tests-matched.sh` | Missing; Wave 0 dependency |
| Contract/schema/digest fixtures | `scripts/verify-release-contract-fixtures.sh` | Missing; Wave 0 dependency |
| Foundation aggregate | `scripts/verify-release-contract.sh` | Missing; owning plan required |
| Go package tests | `go test` through the matched-test helper | Packages do not yet exist |
| Temporary Git identity fixtures | `src/server/internal/buildinfo/*_test.go` | Missing; Wave 0 dependency |
| Atomic writer failure injection | `src/server/internal/surfacereport/*_test.go` | Missing; Wave 0 dependency |
| Binary/OCI/package inspection | foundation build fixture invoked by `verify-release-contract.sh` | Missing; owning plan required |

Existing `scripts/check.sh`, `scripts/verify-quality-gates.sh` and `scripts/verify-commercial-completion.sh` are not direct Phase 31 verification entrypoints. The unique aggregate wiring belongs to Phase 31.2.

## Nyquist Coverage Contract

| Validation ID | Foundation behavior | Fast proof | Negative proof | Completion evidence |
|---|---|---|---|---|
| V31-F01 | Authored contract passes JSON Schema and strict typed semantic validation. | focused loader tests | unknown/duplicate/trailing/enum/reference mutations | deterministic diagnostics and nonzero exit |
| V31-F02 | Contract canonical bytes and `sha256:` digest are stable. | golden digest tests | formatting/order vs semantic mutations | golden bytes and expected digest |
| V31-F03 | Explicit profile and `OperationRef` are safe and profile-bound. | resolver/dispatcher tests | missing/excluded/mismatch/traversal/symlink/shell cases | stable failure code and zero runner calls |
| V31-F04 | `BuildIdentityV1` derives from clean commit/tree plus contract digest only. | temporary clean Git fixture | dirty state and env/flag substitution | exact expected identity tuple |
| V31-F05 | All active binaries, OCI labels and packaged contract agree, and the foundation producer emits their trusted build report. | build/inspect fixture | one-component mismatch, missing contract and caller identity substitution | build `SurfaceReportV1` bound to clean source tuple |
| V31-F06 | `SurfaceReportV1` and its typed details registry enforce nested ownership and trusted identity. | report/registry validation tests | flat fields, duplicate/unregistered details, caller identity and outcome inconsistency | schema-valid deterministic envelope plus foundation build registration |
| V31-F07 | Atomic report output preserves the prior destination on failure. | writer success test | write/sync/close/rename/unwritable failures | no partial/temp files and prior bytes unchanged |
| V31-F08 | Targeted Go tests cannot pass with zero matched tests. | known matching regex | zero-match and invalid regex | helper fails before executing the test run |
| V31-F09 | Foundation claims remain repository-local E1/E2. | report assertion | E3/E4/readiness/full-RELS-01 claim mutation | explicit evidence class and residual risks |

The planner must map every created task to at least one row above and add task-specific commands. No three consecutive tasks may lack executable automated feedback.

## Stage A - Pre-Commit Development Verification

Stage A is allowed while the implementation worktree is dirty because identity tests operate on disposable clean repositories.

Required sequence:

1. Create a temporary Git repository with a known authored contract and committed tree.
2. Derive expected commit, tree and canonical contract digest from that fixture.
3. Run strict contract, operation, identity and report package tests through `scripts/run-go-tests-matched.sh`.
4. Run one-field mutation fixtures and assert stable nonzero error classes.
5. Run `git diff --check` on the product repository.
6. Write all generated reports/build outputs under ignored `.tmp` or the temporary fixture, never into tracked evidence files.

Minimum temporary-Git cases:

- clean committed source succeeds;
- modified tracked file fails;
- staged change fails;
- relevant untracked file fails;
- detached clean `HEAD` succeeds with the same derivation rules;
- `GITHUB_SHA`, env and CLI substitution attempts cannot replace Git results;
- contract mutation changes digest and invalidates a previously embedded identity.

## Stage B - Post-Commit Clean HEAD Verification

Stage B is mandatory for every identity-bearing plan and for Phase 31 completion. It runs only after the implementation has been atomically committed.

Required sequence:

1. Require `git status --porcelain=v1 --untracked-files=all` to be empty; ignored `.tmp` outputs do not affect source identity.
2. Resolve `HEAD^{commit}` and `HEAD^{tree}` directly from Git.
3. Validate the checked-in contract and recompute its canonical digest.
4. Build server, migrate and grpc-smoke from the same identity input.
5. Build/inspect the runtime image without exposing `.git` to Docker.
6. Compare every binary identity, OCI label and packaged contract digest with the clean source tuple.
7. Emit a repository-local report containing environment, commands, result, skips and residual risks.
8. Fail nonzero on any dirty state, mismatch, missing component or skipped committed check.
9. Push only the exact commit that passed this gate.

Stage B must be read-only with respect to tracked files. A failed Stage B leads to a new fix commit and a fresh run; it does not amend the evidence tuple or use an override.

## Zero-Match Go Test Prevention

Raw targeted commands such as `go test ./internal/buildinfo -run TestIdentity` are not acceptable evidence because Go exits successfully when no test matches.

`scripts/run-go-tests-matched.sh` must:

1. run `go test <packages> -list <regex>`;
2. parse concrete matching test names;
3. fail when the regex is invalid or the count is zero;
4. print the matched names/count;
5. only then run `go test <packages> -run <regex> -count=1`;
6. propagate the actual test exit status.

The helper itself needs fixtures for a matching regex, a valid zero-match regex and an invalid regex.

## Contract And Digest Negative Matrix

| Mutation | Required result |
|---|---|
| unknown top-level or nested field | schema/strict-decode failure |
| duplicate JSON key, capability ID or profile ID | deterministic rejection |
| second JSON value/trailing non-whitespace | trailing-input rejection |
| authored commit/tree/digest field | schema rejection |
| zero/multiple default profiles | semantic rejection |
| non-monolith profile marked committed/default | commitment invariant rejection |
| broken capability/reason/surface/catalog reference | reference-closure rejection |
| key or insignificant whitespace reorder | identical canonical digest |
| semantic value change | different canonical digest |

## OperationRef Negative Matrix

| Mutation | Required result |
|---|---|
| runtime profile omitted | `profile_required`, no child call |
| unknown profile | stable nonzero error, no child call |
| excluded profile operation | `profile_excluded`, no child call or side effect |
| owning profile differs from `profileId` | contract/operation mismatch, no child call |
| absolute, traversal or symlink-escape path | unsafe-path rejection |
| shell metacharacters inside an argv element | passed as one literal argument for committed test fixture; never evaluated by a shell |
| path outside allowlist | unsafe-path rejection |

## Build Identity Negative Matrix

| Mutation | Required result |
|---|---|
| invalid/missing commit or tree | `build_identity_missing` or mismatch class |
| dirty source | `source_worktree_dirty` |
| binary differs from clean source tuple | `build_identity_mismatch` |
| OCI label differs | `build_identity_mismatch` |
| packaged contract differs or is absent | `contract_digest_mismatch` or missing class |
| `GITHUB_SHA` differs from Git | comparison failure; no override |
| caller supplies release identity field | rejected input |

## Surface Report And Writer Negative Matrix

| Mutation/failure | Required result |
|---|---|
| legacy flat report fields | `surface_schema_invalid` |
| producer-supplied release identity | rejected construction/input |
| environment/mode stored in `drift` | schema rejection |
| arbitrary unregistered `evidence.details` | schema rejection |
| pass with non-empty errors/skips | outcome invariant rejection |
| write, sync, close or rename failure | nonzero writer error; old destination preserved |
| missing parent | parent created and atomic success when permissions allow |
| unwritable parent | `report_output_unwritable`; no partial/temp artifact |
| producer failure plus writer failure | original producer failure remains nonzero and observable |

## Sampling Rate

- **After each task:** task-specific matched tests/fixture plus `git diff --check`.
- **After each Stage A plan:** all foundation packages owned so far plus contract mutation fixtures.
- **After a wave:** `bash scripts/verify-release-contract.sh` in its non-clean-fixture mode once that entrypoint exists.
- **After each identity-bearing commit:** Stage B clean `HEAD` gate.
- **Before Phase 31 verification:** full Stage A suite followed by Stage B against the exact committed `HEAD`.
- **Maximum normal feedback latency:** 300 seconds excluding the final OCI build; planner must split a faster package/fixture loop from image inspection.

## Wave 0 Dependencies

- [ ] `scripts/run-go-tests-matched.sh` and its self-fixtures.
- [ ] Strict contract/schema/canonical-digest positive and one-field negative fixtures.
- [ ] Temporary clean/dirty Git repository helpers for `buildinfo` tests.
- [ ] Injected runner/sentinel for excluded `OperationRef` zero-call assertions.
- [ ] Fault-injectable filesystem boundary for atomic report writer tests.
- [ ] Multi-binary/OCI/packaged-contract identity inspection fixture.
- [ ] `scripts/verify-release-contract.sh` self-contained foundation entrypoint.

These are implementation dependencies, not evidence that already exists. Their checkboxes remain open until the owning plans create and verify them.

## Manual-Only Verifications

None. Every Phase 31 foundation claim must have automated repository-local proof. Target credentials, live dependencies and external deployment access are neither required nor acceptable substitutes for this phase.

## Evidence And Claim Boundary

- Evidence classes allowed: E1 fixture/unit/static and E2 repository-local build/runtime inspection.
- Every completion report records environment, exact command, migration state when applicable, pass/fail, skipped checks and residual risk.
- Any skipped committed foundation check fails Phase 31 verification.
- Phase 31 completion means foundation contracts are implemented and verified; it does not mean runtime readiness, full `RELS-01`, `RELS-02`, target deployment parity or commercial release readiness.

## Planner Population Gate

After plan creation, the planner must update this document with:

- each real task ID, plan number and wave;
- one executable automated command per task;
- the mapped `V31-Fxx` behavior(s);
- whether every referenced script/test file exists or is a preceding Wave 0 dependency;
- sampling-continuity and file-overlap checks;
- a recalculated `nyquist_compliant` value.

Until that population step occurs, no task-level coverage or Nyquist completion is claimed.

## Validation Sign-Off

- [x] Foundation scope is separated from Phases 31.1, 31.2 and 39.
- [x] Two-stage clean-commit identity verification is explicit.
- [x] Zero-match targeted Go tests are prohibited.
- [x] Negative matrices cover contract, operations, identity, report and atomic output.
- [x] Plan/task/file-count and no-overlap constraints are recorded.
- [x] No nonexistent plan or task rows are represented as covered.
- [ ] Wave 0 dependencies exist and pass.
- [ ] Planner has populated real task mappings.
- [ ] Stage B has passed against the exact Phase 31 implementation commit.

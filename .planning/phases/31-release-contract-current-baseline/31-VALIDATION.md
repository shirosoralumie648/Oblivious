---
phase: 31
slug: release-contract-current-baseline
status: partial
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-15
updated: 2026-07-17
---

# Phase 31 - Foundation Validation Strategy

> Nyquist strategy for authored contract, trusted build identity, shared report protocol, excluded-profile operations and two-stage clean-commit proof.

## Scope And Current Planning State

This validation contract covers only the Phase 31 foundation contribution to `RELS-01`. It does not validate dynamic readiness/runtime enforcement (31.1), surface parity/aggregate gates (31.2), deployment-mode parity (38), or target/supply-chain proof (39).

Six `31-*-PLAN.md` files define 18 executable tasks across five waves. Planning-time sampling coverage is complete, but the 2026-07-17 implementation audit found that clean `HEAD` fails Stage A and the full Go suite. `nyquist_compliant: false` therefore reflects implementation evidence, not a lack of planned commands.

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
| Non-vacuous targeted Go tests | `scripts/run-go-tests-matched.sh` | Exists; self-fixtures pass |
| Contract/schema/digest fixtures | `scripts/verify-release-contract-fixtures.sh` | Exists; blocked by canonical fixture/schema drift |
| Foundation aggregate | `scripts/verify-release-contract.sh` | Exists; Stage A fails at the digest test group |
| Go package tests | `go test` through the matched-test helper | Exists; full suite has four clean-HEAD failures in two packages |
| Temporary Git identity fixtures | `src/server/internal/buildinfo/*_test.go` | Exists; one mutation test references an absent contract field |
| Atomic writer failure injection and post-rename rollback | `src/server/internal/surfacereport/writer_test.go` | Exists and passes |
| Entry-point inspection ordering | three `src/server/cmd/*/main_test.go` files | Exists and passes |
| Binary/OCI/package inspection | `scripts/build-release-image.sh` plus `scripts/verify-release-build-fixtures.sh` | Exists; dynamic fixture passes |

Existing `scripts/check.sh`, `scripts/verify-quality-gates.sh` and `scripts/verify-commercial-completion.sh` are not direct Phase 31 verification entrypoints. The unique aggregate wiring belongs to Phase 31.2.

## Nyquist Coverage Contract

| Validation ID | Foundation behavior | Fast proof | Negative proof | Completion evidence |
|---|---|---|---|---|
| V31-F01 | Authored contract passes JSON Schema and strict typed semantic validation. | focused loader tests | unknown/duplicate/trailing/enum/reference mutations | deterministic diagnostics and nonzero exit |
| V31-F02 | Contract canonical bytes and `sha256:` digest are stable. | golden digest tests | formatting/order vs semantic mutations | golden bytes and expected digest |
| V31-F03 | Explicit profile and `OperationRef` are safe and profile-bound. | resolver/dispatcher tests | missing/excluded/mismatch/traversal/symlink/shell cases | stable failure code and zero runner calls |
| V31-F04 | `BuildIdentityV1` derives from clean commit/tree plus contract digest only. | temporary clean Git fixture | dirty state and env/flag substitution | exact expected identity tuple |
| V31-F05 | All active binaries, OCI labels and packaged contract agree, and the foundation producer emits their trusted build report for an explicit resolver-confirmed committed profile. | dynamic entrypoint/build/inspect fixtures | one-component mismatch, missing contract, caller identity substitution and unknown/excluded profile | build `SurfaceReportV1` bound to clean source tuple plus committed `monolith` |
| V31-F06 | `SurfaceReportV1` and its typed details registry enforce nested ownership and trusted identity. | report/registry validation tests | flat fields, duplicate/unregistered details, caller identity and outcome inconsistency | schema-valid deterministic envelope plus foundation build registration |
| V31-F07 | Atomic report output preserves the prior destination on every returned failure, including post-rename parent sync. | writer success test | write/file-sync/close/rename/parent-sync/unwritable failures | rollback restores prior bytes/absence and leaves no staging/backup files |
| V31-F08 | Targeted Go tests cannot pass with zero matched tests. | known matching regex | zero-match and invalid regex | helper fails before executing the test run |
| V31-F09 | Foundation claims remain repository-local E1/E2. | report assertion | E3/E4/readiness/full-RELS-01 claim mutation | explicit evidence class and residual risks |

## Per-Task Verification Map

| Task ID | Plan | Wave | V31 behavior | Automated command | Artifact state before execution |
|---|---:|---:|---|---|---|
| 31-01-01 | 31-01 | 1 | F01, F09 | full package test plus exact `TestContractSchemaRejectsUnknownAndAuthoredIdentityFields` and `TestAuthoredContractV1ModelsRequiredSections` list assertions, `go mod verify`, and `git diff --check` | schema/model/test package missing; task creates production and named test files together |
| 31-01-02 | 31-01 | 1 | F01, F03, F09 | `bash -n scripts/release-profile-operation.sh && cd src/server && go test ./internal/releasecontract -count=1 && cd ../.. && git diff --check` | authored contract/operation script missing; task creates them |
| 31-01-03 | 31-01 | 1 | F01, F03, F09 | `cd src/server && go test ./internal/releasecontract -count=1 && cd ../.. && git diff --check` | explicit-root strict loader, committed-profile resolver, cwd-divergence tests missing; task creates them |
| 31-02-01 | 31-02 | 2 | F08 | `bash -n scripts/run-go-tests-matched.sh scripts/run-go-tests-matched-fixtures.sh && bash scripts/run-go-tests-matched-fixtures.sh && git diff --check` | matched helper/self-fixture missing; task creates both |
| 31-02-02 | 31-02 | 2 | F02 | `bash scripts/run-go-tests-matched.sh ./internal/releasecontract '^(TestCanonicalBytesGolden\|TestDigestEquivalentFormatting\|TestDigestSemanticMutation)$' && git diff --check` | digest code/vectors missing; helper exists from 31-02-01 |
| 31-02-03 | 31-02 | 2 | F03 | `bash scripts/run-go-tests-matched.sh ./internal/releasecontract '^(TestDispatchCommittedOperationPassesLiteralArgv\|TestDispatchRejectsBeforeRunner\|TestResolveOperationPathRejectsEscapes)$' && git diff --check` | dispatcher/tests missing; helper exists from 31-02-01 |
| 31-03-01 | 31-03 | 3 | F04 | `bash scripts/run-go-tests-matched.sh ./internal/buildinfo '^(TestBuildIdentityV1Validation\|TestParseLinkedIdentityRejectsInvalidValues)$' && git diff --check` | identity/linker package missing; task creates it |
| 31-03-02 | 31-03 | 3 | F04 | matched provider tests include `TestIdentityProvidersUseExplicitRepoRoot`, followed by `git diff --check` | providers/temp-Git/cwd-divergence tests missing; task creates them |
| 31-03-03 | 31-03 | 3 | F01, F02, F03, F04 | matched `internal/buildinfo` HandleInspection tests plus matched `cmd/release-contract` primitive tests, followed by `git diff --check` | shared inspector/tests and foundation CLI/tests missing; task creates them |
| 31-04-01 | 31-04 | 4 | F06 | `bash scripts/run-go-tests-matched.sh ./internal/surfacereport '^(TestSurfaceReportV1NestedValidation\|TestSurfaceReportV1RejectsFlatAndMisplacedFields\|TestDetailsRegistryRejectsDuplicateUnknownAndWrongType)$' && git diff --check` | envelope/registry/tests missing; task creates them |
| 31-04-02 | 31-04 | 4 | F05, F06, F09 | matched build-report tests dynamically cover trusted identity plus committed-profile resolution, raw unknown/excluded profile rejection, caller identity and component mismatch, followed by `git diff --check` | build registration/producer/profile binding missing; task creates them |
| 31-04-03 | 31-04 | 4 | F07 | matched writer tests include post-rename directory-sync rollback and primary-status preservation, followed by `git diff --check` | rollback-backed writer/failure injection missing; task creates them |
| 31-05-01 | 31-05 | 4 | F04, F05 | matched `TestIdentityInspectionPrecedesRuntimeSideEffects` in `cmd/server` and `cmd/migrate`, followed by `git diff --check` | server test missing; migrate test exists but lacks symbol; task owns both main/test pairs |
| 31-05-02 | 31-05 | 4 | F04, F05 | matched `TestIdentityInspectionPrecedesRuntimeSideEffects` in `cmd/grpc-smoke`, followed by `git diff --check` | grpc test exists but lacks symbol; task owns main/test pair |
| 31-05-03 | 31-05 | 4 | F04, F05 | shell syntax plus `bash scripts/verify-release-build-fixtures.sh` and `git diff --check` | wrapper/Docker/build fixture missing; task creates and dynamically exercises all three binaries, OCI labels, package and dirty source |
| 31-06-01 | 31-06 | 5 | F05, F06, F07, F09 | matched report CLI tests cover both trusted resolvers, untrusted profile rejection, schema/identity splice and producer failure, followed by `git diff --check` | report CLI commands missing; shared identity/profile/report APIs exist from 31-01/03/04 |
| 31-06-02 | 31-06 | 5 | F01-F09 | shell syntax plus owned contract/report fixtures and read-only Plan 31-05 build fixtures, followed by `git diff --check` | contract/report fixture missing; build fixture already created by 31-05-03 |
| 31-06-03 | 31-06 | 5 | F01-F09 | `bash -n scripts/verify-release-contract.sh && bash scripts/verify-release-contract.sh --stage-a && git diff --check` | foundation gate missing; task creates it; real Stage B remains post-commit |

Every task has executable automated feedback. No sequence of tasks, let alone three consecutive tasks, lacks a test or fixture command.

## Sampling Continuity And Wave/File Checks

- **Task continuity:** 18 of 18 tasks have an `<automated>` command; 31-01 creates and runs named tests in the same tasks before the helper exists, and all focused Go tasks after 31-02-01 use the list-before-run helper.
- **Plan continuity:** every plan ends with package/fixture feedback and `git diff --check`; Stage A aggregate becomes available in 31-06-03.
- **Wave continuity:** Wave 1 named/full-package contract tests -> Wave 2 matched helper/digest/operation -> Wave 3 identity/shared inspector/CLI -> Wave 4 report and packaging in parallel -> Wave 5 complete contract/report fixtures and aggregate.
- **Same-wave file overlap:** zero. The only parallel wave is Wave 4; 31-04 owns only `internal/surfacereport/*`, while 31-05 owns three entrypoint main/test pairs, `Dockerfile.server`, `scripts/build-release-image.sh`, and its dynamic build fixture.
- **Cross-wave ownership:** `internal/buildinfo/inspect.go`/`inspect_test.go` are created by 31-03 and consumed read-only by 31-05; `verify-release-build-fixtures.sh` is created by 31-05 and consumed read-only by 31-06.
- **Cross-wave overlap:** `cmd/release-contract/main.go` and `main_test.go` intentionally move from primitive CLI ownership in Wave 3 (31-03) to report commands in Wave 5 (31-06), with explicit dependency through 31-04/31-05.
- **Stage boundary:** Stage A is runnable during implementation; Stage B is a read-only, exact clean `HEAD` command after the Phase 31 implementation commit and before push.

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

1. Require explicit `--profile monolith`; resolve it from the checked-in contract through `FileProfileResolver` and reject omitted, unknown, conditional, or excluded values.
2. Require `git status --porcelain=v1 --untracked-files=all` to be empty; ignored `.tmp` outputs do not affect source identity.
3. Resolve `HEAD^{commit}` and `HEAD^{tree}` directly from Git.
4. Validate the checked-in contract with the explicit repo root and recompute its canonical digest.
5. Build server, migrate and grpc-smoke from the same identity input.
6. Build/inspect the runtime image without exposing `.git` to Docker.
7. Compare every binary identity, OCI label and packaged contract digest with the clean source tuple.
8. Emit a repository-local report containing resolver-confirmed profile, environment, commands, result, skips and residual risks.
9. Fail nonzero on any untrusted profile, dirty state, mismatch, missing component or skipped committed check.
10. Push only the exact commit/profile tuple that passed this gate.

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
| report profile omitted, unknown, conditional, or excluded | profile resolver failure; raw input never appears in releaseIdentity |
| Stage B profile is not explicit committed `monolith` | nonzero before report pass |

## Surface Report And Writer Negative Matrix

| Mutation/failure | Required result |
|---|---|
| legacy flat report fields | `surface_schema_invalid` |
| producer-supplied release identity | rejected construction/input |
| environment/mode stored in `drift` | schema rejection |
| arbitrary unregistered `evidence.details` | schema rejection |
| producer supplies raw deploymentProfile | rejected construction/input; profile comes from authored resolver |
| pass with non-empty errors/skips | outcome invariant rejection |
| write, file sync, close or rename failure | nonzero writer error; old destination preserved |
| post-rename parent-directory sync failure | rollback snapshot atomically restores and byte-verifies old destination before nonzero return |
| missing parent | parent created and atomic success when permissions allow |
| unwritable parent | `report_output_unwritable`; no partial/temp artifact |
| producer failure plus writer failure | original producer failure remains nonzero and observable |

## Sampling Rate

- **After each task:** task-specific matched tests/fixture plus `git diff --check`.
- **After each Stage A plan:** all foundation packages owned so far plus contract mutation fixtures.
- **After a wave:** `bash scripts/verify-release-contract.sh` in its non-clean-fixture mode once that entrypoint exists.
- **After each identity-bearing commit:** Stage B clean `HEAD` gate with explicit `--profile monolith`.
- **Before Phase 31 verification:** full Stage A suite followed by Stage B against the exact committed `HEAD`.
- **Maximum normal feedback latency:** 300 seconds excluding the final OCI build; planner must split a faster package/fixture loop from image inspection.

## Wave 0 Dependencies

- [ ] `scripts/run-go-tests-matched.sh` and self-fixtures - owned by 31-02-01 and precedes every focused selector.
- [ ] Strict contract/schema/canonical-digest positive and one-field negative fixtures - schema/package owned by 31-01; digest vectors by 31-02-02; aggregate mutations by 31-06-02.
- [ ] Temporary clean/dirty Git repository helpers for `buildinfo` tests - owned by 31-03-02.
- [ ] Injected runner/sentinel for excluded `OperationRef` zero-call assertions - owned by 31-02-03.
- [ ] Fault-injectable filesystem boundary for atomic report writer tests - owned by 31-04-03.
- [ ] Multi-binary/OCI/packaged-contract identity inspection fixture - wrapper, Docker path, and dynamic mutation fixture owned together by 31-05-03; 31-06 consumes it read-only.
- [ ] `scripts/verify-release-contract.sh` self-contained foundation entrypoint - owned by 31-06-03.

These are implementation dependencies, not evidence that already exists. Their checkboxes remain open until the owning plans create and verify them.

## Manual-Only Verifications

None. Every Phase 31 foundation claim must have automated repository-local proof. Target credentials, live dependencies and external deployment access are neither required nor acceptable substitutes for this phase.

## Evidence And Claim Boundary

- Evidence classes allowed: E1 fixture/unit/static and E2 repository-local build/runtime inspection.
- Every completion report records environment, exact command, migration state when applicable, pass/fail, skipped checks and residual risk.
- Any skipped committed foundation check fails Phase 31 verification.
- Phase 31 completion means foundation contracts are implemented and verified; it does not mean runtime readiness, full `RELS-01`, `RELS-02`, target deployment parity or commercial release readiness.

## Planner Population Gate

The planner population gate is complete: all real task IDs, plans, waves, commands, V31 mappings, missing-artifact owners, sampling continuity, and file-overlap results are recorded above. Nyquist compliance is a planning result only; implementation checkboxes and Stage B remain open until execution.

## Validation Audit 2026-07-17

Environment: detached clean worktree at `e64ead7c858202b8f48a05784871a4e58d1925eb`; no `TEST_DATABASE_URL`; no target/live evidence used.

| Validation ID | Status | Evidence |
|---|---|---|
| V31-F01 | COVERED | Strict loader/schema/semantic tests pass against the checked-in contract. |
| V31-F02 | PARTIAL | All three canonical digest tests fail because their fixtures contain timing fields absent from the committed schema/model. |
| V31-F03 | COVERED | Resolver, dispatcher, path, literal argv, and minimal environment tests pass. |
| V31-F04 | PARTIAL | Clean/dirty Git identity tests pass, but the embedded digest mutation test fails before applying its mutation. |
| V31-F05 | PARTIAL | Entry-point/report/build fixtures pass independently, but the aggregate Stage A gate is red and final-HEAD Stage B is therefore not eligible to close the phase. |
| V31-F06 | COVERED | Nested envelope, typed registry, identity-splice, and report CLI tests pass. |
| V31-F07 | COVERED | Atomic writer and rollback failure-injection tests pass, including post-rename parent sync rollback. |
| V31-F08 | COVERED | Match/list helper positive, zero-match, invalid-regex, missing-package, and failing-test fixtures pass. |
| V31-F09 | COVERED | Reports and scripts retain `repository-local`, explicit residual risks, and no target/live claim. |

Commands and results:

| Command | Result |
|---|---|
| `bash scripts/verify-release-contract.sh --stage-a` | FAIL: three digest tests reject `/profiles/0` against the committed schema. |
| `go test ./... -count=1` | FAIL: the same three digest tests plus `TestEmbeddedProviderRecomputesContractDigest`. |
| `go vet` on the affected packages | PASS. |
| `bash scripts/verify-release-build-fixtures.sh` | PASS. |
| Six `verify artifacts` plus six `verify key-links` commands | Files exist; eight pattern checks require metadata repair or manual interpretation. |

| Metric | Count |
|---|---:|
| Gaps found | 2 |
| Resolved | 0 |
| Escalated | 2 |

Gap 1 is the cross-phase timing-field ownership drift documented in `31-REVIEW.md`. Gap 2 is the resulting absence of an exact final-HEAD Stage B tuple. No missing test file should be generated: the existing tests already reproduce the defect, and implementation/fixture ownership must be reconciled first.

## Validation Sign-Off

- [x] Foundation scope is separated from Phases 31.1, 31.2 and 39.
- [x] Two-stage clean-commit identity verification is explicit.
- [x] Zero-match targeted Go tests are prohibited.
- [x] Negative matrices cover contract, operations, identity, report and atomic output.
- [x] Plan/task/file-count and no-overlap constraints are recorded.
- [x] Every represented plan/task row exists in `31-01-PLAN.md` through `31-06-PLAN.md`.
- [ ] Wave 0 dependencies exist and pass; digest and embedded-provider fixtures are currently red.
- [x] Planner has populated real task mappings and every V31-F01..F09 behavior has executable coverage.
- [ ] Stage B has passed against the exact Phase 31 implementation commit.

**Approval:** blocked 2026-07-17 pending clean-HEAD fixture/contract convergence and a fresh Stage A -> full regression -> Stage B sequence.

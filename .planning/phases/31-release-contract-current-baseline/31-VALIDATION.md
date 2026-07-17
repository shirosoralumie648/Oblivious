---
phase: 31
slug: release-contract-current-baseline
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-15
updated: 2026-07-17
---

# Phase 31 - Foundation Validation Strategy

## Scope

This audit covers the Phase 31 repository-local foundation contribution to `RELS-01`: authored contract, canonical digest, safe operations, trusted Git/build identity, shared surface report, atomic persistence, and Stage A/Stage B gates. Dynamic readiness, surface parity, target deployment, supply-chain proof, and commercial readiness remain later-phase work.

## Test Infrastructure

| Capability | Entrypoint | Status |
|---|---|---|
| Non-vacuous selected Go tests | `scripts/run-go-tests-matched.sh` | PASS |
| Contract/schema/digest/report fixtures | `scripts/verify-release-contract-fixtures.sh` | PASS |
| Multi-binary/OCI/package fixtures | `scripts/verify-release-build-fixtures.sh` | PASS |
| Stage A aggregate | `scripts/verify-release-contract.sh --stage-a` | PASS |
| Full Go regression | `cd src/server && go test ./... -count=1` | PASS |
| Release/docs aggregate | `bash scripts/check.sh docs` | PASS |
| Structural metadata | six artifact + six key-link queries | PASS, 42/42 |
| Clean-HEAD Stage B | `scripts/verify-release-contract.sh --clean-head --profile monolith` | PASS, zero skips |

## Requirement And Validation Map

| Validation ID | Status | Automated evidence |
|---|---|---|
| V31-F01 | COVERED | Strict loader, schema, semantic, duplicate/default, and typed timing tests |
| V31-F02 | COVERED | Canonical golden/equivalent/semantic-mutation tests |
| V31-F03 | COVERED | Explicit profile, operation path/argv/environment, zero-call tests |
| V31-F04 | COVERED | Clean/dirty Git and embedded digest recomputation tests |
| V31-F05 | COVERED | Entry-point inspection, Stage A, real Stage B binary/OCI/package proof |
| V31-F06 | COVERED | Nested report envelope, typed registry, identity-splice tests |
| V31-F07 | COVERED | Atomic write and rollback fault-injection tests |
| V31-F08 | COVERED | List-before-run positive/zero/invalid/failing selector fixtures |
| V31-F09 | COVERED | Repository-local evidence class, zero skips, explicit residual risks |

## Validation Audit 2026-07-17

| Metric | Count |
|---|---:|
| Gaps found | 2 |
| Resolved | 2 |
| Escalated | 0 |
| Automated validation families covered | 9/9 |

The prior partial audit at clean commit `e64ead7` found the timing-field split and missing exact-head Stage B. Commits `2b0f15a`, `1c937b8`, and `ad24612` closed those gaps; Stage B then passed at `ad2461213dabd80616b06a2efc43f08554219700` with zero skips.

## Manual-Only Verifications

None for Phase 31. Target credentials, live dependencies, deployment parity, and target evidence are out of scope and cannot substitute for this phase's automation.

## Sign-Off

- [x] Every V31-F01..F09 family has passing automated evidence.
- [x] Stage A and full Go regression pass from committed source.
- [x] Structural verification passes 42/42 without waiver.
- [x] Stage B passes at a clean commit with explicit `monolith` and zero skips.
- [x] `nyquist_compliant: true` is supported by current results.

**Approval:** Nyquist-compliant for the Phase 31 repository-local foundation. `RELS-01` remains pending.

---
phase: 31
slug: release-contract-current-baseline
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-15
---

# Phase 31 - Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing`, Python/shell negative fixtures, TypeScript compiler/Vitest |
| **Config file** | `src/server/go.mod`, `src/web/vite.config.ts`, existing `scripts/verify-*.sh` wrappers |
| **Quick run command** | `bash scripts/verify-release-contract-fixtures.sh` |
| **Full suite command** | `bash scripts/verify-release-contract.sh && bash scripts/verify-quality-gates.sh` |
| **Estimated runtime** | ~300 seconds after Wave 0 artifacts exist |

---

## Sampling Rate

- **After every task commit:** Run the task's narrow automated command from the map below plus `git diff --check`.
- **After every plan wave:** Run `bash scripts/verify-release-contract.sh` and the surface-specific gate changed in that wave.
- **Before `$gsd-verify-work`:** `bash scripts/verify-release-contract.sh`, `bash scripts/verify-quality-gates.sh`, and the relevant server/web tests must be green.
- **Max feedback latency:** 300 seconds for the normal repository-local loop; environment-backed migration replay may run separately but must record its environment and result.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 31-01-01 | 01 | 1 | RELS-01 | T-31-01 | Unknown keys/enums, invalid profile defaults, and missing reason codes fail closed. | negative fixture | `bash scripts/verify-release-contract-fixtures.sh` | ❌ W0 | ⬜ pending |
| 31-01-02 | 01 | 1 | RELS-01 | T-31-01 | The authored contract is deterministic, schema validated, monolith-only committed, and all other profiles excluded/disabled. | unit + contract | `(cd src/server && go test ./internal/releasecontract -run 'TestContract|TestProfile' -count=1)` | ❌ W0 | ⬜ pending |
| 31-01-03 | 01 | 1 | RELS-01 | T-31-03 | Contract digest and release identity are stable and cannot be replaced by dynamic readiness state. | CLI fixture | `bash scripts/verify-release-contract-fixtures.sh` | ❌ W0 | ⬜ pending |
| 31-02-01 | 02 | 2 | RELS-01 | T-31-02 | Missing/unknown profile and environment attempts to enable excluded capabilities fail before startup side effects. | Go unit | `(cd src/server && go test ./internal/releasecontract ./cmd/server -run 'Test.*(Profile|Capability|Startup)' -count=1)` | ❌ W0 | ⬜ pending |
| 31-02-02 | 02 | 2 | RELS-01 | T-31-02 | Excluded ingress is absent; disabled and blocked paths reject before Provider/tool/financial side effects. | HTTP/unit | `(cd src/server && go test ./internal/http -run 'Test.*(ReleaseContract|Capability)' -count=1)` | ❌ W0 | ⬜ pending |
| 31-02-03 | 02 | 2 | RELS-01 | T-31-04 | Admin/operator output joins contract and readiness while public output redacts secrets and internal addresses. | HTTP/unit | `(cd src/server && go test ./internal/http -run 'Test.*(ReleaseContract|Readiness)' -count=1)` | ❌ W0 | ⬜ pending |
| 31-03-01 | 03 | 2 | RELS-02 | T-31-05 | OpenAPI, derived route manifest, and runtime registry agree bidirectionally on method, security, and capability ID. | contract + Go | `bash scripts/verify-openapi-contract.sh && (cd src/server && go test ./internal/http -run TestRouteSurface -count=1)` | ✅ anchors exist | ⬜ pending |
| 31-03-02 | 03 | 2 | RELS-02 | T-31-05 | Every real feature API client is classified and matches its OpenAPI operation fingerprint. | Node/TS fixture | `bash scripts/verify-frontend-client-contract.sh` | ❌ W0 | ⬜ pending |
| 31-03-03 | 03 | 2 | RELS-01, RELS-02 | T-31-02 | Excluded capabilities are absent from default navigation/docs/client exports; conditional items require enabled readiness. | Vitest + contract | `pnpm --dir src/web test -- --runInBand && bash scripts/verify-frontend-client-contract.sh` | ❌ W0 | ⬜ pending |
| 31-04-01 | 04 | 2 | RELS-02 | T-31-05 | Canonical proto sources regenerate deterministically and every tracked generated consumer is managed. | generation fixture | `bash scripts/verify-protobuf-contract.sh` | ❌ W0 | ⬜ pending |
| 31-04-02 | 04 | 2 | RELS-02 | T-31-01 | Migration file inventory/content digest matches runtime checksum behavior without modifying historical SQL. | static + Go | `bash scripts/verify-migration-contract.sh && (cd src/server && go test ./internal/migrations ./cmd/migrate -count=1)` | ✅ anchors exist | ⬜ pending |
| 31-04-03 | 04 | 2 | RELS-02 | T-31-05 | Migration replay records the exact ledger state or fails with an explicit environment result; no committed check silently skips. | integration | `bash scripts/verify-migration-replay.sh` | ✅ anchor exists | ⬜ pending |
| 31-05-01 | 05 | 3 | RELS-01, RELS-02 | T-31-03 | Unified JSON report binds commit, contract digest, profile, every surface result, drift list, and skipped checks. | negative fixture | `bash scripts/verify-release-contract-fixtures.sh && bash scripts/verify-release-contract.sh` | ❌ W0 | ⬜ pending |
| 31-05-02 | 05 | 3 | RELS-01, RELS-02 | T-31-04 | Quality/commercial gates reject committed drift or skip, while docs retain repository-local E1/E2 claim language. | aggregate gate | `bash scripts/verify-quality-gates.sh && bash scripts/check.sh docs` | ✅ aggregators exist | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠ flaky*

---

## Threat References

| ID | Threat | Required verification |
|----|--------|-----------------------|
| T-31-01 | Authored contract or migration identity is tampered with. | Deterministic digest, strict schema, immutable migration checksum, negative fixture. |
| T-31-02 | Environment/profile override enables excluded functionality or bypasses side-effect guards. | Startup, route, worker, outbound, and financial fail-closed tests. |
| T-31-03 | A readiness report from another commit/contract/profile is spliced into the release view. | Join rejects mismatched release commit, contract digest, or profile. |
| T-31-04 | Operator/public reports leak secrets or internal endpoints, or overstate E3/E4 readiness. | Redaction tests and explicit evidence-class assertions. |
| T-31-05 | OpenAPI, runtime, protobuf, migration, or frontend consumers drift while a partial gate stays green. | Bidirectional source-to-consumer inventory plus committed-skip rejection. |

---

## Wave 0 Requirements

- [ ] `scripts/verify-release-contract-fixtures.sh` - table-driven positive and one-field negative contract/report mutations for RELS-01/02.
- [ ] `src/server/internal/releasecontract/contract_test.go` - strict schema/profile/status/reason/digest tests.
- [ ] `src/server/cmd/server/main_test.go` or a focused startup test - unknown/missing profile and pre-side-effect failure cases.
- [ ] `src/server/internal/http/routes_release_contract_test.go` - Admin join, redaction, 404/disabled/blocked behavior.
- [ ] `scripts/verify-frontend-client-contract.sh` and fixtures - all feature API consumer discovery and fingerprint drift.
- [ ] `scripts/verify-protobuf-contract.sh` and fixtures - canonical source/generated-output map and stale output rejection.
- [ ] `scripts/verify-release-contract.sh` - aggregate structured report and committed-skip blocker.

---

## Manual-Only Verifications

None. All Phase 31 behavior is repository-local and must be automated. Target deployment parity, live dependencies, E3 evidence, and E4 same-commit commercial release are explicitly out of scope and cannot be substituted with a manual Phase 31 sign-off.

---

## Evidence Boundary

- Phase 31 may produce E1 fixture/unit and E2 repository-runtime contract evidence only.
- The final report must record environment/tool versions, release commit, contract digest, selected profile, migration replay mode, pass/fail, skips, and residual risk.
- An unavailable migration replay environment must be reported explicitly and cannot be counted as a passing committed check.
- No Phase 31 result may claim target profile parity, live Provider/payment/storage readiness, or final commercial release.

---

## Validation Sign-Off

- [x] All provisional tasks have an automated command or an explicit Wave 0 dependency.
- [x] Sampling continuity: no three consecutive tasks lack automated verification.
- [x] Wave 0 covers every currently missing verifier/test reference.
- [x] No watch-mode flags are used.
- [x] Normal repository-local feedback latency target is under 300 seconds.
- [x] `nyquist_compliant: true` is set in frontmatter.
- [ ] Wave 0 artifacts are implemented and green.

**Approval:** strategy approved 2026-07-15; implementation pending

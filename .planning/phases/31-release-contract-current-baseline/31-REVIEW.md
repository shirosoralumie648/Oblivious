---
phase: 31-release-contract-current-baseline
status: issues_found
depth: deep
files_reviewed: 45
findings:
  critical: 1
  warning: 1
  info: 0
  total: 2
created: 2026-07-17
---

# Phase 31 Code Review

## Scope

Reviewed the 45 implementation, test, fixture, contract, Docker, and shell files listed by the six Phase 31 summaries. The review used the committed snapshot at `e64ead7c858202b8f48a05784871a4e58d1925eb` in a detached clean worktree so the four uncommitted Phase 31.1 timing changes could not mask Phase 31 behavior.

## Findings

### CR-31-01 - Critical - Clean HEAD cannot pass the Phase 31 test contract

The committed canonical digest fixtures already contain `refreshIntervalSeconds`, `maxAgeSeconds`, and `allowedFutureSkewSeconds`, but the committed Phase 31 schema, typed `DeploymentProfile`, and authored contract do not define those fields. The embedded-provider mutation test also tries to replace `"maxAgeSeconds": 120` in the checked-in contract even though that field is absent at clean HEAD.

Evidence:

- `src/server/internal/releasecontract/testdata/canonical-equivalent-a.json:24`, `:26`, and `:33` contain the three future timing fields.
- `config/release/contract.schema.json:115` closes profile objects with `additionalProperties: false`; its committed properties start at line 131 and do not admit the timing fields.
- `src/server/internal/buildinfo/provider_test.go:128` mutates a field absent from the committed contract and fails at line 130 before exercising digest mismatch behavior.
- `bash scripts/verify-release-contract.sh --stage-a` fails three digest tests with `contract_schema_invalid: field=/profiles/0: value=rejected`.
- `go test ./... -count=1` fails the same three digest tests plus `TestEmbeddedProviderRecomputesContractDigest`.

Impact: Phase 31 cannot satisfy V31-F02, the embedded-provider negative proof, the Stage A gate, the full Go regression gate, or the exact clean-HEAD completion contract. A dirty main worktree containing the four Phase 31.1 timing edits makes these tests pass, but that is not evidence for the committed Phase 31 tuple.

Required remediation: land one coherent ownership change in which the schema, typed model, authored contract, canonical vectors, golden digest, and provider mutation test all describe the same contract version. Do not split that revision across an uncommitted overlay and a supposedly complete earlier phase. Then rerun Stage A and the full Go suite from a clean detached HEAD.

### WR-31-02 - Warning - Structural artifact metadata does not match the implemented syntax

The six `verify artifacts` runs found all declared files, but two content patterns failed. The six `verify key-links` runs reported six additional false negatives or invalid regexes. Manual inspection confirmed the corresponding links exist, for example `releasecontract.Digest` in `provider.go`, `identities.Resolve` in `build.go`, and `build-release-image.sh` calls in both fixture and Stage B scripts.

Impact: the product wiring exists, but the plan metadata cannot provide machine-readable structural proof without manual interpretation. This weakens future drift detection and makes a green artifact count unreliable.

Required remediation: correct the over-escaped and overly literal `must_haves` patterns in the owning plan metadata during gap closure, then rerun all twelve artifact/key-link checks.

## Review Notes

- `go vet` passed for all affected Go packages.
- Shell syntax checks passed for all seven Phase 31 scripts; `shellcheck` is unavailable on this host.
- Race-enabled tests passed for `internal/surfacereport`, `cmd/release-contract`, `cmd/server`, `cmd/migrate`, and `cmd/grpc-smoke`; the buildinfo run was blocked by CR-31-01.
- The standalone dynamic release-build fixture passed from clean HEAD.
- No source fix was applied because the coherent timing implementation is currently present as protected, uncommitted Phase 31.1 work in four overlapping files.

## Verdict

`issues_found`. CR-31-01 blocks Phase 31 verification and push. WR-31-02 should be repaired with the same gap-closure pass so the canonical structural checks become green rather than manually waived.

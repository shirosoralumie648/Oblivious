# Phase 31: 发布合同与可信构建身份 - Research

**Researched:** 2026-07-16
**Scope:** Phase 31 foundation only; `RELS-01` contribution, repository-local E1/E2
**Confidence:** HIGH for ownership and contracts; MEDIUM for final plan slicing until Wave 0 helpers exist

## Foundation-Only Scope Declaration

本研究只回答如何实现以下五个 foundation contracts：

1. schema-validated `AuthoredContractV1`；
2. clean-source-derived `BuildIdentityV1`；
3. shared nested `SurfaceReportV1`；
4. profile-bound `OperationRef` 与 excluded-profile zero-side-effect rejection；
5. pre-commit fixture + post-commit clean `HEAD` 两段验证。

`ReadinessManager`、refresh/freshness、runtime side-effect guards、Admin/app projection、catalog enforcement，以及 readiness/deployment details registrations 和 producers 属于 Phase 31.1。HTTP/frontend/protobuf/migration parity、这些 surface 的 details registrations/producers、aggregate/gate wiring 和 operator docs 属于 Phase 31.2。signature、provenance、immutable image digest 与 E3/E4 proof 属于 Phase 39。

## Source Decisions

`31-CONTEXT.md` 中 `D-FND-01` 至 `D-FND-12` 是 planner 的锁定输入。批准设计为 `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md`。本研究不重新引入拆分前的 runtime/parity decisions，也不把资产存在解释为 profile commitment。

## Recommended Foundation Architecture

```text
config/release/contract.v1.json
          |
          v
strict schema + semantic loader ----> canonical bytes ----> contract digest
          |                                      |
          |                                      v
          |                         clean Git commit/tree
          |                                      |
          +--------------------------> BuildIdentityV1
                                                 |
                            trusted provider -----+----- operation dispatcher
                                                 |
                                                 +----- SurfaceReportV1 writer
                                                 |
                                                 +----- binary/OCI/package verifier
```

The authored contract is the only manually maintained authority. Identity and reports are derived outputs. No derived field may be copied back into the authored contract.

## AuthoredContractV1

### Required Semantic Sections

| Section | Foundation contract |
|---|---|
| `schemaVersion` | Exact supported value; unknown major/minor fails closed. |
| capability catalog | Stable domain and lifecycle-slice IDs, commitment, reason semantics, no duplicates. |
| reason-code catalog | Stable identifiers referenced by contract failures; unknown references fail. |
| `defaultProfile` | Exactly one authored metadata default; currently `monolith`. It is not runtime input. |
| profiles | Explicit commitment plus topology, entrypoints, dependencies and state stores. |
| capability overrides | Every referenced capability exists; excluded assets cannot promote commitment. |
| typed operations | Exactly one migrate/deploy/rollback `OperationRef` per profile. |
| catalog bindings | Stable references only; runtime resolution/enforcement is deferred to Phase 31.1. |
| surface references | Identifiers and canonical-source refs only; parity/fingerprints are deferred to Phase 31.2. |
| readiness requirements | Authored requirements only; no current observations, generation or timestamps. |

### Decode And Validation Order

1. Read bounded UTF-8 bytes and reject empty/non-JSON input.
2. Validate against the pinned JSON Schema implementation and `contract.schema.json`.
3. Strictly decode into Go structs with `json.Decoder.DisallowUnknownFields()`.
4. Require EOF after the first value so trailing JSON cannot be ignored.
5. Run semantic validation for ID uniqueness, enum values, default cardinality, reference closure, profile commitment rules and operation-path safety.
6. Canonicalize the validated typed value and compute `sha256:<lowercase hex>`.

JSON Schema and Go semantic validation are complementary. The schema owns structural constraints; Go owns cross-reference and repository-path invariants. A partial handwritten replacement for JSON Schema is not sufficient.

### Canonical Bytes

Use a single foundation implementation and golden vectors. Recommended rules:

- canonicalize the validated typed value, never the unvalidated raw file;
- UTF-8 compact JSON, sorted object keys, no insignificant whitespace, one defined trailing-newline policy;
- arrays preserve authored order only where order is semantic; set-like arrays are normalized or rejected unless already in canonical order;
- contract numbers are integral and bounded so Go/Python number formatting cannot diverge;
- reject duplicate JSON object keys before canonicalization;
- golden tests include non-ASCII strings, reordered keys, reordered set-like arrays and newline/whitespace variations.

`scripts/target_release_digests.py:canonical_json_bytes` is the nearest repository analog for sorted compact JSON, but the foundation must define its own versioned contract and golden digest. It must not inherit target-evidence self-normalization fields.

## OperationRef

```go
type OperationRef struct {
    ProfileID string   `json:"profileId"`
    Path      string   `json:"path"`
    Argv      []string `json:"argv"`
}
```

Validation and dispatch invariants:

- `ProfileID` must equal the owning profile ID.
- `Path` must be non-empty, slash-normalized, repo-relative and inside a fixed allowlist such as `scripts/`; absolute paths, `..` escape, symlink escape and NUL fail validation.
- `Argv` is passed directly to `exec.CommandContext(path, argv...)`; it is never joined into `sh -c`, `bash -c` or another command string.
- environment inheritance is explicit and minimal; release identity is not accepted from operation environment.
- dispatch loads the contract, resolves the explicit profile and checks commitment before looking up or starting the child.
- excluded profile dispatch returns a stable nonzero error such as `profile_excluded`; tests use a child-launch spy/sentinel and prove zero filesystem/process side effects.
- missing/unknown profile and mismatched `OperationRef.profileId` fail before child startup.

The dispatcher establishes a safe operation contract only. It does not claim that monolith migration/deploy/rollback succeeds in a target environment; operational parity remains Phase 38.

## BuildIdentityV1

```go
type BuildIdentityV1 struct {
    SchemaVersion  string `json:"schemaVersion"`
    ReleaseCommit string `json:"releaseCommit"`
    SourceTree     string `json:"sourceTree"`
    ContractDigest string `json:"contractDigest"`
    Dirty          bool   `json:"dirty"`
    EvidenceClass  string `json:"evidenceClass"`
}
```

Required value shape:

```json
{
  "schemaVersion": "build-identity/v1",
  "releaseCommit": "<40-hex>",
  "sourceTree": "<40-hex>",
  "contractDigest": "sha256:<hex>",
  "dirty": false,
  "evidenceClass": "repository-local"
}
```

### Derivation Boundary

- The trusted build wrapper obtains `git rev-parse HEAD^{commit}` and `git rev-parse HEAD^{tree}` from the repository and requires `git status --porcelain=v1 --untracked-files=all` to be empty. Ignored `.tmp` outputs remain outside this check.
- `GITHUB_SHA` is comparison-only. A CLI flag or environment variable cannot replace the Git-derived commit/tree.
- Because `.dockerignore` excludes `.git`, identity must be derived before Docker build. The wrapper passes a complete validated identity as build input; Dockerfile stages do not synthesize Git identity.
- Linker variables are untrusted until `buildinfo` validates field shapes and recomputes the packaged contract digest. The trusted provider returns an identity only after all comparisons pass.
- Server, migrate and grpc-smoke use the same linker values. The OCI inspection gate compares labels, each binary inspection output and the packaged contract digest to the clean repository identity.
- Direct ad-hoc Docker builds may produce non-release artifacts, but they cannot pass the identity-bearing foundation verifier without matching the clean repository source.

### Trusted Provider Interface

```go
type IdentityProvider interface {
    Resolve(ctx context.Context, contractPath string) (BuildIdentityV1, error)
}
```

Consumers receive a validated value, not raw linker strings. Phase 31 tests and CLI use this provider; Phase 31.1 later injects it into startup and authorization.

## SurfaceReportV1

```go
type SurfaceReportV1 struct {
    SchemaVersion   string          `json:"schemaVersion"`
    ReleaseIdentity ReleaseIdentity `json:"releaseIdentity"`
    SurfaceIdentity SurfaceIdentity `json:"surfaceIdentity"`
    Drift           Drift           `json:"drift"`
    Evidence        Evidence        `json:"evidence"`
    Outcome         Outcome         `json:"outcome"`
}
```

Top-level unknown fields fail. The shared foundation schema fixes these ownership boundaries:

- `releaseIdentity`: trusted build fields plus explicit deployment profile; never producer input.
- `surfaceIdentity`: surface name, canonical source, consumer, version and source/consumer digests.
- `drift`: only sorted `missing`, `extra`, `incompatible` arrays.
- `evidence`: repository-local class, environment, mode, checked-at time, tool versions and schema-allowlisted `details`.
- `outcome`: `pass | fail`, stable error codes and skipped checks.

Phase 31 validates the envelope and shared fields, provides the typed `evidence.details` registry API, and registers the foundation build-identity/build-inspection details used by its own build report producer. Each later phase registers only the schemas it owns when its producers arrive: readiness and deployment in Phase 31.1; HTTP/runtime, frontend, protobuf and migration in Phase 31.2. Foundation tests prove arbitrary, duplicate or unregistered detail keys cannot silently pass; committed-skip aggregation remains Phase 31.2.

### Atomic Writer Interface

```go
type ReportWriter interface {
    Write(ctx context.Context, path string, report SurfaceReportV1) error
}
```

Writer sequence:

1. validate schema, trusted identity and deterministic ordering;
2. create the output parent when absent;
3. create a temporary file in the same directory;
4. write complete bytes, flush, `fsync`, close, then atomic rename;
5. `fsync` the parent where supported;
6. remove temp files on every failure and preserve any prior valid destination.

When a producer fails, it may best-effort write a trusted failure report, but its original nonzero status remains authoritative. An unwritable report path adds `report_output_unwritable`; it must not turn the producer failure into success or leave partial output.

## Exact File Ownership

| Owner | Proposed files | Responsibility |
|---|---|---|
| Authored authority | `config/release/contract.v1.json`, `config/release/contract.schema.json` | Only authored commitment/profile/capability authority and structural schema. |
| Contract domain | `src/server/internal/releasecontract/contract.go`, `load.go`, `validate.go`, `digest.go`, `operation.go` | Typed model, strict load, semantic validation, canonical digest and safe operation dispatch. |
| Contract tests | `src/server/internal/releasecontract/contract_test.go`, `digest_test.go`, `operation_test.go`, `src/server/internal/releasecontract/testdata/` | Positive/negative schema, canonicalization, reference and zero-side-effect operation fixtures. |
| Schema dependency | `src/server/go.mod`, `src/server/go.sum` | One pinned maintained JSON Schema implementation if no existing dependency satisfies strict validation. |
| Build identity | `src/server/internal/buildinfo/identity.go`, `provider.go`, `linker.go` | Linker inputs, validated `BuildIdentityV1` and trusted provider. |
| Build identity tests | `src/server/internal/buildinfo/identity_test.go`, `provider_test.go` | Clean/dirty temp Git repos, substitution rejection and packaged-contract mismatch. |
| Shared report | `src/server/internal/surfacereport/report.go`, `validate.go`, `writer.go` | Nested V1 model, shared validation and atomic output. |
| Shared report tests | `src/server/internal/surfacereport/report_test.go`, `writer_test.go` | Identity injection, schema negatives, failure preservation and unwritable-output behavior. |
| Foundation CLI | `src/server/cmd/release-contract/main.go`, `main_test.go` | Validate/digest/identity/inspect/operation commands using internal packages. |
| Binary inspection | `src/server/cmd/server/main.go`, `src/server/cmd/migrate/main.go`, `src/server/cmd/grpc-smoke/main.go` | Minimal early identity-inspection path shared by all active binaries; no DB, migration, listener or network side effects. |
| Build packaging | `Dockerfile.server`, `scripts/build-release-image.sh` | Derive outside Docker, inject all binaries, package contract, set/inspect OCI labels. |
| Verification helpers | `scripts/run-go-tests-matched.sh`, `scripts/verify-release-contract.sh`, `scripts/verify-release-contract-fixtures.sh` | Nonzero-match Go test guard, foundation gate and mutation/build fixtures. |

The three entrypoints may add only a minimal early identity-inspection path required by the approved build contract. Normal startup ordering, DB-before-listener sequencing and continuous authorization are Phase 31.1 changes.

## Two-Stage Verification

### Stage A - Pre-Commit Development Proof

- Unit tests create disposable Git repositories, write the contract, commit known trees and derive known identities.
- Dirty tracked, staged, untracked and contract-only mutations each fail with stable diagnostics.
- Contract/report mutation fixtures change one field at a time.
- `scripts/run-go-tests-matched.sh` lists matching tests first and fails when a regex selects zero tests; only then may it execute `go test -run`.
- Build tests that do not require the current repository identity may run while the developer worktree is dirty.

### Stage B - Post-Commit Clean HEAD Proof

- Confirm `git status --porcelain=v1 --untracked-files=all` is empty.
- Resolve real `HEAD^{commit}` and `HEAD^{tree}` with no override path.
- Recompute canonical contract digest.
- Build/inspect all active binaries and OCI image from that identity.
- Compare binary identity, OCI labels and packaged contract digest to the clean source tuple.
- Record repository-local environment, commands, results, skips and residual risk. Any skip or mismatch is nonzero.
- Push only after this stage passes for the commit being pushed.

## Foundation Test Matrix

| Risk | Required automated proof |
|---|---|
| Unknown or ambiguous contract | Schema negatives, duplicate-key rejection, trailing JSON, semantic reference closure. |
| Digest drift | Golden canonical bytes across key/whitespace variations and semantic mutations. |
| Identity override/splice | clean/dirty repos, env/flag substitution attempts, commit/tree/digest mismatch. |
| Container identity fabrication | build without trusted wrapper cannot pass clean-HEAD comparison; all binary/OCI/package values are compared. |
| Unsafe operation execution | absolute/traversal/symlink paths, shell metacharacter argv, profile mismatch and excluded zero-launch proof. |
| Flat or producer-owned report identity | top-level schema negatives and producer input rejection. |
| Partial report corruption | write/fsync/rename failure injection preserves destination and removes temp files. |
| False-green targeted tests | zero-match regex is rejected before `go test`. |

## Anti-Patterns To Reject

- Authoring `releaseCommit` or `contractDigest` inside `contract.v1.json`.
- Hashing pretty-printed/raw JSON without a versioned canonicalization rule.
- Accepting release identity through runtime env, ordinary CLI flags or producer JSON.
- Deriving Git identity inside Docker even though `.git` is excluded.
- Flattening `SurfaceReportV1` or letting each producer invent fields.
- Replacing an existing report before the new file is fully synced and validated.
- Executing `OperationRef` through a shell string or validating traversal only with a prefix check.
- Treating excluded operation rejection as deploy/rollback success or profile parity.
- Pulling `ReadinessManager`, OpenAPI/frontend/protobuf/migration parity or aggregate gate ownership back into Phase 31.
- Calling repository-local identity proof E3/E4 or closing full `RELS-01`.

## Implementation Prerequisites And Residual Risks

There is no unresolved product decision blocking planning. The following foundation assets do not yet exist and must be created in Wave 0 or the first owning plan:

- strict schema/semantic validation and canonical digest packages;
- trusted Git/build identity provider and all-binary build injection;
- shared report validator/atomic writer;
- zero-match Go test helper and foundation fixtures;
- a trusted build wrapper that reconciles clean Git identity with Docker's `.git`-free context.

Residual risk after Phase 31 remains intentional: the identity/report contracts will exist, but runtime freshness, side-effect enforcement, surface parity, target deployment proof and supply-chain authenticity stay unproven until their routed phases complete.

## Sources

- `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md`
- `.planning/phases/31-release-contract-current-baseline/31-CONTEXT.md`
- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `Dockerfile.server`
- `.dockerignore`
- `src/server/cmd/server/main.go`
- `src/server/internal/migrations/migrations.go`
- `scripts/target_release_digests.py`
- `scripts/verify-target-release-evidence.sh`

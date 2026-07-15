# Phase 31: 发布合同与可信构建身份 - Pattern Map

**Mapped:** 2026-07-16
**Scope:** `RELS-01` foundation contribution only
**Excluded from this map:** Phase 31.1 runtime/readiness work, Phase 31.2 surface parity/aggregate work, Phase 39 supply-chain proof

## Foundation File Classification

| Target owner | Role | Existing analog | Match | Planning note |
|---|---|---|---|---|
| `config/release/contract.v1.json` | authored config | `docs/api/route-surface-manifest.json` | role only | New file is authority, not another derived route inventory. |
| `config/release/contract.schema.json` | strict schema | none | none | Use a real pinned JSON Schema validator plus Go semantic checks. |
| `src/server/internal/releasecontract/contract.go` | typed domain model | `src/server/internal/config/config.go` | structural | Keep enums and references explicit; no `map[string]any` authority. |
| `src/server/internal/releasecontract/load.go` | strict file loader | `src/server/internal/config/config.go` | partial | Add unknown-field and trailing-token rejection absent from current loader. |
| `src/server/internal/releasecontract/digest.go` | canonical bytes/digest | `src/server/internal/migrations/migrations.go` | strong for SHA-256 only | Reuse hash/error style, define JSON canonicalization separately. |
| `src/server/internal/releasecontract/operation.go` | safe operation resolver/dispatcher | `os/exec.CommandContext` call sites | partial | Validate profile and repo-relative allowlist before child startup; never use a shell string. |
| `src/server/internal/buildinfo/identity.go` | identity value and validation | target-evidence commit checks | role only | No runtime env/CLI override path. |
| `src/server/internal/buildinfo/provider.go` | trusted identity provider | constructor-injected service patterns | structural | Return only validated identity tied to packaged contract. |
| `src/server/internal/buildinfo/linker.go` | build-time variables | `Dockerfile.server` ldflags | gap | All active binaries share the same injected tuple. |
| `src/server/internal/surfacereport/report.go` | nested shared envelope | target evidence typed JSON | role only | Fix top-level ownership before producers are added. |
| `src/server/internal/surfacereport/registry.go` | typed details registry + foundation build registration | none | none | Later phases extend this API with owned schemas; arbitrary or duplicate registration fails. |
| `src/server/internal/surfacereport/build.go` | trusted build identity report adapter | target evidence build manifest | role only | Emit the foundation build report consumed by Phase 31.2; do not accept caller identity. |
| `src/server/internal/surfacereport/writer.go` | atomic output | none | none | Same-directory temp, sync, rename, cleanup, preserve prior destination. |
| `src/server/cmd/release-contract/` | foundation CLI | `src/server/cmd/grpc-smoke/` | strong CLI shape | Thin command over internal packages; stable JSON/errors/nonzero exits. |
| `src/server/cmd/server/main.go`, `src/server/cmd/migrate/main.go`, `src/server/cmd/grpc-smoke/main.go` | binary identity inspection | current entrypoints | direct integration | Add one shared early inspection path with zero DB/network side effects. |
| `scripts/build-release-image.sh` | clean Git derivation/build orchestration | `scripts/assemble-target-release-evidence.sh` | partial | Git is available outside Docker; no mismatch escape hatch. |
| `scripts/run-go-tests-matched.sh` | false-green prevention | none | none | Fail before execution when `go test -list` finds zero matches. |
| `scripts/verify-release-contract*.sh` | foundation verifier/fixtures | existing `verify-*-fixtures.sh` families | strong | Keep self-contained; aggregate ownership is Phase 31.2. |
| `Dockerfile.server` | binary/package/OCI identity | current multi-binary build | direct integration | Inject one validated tuple, package canonical contract, expose labels for inspection. |

## Pattern 1 - Strict JSON Decode And Semantic Closure

**Apply to:** `config/release/*`, `internal/releasecontract/load.go`, `validate.go`.

```go
decoder := json.NewDecoder(reader)
decoder.DisallowUnknownFields()

var contract Contract
if err := decoder.Decode(&contract); err != nil {
    return Contract{}, fmt.Errorf("decode release contract: %w", err)
}
if err := requireJSONEOF(decoder); err != nil {
    return Contract{}, err
}
if err := contract.Validate(); err != nil {
    return Contract{}, err
}
```

Required additions around this standard-library core:

- validate raw bytes against `contract.schema.json` before typed semantic use;
- reject duplicate object keys rather than allowing last-key-wins behavior;
- validate enum values, unique IDs, exactly one default, reference closure and profile commitment invariants;
- collect deterministic diagnostics where safe, but never continue with a partial contract;
- keep current observations, commit and digest fields out of the authored type.

Do not expand `internal/config.Config` with the full release contract. The contract has its own lifecycle, schema and digest authority.

## Pattern 2 - Canonical Digest From Validated Values

**Apply to:** `internal/releasecontract/digest.go` and golden testdata.

Nearest repository analog:

```go
sum := sha256.Sum256(statement)
return hex.EncodeToString(sum[:])
```

Foundation adaptation:

```go
canonical, err := CanonicalBytes(validatedContract)
if err != nil {
    return "", err
}
sum := sha256.Sum256(canonical)
return "sha256:" + hex.EncodeToString(sum[:]), nil
```

Guardrails:

- canonicalization is versioned and covered by golden vectors;
- formatting/key-order changes do not change digest, semantic changes do;
- set-like arrays have one normalized representation;
- callers cannot provide a digest to skip recomputation;
- the checked-in contract is validated against its canonical form during the foundation gate.

`scripts/target_release_digests.py` is a useful algorithmic analog for compact sorted JSON. Its target-evidence normalization behavior is not reusable here.

## Pattern 3 - Clean Git Identity Derived Outside Docker

**Apply to:** build wrapper, `internal/buildinfo/*`, `cmd/release-contract`, `Dockerfile.server`.

Derivation sequence:

```text
require empty `git status --porcelain=v1 --untracked-files=all`
  -> git rev-parse HEAD^{commit}
  -> git rev-parse HEAD^{tree}
  -> strict contract load + canonical digest
  -> construct/validate BuildIdentityV1
  -> inject the same tuple into every binary and OCI labels
  -> package contract
  -> inspect binary + labels + packaged digest against source tuple
```

Rules:

- `GITHUB_SHA` can only be compared with the Git result.
- Linker values are not trusted merely because they are compiled into a binary; provider validation includes shape and packaged-contract digest.
- `.git` remains outside the Docker context. Do not remove `.git` from `.dockerignore` to make derivation easier.
- A build arg is transport, not authority. Only the wrapper-generated tuple can satisfy the post-build comparison.
- server, migrate and grpc-smoke must not drift to separate build commands or identities.
- each entrypoint exposes the same minimal identity-inspection path before config, DB, migration, listener or network work; normal startup behavior stays unchanged.
- direct runtime callers never see setters for identity fields.

## Pattern 4 - Trusted Nested Report And Atomic Writer

**Apply to:** `internal/surfacereport/*`.

Construction pattern:

```go
identity, err := identityProvider.Resolve(ctx, contractPath)
if err != nil {
    return err
}
report := NewReport(identity, surfaceIdentity, drift, evidence, outcome)
return writer.Write(ctx, outputPath, report)
```

The producer supplies surface observations, never release identity. `NewReport` and `Write` both reject invalid nesting, unknown enums, unsorted/duplicate drift entries and unregistered evidence details.

Atomic output sequence:

```text
validate -> mkdir parent -> create temp in destination dir -> write all
         -> file sync -> close -> rename -> directory sync
```

On every error: close and remove temp, leave any existing destination untouched, return a stable nonzero error. A best-effort failure report cannot replace the original producer status.

## Pattern 5 - Profile-Safe Operation Dispatch

**Apply to:** `internal/releasecontract/operation.go` and CLI operation subcommand.

```go
profile, err := contract.RequireExplicitProfile(profileID)
if err != nil {
    return err
}
if profile.Commitment == CommitmentExcluded {
    return ErrProfileExcluded
}
ref, err := profile.Operation(kind)
if err != nil {
    return err
}
resolved, err := resolver.ResolveAllowlistedRepoPath(repoRoot, ref.Path)
if err != nil {
    return err
}
return runner.Run(ctx, resolved, ref.Argv)
```

Use an injected runner in tests so excluded, unknown and invalid-path cases assert zero `Run` calls. Path validation must account for clean/absolute paths and symlink resolution; string-prefix checks alone are insufficient.

## Pattern 6 - Verification That Cannot Pass Vacuously

**Apply to:** `scripts/run-go-tests-matched.sh`, fixture wrappers and plan verification commands.

```text
go test <package> -list <regex>
  -> parse actual matching test names
  -> fail if count == 0
  -> go test <package> -run <anchored-regex> -count=1
```

Additional shell rules:

- `set -euo pipefail`;
- derive `repo_root` from the script location;
- use `mktemp -d` plus cleanup trap for fixture repositories and image metadata;
- mutate one field per negative case and assert both nonzero exit and stable error code;
- do not route the foundation verifier through `verify-quality-gates.sh`; single aggregate ownership is added in Phase 31.2.

## Foundation Negative Fixture Families

| Family | Minimum mutations |
|---|---|
| Contract schema | unknown field, duplicate key/ID, trailing JSON, unknown enum, multiple defaults, broken reference. |
| Profile operations | missing explicit profile, excluded profile, profile/ref mismatch, absolute/traversal/symlink path, shell-like argv. |
| Canonical digest | whitespace/key-order stability, semantic mutation difference, authored self-identity field rejection. |
| Build identity | dirty tracked/staged/untracked, invalid commit/tree, env/flag substitution, contract digest mismatch. |
| Binary/OCI/package | one binary differs, label differs, packaged contract differs, contract absent. |
| Surface report | flat legacy field, caller identity, arbitrary details, identity mismatch, skipped/error outcome inconsistency. |
| Atomic writer | missing parent, unwritable parent, short write/sync/rename failure, prior destination preservation, temp cleanup. |
| Test selection | valid regex, invalid regex and zero-match regex. |

## No Exact Repository Analog

| Concern | Current gap | Required pattern |
|---|---|---|
| strict authored JSON Schema | no pinned validator found | add a maintained validator; do not hand-roll a schema subset |
| duplicate-key-safe canonical JSON | current target digest helper starts from normal `json.loads` | reject duplicate keys before canonicalization and publish golden vectors |
| clean build identity shared by all binaries/OCI | Dockerfile only has `-s -w` | external derivation plus post-build comparison |
| trusted identity provider | target evidence accepts caller fields in several paths | no public setter/override path |
| atomic JSON report writer | existing collectors use direct writes | same-directory temp, sync, rename and failure injection |
| zero-match targeted Go test guard | helper absent | list-before-run wrapper |

## Planner Guardrails

1. Produce **5-6 plans** for Phase 31 foundation, with **2-3 tasks per plan**.
2. Each plan may modify at most **9 files**; aim for 5-8. Split by ownership when a slice exceeds the limit.
3. Plans in the same wave must have no file-write overlap. Shared files such as `Dockerfile.server`, `go.mod` and verifier wrappers require explicit sequential ownership.
4. A sensible ownership sequence is: contract/schema -> loader/digest -> build identity/CLI -> shared report/writer -> operation dispatch -> packaging/two-stage gate. Planner may combine adjacent slices only within the file/task caps.
5. Create `scripts/run-go-tests-matched.sh` before any targeted `go test -run` is accepted as evidence.
6. Every task has a narrow automated command and `git diff --check`; every identity-bearing plan identifies whether its proof is Stage A or Stage B.
7. The final Phase 31 clean-identity gate is necessarily post-commit. Plans must state that boundary instead of weakening the dirty-worktree check.
8. Do not edit `31-DISCUSSION-LOG.md`, create runtime manager/routes, alter OpenAPI/frontend/protobuf/migrations, or wire the commercial/quality aggregate in this phase.
9. Do not close full `RELS-01`; report only foundation contribution and explicit residual risk.

## Metadata

**Analog search scope:** `config`, `src/server`, `scripts`, `Dockerfile.server`, `.dockerignore`, `docker-compose.yml`

**Strong analogs:** migration SHA-256/sorting, typed CLI JSON, fail-closed config, shell fixture harnesses

**New primitives required:** strict contract authority, trusted build identity, nested report writer, safe operation dispatcher, matched-test helper

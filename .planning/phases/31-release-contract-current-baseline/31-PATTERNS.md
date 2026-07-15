# Phase 31: 发布合同与当前基线 - Pattern Map

**Mapped:** 2026-07-15  
**Scope:** RELS-01, RELS-02; repository-local E1/E2 only  
**Authority note:** `.planning/intel/API-SURFACE.md` 未作为证据；当前 OpenAPI、runtime registry、migration ledger 和真实 feature clients 优先。

## File Classification

| New/Modified File | Role | Data Flow | Closest Existing Analog | Match |
|---|---|---|---|---|
| `config/release/contract.v1.json` | config/model | file-I/O, transform | `docs/api/route-surface-manifest.json` | role-match |
| `config/release/contract.schema.json` | schema/config | validation | none; repository has no JSON Schema authority | none |
| `src/server/internal/releasecontract/contract.go` | service/model | file-I/O, transform | `internal/migrations/migrations.go`, `internal/config/config.go` | role-match |
| `src/server/internal/releasecontract/readiness.go` | service/policy | transform | `internal/http/routes_release_evidence.go` readiness maps | partial |
| `src/server/internal/releasecontract/contract_test.go` | test | negative validation | `internal/config/config_test.go`, `cmd/migrate/main_test.go` | role-match |
| `src/server/cmd/server/main.go` | startup/config | request-response startup | same file: config -> DB -> migration -> server | exact |
| `src/server/cmd/release-contract/main.go` | CLI/utility | file-I/O, batch | `cmd/grpc-smoke/main.go` | exact role |
| `src/server/internal/http/routes_release_contract.go` | route/controller | request-response | `routes_release_evidence.go` | exact role |
| `docker-compose.yml`, `deploy/kubernetes/{server.yaml,configmap.yaml}` | deployment config | startup injection | current monolith service/config injection | exact |
| `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json` | canonical/derived contract | transform | current OpenAPI -> route manifest pair | exact |
| `scripts/verify_openapi_contract.py`, `internal/http/route_surface_test.go` | validator/test | bidirectional parity | current route parity functions/tests | exact |
| `scripts/generate_frontend_api_fingerprints.py` | generator | batch, transform | OpenAPI route extraction in `verify_openapi_contract.py` | partial |
| `scripts/verify_frontend_client_contract.mjs`, `scripts/verify-frontend-client-contract.sh` | validator/wrapper | AST, batch | wrapper pattern exists; no TS AST validator exists | partial |
| `src/web/src/generated/api-contract.ts` | generated model | transform | `src/web/src/types/api.ts` | role-match |
| 15 `src/web/src/features/**/*Api.ts` / `api.ts` modules | typed clients | request-response | `features/agents/agentsApi.ts` | exact |
| `api/proto/capability.proto`, six existing `api/proto/*.proto` | canonical contract | RPC | `api/proto/agent.proto` | role-match |
| `src/server/Makefile`, `scripts/verify-protobuf-contract.sh` | generator/gate | batch, file-I/O | `Makefile:proto-gen`, shell wrapper pattern | role-match |
| `scripts/verify-migration-{contract,replay}.sh`, `internal/migrations`, `cmd/migrate/main_test.go` | gate/store/test | file-I/O, ledger | existing checksum/ledger implementation | exact |
| `scripts/verify-release-contract.sh`, `scripts/verify_release_contract.py` | wrapper/validator | batch, structured report | OpenAPI wrapper + target evidence assembler | exact role |
| `scripts/verify-release-contract-fixtures.sh` | negative fixture test | mutation, batch | `verify-target-release-evidence-fixtures.sh` | exact |
| `scripts/{check,verify-quality-gates,verify-commercial-completion}.sh` | aggregate gates | sequential batch | their current explicit gate wiring | exact |
| `docs/release/rc-checklist.md`, `docs/architecture/current-system-contracts.md` | operator docs | human-readable projection | modify in place; preserve evidence-class language | exact |

The 15 real frontend consumers are: `admin/api.ts`, `agents/{agentsApi,memoriesApi,planStepsApi}.ts`, `auth/api.ts`, `chat/api.ts`, `console/api.ts`, `knowledge/api.ts`, `marketplace/api.ts`, `mcp/mcpServersApi.ts`, `notifications/notificationsApi.ts`, `publishingChannels/publishingChannelsApi.ts`, `scheduledTasks/scheduledTasksApi.ts`, `tasks/api.ts`, and `workflows/workflowsApi.ts`.

## Pattern Assignments

### 1. Authored Contract, Strict Go Loader, And Startup Policy

**Apply to:** `config/release/*`, `internal/releasecontract/*`, `cmd/server/main.go`, `cmd/release-contract/main.go`.

Deterministic file inventory and digest pattern (`internal/migrations/migrations.go:69-90,117-140`):

```go
migrationPaths, err := LoadFiles(migrationsDir)
statement, err := os.ReadFile(migrationPath)
checksum := Checksum(statement)
if existingChecksum != checksum {
    return result, fmt.Errorf("migration %s checksum mismatch", version)
}
sort.Strings(migrationPaths)
sum := sha256.Sum256(statement)
```

Fail-closed config errors (`internal/config/config.go:178-208,318-353`) return before construction, e.g. `return Config{}, fmt.Errorf("DATABASE_URL is required")`; production-disabled required workers/Relay are also rejected.

Startup placement (`cmd/server/main.go:19-39`):

```go
cfg, err := config.Load()
// fatal on error
database, err := db.Open(cfg.DatabaseURL)
// migrations.Apply(...)
server := serverhttp.NewServer(cfg, database)
```

Insert contract load + explicit profile selection after `config.Load()` and **before** `db.Open`, migrations, workers, or server construction. Pass an immutable policy dependency onward; do not re-read mutable environment flags at side-effect sites.

There is no existing `DisallowUnknownFields` loader. New loader must use `json.Decoder.DisallowUnknownFields()`, reject trailing JSON, validate enums/default-profile cardinality/reason codes/references, then compute SHA-256 over a defined canonical byte representation. Suggested public boundary: `Load(path string, selectedProfile string) (Contract, Policy, error)`.

CLI JSON pattern (`cmd/grpc-smoke/main.go:61-80`): build a typed report, `json.NewEncoder(output)`, `SetIndent("", "  ")`, return wrapped write errors, and exit nonzero from `main`.

Tests should copy the table-driven rejection shape from `internal/config/config_test.go:444-468`; checksum mutation proof is in `cmd/migrate/main_test.go:57-86`.

### 2. Read-Only Operator Join And Fail-Closed Ingress

**Apply to:** `routes_release_contract.go`, router options/registration, OpenAPI operation, readiness policy.

Closest route (`routes_release_evidence.go:394-428`):

```go
func registerReleaseEvidenceRoutes(mux *http.ServeMux, authMiddleware interface {
    requireAdmin(http.Handler) http.Handler
}, handler releaseEvidenceHandler) {
    mux.Handle(releaseEvidenceRoutePrefix, newReleaseEvidenceRouter(authMiddleware, handler))
}
// router is requireAdmin-wrapped, GET-only, uses stable writeError codes,
// returns unknown inventory keys as 404, and writes success via writeSuccess.
```

Copy the Admin guard, GET-only dispatch, `writeError` envelope, and `writeSuccess`. The response joins immutable commitment/profile data with dynamic availability; it must include contract digest/release commit/profile/capability ID and never rewrite the authored contract.

Policy behavior belongs before handlers/workers/provider or financial side effects: excluded routes are not registered (404); conditional-disabled returns `capability_disabled`; expected-but-unready returns 503 `capability_blocked`. Unknown contract/profile/readiness is treated as not enabled.

### 3. OpenAPI, Runtime, And Frontend Bidirectional Parity

**Apply to:** OpenAPI, route manifest, Python verifier, route test, fingerprint generator/verifier, generated TS contract, all 15 clients.

Manifest shape (`docs/api/route-surface-manifest.json:1-16`) already records `generatedFrom`, method, path, samplePath, security, csrf, operationId, and tags. Extend each operation/derived row with the stable capability ID; do not copy request/response schemas into the release contract.

Python parity core (`verify_openapi_contract.py:508-576`):

```python
openapi_routes[(method.upper(), path)] = {"security": kind, "operationId": op.get("operationId")}
manifest_routes[(method, path)] = route
missing_manifest = [key for key in openapi_routes if key not in manifest_routes]
stale_manifest = [key for key in manifest_routes if key not in openapi_routes]
# compare security/csrf/operationId/tags, then fail with all diagnostics
```

Add capability-ID presence/existence/equality to this same comparison. Preserve aggregated missing/stale/mismatch diagnostics rather than first-error-only behavior.

Runtime parity is already two-way (`internal/http/route_surface_test.go:2458-2474,2501-2544`): runtime registrations must be covered by the manifest, and every manifest route is dispatched with its declared auth/CSRF shape. Extend `routeSurfaceManifestRoute` (`2238-2243`) with `CapabilityID` and assert policy registration/exclusion here.

Frontend client pattern (`services/http/client.ts:21-66`, `features/agents/agentsApi.ts:103-118`):

```ts
export function createAgentsApi(client: HttpClient): AgentsApi {
  const path = '/api/v1/app/agents';
  return { listAgents: () => client.get<AgentSummary[]>(path) };
}
// shared client unwraps Envelope<T> and throws HttpError(status, message, { code, data })
```

The new TS-compiler AST gate must discover all 15 modules, resolve template literals/constants and `client.get/post/put/delete/request`, compare method/path/request/response fingerprints, and fail on every unclassified call. No in-repo AST analog exists; do not substitute regex or validate only `types/api.ts`.

### 4. Protobuf And Migration Identity

**Apply to:** `capability.proto`, six service protos, `Makefile`, protobuf verifier, migration gates/report.

Proto style (`api/proto/agent.proto:1-38`) uses `proto3`, versioned package/go_package, explicit service/RPC declarations. `Makefile:3-27` is the current six-source output map using `--go_out` and `--go-grpc_out` with `paths=source_relative`.

Define capability options centrally, import them into each canonical proto, and make the source-to-generated-output map explicit. The new verifier should generate into a temp tree and compare managed tracked outputs; unmanaged duplicate `*.pb.go` copies are incompatible, not silently ignored. There is no current stale-generation verifier to copy.

Migration runtime identity (`internal/migrations/migrations.go:74-98,104-140`) reads immutable SQL, compares SHA-256 with `schema_migrations`, fails on mismatch, and sorts filenames. Static naming/history exceptions remain in `verify-migration-contract.sh:1-75`. Reuse this checksum; do not edit applied SQL or create a second ledger/algorithm. Unified reporting may add file-set digest and ledger parity, while DB-unavailable replay must be reported as skipped and fail any committed check.

### 5. Shell Wrapper, Structured Validator, Negative Fixtures, And Gate Wiring

**Apply to:** `verify-release-contract*`, all three aggregate gates, JSON drift report.

Minimal wrapper (`verify-openapi-contract.sh:1-7`):

```bash
#!/usr/bin/env bash
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
exec "$python_bin" "$repo_root/scripts/verify_release_contract.py" "$@"
```

Structured JSON pattern (`assemble_target_release_evidence.py:1092-1112,1243-1298`) uses typed top-level identity, explicit `skippedChecks`, required `--output`, catches parse/validation errors to stderr, and writes deterministic indented JSON plus newline. Borrow the serialization/error contract only: Phase 31 report must label evidence as repository-local E1/E2, not target E3/E4.

Negative fixture harness (`verify-target-release-evidence-fixtures.sh:9-18,29-55,76-94`) uses `mktemp -d` + trap, a Python one-field mutation helper, expects nonzero, and asserts a stable diagnostic substring. Copy `make_invalid_case`/`expect_failure`; cover every required mutation from RESEARCH, including committed `skipped` and cross-commit/digest report splicing.

Gate wiring pattern:

- `check.sh:211-217,289-296` runs release assets and then explicit deployment/OpenAPI/migration gates; add the unified gate in this docs/all path without creating recursion.
- `verify-quality-gates.sh:2335-2345,2411-2417,2442-2448` self-checks that wrappers, implementations, and aggregate wiring contain required anchors; add equivalent assertions for contract/schema/report/fixtures and frontend/proto gates.
- `verify-commercial-completion.sh:198-205,388-409` wraps blockers with `run_step`; invoke the unified contract gate explicitly before expensive suites so drift fails early.

The validator report should always contain: release commit, contract digest/schema version, selected profile, surface, canonical source, consumer, digest/version, `missing`, `extra`, `incompatible`, result, skipped checks, and stable error classes. Sort arrays/keys deterministically; a committed skip makes the aggregate fail.

## Shared Patterns

- **Single authority:** authored release contract owns commitment/profile/capability IDs; OpenAPI, proto, migration, and clients retain their own canonical schemas and are referenced by digest/version.
- **Fail closed before side effects:** startup/profile errors precede DB/migrations; ingress, worker, outbound provider/tool, and financial paths consume the same policy.
- **Deterministic diagnostics:** stable IDs/reason codes, sorted inventories, SHA-256, aggregated missing/extra/incompatible arrays, nonzero exit on drift.
- **Admin-only full inventory:** public/default UI hides excluded items; operator route may show them but does not promote commitment.
- **Claim discipline:** candidate microservice assets remain excluded. `docker-compose.yml:111-163` is the live analog: monolith has no profile and gateway is opt-in `profiles: ["microservices"]`; add an explicit validated `monolith` selection to compose and Kubernetes ConfigMap, whose injection path is `server.yaml:41-45`.

## No Exact Analog

| File/Concern | Gap | Planner Direction |
|---|---|---|
| `contract.schema.json` | no repository JSON Schema validator | implement strict schema + semantic validation in Python and equivalent Go invariants |
| Go strict JSON loader | no `DisallowUnknownFields` usage found | use stdlib strict decode, trailing-token rejection, enum/reference validation |
| frontend compiler-AST parity | no compiler API analyzer exists | use installed TypeScript compiler; reject unclassified clients |
| protobuf stale-output gate/options | Makefile generates but does not temp-compare | add explicit source/output map and temp regeneration diff |

## Planner Guardrails

1. Keep plans aligned to the five research slices: authority, runtime/operator, HTTP/frontend, proto/migration, aggregate/docs.
2. Do not modify historical SQL, infer profile from asset existence, or let environment flags promote excluded capabilities.
3. Do not count fixture success, generated files, or local JSON reports as target/live readiness proof.
4. Every plan runs its narrow negative fixtures/tests plus `git diff --check`; final validation records environment, migration mode, skips, and residual risk.

## Metadata

**Analog search scope:** `config`, `src/server`, `src/web`, `api/proto`, `scripts`, `deploy`, `docs/api`, `docs/release`, `docs/architecture`  
**Strong analog families:** 5  
**Files/families classified:** 22  
**Pattern extraction date:** 2026-07-15

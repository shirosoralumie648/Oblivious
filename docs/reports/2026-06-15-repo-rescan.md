# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this scan starts from commit `ff37513 test(frontend): prove admin usage logs browser analytics`.
- Worktree status during the scan: dirty only because this slice adds `src/server/internal/http/workflow_secret_response_test.go` and updates the DB evidence runner, OpenAPI contract notes, release evidence pack, audit, matrix, and this rescan checkpoint.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan remains **84/100**. The new Workflow secret-response proof narrows Workflow, API contract, Security, and release evidence for persisted workflow definitions, versions, and execution snapshots, but it does not close target-environment workflow telemetry, at-rest encryption, target secret audits, deployment validation, or final no-skip release readiness.

## What Changed In This Rescan

- The no-skip `secret-response-safety` DB evidence profile now includes Workflow HTTP response safety.
- The new test drives the real Workflow router with PostgreSQL-backed stores and creates a published workflow containing top-level `webhook_secret`, top-level `webhookSecret`, nested node `secret`, and nested trigger `secret` values.
- Create, list, detail, update, version, execute, execution-list, execution-detail, and debug-snapshot responses are checked for raw-secret omission and redacted-marker behavior.
- Direct SQL checks prove raw secrets remain in `workflows.definition`, `workflow_versions.definition`, and `workflow_executions.workflow_snapshot` for runtime execution, and marker-only updates preserve stored raw values.
- This is repository-local PostgreSQL response-safety proof, not at-rest encryption or target-environment secret-audit proof.

## Repository Inventory

- First-party tracked file distribution after this slice is intended to be:
  - `src`: 973 files
  - `.planning`: 210 files
  - `docs`: 92 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape remains:
  - `src/server/internal`: 588 tracked files
  - `src/server/migrations`: 106 SQL migration files
  - largest active server domains remain `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape remains:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory after this slice:
  - Go test files: 227
  - Web component/API test files: 67
  - Web Playwright specs: 8 specs
  - Web E2E fixture files: 8 files
- Latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- Project-local `AGENTS.md`: none at the main repo root or under first-party source; dependency caches and nested `reference/*` repositories are excluded from this first-party scan.

## Completion Matrix Snapshot

Proven rows remain:

- API gateway and relay
- Knowledge base and RAG
- Multi-channel publishing
- Database schema and migrations

Partial rows remain:

- Workflow engine
- Agent system
- Billing and monetization
- Marketplace ecosystem
- Frontend shell and core pages
- Observability metrics, logs, alerts, recovery
- API contract
- Deployment and operations
- Security and tenant isolation
- Migration strategy and release readiness

This scan does not reclassify any Partial row to Proven. Workflow secret-response proof improves evidence for Workflow, API contract, Security, and release readiness, but broader runtime, target-environment, at-rest encryption, and final release proof remain open.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 12
git ls-files | awk '...inventory counters...'
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' -o -path './.git/*' \) -prune -o -name AGENTS.md -print
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**'
gofmt -w src/server/internal/http/workflow_secret_response_test.go
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh secret-response-safety
bash scripts/verify-openapi-contract.sh
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```

Result:

- The narrow Go test compiled; it skipped locally without `TEST_DATABASE_URL`, which is expected for the direct package command.
- `scripts/verify-commercial-db-evidence.sh secret-response-safety` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `bash scripts/verify-openapi-contract.sh` passed.
- `bash scripts/check.sh docs` passed.
- `git diff --check` passed.
- The first-party AGENTS scan found no main-root or first-party source `AGENTS.md`.
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain the known release-boundary items from the June 14 scan: disabled future Relay surfaces, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, and the service-template migration TODO.

## Notable Scan Findings

- The live worktree before documentation updates had only Workflow secret-response files dirty: one new Go test plus script/OpenAPI/evidence-pack changes.
- Existing DB-backed tenant-isolation evidence remains stronger than target-environment evidence; this slice adds DB-backed response-safety proof for another persisted Workflow surface.
- `src/server/internal/http/workflow_secret_response_test.go` complements the earlier Workflow SQL active-organization isolation profile by proving the same route family redacts persisted secret-like fields in API responses while preserving runtime storage.
- The recommended security queue should now treat Observability alert providers, Publishing channel configs, Admin Relay channel API keys, and Workflow definitions/versions/execution snapshots as covered local PostgreSQL secret-response paths.

## Recommended Next Slices

1. Continue DB-backed security depth for remaining tenant-isolation and response-safety surfaces not covered by the current no-skip profiles.
2. Expand browser/E2E proof to the next high-value commercial journey not already covered by the current local Playwright paths.
3. Rerun the strict commercial verifier on target infrastructure with deploy and backup/restore enabled before renewing any final readiness claim.
4. Extend Observability/recovery proof from repository-owned panic recovery into target-environment OOM/crash restart, scale, and failover evidence.
5. Keep Deployment and release readiness open until Kubernetes, backup/restore, migration replay, and provider/payment rails have target-environment proof.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.

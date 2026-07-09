# Workflow Debug Retention Prune API

Date: 2026-07-05

## Scope

Adds a tenant-scoped API for pruning durable Workflow debug trace entries and variable snapshots before an explicit cutoff timestamp. This closes the repository-local retention API gap for SQL-backed Workflow debug data and adds no-skip disposable PostgreSQL evidence without claiming target-runtime commercial completion.

## Runtime Claim

- `POST /api/v1/workflows/debug-retention/prune` is registered under authenticated Workflow routes and requires CSRF through the existing session middleware.
- The handler accepts `{"before":"<RFC3339>"}`, rejects invalid timestamps before calling the service, and returns `traceEntriesDeleted` plus `variableSnapshotsDeleted`.
- `workflowServiceAdapter.PruneExecutionDebugData` scopes pruning to `session.OrganizationID`.
- `workflow.Service.PruneExecutionDebugData` validates organization and cutoff, requires a retention-capable store, and delegates to `SQLStore.PruneExecutionDebugData`.
- `SQLStore.PruneExecutionDebugData` deletes only rows matching `organization_id` and `created_at < before` from `workflow_debug_trace_entries` and `workflow_debug_variable_snapshots` in one transaction.
- `TestWorkflowSQLStorePrunesDebugRetentionAfterServiceRebuild` rebuilds `workflow.Service` around `NewSQLStore(database)` and proves PostgreSQL pruning deletes only the active tenant's expired debug trace/snapshot rows while preserving newer rows and other-tenant rows.

## Oblivious Files Changed

```text
src/server/internal/workflow/types.go
src/server/internal/workflow/store.go
src/server/internal/workflow/service.go
src/server/internal/workflow/service_test.go
src/server/internal/workflow/store_test.go
src/server/internal/http/workflow_handler.go
src/server/internal/http/workflow_handler_test.go
src/server/internal/http/routes_workflow.go
src/server/internal/http/routes_workflow_test.go
src/server/internal/http/route_surface_test.go
docs/api/openapi.yaml
docs/api/route-surface-manifest.json
scripts/verify_openapi_contract.py
scripts/verify-commercial-db-evidence.sh
scripts/verify-commercial-db-evidence-profiles.sh
docs/release/fusion-spec-evidence-pack.md
docs/release/commercial-completion-audit.md
docs/audit/current-implementation-depth.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/vertical-slice-gap-report.md
docs/audit/implementation-roadmap.md
```

## Contract Changes

- New endpoint: `POST /api/v1/workflows/debug-retention/prune`.
- Security: `cookieAuth` plus `csrfHeader`.
- Request schema: `WorkflowExecutionDebugRetentionPruneRequest`.
- Response schema: `WorkflowExecutionDebugRetentionPruneResultEnvelope`.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestRegisterWorkflowRoutesDispatchesDebugRetentionPrune|TestRegisterWorkflowRoutesRejectsInvalidDebugRetentionPruneCutoff|TestRouteSurfaceWorkflowManagementMutationsRejectCookieWithoutCSRFWithoutDatabase|TestRouteSurfaceWorkflowManagementMutationsDispatchWithCSRFWithoutDatabase' -count=1 -v
result: pass

command: python scripts/verify_openapi_contract.py
result: pass

command: bash scripts/verify-commercial-db-evidence-profiles.sh
result: pass

command: bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation
result: pass; disposable pgvector PostgreSQL run reported skipped tests: none
```

## Failure Evidence

- RED: `TestRegisterWorkflowRoutesDispatchesDebugRetentionPrune` initially failed with `404 route not found`, proving the route did not exist before implementation.
- RED: `bash scripts/verify-commercial-db-evidence-profiles.sh` initially failed with `workflow-sql-isolation must include TestWorkflowSQLStorePrunesDebugRetentionAfterServiceRebuild`, proving the no-skip DB profile did not yet require the retention-prune regression.
- Negative path: `TestRegisterWorkflowRoutesRejectsInvalidDebugRetentionPruneCutoff` verifies an invalid `before` timestamp returns `400 invalid_request` and does not invoke the prune service.
- CSRF path: route-surface tests verify the prune mutation rejects a valid cookie without CSRF with `403`.
- DB path: `bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation` runs `TestWorkflowSQLStorePrunesDebugRetentionAfterServiceRebuild` against disposable PostgreSQL and reports no skipped tests.

## Unsupported / Deferred Surfaces

- This is repository-local runtime evidence plus disposable PostgreSQL no-skip evidence. It does not prove target-environment retention pruning, operator retention scheduling, or deployed restart replay.
- Trigger listener replay/idempotency and full retry/failure replay depth remain incomplete.

## Known Residual Risk

Commercial Workflow completion still requires target DB-backed proof that replay and prune behavior work after restart against the deployed schema, deployed gRPC smoke, target workflow telemetry, trigger replay/idempotency, and operator evidence for retention policy execution.

# Workflow Debugger Legacy Tracer Removal Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: workflow debug state must not depend on the old process-local in-memory tracer helper. Debug snapshots and retention need durable SQL-backed behavior.

## Runtime Contract

- `src/server/internal/workflow/debug/tracer.go` has been removed.
- `git grep` finds no remaining `workflow/debug`, `debug.Tracer`, or `NewTracer` references in `src/server`.
- `src/server/internal/workflow/store.go` persists `workflow_debug_trace_entries` and `workflow_debug_variable_snapshots`.
- `src/server/internal/workflow/service.go` builds debug snapshots from durable trace entries and latest durable variable snapshots when the store supports them.
- `src/server/internal/http/routes_workflow.go` exposes `GET /api/v1/workflows/{id}/executions/{executionID}/debug-snapshot`.
- `src/server/internal/http/routes_workflow.go` exposes `POST /api/v1/workflows/debug-retention/prune`.

## Verification

Commands:

```powershell
git grep -n "workflow/debug\|debug\.Tracer\|NewTracer" -- src/server/internal/workflow src/server/internal/http src/server/cmd src/server/pkg
```

Result: no matches.

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/workflow ./internal/http -run 'Test(WorkflowSQLStoreReplaysExecutionStateAfterServiceRebuild|WorkflowSQLStorePrunesDebugRetentionAfterServiceRebuild|ServiceBuildExecutionDebugSnapshotUsesDurableTraceAndVariableSnapshot|RegisterWorkflowRoutesDispatchesExecutionDebugSnapshot|RegisterWorkflowRoutesDispatchesDebugRetentionPrune|RegisterWorkflowRoutesRejectsInvalidDebugRetentionPruneCutoff)' -count=1 -v
```

Result: service/http durable snapshot and route tests passed; SQL restart replay and retention tests were skipped because `TEST_DATABASE_URL` is not configured in the local environment.

## Remaining Commercial Blockers

This closes the local legacy-tracer and durable-debugger contract. Commercial proof still requires the same debug snapshot and retention workflow to be exercised against the target Postgres deployment during release evidence collection.

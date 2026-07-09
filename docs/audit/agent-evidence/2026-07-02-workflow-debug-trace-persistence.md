# Workflow Debug Trace Persistence - 2026-07-02

## Scope

This slice adds a durable SQL-backed storage path for Workflow debug traces and variable snapshots:

- Added migration `0091_workflow_debug_trace.sql` with `workflow_debug_trace_entries` and `workflow_debug_variable_snapshots`.
- Registered the new tables under the Workflow service boundary in `migrations/microservices/table-ownership.json`.
- Added `SQLStore` append/list helpers for debug trace entries and latest variable snapshots.
- Made SQL node execution creation write trace/snapshot rows in the same transaction as the node row.
- Updated `BuildExecutionDebugSnapshot` to prefer durable trace/snapshot rows when they exist, while retaining node-execution-derived fallback behavior.
- Durable trace/snapshot scans now open protected Workflow secret values through the same secretbox path used by node execution scans before HTTP redaction.
- Added memory-store implementations and service tests proving durable rows replace transient derived node trace/output/performance/log snapshots.

## Files

```text
src/server/internal/workflow/store.go
src/server/internal/workflow/service.go
src/server/internal/workflow/service_test.go
src/server/internal/workflow/store_test.go
src/server/internal/http/server_test.go
src/server/migrations/0091_workflow_debug_trace.sql
src/server/migrations/microservices/table-ownership.json
docs/audit/current-implementation-depth.md
docs/audit/oblivious-gap-matrix.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/implementation-roadmap.md
docs/audit/vertical-slice-gap-report.md
docs/audit/reference-capability-map.md
docs/audit/product-roadmap-v2-from-reference.md
```

## Verification

```text
command: gofmt -w src/server/internal/workflow/store.go src/server/internal/workflow/service.go src/server/internal/workflow/service_test.go src/server/internal/workflow/store_test.go
result: passed

command: go test ./internal/workflow -run "TestServiceBuildExecutionDebugSnapshot|TestWorkflow(DebugTrace|ExecutionEvents)|TestWorkflowStorePersistsDefinitionsAndExecutions" -count=1 -v
result: passed; DB-backed TestWorkflowStorePersistsDefinitionsAndExecutions skipped locally because TEST_DATABASE_URL is not set
```

Passing tests included:

```text
TestServiceBuildExecutionDebugSnapshotDerivesTraceVariablesOutputsPerformanceAndLogs
TestServiceBuildExecutionDebugSnapshotIncludesStatusTransitionEvents
TestServiceBuildExecutionDebugSnapshotUsesDurableTraceAndVariableSnapshot
TestWorkflowExecutionEventsMigrationDeclaresTransitionAudit
TestWorkflowDebugTraceMigrationDeclaresDurableTraceTables
```

## Remaining Gap

This does not yet complete the Workflow commercial debugger. SQL node execution writes durable trace/snapshot rows, but the standalone `debug.Tracer` helper still keeps process-memory maps, and standalone `executor.StateMachine` transition history still needs a durable replay sink. Final commercial Workflow readiness also requires retention/replay APIs and restart/replay evidence from a DB-backed target runtime.

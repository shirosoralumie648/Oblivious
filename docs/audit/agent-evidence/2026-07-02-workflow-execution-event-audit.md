# Workflow Execution Event Audit - 2026-07-02

## Scope

This slice makes Workflow execution status history durable enough for commercial debugging and restart survival:

- Added `workflow_execution_events` with append-only `created` and `status_changed` events.
- Backfilled a deterministic `created` event for existing `workflow_executions` in migration `0087_workflow_execution_events.sql`.
- Changed `SQLStore.CreateExecution` to write a `created` event in the same transaction as the execution row.
- Changed `SQLStore.UpdateExecutionStatus` to lock the execution row, update status, and append a `status_changed` event in one transaction.
- Added `ListExecutionEvents` and returned durable events from `BuildExecutionDebugSnapshot`.
- Updated workflow table ownership metadata so the event table belongs to the Workflow service boundary.

## Files Changed

```text
src/server/internal/workflow/types.go
src/server/internal/workflow/store.go
src/server/internal/workflow/service.go
src/server/internal/workflow/service_test.go
src/server/internal/workflow/store_test.go
src/server/internal/http/schedule_wiring_test.go
src/server/internal/http/workflow_service_test.go
src/server/migrations/0087_workflow_execution_events.sql
src/server/migrations/microservices/table-ownership.json
docs/audit/current-implementation-depth.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/vertical-slice-gap-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/implementation-roadmap.md
docs/audit/reference-capability-map.md
docs/audit/agent-evidence/2026-07-02-workflow-execution-event-audit.md
```

## Verification

```text
command: go test ./internal/workflow -run "Test(ServiceBuildExecutionDebugSnapshot|WorkflowExecutionEventsMigration|WorkflowStorePersistsDefinitionsAndExecutions)" -count=1 -v
result: passed; DB-backed TestWorkflowStorePersistsDefinitionsAndExecutions skipped locally because TEST_DATABASE_URL is not set

command: go test ./internal/http -run "Test(RegisterWorkflowRoutesDispatchesExecutionDebugSnapshot|Workflow|Schedule)" -count=1
result: passed
```

Covered repository-local tests:

```text
TestServiceBuildExecutionDebugSnapshotDerivesTraceVariablesOutputsPerformanceAndLogs
TestServiceBuildExecutionDebugSnapshotIncludesStatusTransitionEvents
TestWorkflowExecutionEventsMigrationDeclaresTransitionAudit
```

## Residual Risk

This closes durable service-level execution status audit for created and status-changed transitions. A later slice added SQL-backed debug trace entries and variable snapshots written alongside node executions, but this event slice still does not persist every standalone `executor.StateMachine` transition or replay/idempotency decision. Final Workflow commercial readiness still needs full state-machine event replay, retention/replay APIs, retry/idempotency semantics, and target-runtime execution evidence.

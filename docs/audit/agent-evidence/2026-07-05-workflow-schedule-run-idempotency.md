# Workflow Schedule Run Idempotency

Date: 2026-07-05

## Runtime Claim

Scheduled Workflow triggers now use the persisted `scheduledTaskRunId` as a durable idempotency key in the Workflow SQL execution store. Replaying the same scheduled run after a service rebuild returns the existing Workflow execution instead of creating a second execution, and the PostgreSQL unique index also handles direct duplicate execution inserts by returning the existing schedule-run execution.

## Reference Inputs

```text
docs/audit/current-implementation-depth.md - scheduled trigger replay/idempotency remained incomplete.
docs/release/fusion-spec-evidence-pack.md - Workflow SQL current delta required no-skip profile evidence.
src/server/internal/schedule/worker.go - scheduled task workers include scheduledTaskRunId in Workflow trigger payloads.
```

## Oblivious Files Changed

```text
src/server/internal/workflow/service.go
src/server/internal/workflow/store.go
src/server/internal/workflow/store_test.go
src/server/migrations/0099_workflow_schedule_run_idempotency.sql
scripts/verify-commercial-db-evidence.sh
scripts/verify-commercial-db-evidence-profiles.sh
docs/audit/agent-evidence/2026-07-05-workflow-schedule-run-idempotency.md
docs/audit/current-implementation-depth.md
docs/audit/implementation-roadmap.md
docs/release/fusion-spec-evidence-pack.md
docs/release/commercial-completion-audit.md
```

## Contract Changes

- Workflow `StartExecution` now checks for an existing schedule-trigger execution when `triggerPayload.scheduledTaskRunId` is present.
- SQL store exposes `FindExecutionByScheduleRunID` for durable lookup after service reconstruction.
- New migration: `src/server/migrations/0099_workflow_schedule_run_idempotency.sql`.
- PostgreSQL unique index: `idx_workflow_executions_schedule_run_idempotency` on `(organization_id, workflow_id, scheduledTaskRunId)` for schedule-trigger executions.
- `workflow-sql-isolation` commercial DB evidence profile now includes `TestWorkflowSQLStoreRejectsDuplicateScheduleRunAfterServiceRebuild` and `TestWorkflowSQLStoreReturnsExistingScheduleRunOnUniqueConflict`.

## Verification Commands

```text
command: disposable pgvector PostgreSQL + go test ./internal/workflow -run '^TestWorkflowSQLStoreRejectsDuplicateScheduleRunAfterServiceRebuild$' -count=1 -v
result: RED first; duplicate scheduled run created a second execution after service rebuild.

command: disposable pgvector PostgreSQL + go test ./internal/workflow -run '^TestWorkflowSQLStoreReturnsExistingScheduleRunOnUniqueConflict$' -count=1 -v
result: RED first; duplicate CreateExecution returned pq unique-constraint error instead of the existing execution.

command: bash scripts/verify-commercial-db-evidence-profiles.sh
result: pass

command: GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation
result: pass; disposable pgvector PostgreSQL run reported skipped tests: none and ran TestWorkflowSQLStoreRejectsDuplicateScheduleRunAfterServiceRebuild plus TestWorkflowSQLStoreReturnsExistingScheduleRunOnUniqueConflict.
```

## Runtime Evidence IDs

```text
organization_id: org_workflow_1
scheduled_task_id: sched_1
scheduled_task_run_id: schedrun_1 and schedrun_conflict
```

## Failure Evidence

- RED: `TestWorkflowSQLStoreRejectsDuplicateScheduleRunAfterServiceRebuild` initially failed because the rebuilt service created a second execution for the same `scheduledTaskRunId`.
- RED: `TestWorkflowSQLStoreReturnsExistingScheduleRunOnUniqueConflict` initially failed with `pq: duplicate key value violates unique constraint "idx_workflow_executions_schedule_run_idempotency"`, proving the SQL uniqueness guard needed to return the existing execution.
- GREEN: the no-skip `workflow-sql-isolation` profile now proves both replay and unique-conflict paths against disposable PostgreSQL.

## Unsupported / Deferred Surfaces

- This is repository-local PostgreSQL evidence, not target scheduler telemetry.
- Full retry/failure replay depth remains incomplete.
- Deployed Workflow gRPC smoke, target workflow telemetry, and final target release proof remain external.

## Known Residual Risk

Final commercial readiness still requires target-environment workflow telemetry, deployed Workflow gRPC smoke, richer retry/failure replay proof, and a no-skip strict release run with target evidence enabled.

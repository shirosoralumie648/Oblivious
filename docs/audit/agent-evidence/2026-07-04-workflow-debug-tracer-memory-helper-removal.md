# Workflow Debug Tracer Memory Helper Removal

Date: 2026-07-04

## Scope

This slice removes the unused `internal/workflow/debug` in-memory tracer helper so it cannot be mistaken for durable workflow debug/replay capability. The product runtime already uses SQL-backed workflow execution events, debug trace entries, variable snapshots, and the state replay API.

## Runtime Changes

- Deleted `src/server/internal/workflow/debug/tracer.go`.
- Verified no production code imports `internal/workflow/debug`.
- Verified the remaining workflow package list no longer includes `oblivious/server/internal/workflow/debug`.

## Evidence

Unused package scan:

```bash
rg -n "internal/workflow/debug|workflow/debug|debug\\." src/server -g'*.go'
```

Result: no matches.

Package list after deletion:

```bash
cd src/server && go list ./internal/workflow/...
```

Result:

```text
oblivious/server/internal/workflow
oblivious/server/internal/workflow/executor
oblivious/server/internal/workflow/failure
oblivious/server/internal/workflow/sandbox
oblivious/server/internal/workflow/trigger
oblivious/server/internal/workflow/version
```

Focused workflow replay command:

```bash
cd src/server && go test ./internal/workflow ./internal/http -run 'TestWorkflowSQLStoreReplaysExecutionStateAfterServiceRebuild|TestServiceBuildExecutionStateReplayReturnsDurableTransitionsWithoutDebugSnapshot|TestServiceBuildExecutionDebugSnapshotReplaysDurableStateTransitions|TestRegisterWorkflowRoutesDispatchesExecutionStateReplay|TestRouteSurfaceWorkflowReadRoutesRequireSessionWithoutDatabase' -count=1 -timeout 300s -v
```

Result:

```text
PASS
ok  	oblivious/server/internal/workflow	0.015s
PASS
ok  	oblivious/server/internal/http	0.033s
```

`TestWorkflowSQLStoreReplaysExecutionStateAfterServiceRebuild` skipped locally because `TEST_DATABASE_URL` is required for DB-backed workflow tests.

## Remaining Boundary

This closes the stale in-memory helper noise. It does not replace target-runtime DB replay evidence; release claims still need `TEST_DATABASE_URL` or target Postgres-backed workflow replay proof.

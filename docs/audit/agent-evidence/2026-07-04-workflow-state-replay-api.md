# Workflow State Replay API

Date: 2026-07-04

## Scope

Adds a focused Workflow execution state replay API on top of the durable execution event ledger. This narrows the prior Workflow replay gap by exposing replay data without requiring clients to fetch the full debug snapshot payload.

## Findings

- `GET /api/v1/workflows/{workflowId}/executions/{executionId}/state-replay` now returns the durable `WorkflowStateReplay` envelope.
- The route is session-protected and does not require CSRF, matching other Workflow read routes.
- `workflow.Service.BuildExecutionStateReplay` reads the tenant-scoped execution plus durable execution events and derives the replay through the same state-machine replay builder used by debug snapshots.
- OpenAPI and the route-surface manifest now document `getWorkflowExecutionStateReplay`, with contract verifier coverage for the response envelope and required replay schemas.

## Verification

```text
command: cd src/server && go test ./internal/workflow -run 'TestServiceBuildExecution(StateReplayReturnsDurableTransitionsWithoutDebugSnapshot|DebugSnapshotReplaysDurableStateTransitions)' -count=1 -v
result: pass

command: cd src/server && go test ./internal/http -run 'TestRegisterWorkflowRoutesDispatchesExecutionStateReplay|TestRouteSurfaceWorkflowReadRoutesRequireSessionWithoutDatabase' -count=1 -v
result: pass
```

## Remaining Boundary

- This is a repository-local replay API slice, not a full Workflow replay completion claim.
- Final Workflow commercial readiness still requires target DB-backed restart replay proof, retention policy/API depth, trigger replay/idempotency hardening, and debugger consolidation.

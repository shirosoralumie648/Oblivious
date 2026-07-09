# Workflow StateMachine Durable Transition Sink

Date: 2026-07-04

## Runtime Claim

Workflow `PauseExecution`, `ResumeExecution`, and `CancelExecution` now transition through `executor.StateMachine` instead of relying only on hand-written service status checks. The StateMachine can record transitions through a `TransitionSink`; service lifecycle transitions use a sink that calls `Store.UpdateExecutionStatus`, so `workflow_execution_events` remains the durable status history.

This is not a full workflow replay completion claim.

## Reference Inputs

```text
reference/coze-studio/backend/types/errno/workflow.go:173 - workflow cancel/timeout/snapshot/debug error vocabulary.
reference/dify/api/services/workflow_event_snapshot_service.py:79 - workflow event snapshot/replay reference pattern.
reference/FastGPT/packages/service/core/workflow/dispatch/index.ts:159 - workflow dispatch state/retry reference pattern.
```

## Oblivious Files Changed

```text
src/server/internal/workflow/executor/state_machine.go
src/server/internal/workflow/executor/state_machine_test.go
src/server/internal/workflow/service.go
src/server/internal/workflow/service_test.go
docs/audit/stub-hardcoded-todo-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/current-implementation-depth.md
docs/audit/vertical-slice-gap-report.md
docs/audit/reference-capability-map.md
docs/audit/agent-evidence/2026-07-04-workflow-state-machine-durable-transition-sink.md
```

## Contract Changes

- `executor.StateMachine` now accepts a `TransitionSink`.
- `TransitionWithContext` writes to the sink before mutating in-memory status/history.
- `PauseExecution`, `ResumeExecution`, and `CancelExecution` now use `transitionExecutionStatus`, which constructs a StateMachine from the current execution status and records the transition through a store-backed sink.
- Existing `workflow_execution_events` schema remains unchanged.

## Runtime Evidence

- `src/server/internal/workflow/executor/state_machine.go:41-164` defines the sink contract and guarantees sink failure prevents memory-only state mutation.
- `src/server/internal/workflow/service.go:1414-1441` routes pause/resume through the StateMachine transition path.
- `src/server/internal/workflow/service.go:1639-1641` routes cancel through the StateMachine transition path.
- `src/server/internal/workflow/service.go:2030-2085` maps StateMachine transitions to `Store.UpdateExecutionStatus`, preserving metrics and queue promotion behavior.
- `src/server/internal/workflow/service_test.go:2162-2200` verifies created/pause/resume/cancel events are persisted in order.

## Verification Commands

```text
command: cd src/server && go test ./internal/workflow/executor -run 'TestStateMachineRecordsTransitionToSink|TestStateMachineKeepsStateWhenSinkFails' -count=1
result: passed, `ok oblivious/server/internal/workflow/executor 0.002s`

command: cd src/server && go test ./internal/workflow -run 'TestServiceLifecycleTransitionsUseDurableStateMachineEvents|TestServiceExecutionLifecycleTransitions|TestServiceRejectsInvalidLifecycleTransitions|TestServiceBuildExecutionDebugSnapshotIncludesStatusTransitionEvents' -count=1
result: passed, `ok oblivious/server/internal/workflow 0.014s`
```

## Failure Evidence

- `TestStateMachineKeepsStateWhenSinkFails` verifies a sink failure returns the sink error and leaves `CurrentStatus` plus `History` unchanged, preventing memory-only transitions from diverging from the durable event sink.
- `TestServiceRejectsInvalidLifecycleTransitions` verifies invalid service lifecycle transitions still return `ErrInvalidTransition`.

## Unsupported / Deferred Surfaces

- Full workflow replay/retention APIs are still missing.
- Trigger listener replay and idempotency remain shallow.
- The standalone `debug.Tracer` helper still has memory maps and has not been retired or backed directly by SQL trace/snapshot tables.
- Target DB-backed restart replay evidence has not been captured in this slice.

## Known Residual Risk

This closes the state-machine/sink wiring for service lifecycle transitions, but commercial workflow readiness still requires target-runtime restart replay proof, retention APIs, trigger idempotency, node registry maturity, and full debugger consolidation.

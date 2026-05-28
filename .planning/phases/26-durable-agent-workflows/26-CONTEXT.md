# Phase 26 Context: Durable Agent Workflows

## Milestone

v08 Product Completeness.

## Why This Phase Exists

Phase 25 closed `PROD-01` by making default built-in MCP tools real or disabled. `PROD-02` is now the next customer-facing product-completeness gap: Agent tool workflows still depend on an in-memory execution loop and message persistence only.

Current Agent behavior:

- `src/server/internal/agent/runner.go` creates user, assistant, and tool messages.
- Tool calls requested by the model are stored inside assistant messages as `tool_calls`.
- Tool results are stored as `role=tool` messages linked by `tool_call_id`.
- `RunResult` exposes `ToolCalls` and `UsedMemory`, but those are transient return fields.
- There is no `agent_runs` state table, no `agent_tool_runs` table, no approval state, no retry state, no durable terminal error state, and no status API for current execution state.
- `buildChatMessages` can inject memory context, but the run does not persist whether memory search executed or how many results were injected.

For a commercial Agent product, message history is not enough. Operators and users need to inspect whether an Agent run is pending approval, running, failed, retryable, completed, or blocked; which tool calls executed; what failed; what was approved; and whether memory was injected.

## Requirement

- **PROD-02:** Agent workflows are durable and observable, with persisted tool runs, human approval points where needed, memory injection, execution state, and failure/retry evidence rather than placeholder tool output.

## Current Evidence And Gaps

Existing evidence:

- `agent_messages` persists conversational turns and tool result content.
- `Runner.RunWithTools` executes function-calling loops and persists tool result messages.
- `ToolExecutor` now blocks disabled default built-ins after Phase 25.
- `Agent.Config.EnableMemory` and `MemorySearcher` support memory injection.
- Agent, conversation, and message rows are tenant-scoped by `organization_id`.

Gaps:

- No durable run record exists for a user request.
- No durable tool-run record exists for each tool call.
- Approval is not modeled. Tools cannot require approval and runs cannot pause before executing a sensitive tool.
- Retry is not modeled. Failed tool calls are just error text in a tool message.
- Memory injection is not recorded as run evidence.
- There is no status/list API for run/tool-run state.
- DB-backed tests do not prove run/tool-run tenant isolation because those tables do not exist.

## Design Direction

Phase 26 should add durable state without changing the Relay-first invariant:

- Add migration `0031_agent_workflow_runs.sql` with `agent_runs` and `agent_tool_runs`.
- Extend Agent tool configuration with `requiresApproval`.
- Add store methods for creating, updating, listing, approving, rejecting, and retrying runs/tool-runs.
- Keep `agent_messages` as the conversation transcript, but treat `agent_runs` and `agent_tool_runs` as execution-state truth.
- Update `Runner.RunWithTools` to create one run at the start, update it through running/completed/failed/pending_approval states, and create one tool-run per model tool call.
- When approval is required, persist a pending tool-run and pause the Agent run without executing the tool.
- When a tool fails, persist failed state and error text before returning.
- Track memory evidence on the run: `memory_enabled`, `memory_searched`, and `memory_result_count`.
- Add service/API inspection endpoints for runs and tool runs so frontend/admin work in later phases has a real status surface.

## Expected Code Areas

- `src/server/migrations/0031_agent_workflow_runs.sql`
- `src/server/internal/agent/store.go`
- `src/server/internal/agent/runner.go`
- `src/server/internal/agent/service.go`
- `src/server/internal/agent/service_test.go`
- `src/server/internal/agent/store_test.go`
- `src/server/internal/http/agent_handler.go`
- `src/server/internal/http/router.go`
- `src/server/internal/http/agent_handler_test.go` if a new focused handler test is needed
- `docs/API.md`
- `docs/release/commercial-gates.md`
- `scripts/verify-quality-gates.sh`

## Verification Design

Phase 26 must prove durable behavior:

- Unit tests prove run state transitions for completed, failed, pending approval, approved, rejected, and retried tool runs.
- Runner tests prove a tool-call loop creates run and tool-run records before writing final messages.
- Tests prove approval-required tools are not executed until approved.
- Tests prove failed tool calls persist error status and retry metadata.
- Memory tests prove memory search evidence is stored.
- DB-backed SQL store tests prove tenant-scoped access to runs and tool runs.
- Handler tests prove status/approval/retry endpoints require authenticated tenant sessions.

## Closeout Boundary

Phase 26 may close only `PROD-02`.

It must not claim:

- Knowledge RAG/source citation or product-copy alignment (`PROD-03`).
- Full Chat/Admin/Marketplace UX hardening (`PROD-04`).
- Public onboarding, pricing, and operator guide completion (`PROD-05`).
- End-to-end commercial journeys or final commercial completion audit (`PROD-06`, `AUDIT-01`).

---

*Phase: 26-durable-agent-workflows*
*Context gathered: 2026-05-28 from current repository evidence*

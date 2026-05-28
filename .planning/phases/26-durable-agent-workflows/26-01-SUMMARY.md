# Phase 26 Summary — Durable Agent Workflows

## Result

Phase 26 implements durable Agent workflow evidence for `PROD-02`.

Agent tool-capable requests now create organization-scoped `agent_runs` records. Tool calls create organization-scoped `agent_tool_runs` records before execution or approval wait. Runs persist request ID, status, memory evidence, iteration/tool counts, final message IDs, terminal errors, and timestamps. Tool runs persist tool call IDs, tool name/type, arguments, approval state, acting approver/rejector, attempt count, result/error state, and timestamps.

## Delivered

- Added `src/server/migrations/0031_agent_workflow_runs.sql` with `agent_runs` and `agent_tool_runs`.
- Added Agent store models and SQL methods for run/tool-run create, list, get, and update.
- Added `requiresApproval` tool configuration.
- Updated `RunWithTools` to persist durable successful, failed, pending approval, and memory-evidence state.
- Added authenticated tenant-scoped APIs:
  - `GET /api/v1/app/agents/conversations/:conversationId/runs`
  - `GET /api/v1/app/agents/runs/:runId`
  - `POST /api/v1/app/agents/tool-runs/:toolRunId/approve`
  - `POST /api/v1/app/agents/tool-runs/:toolRunId/reject`
  - `POST /api/v1/app/agents/tool-runs/:toolRunId/retry`
- Updated API docs and commercial gate docs for durable Agent workflow behavior.
- Extended quality gates to require the Phase 26 schema, tests, docs, and evidence artifacts.

## Verification

Focused DB-backed Agent/HTTP tests passed:

```bash
cd src/server && TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable' OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/agent ./internal/http -run 'AgentRun|ToolRun|Approval|Retry|MemoryEvidence|Tenant|Durable' -count=1
```

Docs and diff hygiene are recorded in `26-VERIFICATION.md`.

## Boundary

Phase 26 closes only `PROD-02`. Phase 27 Knowledge Product Promise Alignment is next. v08 Product Completeness and the final commercial completion audit remain open.

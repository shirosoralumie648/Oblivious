# Agent Custom Python Sandbox Execution Context Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: production custom Python sandbox executions need durable run/tool/request context so target logs and artifact bodies can be joined back to Agent runs and tool calls.

## Runtime Contract

- `src/server/internal/agent/store.go` extends `ToolCall` with optional `runId`, `toolRunId`, and `requestId` context.
- `src/server/internal/agent/executor.go` passes agent ID, run ID, tool run ID, tool call ID, tool name, request ID, org/user, source, inputs, and timeout into `CustomPythonSandboxRequest`.
- `src/server/internal/agent/runner.go` fills run/tool/request context for structured ReAct tool execution after the durable `agent_tool_runs` row is created.
- `src/server/internal/agent/service.go` fills run/request context for plan-step tool execution and run/tool/request context for approved persisted tool-run execution.
- `src/server/cmd/agent/main.go` forwards the Agent sandbox execution context to the Workflow `CodeRunner`.
- `src/server/internal/workflow/sandbox/sandbox.go` exposes the non-secret execution context to Docker as `OBLIVIOUS_EXECUTION_CONTEXT` and returns the same context in `WorkflowCodeResult.Raw["executionContext"]`.
- The Docker sandbox now classifies parent context cancellation separately from timeout, so operator-initiated or upstream-cancelled runs return `sandbox: execution cancelled` instead of an ambiguous container execution failure.
- `WorkflowCodeResult.Raw["evidence"]` now includes non-secret execution context, start/finish timestamps, timeout, code/input sizes, captured stdout/stderr sizes, truncation flags, log line count, and log-retention limits.

## Verification

Command:

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/agent ./cmd/agent ./internal/workflow/sandbox -run 'Test(ToolExecutorUsesCustomPythonSandboxRunnerInProduction|AgentCustomPythonSandboxRunnerMapsWorkflowResult|ExecutionContextPassedAsJSONEnvAndRawEvidence|InputsPassedAsJSONEnv)' -count=1 -v
```

Result: passed.

Additional command:

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/workflow/sandbox -run 'Test(TimeoutKillsExecution|CancellationStopsExecution|ExecutionContextPassedAsJSONEnvAndRawEvidence|OutputByteCapTruncates)' -count=1 -v
```

Result: passed.

Covered checks include:

- Production Agent custom Python requests carry agent/run/tool/request context to the sandbox runner.
- The standalone Agent service adapter forwards that context into the Workflow code runner.
- The Docker sandbox command receives `OBLIVIOUS_EXECUTION_CONTEXT` with agent/run/tool/request identifiers.
- The Workflow sandbox result returns the same execution context in `Raw` for evidence artifact collection.
- Existing `OBLIVIOUS_INPUTS` behavior remains intact.
- Timeout still kills long-running container work.
- Parent cancellation now stops container work and returns a stable cancellation error.
- Raw sandbox evidence carries retention/size/truncation metadata needed by later artifact collectors.

## Remaining Commercial Blockers

This improves auditability and local cancellation semantics but does not prove commercial custom-code readiness. Remaining work still includes target Docker/container proof, request-log/trace propagation into deployed collectors, persisted artifact and log retention, target secret/audit evidence, and operator runbooks.

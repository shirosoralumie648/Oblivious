# Agent Custom Python Production Sandbox Runner

Date: 2026-07-04

## Runtime Claim

Production custom Python remains fail-closed by default. When an operator explicitly enables the existing Workflow Docker sandbox with `WORKFLOW_SANDBOX_ENABLED=true`, the standalone Agent runtime now injects that Docker-backed runner into Agent custom Python execution instead of executing host Python.

This is a runtime integration slice, not a full commercial-readiness claim.

## Reference Inputs

```text
reference/open-webui/backend/open_webui/utils/tools.py:430 - tool schema conversion and runtime tool registry depth.
reference/LibreChat/api/server/routes/agents/tools.js:13 - agent tool route surface.
reference/LibreChat/api/server/controllers/agents/openai.js:253 - structured agent tool loop depth.
reference/NextChat/app/utils/chat.ts:175 - client-side tool loop pattern.
```

## Oblivious Files Changed

```text
src/server/internal/agent/executor.go
src/server/internal/agent/executor_test.go
src/server/internal/agent/service.go
src/server/cmd/agent/main.go
src/server/cmd/agent/main_test.go
docs/audit/stub-hardcoded-todo-report.md
docs/audit/oblivious-gap-matrix.md
docs/audit/current-implementation-depth.md
docs/audit/vertical-slice-gap-report.md
docs/audit/reference-capability-map.md
docs/audit/agent-evidence/2026-07-04-agent-custom-python-production-sandbox-runner.md
```

## Contract Changes

- `internal/agent` now exposes `CustomPythonSandboxRunner`, `CustomPythonSandboxRequest`, and `CustomPythonSandboxResult`.
- `ToolExecutor` and `Service` can receive a sandbox runner without importing `internal/workflow`, avoiding the existing agent/workflow import cycle.
- Standalone Agent runtime builds an `agentCustomPythonSandboxRunner` only when `WorkflowSandboxEnabled` is true.
- The runner reuses existing `WORKFLOW_SANDBOX_*` configuration for allowed languages, memory, CPU, default timeout, and max timeout.

## Runtime Evidence

- `src/server/internal/agent/executor.go:225-232` keeps production custom Python fail-closed when no runner is configured.
- `src/server/internal/agent/executor.go:280-323` delegates production custom Python to the injected runner and maps non-zero sandbox exits to tool errors.
- `src/server/internal/agent/service.go:191-250` propagates the runner to the main agent runner and default plan-step executor.
- `src/server/cmd/agent/main.go:141-188` adapts the Workflow `CodeRunner` to the Agent custom Python runner contract.
- `src/server/internal/workflow/sandbox/sandbox.go:239-259` runs code in Docker with `--network=none`, memory/swap caps, CPU quota, PID limit, read-only root FS, tmpfs workdir, non-root UID/GID, dropped capabilities, and `no-new-privileges`.

## Verification Commands

```text
command: cd src/server && go test ./internal/agent -run 'TestToolExecutorUsesCustomPythonSandboxRunnerInProduction|TestToolExecutorRejectsCustomPythonInProduction|TestToolExecutorOmitsCustomPythonDefinitionsInProduction|TestToolExecutorExecutesCustomPythonTool|TestToolExecutorBlocksCustomPythonImports|TestToolExecutorRejectsCustomPythonOversizedOutput|TestToolExecutorRejectsCustomPythonOversizedSource|TestToolExecutorRejectsCustomPythonOversizedArguments' -count=1
result: passed, `ok oblivious/server/internal/agent 0.065s`

command: cd src/server && go test ./cmd/agent -run 'TestBuildAgentCustomPythonSandboxRunnerDisabledByDefault|TestBuildAgentCustomPythonSandboxRunnerEnabled|TestAgentCustomPythonSandboxRunnerMapsWorkflowResult|TestBuildAgentGatewayProductionRelayDisabledFailsClosed|TestBuildAgentGatewayDevelopmentRelayDisabledKeepsDemoFallback|TestAgentRelayBaseURLUsesDedicatedRuntimeURL' -count=1
result: passed, `ok oblivious/server/cmd/agent 0.009s`

command: bash scripts/verify-fusion-evidence-pack.sh
result: passed, `[fusion-evidence-pack] fusion evidence pack is present and guarded.`

command: bash scripts/verify-quality-gates.sh
result: passed, `[quality-gates] quality gate assets look complete.`
```

## Failure Evidence

- No configured runner: production custom Python returns `custom Python tools are disabled in production until a container sandbox is configured`.
- No configured runner: production tool definitions omit custom Python tools, preventing the model from selecting an unsafe host-Python path.
- Sandbox non-zero exit: the Agent runner surfaces stdout, stderr, logs, or exit code as a tool error instead of treating the execution as successful.

## Unsupported / Deferred Surfaces

- No target Docker runtime proof has been captured in this evidence file.
- No request ID / trace ID / agent run ID / tool run ID join has been proven for sandboxed custom Python.
- No cancellation, artifact persistence, or retention behavior has been proven end to end.
- Non-production local Python remains a development/test path and must not be counted as commercial runtime evidence.

## Known Residual Risk

Commercial custom-code readiness still requires target deployment evidence proving container availability, image policy, network denial, filesystem isolation, resource ceilings, cancellation, logs, artifacts, audit joins, and operator runbooks.

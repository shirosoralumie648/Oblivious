# Agent Custom Python Production Deny Evidence - 2026-07-02

## Claim

Agent custom Python tools no longer execute on the host in production. Until a containerized sandbox or worker is wired, production runtimes fail closed and do not advertise custom Python tool definitions to the model.

## Runtime Behavior

- Non-production custom Python tools keep their existing local runtime for development and tests.
- `APP_ENV=production` causes custom Python execution to return an error result before resolving or launching a Python binary.
- `APP_ENV=production` omits custom Python tools from `GetToolDefinitions`, so structured model calls do not receive unsafe host-Python tool definitions.
- Custom API, MCP, and enabled commercial builtin tools are unchanged.

## Changed Files

- `src/server/internal/agent/executor.go`
- `src/server/internal/agent/executor_test.go`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/stub-hardcoded-todo-report.md`
- `docs/audit/vertical-slice-gap-report.md`

## Verification

```bash
cd src/server
export PATH="/c/Program Files/Go/bin:$PATH"
go test ./internal/agent -run "TestToolExecutor(ExecutesCustomAPI|ExecutesCustomPython|RejectsCustomPythonInProduction|OmitsCustomPythonDefinitionsInProduction|BlocksCustomPythonImports|AllowsWebSearch)" -count=1 -v
```

Result: passed.

## Negative Path

`TestToolExecutorRejectsCustomPythonInProduction` sets `APP_ENV=production` and verifies a Python custom tool returns an error result containing `disabled in production`.

`TestToolExecutorOmitsCustomPythonDefinitionsInProduction` verifies the same production profile keeps custom API definitions but omits custom Python definitions.

## Residual Risk

This is a fail-closed production guard, not full commercial custom-code execution. Shipping custom Python as a production feature still requires a container sandbox or remote worker with filesystem, network, resource, timeout, artifact, and log policy evidence.

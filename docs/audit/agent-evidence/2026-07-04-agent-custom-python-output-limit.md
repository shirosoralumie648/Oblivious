# Agent Custom Python Payload Limits

Date: 2026-07-04

## Scope

This slice reduces the non-production custom Python sandbox blast radius by bounding submitted source code, serialized arguments, and captured stdout/stderr. Production custom Python remains disabled until a container sandbox or remote worker with CPU, memory, network, filesystem, artifact, and log policy evidence is wired.

## Changed Files

- `src/server/internal/agent/executor.go`
- `src/server/internal/agent/executor_test.go`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/oblivious-gap-matrix.md`
- `docs/audit/stub-hardcoded-todo-report.md`

## Verification

- RED: `cd src/server && go test ./internal/agent -run 'TestToolExecutorRejectsCustomPythonOversizedOutput' -count=1`
  - Failed because `customPythonMaxOutputBytes` did not exist and the executor had no custom Python output bound.
- RED: `cd src/server && go test ./internal/agent -run 'TestToolExecutorRejectsCustomPythonOversizedSource|TestToolExecutorRejectsCustomPythonOversizedArguments' -count=1`
  - Failed because `customPythonMaxSourceBytes` and `customPythonMaxArgumentBytes` did not exist and the executor had no source/argument payload bounds.
- GREEN: `cd src/server && go test ./internal/agent -run 'TestToolExecutorExecutesCustomPythonTool|TestToolExecutorRejectsCustomPythonInProduction|TestToolExecutorOmitsCustomPythonDefinitionsInProduction|TestToolExecutorBlocksCustomPythonImports|TestToolExecutorRejectsCustomPythonOversizedOutput|TestToolExecutorRejectsCustomPythonOversizedSource|TestToolExecutorRejectsCustomPythonOversizedArguments' -count=1`
  - Passed after adding source, argument, and output payload guards.

## Remaining Boundary

This is not production custom-code enablement. Commercial readiness still requires a sandboxed worker/container with explicit CPU, memory, network, filesystem, artifact, timeout, cancellation, and audit-log policy before custom Python can be advertised or executed in production.

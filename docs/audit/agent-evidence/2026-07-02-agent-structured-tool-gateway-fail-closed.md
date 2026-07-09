# Agent Structured Tool Gateway Fail-Closed Evidence

Date: 2026-07-02

## Summary

Tool-enabled Agent runs no longer fall back to plain text when the configured gateway does not implement `chat.StructuredReplyGenerator`.

The runner now marks the run failed and returns `ErrStructuredGatewayRequired` before model execution in:

- First tool run: `src/server/internal/agent/runner.go:868-871`
- Resume after approved tool: `src/server/internal/agent/runner.go:995-998`
- ReAct request entrypoint: `src/server/internal/agent/runner.go:2023-2026`

This closes the prior product risk where a tool-capable run could appear completed without executing required tool calls.

## Code Changes

- `src/server/internal/agent/runner.go`
  - Added `ErrStructuredGatewayRequired`.
  - Removed plain `GenerateReply` fallback branches from tool-enabled execution paths.
  - Added a direct `Runner.Run` guard so tool-enabled agents cannot be executed through the lightweight plain gateway path.
  - Failed persisted runs with `structured reply gateway required for tool execution`.
- `src/server/internal/agent/service_test.go`
  - Replaced the old fallback streaming expectation with `TestRunWithToolsRejectsPlainGateway`.
  - Verifies no chunks are streamed, the plain gateway is not invoked, and the run is marked `failed`.
  - Added `TestRunnerRunRejectsToolEnabledAgent`, which verifies direct `Runner.Run` misuse returns `ErrStructuredGatewayRequired` before any plain gateway call or message persistence.

## Verification

```text
go test ./internal/agent -run "TestRunWithToolsRejectsPlainGateway|TestRunWithTools.*Stream|TestServiceSendMessagePlainPath|TestServiceSendMessageAllDisabledTools" -count=1 -v
```

Result: PASS

Relevant passing tests:

- `TestRunWithToolsStreaming`
- `TestRunWithToolsRejectsPlainGateway`
- `TestServiceSendMessagePlainPath`
- `TestServiceSendMessageAllDisabledTools`

## Remaining Agent Gaps

- The lightweight non-tool `Run` path remains plain chat by design; tool-enabled agents must use `RunWithTools`, and direct `Runner.Run` misuse now fails closed (`src/server/internal/agent/runner.go:378-381`).
- Structured Agent streaming still chunks a completed response instead of streaming provider/tool-call deltas live (`src/server/internal/agent/runner.go:937-947`, `src/server/internal/agent/runner.go:2123-2128`).
- Production custom-code execution still needs a sandboxed worker/container before it can be advertised as a commercial feature.

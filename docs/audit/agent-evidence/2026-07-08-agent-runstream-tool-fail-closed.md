# Agent RunStream Tool Fail-Closed

## Scope

This pass closes a direct Runner entry point that could stream a plain model answer for a tool-enabled Agent without using the structured tool loop.

## Runtime Change

- `Runner.RunStream` now checks `hasEnabledTools(agent)` before saving a user message or calling the plain streaming gateway.
- Tool-enabled streaming requests are routed through `RunWithTools`, so they require a `chat.StructuredReplyGenerator`, create a durable run, persist tool-call/tool-result evidence, and stream only the final structured assistant answer.
- A plain streaming gateway now fails closed with `ErrStructuredGatewayRequired` for tool-enabled `RunStream` calls and records the failed run instead of silently returning a non-tool answer.

## Verification

- `go test ./internal/agent -run 'TestRunStreamWithTools(UsesStructuredToolLoop|RejectsPlainGateway)|TestRunWithToolsStreaming|TestRunWithToolsRejectsPlainGateway' -count=1 -v`

## Boundary

This is repository-local Agent runtime hardening. It does not provide target live provider traces, cancellation proof, production tool sandbox evidence, or final no-skip commercial release evidence.

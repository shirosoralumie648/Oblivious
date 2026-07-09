# Agent Runner Tool Path Fail-Closed Evidence

Date: 2026-07-05

## Summary

Direct lightweight `Runner.Run` calls now reject tool-enabled agents before any user message persistence or plain gateway call. This closes the remaining local bypass where code could misuse the no-tool path and receive a plain assistant answer even though enabled tools required the structured loop.

## Code Changes

- `src/server/internal/agent/runner.go`
  - Added an entry guard at `Runner.Run` that returns `ErrStructuredGatewayRequired` when `hasEnabledTools(agent)` is true.
  - The guard runs before message persistence, history loading, memory retrieval, and `GenerateReply`.
- `src/server/internal/agent/service_test.go`
  - Added `TestRunnerRunRejectsToolEnabledAgent`.
  - The regression verifies the error type, zero plain gateway calls, and zero persisted messages.

## Verification

```text
go test ./internal/agent -run TestRunnerRunRejectsToolEnabledAgent -count=1 -v
```

Result: PASS after the guard; before the guard, the test failed because `Runner.Run` returned a plain fallback result and no error.

```text
go test ./internal/agent -run 'TestRunnerRunRejectsToolEnabledAgent|TestRunWithToolsRejectsPlainGateway|TestServiceSendMessageUsesRunnerForToolEnabledAgents|TestServiceSendMessageExplicitReactModeOverridesPlanningDefault|TestServiceSendMessagePropagatesRelayMetadataThroughRunWithTools|TestRunnerInjectsUserManagedAgentMemoriesIntoPrompt|TestRunnerPrefersVectorAgentMemorySearchWhenEmbedderConfigured' -count=1 -v
```

Result: PASS

## Remaining Agent Gaps

- Live structured provider/tool-call streaming is still not implemented; final content is chunked after a full structured reply.
- Target-runtime evidence is still required for tool execution traces, cancellation behavior, sandbox isolation, audit-log joins, and commercial release readiness.

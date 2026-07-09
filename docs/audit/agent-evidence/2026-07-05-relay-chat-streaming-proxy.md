# Relay Chat Streaming Proxy Evidence

Date: 2026-07-05

## Summary

`handler_new` Chat streaming no longer waits for the full upstream SSE response before writing to the client. Streaming requests now mark the context as trusted streaming, use the selected route channel adapter, copy upstream chunks through `StreamChunkCallback`, flush each chunk immediately, and skip the old post-stream captured-body write when the callback has already written the response.

## Code Changes

- `src/server/internal/relay/handler_new/chat.go`
  - Marks Chat stream requests with `types.WithTrustedStreaming`.
  - Builds the upstream adapter from the selected `RouteChannel` instead of the handler's default adapter.
  - Adds `StreamChunkCallback` for streamed requests and writes/flushed chunks as they arrive.
  - Adds `copyChatProviderStream` to capture chunks for downstream response metadata without buffering client delivery.
  - Stops `handleStream` from writing captured stream content again after the callback already wrote it.
- `src/server/internal/relay/handler_new/chat_test.go`
  - Adds `TestChatStreamProxiesFirstProviderChunkBeforeUpstreamCompletes`.
  - Adds `TestChatStreamDoesNotWriteCapturedStreamTwice`.

## Verification

```text
go test ./internal/relay/handler_new -run 'TestChatStreamProxiesFirstProviderChunkBeforeUpstreamCompletes|TestChatStreamDoesNotWriteCapturedStreamTwice' -count=1 -v
```

Result: PASS

```text
go test ./internal/relay/handler_new -count=1
```

Result: PASS

## Remaining Relay Streaming Gaps

- Target-runtime evidence is still required for real provider streaming under production config.
- Client abort/disconnect handling, authoritative provider usage capture, request-log linkage, settlement/refund behavior, and broader route audit proof are still required before counting Relay streaming as fully commercial-ready.

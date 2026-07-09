# Chat Gateway Usage Token Authority

Date: 2026-07-05

## Scope

This repository-local hardening slice advances `P0-Execution-Evidence-Spine` for the Chat application path. It ensures non-streaming Chat usage persistence prefers token counts returned by the structured Relay-backed gateway instead of relying only on local text-length estimates.

## RED

`TestSendMessageRecordsGatewayUsageTokensWhenAvailable` first failed because `SendMessage` persisted estimated usage (`input=6`, `output=4`) even when the gateway returned exact usage (`prompt_tokens=12`, `completion_tokens=8`, `total_tokens=20`).

Command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/chat -run TestSendMessageRecordsGatewayUsageTokensWhenAvailable -count=1
```

Failure:

```text
expected gateway usage tokens 12/8, got input=6 output=4
```

## GREEN

`SendMessage` now uses `StructuredReplyGenerator` when available, carries `CompletionUsage` into `persistAssistantReply`, and falls back to `estimateTokens` only when gateway usage is absent. The same refactor preserves semantic workflow triggering and now passes the persisted user message ID into `SemanticWorkflowTriggerRequest`.

Code:

- `src/server/internal/chat/service.go`
- `src/server/internal/chat/service_test.go`

Verification:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/chat -run 'TestSendMessageRecordsGatewayUsageTokensWhenAvailable|TestSendMessageRecordsUsage|TestSendMessageStreamEmitsAndPersistsAssistantReply' -count=1
```

Result:

```text
ok  	oblivious/server/internal/chat	0.006s
```

Expanded verification:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/chat ./internal/http -count=1
```

Result:

```text
ok  	oblivious/server/internal/chat	2.018s
ok  	oblivious/server/internal/http	1.772s
```

## Commercial Boundary

This closes a repository-local Chat metering gap: when the Chat path uses a structured Relay-backed gateway, app-level usage records can now reflect provider/Relay token usage rather than text estimates.

This is not a renewed commercial-readiness claim. Target-runtime live provider proof, target request-log/usage joins, ClickHouse row evidence, and the final no-skip release evidence remain required before commercial release readiness can be claimed.

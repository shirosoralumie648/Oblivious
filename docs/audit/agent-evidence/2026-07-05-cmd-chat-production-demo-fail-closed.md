# Cmd Chat Production Demo Fail-Closed

Date: 2026-07-05

## Scope

This slice closes the remaining standalone `cmd/chat` demo-output ambiguity. The command still supports a simple development demo generator, but production mode now fails closed instead of returning `"Demo reply"`.

## Runtime Changes

- Added `buildChatReplyGenerator` in `cmd/chat`.
- Production config returns `chat.NewLocalGateway(nil)`, which produces `chat.ErrModelGatewayUnavailable`.
- Development config still uses `demoReplyGenerator`.
- Added focused command tests for both production fail-closed and development demo behavior.
- Updated the stub/hardcoded/TODO report so `cmd/chat` cannot be counted as commercial Chat runtime evidence.

## RED Evidence

Focused RED command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./cmd/chat -run 'TestBuildChatReplyGeneratorProductionDisablesDemoReply|TestBuildChatReplyGeneratorDevelopmentKeepsDemoReply' -count=1 -v
```

Observed failure before introducing the guarded builder:

```text
# oblivious/server/cmd/chat [oblivious/server/cmd/chat.test]
cmd/chat/main_test.go:13:15: undefined: buildChatReplyGenerator
cmd/chat/main_test.go:28:15: undefined: buildChatReplyGenerator
FAIL	oblivious/server/cmd/chat [build failed]
FAIL
```

## GREEN Evidence

Focused GREEN command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./cmd/chat -run 'TestBuildChatReplyGeneratorProductionDisablesDemoReply|TestBuildChatReplyGeneratorDevelopmentKeepsDemoReply' -count=1 -v
```

Result:

```text
=== RUN   TestBuildChatReplyGeneratorProductionDisablesDemoReply
--- PASS: TestBuildChatReplyGeneratorProductionDisablesDemoReply (0.00s)
=== RUN   TestBuildChatReplyGeneratorDevelopmentKeepsDemoReply
--- PASS: TestBuildChatReplyGeneratorDevelopmentKeepsDemoReply (0.00s)
PASS
ok  	oblivious/server/cmd/chat	0.005s
```

## Commercial Boundary

This closes production demo-output ambiguity for the standalone Chat command. It does not prove commercial Chat readiness; target Relay/provider behavior, request-log joins, usage/billing evidence, and streaming proof remain separate release gates.

# Workflow Trigger Replay Ledger

Date: 2026-07-05

## Runtime Claim

Signed Workflow webhooks and chat-driven Workflow conversation/semantic triggers now use a durable replay ledger when mounted through the production router. The router wires signed webhook replay and semantic trigger dispatch to `workflow_webhook_replay_keys`, so replayed signed webhook keys and duplicate chat message trigger events are rejected even when handled by reconstructed handler or dispatcher instances.

## Reference Inputs

```text
docs/audit/implementation-roadmap.md - long-running flows must be durable across process restart.
docs/audit/stub-hardcoded-todo-report.md - Workflow trigger replay/idempotency remained open after debug replay and retention work.
src/server/internal/http/workflow_handler.go - existing signed webhook HMAC/timestamp/replay-key path and chat-driven Workflow trigger dispatcher.
```

## Oblivious Files Changed

```text
src/server/internal/http/workflow_handler.go
src/server/internal/http/workflow_handler_test.go
src/server/internal/http/router.go
src/server/internal/http/server_test.go
src/server/internal/chat/context_types.go
src/server/internal/chat/service.go
src/server/internal/chat/service_test.go
src/server/migrations/0098_workflow_webhook_replay_keys.sql
src/server/migrations/microservices/table-ownership.json
scripts/verify-commercial-db-evidence.sh
scripts/verify-commercial-db-evidence-profiles.sh
docs/audit/agent-evidence/2026-07-05-workflow-webhook-replay-ledger.md
docs/audit/stub-hardcoded-todo-report.md
docs/audit/current-implementation-depth.md
docs/audit/implementation-roadmap.md
docs/release/fusion-spec-evidence-pack.md
docs/release/commercial-completion-audit.md
```

## Contract Changes

- PostgreSQL trigger replay ledger table: `workflow_webhook_replay_keys`.
- New migration: `src/server/migrations/0098_workflow_webhook_replay_keys.sql`.
- Chat now propagates the persisted user `messageId` into `SemanticWorkflowTriggerRequest`; the dispatcher includes it in trigger payloads and replay keys.
- The production router wires `workflowSemanticTriggerDispatcher.replayStore` to the SQL replay ledger.
- `workflow-sql-isolation` commercial DB evidence profile now includes `TestWorkflowHandlerSignedWebhookRejectsReplayAcrossHandlerRestart` and `TestWorkflowSemanticTriggerDispatcherRejectsDuplicateConversationEventAcrossDispatcherRestart`.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^(TestWorkflowHandlerSignedWebhookRejectsReplayAcrossHandlerRestart|TestWorkflowHandlerSignedWebhookRejectsReplay)$' -count=1 -v
result: pass for in-memory replay rejection; DB-backed restart replay test skipped without TEST_DATABASE_URL and was not counted as release evidence.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/chat ./internal/http -run '^(TestSendMessageTriggersSemanticWorkflowsAfterAssistantReply|TestWorkflowSemanticTriggerDispatcherRejectsDuplicateConversationEvent|TestWorkflowSemanticTriggerDispatcherRejectsDuplicateConversationEventAcrossDispatcherRestart)$' -count=1 -v
result: pass for chat messageId propagation and in-memory duplicate conversation/semantic trigger rejection; DB-backed dispatcher restart replay test skipped without TEST_DATABASE_URL and was not counted as release evidence.

command: bash scripts/verify-commercial-db-evidence-profiles.sh
result: pass

command: bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation
result: pass; disposable pgvector PostgreSQL run reported skipped tests: none and ran TestWorkflowHandlerSignedWebhookRejectsReplayAcrossHandlerRestart plus TestWorkflowSemanticTriggerDispatcherRejectsDuplicateConversationEventAcrossDispatcherRestart.
```

## Runtime Evidence IDs

```text
organization_id: org_1 in handler test payload; active generated organization in commercial DB profile
workflow_execution_id: wexec_webhook in handler test fixture
message_id: message_1 in duplicate conversation/semantic trigger replay fixture
```

## Failure Evidence

- RED: `TestWorkflowHandlerSignedWebhookRejectsReplayAcrossHandlerRestart` initially failed because a second handler instance accepted the same signed webhook and returned `201`, proving the old replay guard was process-local.
- RED: `TestWorkflowSemanticTriggerDispatcherRejectsDuplicateConversationEvent` initially failed with four starts for one duplicated chat event, proving conversation and semantic matches were replayed on duplicate message delivery.
- Negative path: the existing `TestWorkflowHandlerSignedWebhookRejectsReplay` still proves same-process duplicate signed webhooks return `409 webhook_replay_detected`.
- DB path: `bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation` reruns the cross-handler signed webhook replay test and the cross-dispatcher duplicate conversation/semantic trigger test against disposable PostgreSQL with no skipped tests.

## Unsupported / Deferred Surfaces

- This is repository-local PostgreSQL evidence, not deployed target webhook traffic proof.
- Scheduled trigger idempotency and broader non-chat trigger replay paths remain open.
- Full retry/failure replay depth and target workflow telemetry remain external release evidence.

## Known Residual Risk

Final commercial readiness still requires target-environment workflow telemetry, deployed Workflow gRPC smoke, and a no-skip strict release run with target evidence enabled.

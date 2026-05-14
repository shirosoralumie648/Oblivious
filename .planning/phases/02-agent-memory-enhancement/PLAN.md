# PLAN.md - Phase 2 Execution Plan

**Phase:** 02-agent-memory-enhancement
**Status:** Ready for execution
**Created:** 2026-04-27

---

## Executive Summary

Phase 2 does not start from zero. The repo already has working `memory`, `agent`, `mcp`, and `quota` packages, but the critical execution path is still incomplete:

1. The Agent stack cannot complete multi-turn tool calling because the chat/relay path collapses responses down to plain text.
2. `agent.Service` still bypasses `Runner`, so tool execution and richer memory-aware loops are not used by the live API.
3. Quota accounting exists as a standalone service, but Relay billing does not know which user to charge and does not call `quota.Service`.
4. Vector search is implemented, but the migration still creates an IVFFlat index while Phase 2 context explicitly chose HNSW.

The plan below closes those gaps without breaking the existing "all LLM traffic goes through Relay" rule.

### Current Implementation Snapshot

| Area | Existing implementation | Gap to close |
|------|-------------------------|--------------|
| Memory | `memory.Service`, `RelayEmbedder`, handlers, pgvector tables exist | HNSW migration follow-up, regression coverage, config hardening |
| Agent tools | `ToolExecutor`, `Runner`, tool-call parsers, tool persistence model exist | Live service path does not use `Runner`; no structured tool-call response path |
| Relay/chat | `RelayGateway` streams plain content; relay handler passes through basic chat payloads | tools/tool_calls/usage and internal billing identity are dropped |
| Quota | `quota.Service` + SQL store implement preconsume/settle/refund | Relay `BillingHook` never calls quota; `RouteWithBilling` lacks user context |

---

## Plan Decisions

1. **Do not mutate old migrations in place.** Keep `0016_pgvector.sql` as historical reality and add a new migration for the HNSW switch.
2. **Preserve Relay as the only LLM path.** Tool-calling support must extend Relay/chat contracts, not bypass Relay from Agent code.
3. **Use structured completions for tool turns.** A plain `string` response cannot carry `tool_calls`; Phase 2 must add a structured response path in `chat` and `relay`.
4. **Propagate user identity intentionally.** Quota is user-scoped, so app-originated Relay requests need a trusted internal user identifier or equivalent context handoff.
5. **Stream the final answer, not the tool decision.** The pragmatic first version may use non-streamed structured completions for tool turns and keep streaming for the final assistant reply.

---

## Execution Waves

### Wave 1: Relay/Chat Contract Expansion

#### Task 1.1: Add structured completion types to `chat`
**Priority:** P0 (Blocking)
**Type:** Backend contract
**Estimate:** 0.5 day

**Files:**
- `src/server/internal/chat/gateway.go`
- `src/server/internal/chat/relay_gateway.go`
- `src/server/internal/chat/relay_gateway_test.go`

**Work:**
- Introduce a structured response type for Agent use, carrying:
  - assistant content
  - tool call payloads
  - finish reason
  - usage metadata when available
- Keep the existing `GenerateReply`/`GenerateReplyStream` behavior for current chat consumers.
- Add a second code path for Agent execution that can return structured tool calls instead of only text.

**Acceptance Criteria:**
- [ ] Agent-facing chat API can receive `tool_calls` without lossy string conversion
- [ ] Existing chat service callers continue compiling without behavioral regression
- [ ] Tests cover plain text replies and structured tool-call replies

---

#### Task 1.2: Extend Relay request/response passthrough for tools
**Priority:** P0 (Blocking)
**Type:** Relay plumbing
**Estimate:** 1 day

**Files:**
- `src/server/internal/relay/types/types.go`
- `src/server/internal/relay/channel/types.go`
- `src/server/internal/relay/handler/common.go`
- `src/server/internal/relay/handler/chat.go`

**Work:**
- Extend `types.ProviderRequest` and chat request marshalling so Relay can carry tool definitions/tool choice fields when Agent requests them.
- Preserve upstream response bodies that contain `tool_calls` instead of assuming content-only payloads.
- Ensure request parsing does not silently drop fields needed for tool calling.

**Acceptance Criteria:**
- [ ] Relay chat path preserves tool definitions sent by the caller
- [ ] Relay chat path returns upstream tool-call payloads intact to `RelayGateway`
- [ ] No regression to existing non-tool chat completions

---

#### Task 1.3: Define trusted user identity propagation for Relay-originated app traffic
**Priority:** P0 (Blocking)
**Type:** Cross-cutting integration
**Estimate:** 0.5 day

**Files:**
- `src/server/internal/chat/relay_gateway.go`
- `src/server/internal/relay/handler/chat.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/billing.go`

**Work:**
- Add a trusted internal identity mechanism for app-originated requests that hit the local Relay endpoint.
- Recommended approach: forward authenticated user identity and request idempotency in internal-only headers from the app service/gateway to Relay.
- Reject any design that allows external clients to spoof quota ownership.

**Acceptance Criteria:**
- [ ] Relay billing code can resolve the authenticated user for app-originated requests
- [ ] Internal user propagation is explicit and bounded to trusted server-to-server traffic
- [ ] Idempotency key handling remains intact for retries

---

### Wave 2: Agent Runner Integration

#### Task 2.1: Route live Agent messaging through `Runner`
**Priority:** P0 (Blocking)
**Type:** Backend refactor
**Estimate:** 1 day

**Files:**
- `src/server/internal/agent/service.go`
- `src/server/internal/agent/runner.go`
- `src/server/internal/http/agent_handler.go`

**Work:**
- Replace the duplicated inline chat loop in `agent.Service.SendMessage`/`SendMessageStream` with `Runner`.
- Select `RunWithTools` when the Agent has enabled tools; otherwise keep the plain path lightweight.
- Ensure message persistence happens once, in the correct order, without duplicate user/assistant rows.

**Acceptance Criteria:**
- [ ] Live `/api/v1/app/agents/conversations/:id/messages` uses `Runner` instead of the old inline logic
- [ ] Memory injection still works when enabled
- [ ] Existing non-tool agents continue to return normal replies

---

#### Task 2.2: Implement automatic tool loop and tool-result persistence
**Priority:** P0 (Blocking)
**Type:** Backend behavior
**Estimate:** 1.5 days

**Files:**
- `src/server/internal/agent/runner.go`
- `src/server/internal/agent/executor.go`
- `src/server/internal/agent/store.go`
- `src/server/internal/agent/service.go`

**Work:**
- Detect tool calls from the structured completion path.
- Execute builtin and MCP tools through `ToolExecutor`.
- Persist tool-call metadata and tool-result messages using `tool_calls` / `tool_call_id`.
- Continue looping until the model returns a final assistant response or `MaxIterations` is reached.
- Return a clear error when the loop exhausts its iteration budget.

**Acceptance Criteria:**
- [ ] Agent can execute at least one builtin tool and continue the conversation
- [ ] Agent can execute at least one MCP tool and continue the conversation
- [ ] Tool results are written as `role=tool` messages linked by `tool_call_id`
- [ ] Iteration cap is enforced and tested

---

#### Task 2.3: Final-answer streaming strategy
**Priority:** P1
**Type:** Backend UX behavior
**Estimate:** 0.5 day

**Files:**
- `src/server/internal/agent/runner.go`
- `src/server/internal/chat/relay_gateway.go`
- `src/server/internal/http/agent_handler.go`

**Work:**
- Keep streamed chunks for the final assistant turn.
- For tool-decision turns, use the structured non-streamed path unless function-call streaming is implemented end-to-end.
- Document the chosen behavior in code comments where it is non-obvious.

**Acceptance Criteria:**
- [ ] Streaming clients still receive final assistant tokens
- [ ] Tool-call turns do not break stream consumers
- [ ] Behavior is deterministic and covered by tests

---

### Wave 3: Memory and Billing Hardening

#### Task 3.1: Add HNSW migration and keep search behavior stable
**Priority:** P1
**Type:** Database / memory
**Estimate:** 0.5 day

**Files:**
- `src/server/migrations/0020_memory_hnsw.sql`
- `src/server/internal/memory/service.go`

**Work:**
- Add a new migration that replaces or supplements the IVFFlat index with the HNSW decision captured in `02-CONTEXT.md`.
- Keep the query operator (`<=>`) and result filtering stable unless measurements demand changes.
- Do not edit already-shipped migration filenames in place.

**Acceptance Criteria:**
- [ ] Migration history remains append-only
- [ ] Memory similarity search still works with the new index strategy
- [ ] Search semantics remain user-isolated and score-filtered

---

#### Task 3.2: Integrate `quota.Service` into Relay billing lifecycle
**Priority:** P0 (Blocking)
**Type:** Billing integration
**Estimate:** 1 day

**Files:**
- `src/server/internal/quota/service.go`
- `src/server/internal/relay/billing.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/billing_test.go`
- `src/server/internal/relay/billing_worker.go`

**Work:**
- Extend `BillingHook` to depend on `quota.Service` or a narrow billing adapter.
- On pre-bill, estimate cost and call `PreConsume`.
- On successful completion, call `Settle` with actual usage-derived cost.
- On failure/timeouts, call `Refund`.
- Preserve idempotency across duplicate request attempts and delayed settlement/refund flows.

**Acceptance Criteria:**
- [ ] Successful Relay calls create preauthorized quota sessions and settle them
- [ ] Failed Relay calls refund quota correctly
- [ ] Duplicate idempotency keys do not double-charge the user

---

### Wave 4: Regression Coverage and Verification

#### Task 4.1: Add focused tests for newly activated paths
**Priority:** P0
**Type:** Test coverage
**Estimate:** 1 day

**Files:**
- `src/server/internal/agent/service_test.go`
- `src/server/internal/agent/store_test.go`
- `src/server/internal/memory/embedder_test.go`
- `src/server/internal/quota/service_test.go`
- `src/server/internal/relay/billing_test.go`
- `src/server/internal/chat/relay_gateway_test.go`

**Work:**
- Add unit tests around:
  - structured tool-call parsing and execution loop
  - memory embedder error handling and batch behavior
  - quota preconsume/settle/refund behavior
  - relay billing idempotency
- Prefer small fakes/httptest over brittle end-to-end fixtures where possible.

**Acceptance Criteria:**
- [ ] New tests fail on the pre-Phase-2 gaps and pass after implementation
- [ ] Tests target the touched packages directly, not only broad `go test ./...`
- [ ] Tool-loop and billing regressions have explicit assertions

---

#### Task 4.2: Execute verification commands and capture remaining gaps
**Priority:** P0
**Type:** Verification
**Estimate:** 0.5 day

**Commands:**
```bash
source ~/.g/env && cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/server && go build ./...
source ~/.g/env && cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/server && go test ./internal/agent/... ./internal/chat/... ./internal/memory/... ./internal/quota/... ./internal/relay/... -count=1
source ~/.g/env && cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious && bash scripts/check.sh all
```

**Functional Checks:**
- [ ] Agent conversation can trigger a builtin tool and return a final assistant answer
- [ ] Agent conversation can trigger an MCP tool and persist tool messages
- [ ] Memory search returns relevant chunks for the authenticated user only
- [ ] Quota balance decreases on pre-auth and settles/refunds correctly after Relay completion

---

## Risks and Guardrails

### Key Risks
- `RelayGateway` and Relay handlers currently assume content-only chat payloads; careless changes can break standard chat.
- Billing integration is cross-cutting and easy to get wrong if user identity is implicit.
- Streaming tool-call support can balloon in scope if attempted end-to-end in one pass.

### Guardrails
- Keep the current chat API surface backward compatible for non-Agent callers.
- Introduce user propagation only for trusted internal Relay requests.
- Prefer append-only migrations and explicit tests around idempotency/refunds.
- Do not weaken the rule that all model traffic must go through Relay.

---

## Success Criteria Summary

- [ ] `EXEC-01~03`: Agent tool execution loop is live through the API and supports multi-turn continuation
- [ ] `MEM-01~03`: Memory search path is verified, indexed with the chosen strategy, and regression-covered
- [ ] `QUOTA-01`: Relay billing drives quota preconsume/settle/refund with idempotency
- [ ] Phase 2 can proceed to execution without unresolved architecture ambiguity in tool calls or billing identity

---

## Next Step After Planning

1. Execute the plan in wave order, starting with the Relay/chat contract work.
2. Verify each blocking wave before moving to the next one.
3. Produce a `SUMMARY.md` for Phase 2 so future `$gsd-next` runs do not hit the same completeness gap as Phase 1.

---

*Phase: 02-agent-memory-enhancement*

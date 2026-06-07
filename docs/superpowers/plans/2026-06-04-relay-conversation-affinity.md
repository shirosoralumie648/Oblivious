# Relay Conversation Affinity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route requests from the same trusted conversation to the same successful Relay channel, and update the affinity after retryable fallback succeeds on another channel.

**Architecture:** Add a trusted conversation id to Relay context metadata, keep an in-memory channel affinity map inside `relay.Router`, prefer the bound channel when it is still eligible, and update the binding after successful routing. Chat's `RelayGateway` will forward `ConversationConfig.ConversationID` through an internal header so app-originated chat requests participate in affinity.

**Tech Stack:** Go 1.22, existing Relay router/load-balancer/channel pool, existing chat Relay HTTP gateway.

---

### Task 1: Relay Context Carries Trusted Conversation ID

**Files:**
- Modify: `src/server/internal/relay/types/types.go`
- Modify: `src/server/internal/chat/relay_gateway_test.go`
- Modify: `src/server/internal/chat/relay_gateway.go`

- [ ] **Step 1: Write the failing test**

Add a `RelayGateway` test that sends `ConversationConfig{ConversationID: "conversation_1"}` and asserts the outbound Relay request includes `X-Oblivious-Internal-Conversation-ID: conversation_1`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/server && GOCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-build GOMODCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-mod go test ./internal/chat -run TestRelayGateway_ForwardsConversationIDHeader -count=1`

Expected: FAIL because the gateway does not set that header yet.

- [ ] **Step 3: Implement minimal metadata support**

Add `HeaderInternalConversationID`, `WithTrustedConversationID`, and `TrustedConversationIDFromContext` in `relay/types`. Set the header in `RelayGateway` when `ConversationConfig.ConversationID` is non-empty.

- [ ] **Step 4: Run target test**

Run the same command from Step 2.

Expected: PASS.

### Task 2: Router Reuses and Updates Conversation Affinity

**Files:**
- Modify: `src/server/internal/relay/router_test.go`
- Modify: `src/server/internal/relay/types/types.go`
- Modify: `src/server/internal/relay/router.go`

- [ ] **Step 1: Write failing router tests**

Add tests proving that a trusted conversation id binds after the first successful route, the second route uses the bound channel even if normal load balancing would choose another channel, and retryable fallback updates the binding to the successful fallback channel.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd src/server && GOCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-build GOMODCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-mod go test ./internal/relay -run 'TestRouter_ConversationAffinity' -count=1`

Expected: FAIL because the router has no conversation affinity map.

- [ ] **Step 3: Implement minimal router affinity**

Add a mutex-protected map from conversation id to channel id. Before load balancing, prefer the bound channel if it exists, is not excluded, is enabled, is healthy, supports the model/route key, and allows the trusted user group. After a successful 2xx or 3xx response, bind the conversation to the selected channel. For retryable failures, keep existing fallback exclusion behavior and update affinity only after the later success.

- [ ] **Step 4: Run target tests**

Run the same command from Step 2.

Expected: PASS.

- [ ] **Step 5: Run regressions**

Run:

```bash
cd src/server && GOCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-build GOMODCACHE=/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/.tmp/go-mod go test ./internal/relay/... ./internal/chat ./internal/http -count=1
```

Expected: PASS.

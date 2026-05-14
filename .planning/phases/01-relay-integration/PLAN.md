# PLAN.md - Phase 1 Verification

**Phase:** 01-relay-integration
**Status:** Verification Required
**Created:** 2026-04-27

---

## Executive Summary

Phase 1 implementation is **substantially complete**. All 29 requirements have corresponding implementations. This verification plan validates each milestone's success criteria and identifies any gaps.

### Implementation Status Overview

| Milestone | Requirements | Implementation | Tests | Status |
|-----------|-------------|----------------|-------|--------|
| M1.1 Relay Integration | RELAY-01~07 | Complete | Partial | Needs Verification |
| M1.2 Chat via Relay | CHAT-01~05 | Complete | Partial | Needs Verification |
| M1.3 Agent Runtime | AGENT-01~10 | Complete | Missing | Needs Tests |
| M1.4 MCP Client | MCP-01~07 | Complete | Missing | Needs Tests |

---

## Verification Tasks

### Wave 1: Build & Static Verification

#### Task 1.1: Verify Go Build
**Priority:** P0 (Blocking)
**Type:** Automated
**Estimate:** 2 min

**Command:**
```bash
cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/server
go build ./...
```

**Success Criteria:**
- [ ] Build completes with exit code 0
- [ ] No compilation errors
- [ ] Binary produced in `./` directory

**Implementation Verified:**
- `internal/http/server.go` - Relay mounting (D-01)
- `internal/relay/relay.go` - Relay core
- `internal/relay/store.go` - Database persistence (D-03)
- `internal/config/config.go` - RelayEnabled config (D-10)

---

#### Task 1.2: Verify Database Migrations
**Priority:** P0 (Blocking)
**Type:** Automated
**Estimate:** 2 min

**Command:**
```bash
ls -la /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/server/migrations/ | grep -E '001[3-5]'
```

**Expected Migrations:**
| Migration | Tables | Requirement |
|-----------|--------|-------------|
| 0013_channels.sql | channels, model_routes, model_channel_weights | RELAY-03, RELAY-04 |
| 0014_agents.sql | agents, agent_conversations, agent_messages | AGENT-01~03 |
| 0015_mcp_servers.sql | mcp_servers | MCP-01 |

**Success Criteria:**
- [ ] All 3 migration files exist
- [ ] migrations contain expected tables
- [ ] Foreign key constraints are valid

---

#### Task 1.3: Verify Frontend Build (if applicable)
**Priority:** P1
**Type:** Automated
**Estimate:** 3 min

**Command:**
```bash
cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/web
pnpm build 2>&1 || echo "Frontend not configured or build skipped"
```

**Success Criteria:**
- [ ] Build completes or skipped gracefully
- [ ] No blocking errors

---

### Wave 2: Test Execution

#### Task 2.1: Run Existing Go Tests
**Priority:** P0 (Blocking)
**Type:** Automated
**Estimate:** 5 min

**Command:**
```bash
cd /media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/src/server
go test ./... -count=1 -v 2>&1 | tee /tmp/test-output.log
```

**Test Coverage by Module:**
| Module | Test File | Status |
|--------|-----------|--------|
| relay | router_test.go, billing_test.go, etc. | Existing |
| chat | gateway_test.go, relay_gateway_test.go, service_test.go | Existing |
| config | config_test.go | Existing |
| agent | **MISSING** | Needs Creation |
| mcp | **MISSING** | Needs Creation |
| memory | **MISSING** | Needs Creation |

**Success Criteria:**
- [ ] All existing tests pass
- [ ] No test panics
- [ ] Test output logged for analysis

**Gaps Identified:**
- `internal/agent/service_test.go` - MISSING
- `internal/agent/store_test.go` - MISSING
- `internal/mcp/client_test.go` - MISSING
- `internal/mcp/builtin_test.go` - MISSING
- `internal/memory/embedder_test.go` - MISSING

---

#### Task 2.2: Generate Missing Test Stubs
**Priority:** P1
**Type:** Automated
**Estimate:** 10 min

**Action:** Create test file stubs for untested modules:

**Files to Create:**
1. `internal/agent/service_test.go`
2. `internal/agent/store_test.go`
3. `internal/mcp/client_test.go`
4. `internal/mcp/builtin_test.go`
5. `internal/memory/embedder_test.go`

**Template Pattern:**
```go
package agent

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestService_CreateAgent(t *testing.T) {
    // TODO: Implement test
    t.Skip("requires database fixture")
}

func TestService_SendMessage(t *testing.T) {
    // TODO: Implement test
    t.Skip("requires RelayGateway mock")
}
```

**Success Criteria:**
- [ ] Test stubs compile
- [ ] `go test ./...` includes new test files
- [ ] No import errors

---

### Wave 3: Functional Verification

#### Task 3.1: Verify M1.1 - Relay Integration
**Priority:** P0
**Type:** Manual/Integration
**Estimate:** 10 min

**Requirements Covered:** RELAY-01 through RELAY-07

**Verification Steps:**

1. **RELAY-01: Relay module mounted to HTTP server**
   ```bash
   # Verify server.go contains Relay mounting
   grep -n "RelayEnabled" src/server/internal/http/server.go
   grep -n "combineHandlers" src/server/internal/http/server.go
   ```
   - [ ] `RelayEnabled` check present (line ~22)
   - [ ] `combineHandlers` function routes /v1/* to Relay (line ~79-89)

2. **RELAY-02: /v1/* routes through Relay Engine**
   ```bash
   # Verify routing logic
   grep -n 'r.URL.Path\[:3\] == "/v1"' src/server/internal/http/server.go
   ```
   - [ ] Path check for /v1 prefix present
   - [ ] Relay engine serves /v1/* requests

3. **RELAY-03: Channel config from database**
   ```bash
   # Verify RelayStore implementation
   grep -n "LoadPoolFromStore" src/server/internal/relay/store.go
   grep -n "ListChannels" src/server/internal/relay/store.go
   ```
   - [ ] `LoadPoolFromStore` loads channels from DB
   - [ ] `ListChannels` queries `channels` table

4. **RELAY-04: Model routing config**
   ```bash
   # Verify model_routes support
   grep -n "GetModelRoute" src/server/internal/relay/store.go
   grep -n "model_routes" src/server/migrations/0013_channels.sql
   ```
   - [ ] `GetModelRoute` retrieves routing config
   - [ ] `model_routes` table defined

5. **RELAY-05: Default channel auto-creation**
   ```bash
   # Verify default channel creation
   grep -n "ensureDefaultChannel" src/server/internal/http/server.go
   ```
   - [ ] Function creates default OpenAI channel
   - [ ] Triggers when no channels exist + API key available

6. **RELAY-06: GET /v1/models endpoint**
   - Requires running server
   - Manual verification needed
   - [ ] Endpoint returns model list
   - [ ] Response format matches OpenAI spec

7. **RELAY-07: POST /v1/chat/completions through Relay**
   - Requires running server + valid channel
   - Manual verification needed
   - [ ] Request routes through Relay Router
   - [ ] Response from configured provider

**Gaps:**
- Functional tests for RELAY-06, RELAY-07 require live server
- No integration tests for end-to-end Relay flow

---

#### Task 3.2: Verify M1.2 - Chat via Relay
**Priority:** P0
**Type:** Manual/Code Review
**Estimate:** 10 min

**Requirements Covered:** CHAT-01 through CHAT-05

**Verification Steps:**

1. **CHAT-01: Chat Gateway interface design**
   ```bash
   # Verify ReplyGenerator interface
   grep -n "type ReplyGenerator interface" src/server/internal/chat/gateway.go
   grep -n "type ChatGateway interface" src/server/internal/chat/relay_gateway.go
   ```
   - [ ] `ReplyGenerator` interface exists
   - [ ] `ChatGateway` extends with stream support

2. **CHAT-02: RelayGateway implementation**
   ```bash
   # Verify RelayGateway
   grep -n "type RelayGateway struct" src/server/internal/chat/relay_gateway.go
   grep -n "func (g \*RelayGateway) GenerateReply" src/server/internal/chat/relay_gateway.go
   ```
   - [ ] `RelayGateway` struct defined
   - [ ] `GenerateReply` sends to `/v1/chat/completions`
   - [ ] Uses relay URL from config

3. **CHAT-03: Streaming SSE support**
   ```bash
   # Verify streaming
   grep -n "GenerateReplyStream" src/server/internal/chat/relay_gateway.go
   grep -n "text/event-stream" src/server/internal/chat/relay_gateway.go
   ```
   - [ ] `GenerateReplyStream` method exists
   - [ ] SSE parsing for `data:` prefix
   - [ ] Handles `[DONE]` marker

4. **CHAT-04: Token usage recording**
   - [ ] Response includes `usage` field parsing
   - [ ] `usage_records` table populated
   - Verify in: `internal/chat/relay_gateway.go` usage parsing

5. **CHAT-05: Relay mode toggle**
   ```bash
   # Verify config toggle
   grep -n "RelayEnabled" src/server/internal/http/router.go
   grep -n "CompositeGateway" src/server/internal/http/router.go
   ```
   - [ ] `RelayEnabled` controls gateway selection
   - [ ] `CompositeGateway` provides fallback

**Implementation Verified:**
- D-06: ReplyGenerator interface
- D-07: RelayGateway via Relay
- D-08: CompositeGateway with fallback
- D-09: Stream support (RelayGateway.CompleteStream)
- D-10: RelayEnabled config

**Gaps:**
- Token usage recording needs verification in production flow
- Fallback behavior not tested

---

#### Task 3.3: Verify M1.3 - Agent Runtime
**Priority:** P0
**Type:** Manual/Code Review
**Estimate:** 10 min

**Requirements Covered:** AGENT-01 through AGENT-10

**Verification Steps:**

1. **AGENT-01~03: Database migrations**
   ```bash
   cat src/server/migrations/0014_agents.sql
   ```
   - [ ] `agents` table with user_id, model, system_prompt
   - [ ] `agent_conversations` table
   - [ ] `agent_messages` table with role, content, tool_calls

2. **AGENT-04: Agent Service CRUD**
   ```bash
   grep -n "func (s \*Service) CreateAgent" src/server/internal/agent/service.go
   grep -n "func (s \*Service) GetAgent" src/server/internal/agent/service.go
   grep -n "func (s \*Service) UpdateAgent" src/server/internal/agent/service.go
   grep -n "func (s \*Service) DeleteAgent" src/server/internal/agent/service.go
   ```
   - [ ] All CRUD operations implemented
   - [ ] Ownership validation

3. **AGENT-05: Create conversation**
   ```bash
   grep -n "func (s \*Service) CreateConversation" src/server/internal/agent/service.go
   ```
   - [ ] Creates conversation linked to agent
   - [ ] User ownership verified

4. **AGENT-06: Send message through Relay**
   ```bash
   grep -n "s.gateway.GenerateReply" src/server/internal/agent/service.go
   ```
   - [ ] Uses injected `chat.ChatGateway`
   - [ ] Messages formatted correctly

5. **AGENT-07: Stream message response**
   ```bash
   grep -n "SendMessageStream" src/server/internal/agent/service.go
   ```
   - [ ] `SendMessageStream` method exists
   - [ ] Uses `gateway.GenerateReplyStream`

6. **AGENT-08: Agent HTTP Handler**
   ```bash
   grep -n "agentHandler" src/server/internal/http/router.go
   ```
   - [ ] REST routes registered in router.go
   - [ ] CRUD endpoints: GET, POST, PUT, DELETE /api/v1/app/agents
   - [ ] Conversation endpoints

7. **AGENT-09: Frontend Agent page**
   - Requires frontend build verification
   - [ ] Agent list page
   - [ ] Agent detail page
   - [ ] Conversation UI

8. **AGENT-10: Conversation history persistence**
   ```bash
   grep -n "s.store.CreateMessage" src/server/internal/agent/service.go
   ```
   - [ ] User message saved before LLM call
   - [ ] Assistant message saved after response

**Implementation Verified:**
- D-11: Agent Service CRUD complete
- D-12: Agent uses Relay via gateway
- D-14: Memory integration (SetMemory)
- D-15: MCP integration (SetMCPClient)

**Gaps:**
- No test coverage for agent service
- Frontend pages need verification
- Memory injection path not tested

---

#### Task 3.4: Verify M1.4 - MCP Client
**Priority:** P0
**Type:** Manual/Code Review
**Estimate:** 10 min

**Requirements Covered:** MCP-01 through MCP-07

**Verification Steps:**

1. **MCP-01: Database migration**
   ```bash
   cat src/server/migrations/0015_mcp_servers.sql
   ```
   - [ ] `mcp_servers` table exists
   - [ ] Fields: id, user_id, name, url, auth_token_encrypted, status

2. **MCP-02: Connection management**
   ```bash
   grep -n "func (c \*Client) Connect" src/server/internal/mcp/client.go
   grep -n "func (c \*Client) Disconnect" src/server/internal/mcp/client.go
   ```
   - [ ] `Connect` sends initialize request
   - [ ] `Disconnect` updates status
   - [ ] Server status tracked

3. **MCP-03: Tool discovery (ListTools)**
   ```bash
   grep -n "func (c \*Client) ListTools" src/server/internal/mcp/client.go
   grep -n "tools/list" src/server/internal/mcp/client.go
   ```
   - [ ] `ListTools` returns tool definitions
   - [ ] `tools/list` JSON-RPC method used

4. **MCP-04: Tool invocation (CallTool)**
   ```bash
   grep -n "func (c \*Client) CallTool" src/server/internal/mcp/client.go
   grep -n "tools/call" src/server/internal/mcp/client.go
   ```
   - [ ] `CallTool` executes remote tool
   - [ ] `tools/call` JSON-RPC method used
   - [ ] Result parsing

5. **MCP-05: Protocol message structures**
   ```bash
   grep -n "jsonrpc" src/server/internal/mcp/client.go
   ```
   - [ ] JSON-RPC 2.0 format
   - [ ] Initialize, tools/list, tools/call methods

6. **MCP-06: Built-in tools**
   ```bash
   grep -n "BuiltinTools" src/server/internal/mcp/builtin.go
   ```
   - [ ] web_search tool
   - [ ] calculator tool
   - [ ] datetime tool
   - [ ] http_request tool

7. **MCP-07: MCP HTTP Handler**
   ```bash
   grep -n "mcpHandler" src/server/internal/http/router.go
   ```
   - [ ] Routes: /api/v1/app/mcp-servers
   - [ ] Connect/disconnect endpoints
   - [ ] Tool execution endpoint

**Implementation Verified:**
- D-16: MCP Client connection management
- D-17: Tool discovery and invocation
- D-18: Built-in tools (web_search, calculator, datetime, http_request)
- D-19: Migration 0015_mcp_servers.sql

**Gaps:**
- No test coverage for MCP client
- Built-in tools have placeholder implementations
- No integration test with real MCP server

---

### Wave 4: Integration Testing

#### Task 4.1: End-to-End Relay Flow Test
**Priority:** P1
**Type:** Integration
**Estimate:** 15 min

**Prerequisites:**
- Database running with migrations applied
- OpenAI API key configured
- Server running on port 8080

**Test Script:**
```bash
#!/bin/bash
# e2e-relay-test.sh

# 1. Health check
curl -s http://localhost:8080/healthz | jq .

# 2. Models endpoint
curl -s http://localhost:8080/v1/models | jq .

# 3. Chat completion
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello"}]
  }' | jq .
```

**Success Criteria:**
- [ ] Health check returns 200
- [ ] Models endpoint returns list
- [ ] Chat completion returns response
- [ ] Usage recorded in database

---

#### Task 4.2: Agent Conversation Flow Test
**Priority:** P1
**Type:** Integration
**Estimate:** 15 min

**Test Script:**
```bash
#!/bin/bash
# e2e-agent-test.sh

# Requires authenticated session
TOKEN="your-session-token"

# 1. Create agent
AGENT_ID=$(curl -s http://localhost:8080/api/v1/app/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Agent", "model": "gpt-4o-mini"}' | jq -r .id)

# 2. Create conversation
CONV_ID=$(curl -s http://localhost:8080/api/v1/app/agents/$AGENT_ID/conversations \
  -H "Authorization: Bearer $TOKEN" \
  -X POST | jq -r .id)

# 3. Send message
curl -s http://localhost:8080/api/v1/app/agents/conversations/$CONV_ID/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello, agent!"}' | jq .
```

**Success Criteria:**
- [ ] Agent created successfully
- [ ] Conversation created
- [ ] Message sent and response received
- [ ] Messages persisted in database

---

#### Task 4.3: MCP Tool Execution Test
**Priority:** P1
**Type:** Integration
**Estimate:** 10 min

**Test Script:**
```bash
#!/bin/bash
# e2e-mcp-test.sh

TOKEN="your-session-token"

# 1. Test built-in datetime tool
curl -s http://localhost:8080/api/v1/app/agents/$AGENT_ID/tools/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tool": "datetime", "args": {}}' | jq .

# 2. Test calculator tool
curl -s http://localhost:8080/api/v1/app/agents/$AGENT_ID/tools/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tool": "calculator", "args": {"expression": "2+2"}}' | jq .
```

**Success Criteria:**
- [ ] Datetime tool returns current time
- [ ] Calculator tool executes (placeholder result acceptable)
- [ ] Error handling works for invalid tools

---

## Gap Summary

### Missing Tests (High Priority)
| Module | File | Priority |
|--------|------|----------|
| agent | service_test.go | P0 |
| agent | store_test.go | P0 |
| mcp | client_test.go | P0 |
| mcp | builtin_test.go | P1 |
| memory | embedder_test.go | P1 |

### Incomplete Implementations
| Component | Issue | Priority |
|-----------|-------|----------|
| web_search tool | Placeholder implementation | P2 |
| calculator tool | Placeholder implementation | P2 |
| Token usage recording | Needs verification | P1 |
| Frontend Agent pages | Status unknown | P1 |

### Integration Tests Needed
| Test | Dependencies | Priority |
|------|--------------|----------|
| E2E Relay flow | Running server + API key | P1 |
| E2E Agent conversation | Auth + Relay | P1 |
| E2E MCP execution | Auth + Agent | P2 |

---

## Success Criteria Summary

### M1.1 Relay Integration
- [ ] Build passes
- [ ] Migrations applied
- [ ] Relay mounted in server.go
- [ ] /v1/* routes to Relay
- [ ] Channels load from database
- [ ] Default channel created when needed

### M1.2 Chat via Relay
- [ ] RelayGateway implements ChatGateway
- [ ] CompositeGateway provides fallback
- [ ] Streaming SSE supported
- [ ] Token usage recorded
- [ ] Config toggle works

### M1.3 Agent Runtime
- [ ] Agent CRUD operations work
- [ ] Conversations created and persisted
- [ ] Messages sent through Relay
- [ ] Stream support functional
- [ ] Memory injection works
- [ ] MCP integration works

### M1.4 MCP Client
- [ ] MCP servers managed in database
- [ ] Connection/disconnection works
- [ ] Tool discovery functional
- [ ] Tool invocation works
- [ ] Built-in tools available

---

## Execution Order

```
Wave 1: Build Verification
├── Task 1.1: Go build
├── Task 1.2: Migrations check
└── Task 1.3: Frontend build

Wave 2: Test Execution
├── Task 2.1: Run existing tests
└── Task 2.2: Generate test stubs

Wave 3: Functional Verification
├── Task 3.1: M1.1 Relay verification
├── Task 3.2: M1.2 Chat verification
├── Task 3.3: M1.3 Agent verification
└── Task 3.4: M1.4 MCP verification

Wave 4: Integration Testing (Optional)
├── Task 4.1: E2E Relay test
├── Task 4.2: E2E Agent test
└── Task 4.3: E2E MCP test
```

---

## Next Steps After Verification

1. **If all checks pass:**
   - Mark Phase 1 as COMPLETE
   - Update REQUIREMENTS.md status
   - Proceed to Phase 2 planning

2. **If gaps found:**
   - Create gap closure tasks
   - Execute fixes
   - Re-run verification

3. **Test coverage:**
   - Implement missing test files
   - Target: 70%+ coverage for agent/mcp modules

---

## Appendix: File Reference

### Core Implementation Files
```
src/server/internal/http/server.go         # Relay mounting
src/server/internal/http/router.go         # Service assembly
src/server/internal/relay/relay.go         # Relay core
src/server/internal/relay/store.go         # DB persistence
src/server/internal/chat/relay_gateway.go  # Chat via Relay
src/server/internal/agent/service.go       # Agent service
src/server/internal/mcp/client.go          # MCP client
src/server/internal/mcp/builtin.go         # Built-in tools
src/server/internal/memory/embedder.go     # Relay embedder
src/server/internal/config/config.go       # Configuration
```

### Migration Files
```
src/server/migrations/0013_channels.sql     # RELAY-03, RELAY-04
src/server/migrations/0014_agents.sql       # AGENT-01~03
src/server/migrations/0015_mcp_servers.sql  # MCP-01
```

### Test Files (Existing)
```
src/server/internal/relay/router_test.go
src/server/internal/relay/billing_test.go
src/server/internal/chat/gateway_test.go
src/server/internal/chat/relay_gateway_test.go
src/server/internal/config/config_test.go
```

### Test Files (Missing - Need Creation)
```
src/server/internal/agent/service_test.go
src/server/internal/agent/store_test.go
src/server/internal/mcp/client_test.go
src/server/internal/mcp/builtin_test.go
src/server/internal/memory/embedder_test.go
```

---

*Plan created: 2026-04-27*
*Phase: 01-relay-integration*
*Type: Verification*

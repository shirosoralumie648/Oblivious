# Current System Contracts

Date: 2026-05-29

This file is the current v08 Product Completeness contract baseline for `src/server` and `src/web`. It records implemented behavior and the commercial boundaries that public docs, API docs, release runbooks, and tests must keep aligned.

- Mainline delivery scope: `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`
- Reference repositories: `lobehub/`, `new-api/`
- Commercial program design: `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- Commercial gate contract: `docs/release/commercial-gates.md`
- Product docs: `docs/product/public-overview.md`, `docs/product/onboarding.md`, `docs/product/pricing.md`, `docs/product/operator-guide.md`

## 1. Scope

Oblivious is now documented as a multi-tenant AI SaaS platform that integrates LobeHub-style C-end experience with New-API-style B-end operations. The current mainline includes Auth, tenant membership, Chat, Agent, Memory, MCP, Knowledge RAG, Notification, Quota, Console, Admin, Marketplace, billing, Stripe webhook ledger, and Relay `/v1/*` surfaces.

This file records current code contracts only. Phase 29 documentation alignment closes `PROD-05`; Phase 30 still owns end-to-end commercial journey proof and `AUDIT-01`.

`no-final-readiness`: this contract does not claim final commercial readiness.

## 2. Mainline Boundaries

```text
Browser
  -> src/web (React + React Router + Vite)
  -> /api/*
  -> src/server (Go net/http + PostgreSQL)
  -> PostgreSQL + pgvector
```

Boundary notes:

- `src/web` is the only mainline frontend.
- `src/server` is the only mainline backend.
- `config`, `scripts`, Docker assets, Kubernetes manifests, and `.github/workflows` are part of the execution baseline.
- `new-api/` and `lobehub/` are repository-local reference code and are excluded from root workspace, root CI, and release scope.
- Provider-facing AI calls must go through Relay. Production app services must not call upstream provider SDKs or provider URLs directly.

## 3. HTTP Envelope

The app API returns a JSON envelope for normal success and error responses.

Success:

```json
{
  "ok": true,
  "data": {},
  "error": null
}
```

Failure:

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "invalid_request",
    "message": "invalid json body"
  }
}
```

Common app API error codes include `invalid_request`, `invalid_credentials`, `unauthorized`, `method_not_allowed`, `not_found`, and `internal_error`.

Relay `/v1/*` handlers return OpenAI-compatible response shapes or OpenAI-style `error` objects depending on the handler and upstream result.

## 4. Auth, Session, And Tenant Contract

### 4.1 Frontend Auth State

The frontend auth state machine is:

- `idle`
- `authenticated`
- `unauthenticated`

`AuthStore`, `useAuthBootstrap`, `ProtectedRoute`, and `useAppContext` consume this state.

### 4.2 Session Cookie

Server sessions use an HttpOnly cookie from `auth_middleware.go`:

- cookie name: `SESSION_COOKIE_NAME`, default `oblivious_session`
- path: `/`
- `HttpOnly: true`
- `SameSite: Lax`
- `Secure: SESSION_COOKIE_SECURE`
- cookie value: signed session token

### 4.3 Tenant Boundary

Organizations are first-class tenants. Tenant-scoped domains carry organization identity, and representative cross-tenant tests deny reads and writes for Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data.

## 5. Backend Route Matrix

### 5.1 Public

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Health check |
| `GET` | `/metrics` | Prometheus metrics |
| `POST` | `/api/v1/auth/register` | Register and establish a session |
| `POST` | `/api/v1/auth/login` | Login and establish a session |

### 5.2 Auth

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/auth/me` | Return current user, session, workspace, organization context, and preferences |
| `POST` | `/api/v1/auth/logout` | Clear the current session |

### 5.3 Preferences And Models

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/me/preferences` | Read current user preferences |
| `PUT` | `/api/v1/app/me/preferences` | Update current user preferences |
| `GET` | `/api/v1/app/models` | Return available app models |

### 5.4 Chat

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/conversations` | List conversations |
| `POST` | `/api/v1/app/conversations` | Create a conversation |
| `GET` | `/api/v1/app/conversations/{conversationId}/messages` | List messages |
| `POST` | `/api/v1/app/conversations/{conversationId}/messages` | Send a message through Relay-backed generation |
| `GET` | `/api/v1/app/conversations/{conversationId}/config` | Read conversation configuration |
| `PUT` | `/api/v1/app/conversations/{conversationId}/config` | Update conversation configuration |
| `POST` | `/api/v1/app/conversations/{conversationId}/convert-to-task` | Convert a conversation into a SOLO task draft |

### 5.5 Knowledge

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/knowledge-bases` | List knowledge bases |
| `POST` | `/api/v1/app/knowledge-bases` | Create a knowledge base |
| `GET` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | Get a knowledge base |
| `PUT` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | Update a knowledge base |
| `DELETE` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}` | Delete a knowledge base |
| `GET` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents` | List documents |
| `POST` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents` | Create a document and index chunks |
| `PUT` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}` | Update a document and reindex chunks |
| `DELETE` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}` | Delete a document |
| `POST` | `/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve` | Retrieve source-cited RAG chunks |

Knowledge document create and update paths index chunks with Relay embeddings. Retrieval embeds the query through Relay `/v1/embeddings`, searches `knowledge_document_chunks.embedding` with pgvector under organization scope, and returns `embedding_rag` results with source citations.

### 5.6 SOLO Tasks And Agent Workflows

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/tasks` | List SOLO tasks |
| `POST` | `/api/v1/app/tasks` | Create a SOLO task |
| `GET` | `/api/v1/app/tasks/{taskId}` | Get a task |
| `POST` | `/api/v1/app/tasks/{taskId}/start` | Start a task |
| `POST` | `/api/v1/app/tasks/{taskId}/approve` | Approve a task |
| `POST` | `/api/v1/app/tasks/{taskId}/pause` | Pause a task |
| `POST` | `/api/v1/app/tasks/{taskId}/resume` | Resume a task |
| `POST` | `/api/v1/app/tasks/{taskId}/cancel` | Cancel a task |
| `POST` | `/api/v1/app/tasks/{taskId}/budget` | Update task budget |

Agent workflows persist durable `agent_runs` and `agent_tool_runs`. Approval-required tools pause before execution, rejection records a reason, failed tool executions preserve error evidence, and retry transitions are tenant-scoped.

### 5.7 Agent, Memory, MCP, Notification, And Quota

| Area | Method | Path | Purpose |
| --- | --- | --- | --- |
| Agent | `GET/POST` | `/api/v1/app/agents` | List or create agents |
| Agent | `GET/PUT/DELETE` | `/api/v1/app/agents/{agentId}` | Read, update, or delete an agent |
| Agent | `GET/POST` | `/api/v1/app/agents/{agentId}/conversations` | List or create agent conversations |
| Agent | `GET` | `/api/v1/app/agents/{agentId}/tools` | List available agent tools |
| Agent | `GET/DELETE` | `/api/v1/app/agents/conversations/{conversationId}` | Read or delete an agent conversation |
| Agent | `GET/POST` | `/api/v1/app/agents/conversations/{conversationId}/messages` | List or send agent conversation messages |
| Agent | `GET` | `/api/v1/app/agents/conversations/{conversationId}/runs` | List durable Agent runs |
| Agent | `GET` | `/api/v1/app/agents/runs/{runId}` | Get durable Agent run detail |
| Agent | `POST` | `/api/v1/app/agents/tool-runs/{toolRunId}/approve` | Approve a pending tool run |
| Agent | `POST` | `/api/v1/app/agents/tool-runs/{toolRunId}/reject` | Reject a pending tool run |
| Agent | `POST` | `/api/v1/app/agents/tool-runs/{toolRunId}/retry` | Retry a failed tool run |
| Memory | `GET/POST` | `/api/v1/app/memory/documents` | List or add memory documents |
| Memory | `GET/PUT/DELETE` | `/api/v1/app/memory/documents/{documentId}` | Read, update, or delete memory documents |
| Memory | `GET` | `/api/v1/app/memory/documents/{documentId}/chunks` | List chunks |
| Memory | `POST` | `/api/v1/app/memory/search` | Tenant-scoped memory search |
| MCP | `GET/POST` | `/api/v1/app/mcp-servers` | List or add MCP servers |
| MCP | `GET/DELETE` | `/api/v1/app/mcp-servers/{serverId}` | Read or delete MCP servers |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/connect` | Connect an MCP server |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/disconnect` | Disconnect an MCP server |
| MCP | `GET` | `/api/v1/app/mcp-servers/{serverId}/tools` | List MCP tools |
| MCP | `GET` | `/api/v1/app/mcp-servers/{serverId}/status` | Read MCP status |
| MCP | `POST` | `/api/v1/app/mcp-servers/{serverId}/execute` | Execute an MCP tool |
| Notification | `GET/POST` | `/api/v1/app/notifications` | List or create notifications |
| Notification | `GET` | `/api/v1/app/notifications/unread-count` | Count unread notifications |
| Notification | `POST` | `/api/v1/app/notifications/mark-all-read` | Mark all notifications read |
| Notification | `PATCH/DELETE` | `/api/v1/app/notifications/{notificationId}` | Mark read or delete |
| Quota | `GET` | `/api/v1/app/quota` | Read quota balance and usage |
| Quota | `GET` | `/api/v1/app/packages` | List quota packages |
| Quota | `POST` | `/api/v1/app/quota/topup` | Top up quota |

### 5.8 Admin

Admin API routes require an authenticated admin session.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/stats` | System statistics |
| `GET/POST` | `/api/v1/admin/channels` | List or create channels |
| `POST` | `/api/v1/admin/channels/batch` | Batch update channels |
| `GET/PUT/DELETE` | `/api/v1/admin/channels/{channelId}` | Read, update, or delete channels |
| `POST` | `/api/v1/admin/channels/{channelId}/test` | Test a channel |
| `GET` | `/api/v1/admin/channels/{channelId}/health` | Read channel health |
| `GET/POST` | `/api/v1/admin/routes` | List or create model routes |
| `GET/PUT/DELETE` | `/api/v1/admin/routes/{routeId}` | Read, update, or delete model routes |
| `GET/POST` | `/api/v1/admin/plans` | List or create plans |
| `GET/PUT/DELETE` | `/api/v1/admin/plans/{planId}` | Read, update, or deactivate plans |
| `GET` | `/api/v1/admin/billing/summary` | Billing summary |
| `GET` | `/api/v1/admin/billing/sessions` | Relay billing sessions |
| `GET` | `/api/v1/admin/billing/payment-intents` | Payment intent records |
| `GET` | `/api/v1/admin/billing/webhook-events` | Stripe webhook ledger events |
| `GET` | `/api/v1/admin/billing/subscriptions` | Subscription lifecycle state |
| `GET` | `/api/v1/admin/billing/topups` | Top-up order state |
| `GET` | `/api/v1/admin/billing/invoices` | Invoice state |
| `GET` | `/api/v1/admin/billing/refunds` | Refund state |
| `GET` | `/api/v1/admin/billing/settlements` | Marketplace settlement state |
| `GET` | `/api/v1/admin/billing/payouts` | Marketplace payout state |
| `GET` | `/api/v1/admin/users` | List users |
| `GET/PUT/PATCH/DELETE` | `/api/v1/admin/users/{userId}` | Read, update, adjust quota, or delete users |
| `POST` | `/api/v1/admin/users/{userId}/disable` | Disable a user |
| `POST` | `/api/v1/admin/users/{userId}/enable` | Enable a user |
| `GET` | `/api/v1/admin/audit-logs` | Audit log entries |
| `GET` | `/api/v1/admin/reviews` | Pending Marketplace reviews |
| `POST` | `/api/v1/admin/reviews/{agentId}/approve` | Approve an agent |
| `POST` | `/api/v1/admin/reviews/{agentId}/reject` | Reject an agent |

### 5.9 Marketplace

Discovery endpoints are public. Publisher, install, review submission, owner-specific, and stats endpoints require an authenticated session.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/marketplace/featured` | Featured agents |
| `GET` | `/api/v1/marketplace/curated` | Curated sections |
| `GET` | `/api/v1/marketplace/categories` | Categories |
| `GET` | `/api/v1/marketplace/search` | Search agents |
| `GET/POST` | `/api/v1/marketplace/agents` | List or publish agents |
| `GET` | `/api/v1/marketplace/my-agents` | Current user's published agents |
| `GET` | `/api/v1/marketplace/installs` | Current user's installed agents |
| `DELETE` | `/api/v1/marketplace/installs/{agentId}` | Uninstall an agent |
| `GET` | `/api/v1/marketplace/publisher/stats` | Publisher statistics |
| `GET/PUT/DELETE` | `/api/v1/marketplace/agents/{agentId}` | Read, update, or delete agents |
| `POST/DELETE` | `/api/v1/marketplace/agents/{agentId}/install` | Install or uninstall an agent |
| `GET/POST` | `/api/v1/marketplace/agents/{agentId}/reviews` | List or submit reviews |
| `GET` | `/api/v1/marketplace/agents/{agentId}/versions` | Agent versions |
| `GET` | `/api/v1/marketplace/agents/{agentId}/stats` | Agent statistics |

### 5.10 WebSocket And Relay

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/ws` | Authenticated real-time WebSocket |
| `POST` | `/v1/chat/completions` | Relay Chat Completions |
| `POST` | `/v1/responses` | Relay Responses API |
| `POST` | `/v1/embeddings` | Relay Embeddings |
| `POST` | `/v1/images/generations` | Relay image generation |
| `POST` | `/v1/audio/speech` | Relay audio speech |
| `POST` | `/v1/moderations` | Relay moderation |
| `POST` | `/v1/completions` | Relay legacy completions |

Relay also registers files, fine-tuning, assistants, threads, runs, batch, audio transcription/translation, image edit/variation, and realtime routes. The complete route index and commercial route classes live in `docs/API.md` and `docs/release/relay-route-table.md`.

## 6. Frontend Route Matrix

| Area | Path | Status |
| --- | --- | --- |
| Marketing | `/` | Mounted |
| Marketing | `/login` | Mounted |
| Marketing | `/register` | Mounted |
| Workspace | `/onboarding` | Mounted first-run onboarding route |
| Workspace | `/chat` | Mounted default workspace entry |
| Workspace | `/chat/:conversationId` | Mounted conversation detail |
| Workspace | `/knowledge` | Mounted Knowledge list/create route |
| Workspace | `/knowledge/:knowledgeBaseId` | Mounted Knowledge document and RAG retrieval route |
| Workspace | `/solo` | Mounted Agent/SOLO route |
| Workspace | `/solo/new` | Mounted task creation route |
| Workspace | `/marketplace` | Mounted Marketplace browse/search route |
| Workspace | `/marketplace/agents/:agentId` | Mounted agent detail/install/review route |
| Workspace | `/marketplace/publish` | Mounted publisher route |
| Workspace | `/marketplace/my-agents` | Mounted publisher management route |
| Workspace | `/settings` | Mounted preferences route |
| Console | `/console` | Mounted operations overview |
| Console | `/console/models` | Mounted model drill-down |
| Console | `/console/usage` | Mounted usage drill-down |
| Console | `/console/billing` | Mounted billing drill-down |
| Console | `/console/access` | Mounted access drill-down |
| Admin | `/admin` | Mounted Admin dashboard |
| Admin | `/admin/channels` | Mounted channel management |
| Admin | `/admin/routes` | Mounted model route management |
| Admin | `/admin/plans` | Mounted plan management |
| Admin | `/admin/billing` | Mounted billing inspection |
| Admin | `/admin/users` | Mounted user management |
| Admin | `/admin/audit-log` | Mounted audit log |
| Admin | `/admin/reviews` | Mounted Marketplace review queue |

### Existing Pages Not Mounted

| Planned Path | Current State |
| --- | --- |
| none | No existing mainline route pages are intentionally left unmounted. |

## 7. Commercial Billing Contract

The billing system records:

- Relay billing sessions.
- Quota preauthorization, settlement, and refund.
- Payment intents.
- Stripe webhook ledger events.
- Subscription lifecycle events.
- Top-up orders.
- Invoices.
- Refunds.
- Marketplace orders, settlements, platform fee, payout state, and refund impact.

Admin Billing is read-only inspection for these records.

## 8. Operations Contract

| Gate | Command | Notes |
| --- | --- | --- |
| Docs and release assets | `bash scripts/check.sh docs` | Verifies docs, contracts, release assets, env contract, and workspace boundaries |
| Web build | `bash scripts/check.sh web` | Runs `pnpm --dir src/web build` |
| Server release checks | `bash scripts/check.sh server` | Runs `go test ./... -count=1` |
| Web tests | `bash scripts/test.sh web` | Vitest suite |
| Server tests | `bash scripts/test.sh server` | Server unit tests; local integration tests skip explicitly without `TEST_DATABASE_URL`; CI uses required DB mode |
| Browser E2E | `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` | Admin and Marketplace Playwright gate |
| Deployment validation | `bash scripts/deploy-validate.sh` | Builds, starts, migrates, and smokes app/Relay paths |
| Backup/restore smoke | `bash scripts/backup-restore-smoke.sh` | Proves PostgreSQL tenant-commercial data recovery and migration ledger integrity |
| Kubernetes recovery policy | `bash scripts/verify-k8s-recovery-policy.sh` | Verifies restart probes and HPA recovery policy encoded in repository manifests |

Release and rollback use `docs/release/release-rollback-runbook.md`. Backup and restore use `docs/release/backup-restore-runbook.md`. Incident and disaster recovery use `docs/release/incident-response-runbook.md` and `docs/release/disaster-recovery-runbook.md`.

Functional Logic 9.3 infrastructure failover uses `docs/release/recovery-platform-contract.md`: the repository owns app recovery policy, HPA manifest validation, and recovery action evidence; production PostgreSQL Patroni or managed failover, Redis Sentinel or managed failover, Kafka leader election, load-balancer target removal/rejoin, and any literal `<30%` custom autoscaler trigger are deployment-platform evidence.

## 9. Environment Variable Matrix

### 9.1 Frontend Local Development

| Name | Required | Default | Current Use |
| --- | --- | --- | --- |
| `WEB_PORT` | No | `5173` | Frontend local port |
| `WEB_API_BASE_URL` | No | `http://localhost:8080` | Frontend backend base URL |

### 9.2 Backend Runtime

| Name | Required | Default | Status |
| --- | --- | --- |
| `SERVER_PORT` | No | `8080` | Consumed |
| `APP_ENV` | No | `development` | Consumed |
| `CORS_ALLOWED_ORIGINS` | No | empty | Consumed by HTTP middleware |
| `DATABASE_URL` | Yes | none | Consumed |
| `SESSION_SECRET` | Yes | none | Consumed for HMAC session cookie signing |
| `SESSION_COOKIE_NAME` | No | `oblivious_session` | Consumed |
| `SESSION_COOKIE_SECURE` | No | `false` | Consumed |
| `LLM_BASE_URL` | No | empty | Consumed by non-commercial local reply configuration |
| `LLM_API_KEY` | No | empty | Consumed by non-commercial local reply configuration |
| `LLM_TIMEOUT_MS` | No | `30000` | Consumed |
| `MODEL_DEFAULT_NAME` | No | `demo-reply` | Consumed |
| `RELAY_ENABLED` | No | `true` | Controls Relay mounting |
| `RELAY_DEFAULT_MODEL` | No | `gpt-4o-mini` | Relay default model |
| `RAG_RERANKER_BASE_URL` | No | empty | Enables Cohere-compatible `/rerank` calls for Knowledge `hybrid_rerank` |
| `RAG_RERANKER_API_KEY` | No | empty | Bearer token for the RAG reranker service |
| `RAG_RERANKER_MODEL` | No | `bge-reranker-large` | RAG reranker model name |
| `RAG_RERANKER_TOP_K` | No | `5` | RAG reranker candidate count; invalid values fail startup config load |
| `OPENAI_API_KEY` | No | empty | Development default channel key |
| `OPENAI_BASE_URL` | No | `https://api.openai.com` | Development default channel base URL |

### 9.3 Backend Test Runtime

| Name | Required | Default | Status |
| --- | --- | --- |
| `TEST_DATABASE_URL` | CI yes; local no | empty | HTTP integration tests use it; local absence skips integration group explicitly |
| `OBLIVIOUS_REQUIRE_TEST_DATABASE` | CI yes; local no | `false` | When true, missing `TEST_DATABASE_URL` fails `scripts/test.sh server` |

## 10. Change Control Rules

1. Backend API shape changes must update this file, `docs/API.md`, and frontend types where applicable.
2. New frontend workspace routes must update the route matrix.
3. Environment variable changes must update `config/.env.example`, `src/server/internal/config/config.go`, and this file.
4. Commercial docs must not claim behavior that lacks current repository evidence.
5. AI provider access changes must preserve the Relay-only invariant.

## 11. Non-Goals For This Document

- Production price amounts.
- Live provider, Stripe, payout, kubeconfig, or observability-vendor secrets.
- Phase 30 end-to-end journey evidence.
- Final commercial completion audit.

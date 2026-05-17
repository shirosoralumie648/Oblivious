# Oblivious API Documentation

This document lists the current routed HTTP surface for the v03.3 consolidated mainline.
Routes are reconciled against `src/server/internal/http/router.go`, `src/server/internal/http/server.go`, and `src/server/internal/relay/handler/router.go`.

## Base URLs

| Surface | Base URL | Source |
| --- | --- | --- |
| App API | `http://localhost:8080/api/v1` | `src/server/internal/http/router.go` |
| Relay API | `http://localhost:8080/v1` | `src/server/internal/relay/handler/router.go` mounted by `src/server/internal/http/server.go` |
| Health and metrics | `http://localhost:8080` | `src/server/internal/http/router.go` |

`/v1/*` Relay routes are available only when `RELAY_ENABLED=true`.

## Response Envelope

The app API returns a JSON envelope for normal success and error responses.

Success:

```json
{
  "ok": true,
  "data": {},
  "error": null
}
```

Error:

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

## Authentication

Authenticated app endpoints require the `oblivious_session` cookie, or the value configured by `SESSION_COOKIE_NAME`.
Admin endpoints additionally require an admin role through the server-side session.

Public app routes:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Main app health check |
| `GET` | `/metrics` | Prometheus metrics |
| `POST` | `/api/v1/auth/login` | Login and establish a session |
| `POST` | `/api/v1/auth/register` | Register and establish a session |

## Auth Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | Login with email and password |
| `POST` | `/api/v1/auth/register` | Register a user |
| `GET` | `/api/v1/auth/me` | Return current user, session, workspace, and preferences |
| `POST` | `/api/v1/auth/logout` | Clear the current session |

## Chat Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/models` | List available chat models |
| `GET` | `/api/v1/app/conversations` | List conversations |
| `POST` | `/api/v1/app/conversations` | Create a conversation |
| `GET` | `/api/v1/app/conversations/:conversationId/messages` | List conversation messages |
| `POST` | `/api/v1/app/conversations/:conversationId/messages` | Send a message |
| `GET` | `/api/v1/app/conversations/:conversationId/config` | Read conversation configuration |
| `PUT` | `/api/v1/app/conversations/:conversationId/config` | Update conversation configuration |
| `POST` | `/api/v1/app/conversations/:conversationId/convert-to-task` | Convert a conversation into a SOLO task draft |

## Agent Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/agents` | List agents |
| `POST` | `/api/v1/app/agents` | Create an agent |
| `GET` | `/api/v1/app/agents/:agentId` | Get an agent |
| `PUT` | `/api/v1/app/agents/:agentId` | Update an agent |
| `DELETE` | `/api/v1/app/agents/:agentId` | Delete an agent |
| `GET` | `/api/v1/app/agents/:agentId/conversations` | List agent conversations |
| `POST` | `/api/v1/app/agents/:agentId/conversations` | Create an agent conversation |
| `GET` | `/api/v1/app/agents/:agentId/tools` | List tools available to an agent |
| `GET` | `/api/v1/app/agents/conversations/:conversationId` | Get an agent conversation |
| `DELETE` | `/api/v1/app/agents/conversations/:conversationId` | Delete an agent conversation |
| `GET` | `/api/v1/app/agents/conversations/:conversationId/messages` | List agent conversation messages |
| `POST` | `/api/v1/app/agents/conversations/:conversationId/messages` | Send an agent conversation message |

## Memory Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/memory/documents` | List memory documents |
| `POST` | `/api/v1/app/memory/documents` | Add a memory document |
| `GET` | `/api/v1/app/memory/documents/:documentId` | Get a memory document |
| `PUT` | `/api/v1/app/memory/documents/:documentId` | Update a memory document |
| `DELETE` | `/api/v1/app/memory/documents/:documentId` | Delete a memory document |
| `GET` | `/api/v1/app/memory/documents/:documentId/chunks` | List chunks for a memory document |
| `POST` | `/api/v1/app/memory/search` | Search memory documents |

## MCP Server Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/mcp-servers` | List MCP servers |
| `POST` | `/api/v1/app/mcp-servers` | Add an MCP server |
| `GET` | `/api/v1/app/mcp-servers/:serverId` | Get an MCP server |
| `DELETE` | `/api/v1/app/mcp-servers/:serverId` | Delete an MCP server |
| `POST` | `/api/v1/app/mcp-servers/:serverId/connect` | Connect to an MCP server |
| `POST` | `/api/v1/app/mcp-servers/:serverId/disconnect` | Disconnect from an MCP server |
| `GET` | `/api/v1/app/mcp-servers/:serverId/tools` | List tools exposed by an MCP server |
| `GET` | `/api/v1/app/mcp-servers/:serverId/status` | Read MCP server connection status |
| `POST` | `/api/v1/app/mcp-servers/:serverId/execute` | Execute an MCP tool |

## Notification Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/notifications` | List notifications |
| `POST` | `/api/v1/app/notifications` | Create a notification |
| `GET` | `/api/v1/app/notifications/unread-count` | Count unread notifications |
| `POST` | `/api/v1/app/notifications/mark-all-read` | Mark all notifications as read |
| `PATCH` | `/api/v1/app/notifications/:notificationId` | Mark one notification as read |
| `DELETE` | `/api/v1/app/notifications/:notificationId` | Delete a notification |

## Quota Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/quota` | Read current quota balance and usage |
| `GET` | `/api/v1/app/packages` | List quota packages |
| `POST` | `/api/v1/app/quota/topup` | Top up quota balance |

## Console Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/console/usage` | Usage summary |
| `GET` | `/api/v1/console/access` | Access and session context |
| `GET` | `/api/v1/console/models` | Model summary |
| `GET` | `/api/v1/console/billing` | Billing summary |

## Preferences Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/me/preferences` | Read current user preferences |
| `PUT` | `/api/v1/app/me/preferences` | Update current user preferences |

## Task Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/tasks` | List SOLO tasks |
| `POST` | `/api/v1/app/tasks` | Create a SOLO task |
| `GET` | `/api/v1/app/tasks/:taskId` | Get a task |
| `POST` | `/api/v1/app/tasks/:taskId/start` | Start a task |
| `POST` | `/api/v1/app/tasks/:taskId/approve` | Approve a task |
| `POST` | `/api/v1/app/tasks/:taskId/pause` | Pause a task |
| `POST` | `/api/v1/app/tasks/:taskId/resume` | Resume a task |
| `POST` | `/api/v1/app/tasks/:taskId/cancel` | Cancel a task |
| `POST` | `/api/v1/app/tasks/:taskId/budget` | Update task budget |

## Knowledge Base Endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/app/knowledge-bases` | List knowledge bases |
| `POST` | `/api/v1/app/knowledge-bases` | Create a knowledge base |
| `GET` | `/api/v1/app/knowledge-bases/:knowledgeBaseId` | Get a knowledge base |
| `PUT` | `/api/v1/app/knowledge-bases/:knowledgeBaseId` | Update a knowledge base |
| `DELETE` | `/api/v1/app/knowledge-bases/:knowledgeBaseId` | Delete a knowledge base |
| `GET` | `/api/v1/app/knowledge-bases/:knowledgeBaseId/documents` | List knowledge documents |
| `POST` | `/api/v1/app/knowledge-bases/:knowledgeBaseId/documents` | Create a knowledge document |
| `PUT` | `/api/v1/app/knowledge-bases/:knowledgeBaseId/documents/:documentId` | Update a knowledge document |
| `DELETE` | `/api/v1/app/knowledge-bases/:knowledgeBaseId/documents/:documentId` | Delete a knowledge document |
| `POST` | `/api/v1/app/knowledge-bases/:knowledgeBaseId/retrieve` | Retrieve relevant document chunks |

## WebSocket Endpoint

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/ws` | Authenticated WebSocket for real-time updates |

## Admin Endpoints

All admin endpoints require an authenticated admin session.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/stats` | System statistics |
| `GET` | `/api/v1/admin/channels` | List channels |
| `POST` | `/api/v1/admin/channels` | Create a channel |
| `POST` | `/api/v1/admin/channels/batch` | Batch update channels |
| `GET` | `/api/v1/admin/channels/:channelId` | Get a channel |
| `PUT` | `/api/v1/admin/channels/:channelId` | Update a channel |
| `DELETE` | `/api/v1/admin/channels/:channelId` | Delete a channel |
| `POST` | `/api/v1/admin/channels/:channelId/test` | Test a channel |
| `GET` | `/api/v1/admin/channels/:channelId/health` | Get channel health |
| `GET` | `/api/v1/admin/routes` | List model routes |
| `POST` | `/api/v1/admin/routes` | Create a model route |
| `GET` | `/api/v1/admin/routes/:routeId` | Get a model route |
| `PUT` | `/api/v1/admin/routes/:routeId` | Update a model route |
| `DELETE` | `/api/v1/admin/routes/:routeId` | Delete a model route |
| `GET` | `/api/v1/admin/plans` | List quota plans |
| `POST` | `/api/v1/admin/plans` | Create a quota plan |
| `GET` | `/api/v1/admin/plans/:planId` | Get a quota plan |
| `PUT` | `/api/v1/admin/plans/:planId` | Update a quota plan |
| `DELETE` | `/api/v1/admin/plans/:planId` | Deactivate a quota plan |
| `GET` | `/api/v1/admin/users` | List users |
| `GET` | `/api/v1/admin/users/:userId` | Get a user |
| `PUT` | `/api/v1/admin/users/:userId` | Update a user |
| `PATCH` | `/api/v1/admin/users/:userId` | Update user quota |
| `DELETE` | `/api/v1/admin/users/:userId` | Delete a user |
| `POST` | `/api/v1/admin/users/:userId/disable` | Disable a user |
| `POST` | `/api/v1/admin/users/:userId/enable` | Enable a user |
| `GET` | `/api/v1/admin/audit-logs` | List audit log entries |
| `GET` | `/api/v1/admin/reviews` | List pending marketplace reviews |
| `POST` | `/api/v1/admin/reviews/:agentId/approve` | Approve a marketplace agent |
| `POST` | `/api/v1/admin/reviews/:agentId/reject` | Reject a marketplace agent |

## Marketplace Endpoints

Public discovery endpoints do not require a session. Publisher, install, review submission, and owner-specific endpoints require an authenticated session.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/marketplace/featured` | List featured agents |
| `GET` | `/api/v1/marketplace/curated` | List curated marketplace sections |
| `GET` | `/api/v1/marketplace/categories` | List marketplace categories |
| `GET` | `/api/v1/marketplace/search` | Search marketplace agents |
| `GET` | `/api/v1/marketplace/agents` | List marketplace agents |
| `POST` | `/api/v1/marketplace/agents` | Publish an agent |
| `GET` | `/api/v1/marketplace/my-agents` | List agents owned by the current user |
| `GET` | `/api/v1/marketplace/installs` | List installed agents |
| `DELETE` | `/api/v1/marketplace/installs/:agentId` | Uninstall an agent |
| `GET` | `/api/v1/marketplace/publisher/stats` | Publisher statistics for the current user |
| `GET` | `/api/v1/marketplace/agents/:agentId` | Get marketplace agent details |
| `PUT` | `/api/v1/marketplace/agents/:agentId` | Update a published agent |
| `DELETE` | `/api/v1/marketplace/agents/:agentId` | Delete a published agent |
| `POST` | `/api/v1/marketplace/agents/:agentId/install` | Install an agent |
| `DELETE` | `/api/v1/marketplace/agents/:agentId/install` | Uninstall an agent |
| `GET` | `/api/v1/marketplace/agents/:agentId/reviews` | List reviews for an agent |
| `POST` | `/api/v1/marketplace/agents/:agentId/reviews` | Submit a review |
| `GET` | `/api/v1/marketplace/agents/:agentId/versions` | List agent versions |
| `GET` | `/api/v1/marketplace/agents/:agentId/stats` | Agent statistics for the publisher |

## Relay /v1 Endpoints

These routes are registered by `src/server/internal/relay/handler/router.go` and mounted under `/v1/*` by `src/server/internal/http/server.go`.

| Method | Path | Strategy |
| --- | --- | --- |
| `POST` | `/v1/chat/completions` | Native |
| `POST` | `/v1/responses` | Native |
| `GET` | `/v1/realtime` | Native stream |
| `POST` | `/v1/embeddings` | Native |
| `POST` | `/v1/images/generations` | Native |
| `POST` | `/v1/images/edits` | Native |
| `POST` | `/v1/images/variations` | Native |
| `POST` | `/v1/videos` | Native |
| `POST` | `/v1/audio/speech` | Native |
| `POST` | `/v1/audio/transcriptions` | Native |
| `POST` | `/v1/audio/translations` | Native |
| `POST` | `/v1/moderations` | Native |
| `POST` | `/v1/completions` | Native |
| `POST` | `/v1/batch` | Native |
| `GET` | `/v1/batches` | Passthrough |
| `GET` | `/v1/batches/:id` | Passthrough |
| `POST` | `/v1/files` | File proxy |
| `GET` | `/v1/files` | Passthrough |
| `GET` | `/v1/files/:id` | Passthrough |
| `DELETE` | `/v1/files/:id` | Passthrough |
| `GET` | `/v1/files/:id/content` | Passthrough |
| `POST` | `/v1/fine_tuning/jobs` | Passthrough |
| `GET` | `/v1/fine_tuning/jobs` | Passthrough |
| `GET` | `/v1/fine_tuning/jobs/:id` | Passthrough |
| `POST` | `/v1/fine_tuning/jobs/:id/cancel` | Passthrough |
| `GET` | `/v1/fine_tuning/jobs/:id/events` | Passthrough |
| `POST` | `/v1/assistants` | Passthrough |
| `GET` | `/v1/assistants` | Passthrough |
| `GET` | `/v1/assistants/:id` | Passthrough |
| `POST` | `/v1/threads` | Passthrough |
| `GET` | `/v1/threads/:id` | Passthrough |
| `POST` | `/v1/threads/:id/runs` | Passthrough |
| `GET` | `/v1/threads/:id/runs/:rid` | Passthrough |
| `POST` | `/v1/threads/:id/runs/:rid/submit` | Passthrough |

## Not Routed In This Release Candidate

`GET /v1/models` is used by the relay health checker against upstream providers, but it is not registered as an inbound Oblivious Relay endpoint in `src/server/internal/relay/handler/router.go`.

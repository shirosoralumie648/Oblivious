---
description: External integrations and dependencies
created: 2026-04-27
---
# Integrations

**Analysis Date:** 2026-04-27

## External Services

### LLM Providers

| Service | Purpose | Integration Type | Configuration |
|---------|---------|------------------|---------------|
| OpenAI | Primary LLM provider | HTTP API | `OPENAI_API_KEY`, `OPENAI_BASE_URL` |
| Custom LLM | Configurable LLM backend | HTTP API | `LLM_BASE_URL`, `LLM_API_KEY`, `MODEL_DEFAULT_NAME` |

**Key Files:**
- `src/server/internal/config/config.go` - Configuration loading
- `src/server/internal/relay/handler.go` - LLM relay implementation

### Database & Storage

| Service | Purpose | Integration Type | Configuration |
|---------|---------|------------------|---------------|
| PostgreSQL | Primary relational database | SQL driver (lib/pq) | `DATABASE_URL` |
| MongoDB | Knowledge base storage | Native driver | Configured via connection string |
| Redis | Task queue backend, caching | Redis client | Used by asynq for background jobs |

**Key Files:**
- `src/server/internal/db/db.go` - Database connection
- `src/server/go.mod` - MongoDB driver dependency

### Background Job Processing

| Service | Purpose | Integration Type | Configuration |
|---------|---------|------------------|---------------|
| Asynq | Task queue for background jobs | Go library with Redis backend | `REDIS_URL` (via asynq) |

**Key Files:**
- `src/server/internal/billing/worker.go` - Billing background worker
- `src/server/go.mod` - Asynq dependency

### Observability

| Service | Purpose | Integration Type | Configuration |
|---------|---------|------------------|---------------|
| Prometheus | Metrics collection | Client library | Metrics exposed at `/metrics` endpoint |

**Key Files:**
- `src/server/internal/http/server.go` - Metrics endpoint setup
- `src/server/go.mod` - Prometheus client dependency

## Internal Services/APIs

### HTTP API Structure

| Service | Protocol | Endpoint/Location | Purpose |
|---------|----------|-------------------|---------|
| Auth API | HTTP/REST | `/api/auth/*` | Authentication endpoints |
| Chat API | HTTP/REST | `/api/chat/*` | Chat management endpoints |
| Console API | HTTP/REST | `/api/console/*` | Console overview endpoints |
| Knowledge API | HTTP/REST | `/api/knowledge/*` | Knowledge base endpoints |
| Relay API | HTTP/REST | `/api/relay/*` | LLM relay endpoints |
| WebSocket | WebSocket (gorilla) | `/ws` | Real-time chat |
| Metrics | HTTP | `/metrics` | Prometheus metrics |

**Key Files:**
- `src/server/internal/http/auth_handler.go` - Auth endpoints
- `src/server/internal/http/chat_handler.go` - Chat endpoints
- `src/server/internal/http/console_handler.go` - Console endpoints
- `src/server/internal/http/knowledge_handler.go` - Knowledge endpoints
- `src/server/internal/http/relay_handler.go` - Relay endpoints
- `src/server/internal/chat/gateway.go` - WebSocket gateway

### Domain Services

| Service | Purpose | Key File |
|---------|---------|----------|
| Auth Service | Authentication, session management | `src/server/internal/auth/service.go` |
| Chat Service | Chat management, messaging | `src/server/internal/chat/service.go` |
| Console Service | Console overview data | `src/server/internal/console/service.go` |
| Knowledge Service | Knowledge base CRUD | `src/server/internal/knowledge/service.go` |
| Settings Service | User preferences | `src/server/internal/settings/service.go` |
| Relay Service | LLM proxy/relay | `src/server/internal/relay/handler.go` |
| Billing Service | Token counting, billing | `src/server/internal/billing/hook.go` |

## Authentication

| Provider | Flow | Scope | Configuration |
|----------|------|-------|---------------|
| Custom Session | Cookie-based session | Full application | `SESSION_SECRET`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE` |

**Key Files:**
- `src/server/internal/auth/service.go` - Auth service
- `src/server/internal/http/auth_middleware.go` - Auth middleware
- `src/server/internal/config/config.go` - Session configuration

## Data Stores

| Store | Purpose | Driver/Client | Connection |
|-------|---------|---------------|------------|
| PostgreSQL | Primary database (users, sessions, chats, knowledge) | `lib/pq` | `DATABASE_URL` |
| MongoDB | Knowledge base content storage | `mongo-driver/v2` | Via connection string |
| Redis | Background job queue (asynq), caching | `go-redis/v9` | Via asynq configuration |

**Key Files:**
- `src/server/internal/db/db.go` - Database connection management
- `src/server/internal/chat/store.go` - Chat data access
- `src/server/internal/auth/store.go` - Auth data access
- `src/server/internal/knowledge/store.go` - Knowledge data access

## API Contracts

### Key HTTP Endpoints

| Endpoint | Method | Purpose | Handler |
|----------|--------|---------|---------|
| `/api/auth/register` | POST | User registration | `auth_handler.go` |
| `/api/auth/login` | POST | User login | `auth_handler.go` |
| `/api/auth/logout` | POST | User logout | `auth_handler.go` |
| `/api/auth/me` | GET | Current user info | `auth_handler.go` |
| `/api/chat` | GET | List chats | `chat_handler.go` |
| `/api/chat` | POST | Create chat | `chat_handler.go` |
| `/api/chat/:id` | GET | Get chat | `chat_handler.go` |
| `/api/chat/:id` | DELETE | Delete chat | `chat_handler.go` |
| `/api/chat/:id/message` | POST | Send message | `chat_handler.go` |
| `/api/relay/chat/completions` | POST | LLM relay endpoint | `relay_handler.go` |
| `/api/knowledge` | GET | List knowledge bases | `knowledge_handler.go` |
| `/api/knowledge` | POST | Create knowledge base | `knowledge_handler.go` |
| `/api/knowledge/:id` | GET | Get knowledge base | `knowledge_handler.go` |
| `/api/knowledge/:id` | PUT | Update knowledge base | `knowledge_handler.go` |
| `/api/knowledge/:id` | DELETE | Delete knowledge base | `knowledge_handler.go` |
| `/api/console/overview` | GET | Console overview data | `console_handler.go` |
| `/ws` | WebSocket | Real-time chat | `chat/gateway.go` |
| `/metrics` | GET | Prometheus metrics | `http/server.go` |
| `/health` | GET | Health check | `http/server.go` |

### WebSocket Events

| Event | Direction | Purpose |
|-------|-----------|---------|
| `message` | Server -> Client | New chat message |
| `typing` | Bidirectional | Typing indicators |

## Third-Party SDKs

| SDK/Package | Purpose | Version |
|-------------|---------|---------|
| `github.com/gin-gonic/gin` | HTTP web framework | v1.12.0 |
| `github.com/gorilla/websocket` | WebSocket implementation | v1.5.3 |
| `github.com/lib/pq` | PostgreSQL driver | v1.10.9 |
| `github.com/hibiken/asynq` | Background task processing | v0.26.0 |
| `github.com/redis/go-redis/v9` | Redis client | v9.14.1 |
| `github.com/prometheus/client_golang` | Metrics collection | v1.23.2 |
| `go.mongodb.org/mongo-driver/v2` | MongoDB driver | v2.5.0 |
| `github.com/pkoukk/tiktoken-go` | OpenAI token counting | v0.1.8 |
| `golang.org/x/crypto` | Cryptographic functions | v0.48.0 |
| `github.com/google/uuid` | UUID generation | v1.6.0 |

## Integration Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Clients                                   │
│  (React Web App)                                                     │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ HTTP / WebSocket
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Go API Server (Gin)                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │  Auth    │ │  Chat    │ │Knowledge│ │  Relay  │ │ Console │   │
│  │  HTTP    │ │  HTTP/WS │ │  HTTP   │ │  HTTP   │ │  HTTP   │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└──────────────────────────┬──────────────────────────────────────────┘
                           │ SQL / NoSQL / Queue
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Data & Message Layer                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  PostgreSQL  │  │   MongoDB    │  │    Redis     │               │
│  │ Primary DB   │  │  Knowledge   │  │   Asynq      │               │
│  │(users,chats) │  │   Store      │  │   Queue      │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└─────────────────────────────────────────────────────────────────────┘
                           │ HTTP
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      External Services                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │   OpenAI     │  │  Custom LLM  │  │ Prometheus   │               │
│  │   API        │  │    API       │  │  Scraping     │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
└─────────────────────────────────────────────────────────────────────┘
```

---

*Integration audit: 2026-04-27*

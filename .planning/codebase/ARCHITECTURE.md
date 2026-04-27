# Architecture

**Analysis Date:** 2026-04-27

## System Overview

Oblivious is a multi-tenant AI platform with a Go-based backend and React-based frontend. The architecture follows a layered design with clear separation between API handlers, business services, and data stores. The system supports chat, agent orchestration, knowledge bases, and multi-channel LLM relay.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client Layer                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │   Web App    │  │  Admin UI    │  │   Mobile     │  │   CLI/API    │    │
│  │  (React)     │  │  (LobeHub)   │  │  (Future)    │  │   Clients    │    │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘    │
└─────────┼─────────────────┼─────────────────┼─────────────────┼────────────┘
          │                 │                 │                 │
          └─────────────────┴─────────────────┴─────────────────┘
                                    │
                              HTTP/WebSocket
                                    │
┌─────────────────────────────────────────────────────────────────────────────┐
│                             API Gateway Layer                                │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │                    HTTP Router (`internal/http`)                     │   │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐  │   │
│  │  │ Auth MW     │ │ CORS        │ │ Request ID  │ │ Recovery    │  │   │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘  │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                              ┌─────────────┐                                │
│                              │   Relay     │  ← OpenAI-compatible API      │
│                              │  (`/v1/*`)  │    with load balancing        │
│                              └─────────────┘                                │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Service Layer (Domain)                           │
│                                                                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │   Auth      │ │    Chat     │ │   Agent     │ │  Knowledge  │           │
│  │  Service    │ │  Service    │ │  Service    │ │   Service   │           │
│  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘           │
│         │               │               │               │                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │   Task      │ │   Memory    │ │    MCP      │ │   Console   │           │
│  │  Service    │ │  Service    │ │  Client     │ │  Service    │           │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Data Layer (Repository)                          │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                     PostgreSQL (`internal/db`)                        │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │  │
│  │  │   Users     │ │Conversations│ │   Agents    │ │   Tasks     │    │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘    │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │  │
│  │  │  Knowledge  │ │   Memory    │ │    MCP      │ │   Usage     │    │  │
│  │  │   Bases     │ │  Documents  │ │   Servers   │ │   Records   │    │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘    │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │               External Services (via `internal/relay`)                │
│  │                                                                     │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐   │  │
│  │  │  OpenAI     │ │  Anthropic  │ │   Google    │ │   Azure     │   │  │
│  │  │   API       │ │  Claude     │ │   Gemini    │ │  OpenAI     │   │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘   │  │
│  │                                                                     │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                     │  │
│  │  │  Mistral    │ │   Groq      │ │   Cohere    │  (Future providers) │  │
│  │  │    AI       │ │   API       │ │   API       │                     │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘                     │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | Key Files |
|-----------|----------------|------|
| **HTTP Router** | Request routing, middleware, auth | `internal/http/router.go`, `internal/http/server.go` |
| **Auth Service** | Session management, user auth | `internal/auth/service.go`, `internal/auth/store.go` |
| **Chat Service** | Conversation management, message handling | `internal/chat/service.go`, `internal/chat/gateway.go` |
| **Agent Service** | Agent orchestration, tool execution | `internal/agent/service.go`, `internal/agent/executor.go` |
| **Knowledge Service** | RAG, document indexing | `internal/knowledge/service.go` |
| **Memory Service** | Document storage, embedding | `internal/memory/service.go` |
| **Relay System** | Multi-channel LLM routing | `internal/relay/relay.go`, `internal/relay/router.go` |
| **Task Service** | Background task management | `internal/task/service.go` |

## Pattern Overview

**Overall:** Clean Architecture / Layered Architecture with Domain-Driven Design patterns

**Key Characteristics:**
- Clear separation between handlers (HTTP), services (business logic), and stores (data access)
- Interface-based design for testability (Store interfaces, Gateway interfaces)
- Dependency injection via constructor functions
- Middleware chain for cross-cutting concerns (auth, logging, recovery)

## Layers

**Handler Layer (HTTP):**
- Purpose: HTTP request handling, routing, middleware
- Location: `internal/http/`
- Contains: Route handlers, middleware, request/response DTOs
- Depends on: Service layer
- Used by: External clients (Web, CLI, other services)

**Service Layer (Business Logic):**
- Purpose: Business logic, domain operations, orchestration
- Location: `internal/*/service.go` (per domain)
- Contains: Business rules, workflow orchestration, external integrations
- Depends on: Store layer, other services
- Used by: Handler layer

**Store Layer (Data Access):**
- Purpose: Data persistence, database operations
- Location: `internal/*/store.go` (per domain)
- Contains: SQL queries, transaction management
- Depends on: Database (PostgreSQL)
- Used by: Service layer

**Relay Layer (External LLM Integration):**
- Purpose: Multi-channel LLM routing, load balancing, circuit breaking
- Location: `internal/relay/`
- Contains: Channel pool, load balancer, circuit breakers, billing hooks
- Depends on: External LLM providers (OpenAI, Anthropic, etc.)
- Used by: Chat service, Agent service

## Data Flow

### Primary Request Path (Chat)

1. **Entry** — HTTP request arrives at `internal/http/server.go:NewServer()`
2. **Routing** — `internal/http/router.go:NewRouter()` matches route
3. **Middleware** — Auth, CORS, logging, recovery applied
4. **Handler** — Domain-specific handler extracts request, calls service
5. **Service** — Business logic executes, calls store and external services
6. **Store** — Database operations performed
7. **Response** — Data flows back through layers, JSON response returned

### Relay Request Flow (LLM Routing)

1. **Entry** — Request to `/v1/*` arrives at Relay engine
2. **Handler** — `internal/relay/handler/` normalizes request
3. **Router** — `internal/relay/router.go` selects channel via load balancer
4. **Circuit Breaker** — Health check ensures channel availability
5. **Token Bucket** — Rate limiting applied
6. **Billing Hook** — Pre/post billing and idempotency checks
7. **Upstream** — Request forwarded to selected LLM provider
8. **Response** — Streamed or synchronous response returned

### WebSocket Event Flow

1. **Connection** — Client connects to `/api/v1/ws`
2. **Auth** — Session validated via auth middleware
3. **Hub Registration** — Connection registered in `internal/ws/` hub
4. **Message Routing** — Messages routed to appropriate handler
5. **Broadcast** — Events broadcast to relevant connections

## Key Abstractions

**Store Interface Pattern:**
- Purpose: Abstract data access for testability
- Examples: `internal/chat/store.go:Store`, `internal/auth/store.go:Store`
- Pattern: Interface defines CRUD operations, SQL implementation in same package

**Gateway Pattern:**
- Purpose: Abstract external service integration
- Examples: `internal/chat/gateway.go:ChatGateway`, `internal/memory/embedder.go:Embedder`
- Pattern: Interface hides implementation details (HTTP, Relay, etc.)

**Service Facade:**
- Purpose: Coordinate multiple domain operations
- Examples: `internal/agent/service.go`, `internal/chat/service.go`
- Pattern: Service orchestrates stores, gateways, and other services

**Middleware Chain:**
- Purpose: Cross-cutting concerns (auth, logging, recovery)
- Location: `internal/http/middleware.go`, `internal/http/router.go:applyMiddleware()`
- Pattern: Higher-order functions wrap handlers

## Entry Points

**Main Application:**
- Location: `src/server/cmd/server/main.go`
- Triggers: System startup, process execution
- Responsibilities: Config loading, database init, server creation, graceful shutdown

**Database Migrations:**
- Location: `src/server/cmd/migrate/main.go`
- Triggers: Manual execution, deployment scripts
- Responsibilities: Schema migrations, seed data

**Web Frontend (Development):**
- Location: `src/web/vite.config.ts`, `src/web/package.json`
- Triggers: `pnpm dev` command
- Responsibilities: Vite dev server, HMR, proxy configuration

## Architectural Constraints

**Threading:** Single-threaded event loop per server instance (Go runtime multiplexes goroutines). No shared mutable state between requests except explicitly synchronized stores (database).

**Global State:** Minimal global state. `ws.DefaultHub()` is a package-level singleton for WebSocket management. Configuration is passed explicitly to components.

**Circular Imports:** None detected. Internal packages follow hierarchical dependency direction: `cmd` -> `http` -> `service` -> `store` -> `db`. `relay` is a standalone subsystem.

**Database Constraints:** PostgreSQL required. Migrations must be applied before application startup. Connection pooling handled by `database/sql` with `lib/pq`.

**External Dependencies:** Runtime dependencies on external LLM providers (OpenAI, Anthropic, etc.) for core functionality. Circuit breakers protect against provider outages.

## Anti-Patterns

### Large Handler Functions

**What happens:** Some HTTP handlers in `internal/http/` contain substantial inline route registration logic.

**Why it's wrong:** Makes testing difficult, violates single responsibility, complicates route overview.

**Do this instead:** Extract route registration into separate functions or use a declarative route table. See `internal/http/routes_*.go` for partial examples.

### Store Interface Proliferation

**What happens:** Each domain package defines its own Store interface with similar CRUD patterns.

**Why it's wrong:** Code duplication across packages, inconsistent naming (Create vs Insert), harder to implement generic utilities.

**Do this instead:** Consider a generic Store interface with type parameters (Go 1.18+) or code generation for boilerplate CRUD.

## Error Handling

**Strategy:** Centralized error handling via middleware with structured logging. Domain errors wrapped with context at service layer.

**Patterns:**
- Handlers return HTTP status codes based on error types (400 client, 500 server)
- Services return domain-specific errors (e.g., `ErrNotFound`, `ErrUnauthorized`)
- Stores return SQL errors wrapped with operation context
- Recovery middleware catches panics, logs stack traces, returns 500

## Cross-Cutting Concerns

**Logging:** Structured logging via Go's standard `log` package. Request ID injection for tracing. Log levels: info (requests), error (failures), debug (development).

**Validation:** Input validation at handler layer before service calls. JSON schema validation for complex payloads.

**Authentication:** Session-based auth with cookie storage. JWT for external API access (future). Middleware extracts session, attaches to context.

**Authorization:** Role-based access control (RBAC) via `requireAdmin` middleware. Resource-level permissions not yet implemented.

**Metrics:** Prometheus metrics exposed at `/metrics`. HTTP request duration, throughput, error rates.

---

*Architecture analysis: 2026-04-27*

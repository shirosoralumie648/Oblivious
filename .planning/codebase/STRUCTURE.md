---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

# Codebase Structure

**Analysis Date:** 2026-04-28

## Directory Layout

```
Oblivious/
├── .claude/                 # Claude Code agent definitions (16 agent types)
├── .claude-flow/            # Claude Flow internal data
├── .github/workflows/       # CI pipeline (GitHub Actions)
├── .planning/               # GSD planning artifacts
│   ├── codebase/            # Codebase map documents (this file)
│   ├── phases/              # Phase execution plans
│   ├── config.json          # Planning configuration
│   ├── PROJECT.md           # Project overview
│   ├── REQUIREMENTS.md      # Requirements tracking
│   ├── ROADMAP.md           # Development roadmap
│   └── STATE.md             # Current state snapshot
├── config/                  # Application configuration
│   └── .env.example         # Environment variable template
├── docs/                    # Project documentation
│   ├── API.md               # API endpoint reference
│   ├── architecture/        # Architecture decision records
│   ├── governance/          # Owner matrix, blocker escalation, weekly status
│   ├── release/             # Release candidate checklist
│   ├── reports/             # Progress/execution reports
│   └── superpowers/         # Design specs (7 design documents)
├── lobehub/                 # External reference: LobeChat frontend codebase
├── new-api/                 # External reference: new-api relay implementation
├── scripts/                 # Build/test/check automation
│   ├── check.sh             # Quality gate check
│   ├── dev.sh               # Development startup
│   ├── test.sh              # Test runner
│   └── verify-quality-gates.sh # CI quality gate verification
├── src/
│   ├── server/              # Go backend
│   │   ├── cmd/             # Entry points
│   │   │   ├── migrate/     # Database migration runner
│   │   │   └── server/      # HTTP server entry
│   │   ├── internal/        # All backend business logic
│   │   │   ├── admin/       # Admin service (stats, user management)
│   │   │   ├── agent/       # AI agent service (Runner, Executor)
│   │   │   ├── auth/        # Authentication (service + store)
│   │   │   ├── chat/        # Chat service, LLM gateways
│   │   │   ├── config/      # Environment config loader
│   │   │   ├── console/     # Console dashboard data
│   │   │   ├── db/          # Database connection
│   │   │   ├── http/        # HTTP routing, handlers, middleware
│   │   │   ├── knowledge/   # Knowledge base service
│   │   │   ├── mcp/         # MCP client and builtin tools
│   │   │   ├── memory/      # Document memory with pgvector
│   │   │   ├── metrics/     # Prometheus metrics
│   │   │   ├── notification/ # Notification service
│   │   │   ├── quota/       # Quota management
│   │   │   ├── relay/       # OpenAI-compatible API proxy
│   │   │   │   ├── channel/ # Provider adapters (OpenAI)
│   │   │   │   ├── handler/ # 35 API route handlers (chat, images, audio, etc.)
│   │   │   │   └── types/   # Shared relay types
│   │   │   ├── task/        # Long-running task state machine
│   │   │   ├── usage/       # Usage recording
│   │   │   ├── userprefs/   # User preferences
│   │   │   └── ws/          # WebSocket hub for real-time events
│   │   ├── migrations/      # 19 PostgreSQL migration files
│   │   ├── go.mod           # Go module definition
│   │   └── go.sum           # Go dependency checksums
│   └── web/                 # React SPA frontend
│       ├── dist/            # Vite build output (generated, not committed)
│       ├── node_modules/    # npm dependencies (not committed)
│       ├── src/
│       │   ├── app/         # App shell, router, providers, context
│       │   ├── features/    # Feature modules (auth, chat, console, knowledge, layouts, tasks)
│       │   ├── routes/      # Page components per route
│       │   │   ├── admin/       # Admin pages
│       │   │   ├── console/     # Console dashboard pages
│       │   │   ├── marketing/   # Public pages (home, login, register)
│       │   │   └── workspace/   # Authenticated workspace pages
│       │   ├── services/    # Infrastructure (HTTP client, streaming, uploads)
│       │   ├── store/       # Global state (app store - unused)
│       │   ├── test/        # Test setup
│       │   ├── theme/       # CSS tokens and global styles
│       │   └── types/       # Shared TypeScript type definitions
│       ├── index.html       # Vite entry HTML
│       ├── package.json     # Web dependencies
│       ├── tsconfig.json    # TypeScript config
│       └── vite.config.ts   # Vite configuration
├── .gitignore               # Git ignore rules
├── .mcp.json                # MCP server configuration
├── package.json             # Root package (pnpm workspace orchestration)
├── pnpm-lock.yaml           # pnpm lockfile
├── pnpm-workspace.yaml      # pnpm workspace config (src/web only)
├── CLAUDE.md                # Claude Code project instructions
├── README.md                # Project README
├── ARCHAEOLOGY_REPORT.md    # Initial codebase archaeology
├── CURRENT_STATUS.md        # Current project status
└── ROADMAP.md               # Development roadmap
```

## Directory Purposes

**`src/server/`:**
- Purpose: Go backend — all server-side logic, HTTP API, database access, LLM relay
- Contains: Go source files (`*.go`), SQL migrations (`*.sql`), module files
- Key files: `cmd/server/main.go`, `internal/http/router.go`, `internal/relay/relay.go`, `go.mod`

**`src/web/`:**
- Purpose: React SPA frontend — user interface for console, workspace, chat, knowledge
- Contains: TypeScript/TSX files, CSS, Vite config, HTML entry
- Key files: `src/main.tsx`, `src/app/router.tsx`, `src/app/App.tsx`

**`src/server/internal/`:**
- Purpose: All backend business logic organized by bounded context (DDD-aligned)
- Contains: 18 sub-packages, each with service + store pattern
- Key files: `http/router.go` (central routing), `chat/service.go`, `relay/relay.go`

**`src/server/internal/relay/`:**
- Purpose: OpenAI-compatible API proxy with multi-provider routing, load balancing, billing
- Contains: Core relay, router, load balancer, circuit breaker, token bucket, billing hook, 35 API handlers, provider adapters
- Key files: `relay.go`, `router.go`, `handler/router.go`, `pool.go`, `loadbalancer.go`

**`src/server/migrations/`:**
- Purpose: Sequential PostgreSQL schema migrations
- Contains: 19 numbered SQL files (0001 through 0019)
- Key files: `0001_phase1_foundation.sql`, `0016_pgvector.sql`

**`src/web/src/features/`:**
- Purpose: Feature-based modules — each feature contains its own API, store, components, and tests
- Contains: `auth/`, `chat/`, `console/`, `knowledge/`, `layouts/`, `tasks/`
- Key files: `auth/store.ts`, `console/api.ts`, `layouts/WorkspaceLayout.tsx`

**`src/web/src/routes/`:**
- Purpose: Page-level components organized by route hierarchy
- Contains: `admin/`, `console/`, `marketing/`, `workspace/` subdirectories
- Key files: `workspace/ChatPage.tsx`, `console/ConsoleHomePage.tsx`

**`src/web/src/services/`:**
- Purpose: Infrastructure-level HTTP client, streaming, envelope parsing, error handling
- Contains: `http/` subdirectory with client, errors, envelope, stream, upload modules
- Key files: `http/client.ts`, `http/envelope.ts`, `http/errors.ts`

**`config/`:**
- Purpose: Environment configuration templates
- Contains: `.env.example` (template with documented variables)
- Key files: `.env.example`

**`scripts/`:**
- Purpose: Automation scripts for development, testing, and quality checks
- Contains: Shell scripts
- Key files: `dev.sh`, `test.sh`, `check.sh`, `verify-quality-gates.sh`

**`docs/`:**
- Purpose: Project documentation — architecture decisions, design specs, governance, reports
- Contains: API docs, ADRs, design specifications, governance templates, progress reports
- Key files: `API.md`, `architecture/current-system-contracts.md`

**`lobehub/` and `new-api/`:**
- Purpose: External reference implementations — LobeChat (frontend patterns) and new-api (relay patterns)
- Contains: Full reference codebases used as design references, not compiled or tested
- Key files: Available for reference reading only

**`.planning/`:**
- Purpose: GSD planning system artifacts — phase plans, codebase map, project state
- Contains: Markdown documents used by GSD orchestrator for plan/execute cycles
- Key files: `config.json`, `PROJECT.md`, `ROADMAP.md`, `STATE.md`

## Key File Locations

**Entry Points:**
- `src/server/cmd/server/main.go`: Go HTTP server entry — loads config, opens DB, creates and runs server
- `src/server/cmd/migrate/main.go`: Database migration runner entry
- `src/web/src/main.tsx`: React app entry — renders `<App />` into DOM
- `src/web/index.html`: Vite HTML entry point

**Configuration:**
- `src/server/internal/config/config.go`: Backend env config loader (SERVER_PORT, DATABASE_URL, SESSION_SECRET, etc.)
- `config/.env.example`: Documented env var template
- `src/web/vite.config.ts`: Vite build configuration
- `src/web/tsconfig.json`: TypeScript compiler configuration
- `src/server/go.mod`: Go module definition (module `oblivious/server`)
- `package.json`: Root monorepo orchestrator (pnpm workspace)
- `pnpm-workspace.yaml`: Workspace members (`src/web`)

**Core Logic:**
- `src/server/internal/http/router.go`: Central route registration (~726 lines, all API endpoints)
- `src/server/internal/http/server.go`: HTTP server factory, Relay integration, handler combining
- `src/server/internal/chat/service.go`: Conversation and message management
- `src/server/internal/chat/gateway.go`: LLM gateway interfaces and direct HTTP implementation
- `src/server/internal/chat/relay_gateway.go`: Relay-based LLM gateway with streaming
- `src/server/internal/agent/service.go`: AI agent lifecycle, tool execution, memory injection
- `src/server/internal/agent/runner.go`: Agent tool-loop execution engine
- `src/server/internal/relay/relay.go`: Relay subsystem core (Gin engine, dependency chain)
- `src/server/internal/relay/router.go`: Channel selection, rate limiting, billing integration
- `src/server/internal/relay/pool.go`: In-memory channel pool with RWMutex
- `src/server/internal/relay/loadbalancer.go`: Weighted/priority/cost-aware channel selection
- `src/server/internal/relay/circuitbreaker.go`: Per-channel circuit breaker
- `src/server/internal/relay/billing.go`: Pre/post billing hooks
- `src/server/internal/auth/service.go`: Auth with bcrypt, session management
- `src/server/internal/task/runtime.go`: Task state machine (draft/running/paused/completed/cancelled)

**HTTP Handlers:**
- `src/server/internal/http/chat_handler.go`: Conversations, messages, model listing
- `src/server/internal/http/agent_handler.go`: Agent CRUD, conversations, tools
- `src/server/internal/http/knowledge_handler.go`: Knowledge base/document CRUD, retrieval
- `src/server/internal/http/task_handler.go`: Task lifecycle operations
- `src/server/internal/http/console_handler.go`: Usage, billing, models, access data
- `src/server/internal/http/memory_handler.go`: Memory document CRUD, search
- `src/server/internal/http/mcp_handler.go`: MCP server management
- `src/server/internal/http/auth_handler.go`: Login, register, me, logout
- `src/server/internal/http/auth_middleware.go`: Session extraction, HMAC validation, requireAdmin
- `src/server/internal/http/middleware.go`: Recover, request ID, logging, CORS
- `src/server/internal/http/response.go`: Envelope JSON responses (writeSuccess, writeError)

**Testing:**
- Co-located with source files: `*_test.go` alongside `*.go` in backend, `*.test.ts*` alongside `*.ts*` in frontend
- `src/web/src/test/setup.ts`: Frontend test environment setup

**Documentation:**
- `docs/API.md`: API endpoint reference
- `docs/architecture/current-system-contracts.md`: Current system contracts
- `docs/architecture/knowledge-evolution-decision.md`: Knowledge system decision record
- `docs/architecture/solo-runtime-decision.md`: SOLO runtime decision record
- `docs/superpowers/specs/`: 7 design specification documents

## Naming Conventions

**Files:**
- Go source: `snake_case.go` (e.g., `chat_handler.go`, `relay_gateway.go`, `circuitbreaker.go`)
- Go tests: `*_test.go` suffix (e.g., `chat_handler_test.go`, `router_test.go`)
- TypeScript/React: `camelCase.ts(x)` (e.g., `ChatPage.tsx`, `appContext.tsx`, `useAuthBootstrap.ts`)
- Tests: `*.test.ts(x)` suffix (e.g., `ChatPage.test.tsx`, `store.test.ts`)
- SQL migrations: Zero-padded numbered prefix with snake_case description (e.g., `0001_phase1_foundation.sql`)
- Documentation: UPPERCASE.md for project-level docs in root (e.g., `ROADMAP.md`), kebab-case for specs (e.g., `2026-04-06-workspace-main-flow-design.md`)

**Directories:**
- Go packages: Single lowercase word (e.g., `auth/`, `chat/`, `relay/`)
- React features: Same as Go — single lowercase word (e.g., `auth/`, `chat/`, `console/`, `layouts/`)
- React routes: Same word used for package and page (e.g., `workspace/ChatPage.tsx` in `routes/workspace/`)
- `.planning/`: UPPERCASE.md for documents, `phases/` with numbered directory names

**Functions (Go):**
- Export rules: CamelCase (uppercase first for exported, lowercase for unexported)
- Constructors: `New*` pattern (e.g., `NewServer`, `NewRouter`, `NewService`, `NewSQLStore`, `NewRelayGateway`)
- Options pattern: `With*` closures (e.g., `WithRelayURL`, `WithDefaultModel`, `WithHTTPClient`)
- Handler methods: lowercase verb (e.g., `createConversation`, `listMessages`, `sendMessage`)

**Functions (TypeScript):**
- React components: PascalCase (e.g., `ChatPage`, `WorkspaceLayout`, `ProtectedRoute`)
- Hooks: `use*` prefix (e.g., `useAppContext`, `useAuthBootstrap`)
- Factory functions: `create*` prefix (e.g., `createHttpClient`, `createAppRouter`, `createAuthStore`, `createConsoleApi`)
- API functions: descriptive verb phrase (e.g., `getAccess`, `getBilling`, `sendMessage`)

**Types:**
- Go: PascalCase structs (e.g., `Service`, `Config`, `Session`, `ConversationConfig`)
- TypeScript: PascalCase types/interfaces (e.g., `ApiUser`, `ConversationSummary`, `AuthState`)
- TypeScript: Envelope types prefixed with `Api` (e.g., `ApiEnvelope`, `ApiUser`, `ApiSession`)

## Where to Add New Code

**New Feature (backend):**
- Primary code: `src/server/internal/{feature}/` — create `service.go` and `store.go`
- Handler: `src/server/internal/http/{feature}_handler.go`
- Routes: Register in `src/server/internal/http/router.go` in `NewRouter()` function
- Tests: `src/server/internal/{feature}/service_test.go`, `src/server/internal/http/{feature}_handler_test.go`
- Migrations: `src/server/migrations/{NNNN}_{feature_name}.sql`

**New Feature (frontend):**
- Primary code: `src/web/src/features/{feature}/` — create API module, components, store
- Pages: `src/web/src/routes/{section}/{FeaturePage}.tsx`
- Route registration: Add to `src/web/src/app/router.tsx`
- Types: `src/web/src/types/api.ts` (add new types)
- Tests: Co-located `*.test.ts(x)` files alongside source

**New Relay Provider Adapter:**
- Implementation: `src/server/internal/relay/channel/{provider}_adapter.go`
- Types: Add to `src/server/internal/relay/types/types.go` if new APIType needed
- Handler: `src/server/internal/relay/handler/{endpoint}.go` if new endpoint type
- Route: Add to `getOpenAIRoutes()` in `src/server/internal/relay/handler/router.go`

**New Service Dependency:**
- When a new service needs to be wired into existing services: Add constructor parameter and initialization in `NewRouter()` (`src/server/internal/http/router.go`)
- Share the gateway: Pass `chatService.ChatGateway()` if sharing LLM access

**Utilities/Helpers:**
- Shared backend helpers: Add to relevant package or create new package under `src/server/internal/`
- Shared frontend helpers: `src/web/src/services/` for infrastructure, keep feature-specific logic in feature directories

## Special Directories

**`src/server/.claude-flow/`:**
- Purpose: Server-specific Claude Flow internal state (session data)
- Generated: Yes, by Claude Flow CLI
- Committed: No (runtime data)

**`src/web/dist/`:**
- Purpose: Vite production build output
- Generated: Yes, by `vite build`
- Committed: No (build artifact)

**`src/web/node_modules/`:**
- Purpose: npm dependencies
- Generated: Yes, by `pnpm install`
- Committed: No

**`.tmp/`:**
- Purpose: Temporary build artifacts (Go build cache, pnpm corepack home)
- Generated: Yes, by build scripts
- Committed: No

**`.worktrees/`:**
- Purpose: Git worktrees for parallel branch development
- Generated: Yes, by git commands
- Committed: No (git-managed)

**`lobehub/` and `new-api/`:**
- Purpose: Reference implementations for design patterns
- Generated: No (manually placed for reference)
- Committed: Yes (reference code, not built or tested)

---

*Structure analysis: 2026-04-28*

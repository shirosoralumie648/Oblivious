# Codebase Structure

**Analysis Date:** 2026-04-27

## Directory Layout

```
/media/shirosora/4A183E5C183E46EB/codestorage/Oblivious/
├── .claude/                    # Claude Code configuration
│   ├── helpers/                # Automation scripts
│   ├── skills/                 # Skill definitions
│   └── settings.json           # Claude settings
│
├── .claude-flow/               # Claude Flow metrics
│   ├── metrics/                # Performance tracking
│   └── security/               # Security audit status
│
├── .planning/                  # Planning documents
│   └── codebase/               # Codebase analysis docs
│       ├── ARCHITECTURE.md
│       └── STRUCTURE.md
│
├── config/                     # Configuration files
│   └── .env.example            # Environment template
│
├── docs/                       # Documentation
│   ├── architecture/           # System design docs
│   ├── governance/             # Process docs
│   ├── reports/                # Progress reports
│   └── superpowers/specs/      # Feature specs
│
├── lobehub/                    # LobeHub UI (Next.js)
│   ├── src/
│   │   ├── app/                # Next.js app router
│   │   ├── components/         # React components
│   │   ├── features/           # Feature modules
│   │   ├── hooks/              # Custom hooks
│   │   ├── libs/               # Library wrappers
│   │   ├── services/           # API services
│   │   ├── store/              # State management
│   │   └── types/              # TypeScript types
│   ├── drizzle.config.ts       # Database config
│   └── package.json
│
├── new-api/                    # Original Go API (legacy)
│   ├── common/                 # Shared utilities
│   ├── constant/               # Constants
│   ├── controller/             # HTTP handlers
│   ├── dto/                    # Data transfer objects
│   ├── middleware/             # HTTP middleware
│   ├── model/                  # Database models
│   ├── relay/                  # LLM relay logic
│   ├── router/                 # Route definitions
│   ├── service/                # Business logic
│   └── main.go
│
├── scripts/                    # Build/utility scripts
│   ├── check.sh                # Quality checks
│   ├── dev.sh                  # Development server
│   ├── test.sh                 # Test runner
│   └── verify-quality-gates.sh # CI verification
│
├── src/                        # Main source code
│   ├── server/                 # Go backend
│   │   ├── cmd/
│   │   │   ├── migrate/       # Migration tool
│   │   │   └── server/        # Main server
│   │   ├── internal/          # Internal packages
│   │   │   ├── admin/         # Admin operations
│   │   │   ├── agent/         # Agent orchestration
│   │   │   ├── auth/          # Authentication
│   │   │   ├── chat/          # Chat/Conversations
│   │   │   ├── config/        # Configuration
│   │   │   ├── console/       # Console operations
│   │   │   ├── db/            # Database connection
│   │   │   ├── http/          # HTTP handlers
│   │   │   ├── knowledge/     # Knowledge bases
│   │   │   ├── mcp/           # MCP client
│   │   │   ├── memory/        # Memory/documents
│   │   │   ├── metrics/       # Prometheus metrics
│   │   │   ├── notification/  # Notifications
│   │   │   ├── quota/         # Quota management
│   │   │   ├── relay/         # LLM relay system
│   │   │   ├── task/          # Task management
│   │   │   ├── usage/         # Usage tracking
│   │   │   ├── userprefs/     # User preferences
│   │   │   └── ws/            # WebSocket hub
│   │   ├── migrations/        # SQL migrations
│   │   └── go.mod             # Go module
│   │
│   └── web/                    # React/Vite frontend
│       ├── src/
│       │   ├── app/            # Application routes
│       │   ├── components/     # Reusable components
│       │   ├── features/       # Feature modules
│       │   ├── hooks/          # Custom React hooks
│       │   ├── libs/           # Library utilities
│       │   ├── routes/         # Route definitions
│       │   ├── services/       # API client services
│       │   ├── store/          # State management
│       │   ├── types/          # TypeScript types
│       │   └── main.tsx        # Entry point
│       ├── index.html
│       ├── package.json
│       ├── tsconfig.json
│       └── vite.config.ts
│
├── .github/
│   └── workflows/              # CI/CD workflows
│       └── ci.yml
│
├── package.json               # Root package (pnpm workspace)
├── pnpm-workspace.yaml        # Workspace config
├── pnpm-lock.yaml             # Lock file
├── README.md                  # Project overview
├── ROADMAP.md                 # Development roadmap
└── CLAUDE.md                  # Claude Code instructions
```

## Key File Locations

### Entry Points

| Entry | File | Description |
|-------|------|-------------|
| Server Main | `src/server/cmd/server/main.go` | HTTP server startup, graceful shutdown |
| Migration Tool | `src/server/cmd/migrate/main.go` | Database migration runner |
| Web App | `src/web/src/main.tsx` | React application entry |
| LobeHub | `lobehub/src/index.ts` | Next.js application entry |
| Legacy API | `new-api/main.go` | Original API server (legacy) |

### Configuration

| File | Purpose |
|------|---------|
| `src/server/internal/config/config.go` | Server configuration loading from env |
| `src/server/go.mod` | Go module dependencies |
| `src/web/vite.config.ts` | Vite build configuration |
| `src/web/tsconfig.json` | TypeScript configuration |
| `config/.env.example` | Environment variables template |
| `pnpm-workspace.yaml` | pnpm workspace definition |

### Core Logic

| File | Purpose |
|------|---------|
| `src/server/internal/http/router.go` | HTTP route definitions |
| `src/server/internal/http/server.go` | HTTP server factory |
| `src/server/internal/chat/service.go` | Chat business logic |
| `src/server/internal/agent/service.go` | Agent orchestration |
| `src/server/internal/relay/relay.go` | LLM relay system |
| `src/server/internal/relay/router.go` | Relay routing logic |

### Testing

| Location | Purpose |
|----------|---------|
| `src/server/internal/*/service_test.go` | Unit tests per service |
| `src/server/internal/*/store_test.go` | Store tests |
| `src/server/internal/http/*_test.go` | Handler tests |
| `src/web/src/test/` | Frontend test utilities |

## Source Organization

| Directory | Language | Files | Purpose |
|-----------|----------|-------|---------|
| `src/server/cmd/` | Go | 2 | Application entry points |
| `src/server/internal/admin/` | Go | 2 | Admin operations service |
| `src/server/internal/agent/` | Go | 4 | Agent orchestration |
| `src/server/internal/auth/` | Go | 4 | Authentication & sessions |
| `src/server/internal/chat/` | Go | 8 | Chat & conversations |
| `src/server/internal/config/` | Go | 2 | Configuration management |
| `src/server/internal/console/` | Go | 4 | Console operations |
| `src/server/internal/db/` | Go | 1 | Database connection |
| `src/server/internal/http/` | Go | 15 | HTTP handlers & routing |
| `src/server/internal/knowledge/` | Go | 4 | Knowledge base management |
| `src/server/internal/mcp/` | Go | 4 | MCP client implementation |
| `src/server/internal/memory/` | Go | 6 | Memory & document storage |
| `src/server/internal/metrics/` | Go | 2 | Prometheus metrics |
| `src/server/internal/notification/` | Go | 2 | Notifications |
| `src/server/internal/quota/` | Go | 4 | Quota management |
| `src/server/internal/relay/` | Go | 20 | LLM relay system |
| `src/server/internal/task/` | Go | 4 | Task management |
| `src/server/internal/usage/` | Go | 2 | Usage tracking |
| `src/server/internal/userprefs/` | Go | 2 | User preferences |
| `src/server/internal/ws/` | Go | 2 | WebSocket hub |
| `src/server/migrations/` | SQL | 10+ | Database migrations |
| `src/web/src/app/` | TypeScript/React | 5 | Application routes |
| `src/web/src/components/` | TypeScript/React | 10+ | Reusable components |
| `src/web/src/features/` | TypeScript/React | 15+ | Feature modules |
| `src/web/src/hooks/` | TypeScript | 10+ | Custom React hooks |
| `src/web/src/services/` | TypeScript | 8 | API client services |
| `src/web/src/store/` | TypeScript | 5 | State management |
| `src/web/src/types/` | TypeScript | 5 | TypeScript types |

## Where to Add New Code

### New Domain/Feature

**Backend:**
- Service: `src/server/internal/<domain>/service.go`
- Store: `src/server/internal/<domain>/store.go`
- Handler: `src/server/internal/http/<domain>_handler.go`
- Routes: Add to `src/server/internal/http/router.go`

**Frontend:**
- Feature: `src/web/src/features/<feature>/`
- API Service: `src/web/src/services/<feature>Service.ts`
- Types: `src/web/src/types/<feature>.ts`

### New API Endpoint

1. Add handler method in `src/server/internal/http/<domain>_handler.go`
2. Register route in `src/server/internal/http/router.go`
3. Add tests in `src/server/internal/http/<domain>_handler_test.go`

### New Database Table

1. Create migration in `src/server/migrations/NNN_<name>.sql`
2. Add methods to store interface in `src/server/internal/<domain>/store.go`
3. Implement SQL operations in store

### New LLM Provider (Relay)

1. Add channel type in `src/server/internal/relay/types/`
2. Implement provider handler in `src/server/internal/relay/handler/`
3. Register in `src/server/internal/relay/router.go`
4. Add pricing in `src/server/internal/relay/pricing.go`

## Special Directories

| Directory | Purpose | Generated | Committed |
|-----------|---------|-----------|-----------|
| `src/web/dist/` | Production build output | Yes | No |
| `src/web/node_modules/` | Node.js dependencies | Yes | No |
| `.tmp/` | Temporary build cache | Yes | No |
| `src/server/migrations/` | SQL schema migrations | No | Yes |
| `.claude-flow/metrics/` | Performance metrics | Yes | No |
| `.planning/codebase/` | Documentation | No | Yes |

---

*Structure analysis: 2026-04-27*

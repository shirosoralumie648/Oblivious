---
description: Technology stack inventory
created: 2026-04-27
---
# Technology Stack

**Analysis Date:** 2026-04-27

## Core Technologies

| Component | Technology | Version | Purpose |
|-----------|------------|---------|----------|
| Backend Language | Go | 1.25.0 | API server, business logic |
| Frontend Language | TypeScript | 5.6.3 | React application |
| Frontend Framework | React | 18.3.1 | UI components |
| Frontend Build | Vite | 5.4.10 | Dev server, bundling |
| Routing | React Router | 6.28.0 | Client-side navigation |
| Database | PostgreSQL | 14+ | Primary data store |

## Backend Stack (Go)

| Component | Technology | Version | Purpose |
|-----------|------------|---------|----------|
| HTTP Framework | Gin | v1.12.0 | HTTP routing, middleware |
| Database Driver | lib/pq | v1.10.9 | PostgreSQL driver |
| WebSocket | gorilla/websocket | v1.5.3 | Real-time chat |
| UUID | google/uuid | v1.6.0 | ID generation |
| Crypto | golang.org/x/crypto | v0.48.0 | Password hashing (bcrypt) |
| Task Queue | asynq | v0.26.0 | Background job processing |
| Redis Client | go-redis/v9 | v9.14.1 | Asynq backend |
| Metrics | prometheus/client_golang | v1.23.2 | Metrics collection |
| Tiktoken | tiktoken-go | v0.1.8 | Token counting for billing |
| MongoDB | mongo-driver/v2 | v2.5.0 | Knowledge base storage |
| YAML | go-yaml | v1.19.2 | Configuration parsing |

## Frontend Stack (React/TypeScript)

| Component | Technology | Version | Purpose |
|-----------|------------|---------|----------|
| Runtime | Node.js | 20+ | JavaScript runtime |
| Package Manager | pnpm | 10.6.0 | Dependency management |
| Build Tool | Vite | 5.4.10 | Development & production builds |
| Type Checking | TypeScript | 5.6.3 | Static type checking |
| Testing | Vitest | 2.1.4 | Unit testing framework |
| DOM Testing | jsdom | 25.0.1 | DOM environment for tests |
| React Testing | testing-library/react | 16.1.0 | React component testing |
| Assertions | testing-library/jest-dom | 6.6.3 | Custom DOM matchers |

## Build & Tooling

| Tool | Purpose | Configuration File |
|------|---------|-------------------|
| pnpm | Package management, workspaces | `pnpm-workspace.yaml` |
| Go Modules | Go dependency management | `src/server/go.mod` |
| Vite | Frontend build & dev server | `src/web/vite.config.ts` |
| TypeScript | Type checking | `src/web/tsconfig.json` |
| Vitest | Frontend testing | Built into `package.json` scripts |
| GitHub Actions | CI/CD | `.github/workflows/ci.yml` |
| Shell Scripts | Dev automation | `scripts/dev.sh`, `scripts/test.sh`, `scripts/check.sh` |

## Key Dependencies Analysis

### Production-Critical Go Packages

| Package | Purpose | Impact |
|---------|---------|--------|
| `github.com/gin-gonic/gin` | HTTP framework | Core API routing |
| `github.com/lib/pq` | PostgreSQL driver | Database connectivity |
| `github.com/gorilla/websocket` | WebSocket implementation | Real-time chat |
| `github.com/hibiken/asynq` | Background task queue | Async job processing |
| `github.com/redis/go-redis/v9` | Redis client | Asynq backend, caching |
| `github.com/prometheus/client_golang` | Metrics | Observability |
| `github.com/pkoukk/tiktoken-go` | Token counting | LLM billing |
| `go.mongodb.org/mongo-driver/v2` | MongoDB driver | Knowledge base |

### Production-Critical Node Packages

| Package | Purpose | Impact |
|---------|---------|--------|
| `react` | UI library | Core frontend framework |
| `react-dom` | DOM renderer | Component rendering |
| `react-router-dom` | Client routing | Navigation |
| `vite` | Build tool | Development & production |

## Summary Stats

- **Languages**: Go (~60%), TypeScript (~35%), SQL (~5%)
- **Go Dependencies**: 50+ direct and indirect packages
- **Node Dependencies**: 20+ production, 15+ development
- **Main Entry Points**:
  - Backend: `src/server/cmd/server/main.go`
  - Migration: `src/server/cmd/migrate/main.go`
  - Frontend: `src/web/src/main.tsx` (inferred)
- **Workspace Structure**: pnpm monorepo with Go submodule

## Platform Requirements

**Development:**
- Go 1.22+
- Node.js 20+
- pnpm 10.6.0
- PostgreSQL 14+
- Redis (for asynq background jobs)

**Production:**
- Containerized deployment (Docker-compatible)
- PostgreSQL 14+
- Redis for task queue
- Prometheus scraping endpoint (`/metrics`)

---

*Stack analysis: 2026-04-27*

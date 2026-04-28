---
last_mapped_commit: f4dc5e48826c9893706249151aa081638e295dc1
---

# Technology Stack

**Analysis Date:** 2026-04-28

## Languages

**Primary:**
- Go 1.25.0 - Backend server (`src/server/go.mod`), relay engine, all services
- TypeScript 5.6.3 - Web frontend (`src/web/package.json`)

**Secondary:**
- JavaScript (JSX/TSX) - React components in web frontend

## Runtime

**Environment:**
- Go 1.25.0 (backend)
- Node.js 20 (frontend, CI-enforced in `.github/workflows/ci.yml`)

**Package Manager:**
- pnpm 10.6.0 (`package.json` root: `"packageManager": "pnpm@10.6.0"`)
- Lockfile: `pnpm-lock.yaml` (present)

## Frameworks

**Core:**
- Gin (github.com/gin-gonic/gin v1.12.0) - HTTP framework for the main server via `src/server/internal/relay/relay.go` (Gin engine used for Relay /v1/* routes)
- net/http (stdlib) - Primary HTTP router for the main server API routes (`src/server/internal/http/router.go`)
- React 18.3.1 - Frontend UI library (`src/web/package.json`)
- Vite 5.4.10 - Frontend build tool and dev server (`src/web/vite.config.ts`)
- React Router DOM 6.28.0 - Client-side routing (`src/web/package.json`)

**Testing:**
- vitest 2.1.4 - Frontend unit tests (`src/web/package.json`)
- @testing-library/react 16.1.0 - React component testing
- @testing-library/jest-dom 6.6.3 - DOM assertion matchers
- Go stdlib `testing` + test files co-located with source - Backend tests

**Build/Dev:**
- Vite 5.4.10 - Frontend build
- @vitejs/plugin-react 4.3.4 - React JSX transform
- tsc --noEmit - Frontend type checking
- Go compiler (go build) - Backend build
- pnpm scripts + shell scripts - Orchestration

## Key Dependencies

### Backend (Go - src/server)

**Critical:**
- `github.com/gin-gonic/gin` v1.12.0 - Web framework (used in relay engine)
- `github.com/gorilla/websocket` v1.5.3 - WebSocket connections (real-time notifications, agent status, billing events)
- `github.com/lib/pq` v1.10.9 - PostgreSQL driver (`src/server/internal/db/db.go`)
- `golang.org/x/crypto` v0.48.0 - bcrypt password hashing (`src/server/internal/auth/service.go`)
- `github.com/google/uuid` v1.6.0 - UUID generation

**Infrastructure:**
- `github.com/prometheus/client_golang` v1.23.2 - Prometheus metrics (`/metrics` endpoint in `src/server/internal/metrics/prometheus.go`)
- `github.com/redis/go-redis/v9` v9.14.1 - Redis client (indirect dependency; referenced in relay router for billing timeout tasks)
- `github.com/hibiken/asynq` v0.26.0 - Task queue/worker framework (indirect dependency)
- `github.com/pkoukk/tiktoken-go` v0.1.8 - Tokenizer library for token counting

### Frontend (TypeScript - src/web)

- `react` / `react-dom` 18.3.1 - UI framework
- `react-router-dom` 6.28.0 - Routing
- `vite` 5.4.10 - Build tool
- `jsdom` 25.0.1 - DOM simulation for tests

### Sub-project: new-api (separate Go project)

Additional dependencies in `new-api/go.mod`:
- `github.com/stripe/stripe-go/v81` v81.4.0 - Stripe payment integration
- `github.com/go-redis/redis/v8` v8.11.5 - Redis caching and task queue
- `github.com/aws/aws-sdk-go-v2` + bedrockruntime - AWS Bedrock model provider
- `github.com/gorilla/websocket` v1.5.0 - WebSocket
- `github.com/golang-jwt/jwt/v5` v5.3.0 - JWT auth
- `gorm.io/gorm` v1.25.2 - ORM (with PostgreSQL and MySQL drivers)
- `github.com/glebarez/sqlite` v1.9.0 - Embedded SQLite
- `github.com/go-webauthn/webauthn` v0.14.0 - WebAuthn/Passkey support
- `github.com/pquerna/otp` v1.5.0 - OTP/TOTP support
- `github.com/gin-contrib/sessions` v0.0.5 - Session management
- `github.com/grafana/pyroscope-go` v1.2.7 - Continuous profiling
- `github.com/joho/godotenv` v1.5.1 - .env file loading

## Configuration

**Environment:**
- Config loaded from environment variables via `src/server/internal/config/config.go` (Go `os.Getenv` with defaults)
- `config/.env.example` present
- No dotenv loading in the main server (uses system env vars directly)
- new-api uses `github.com/joho/godotenv` for .env file loading

**Key Configs Required (main server):**
- `DATABASE_URL` - PostgreSQL connection string (required, validated at startup)
- `SESSION_SECRET` - Session signing secret (required)
- `SERVER_PORT` - Listen port (default: 8080)
- `RELAY_ENABLED` - Enable/disable relay gateway (boolean, default: true)
- `OPENAI_API_KEY` - OpenAI API key for default channel
- `OPENAI_BASE_URL` - OpenAI API base URL (default: https://api.openai.com)
- `LLM_BASE_URL` - LLM fallback endpoint
- `LLM_API_KEY` - LLM fallback API key
- `CORS_ALLOWED_ORIGINS` - Comma-separated CORS origins

**Build:**
- `src/web/tsconfig.json` - TypeScript config
- `src/web/vite.config.ts` - Vite config (includes vitest config inline)
- Go module: `src/server/go.mod`

## Platform Requirements

**Development:**
- Go 1.25.0+
- Node.js 20+
- pnpm 10.6.0
- PostgreSQL instance (connection string via `DATABASE_URL`)

**Production:**
- GitHub Actions CI (`ci.yml`: Ubuntu latest, pnpm 10.6.0, Node 20, Go from go.mod)
- PostgreSQL database
- Environment variables configured via deployment platform

---

*Stack analysis: 2026-04-28*

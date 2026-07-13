# Technology Stack

**Analysis Date:** 2026-07-13

## Languages

**Primary:**
- Go 1.25.0 - backend services, CLI commands, HTTP handlers, gRPC adapters, persistence, Relay, Agent, Workflow, Knowledge, Marketplace, Billing, and operations code under `src/server/`.
- TypeScript - React frontend routes, feature API clients, unit tests, and Playwright fixtures under `src/web/src/` and `src/web/e2e/`.

**Secondary:**
- Python 3 - target evidence collectors, manifest validators, OpenAPI validators, and release automation under `scripts/*.py`.
- Bash - migration, deploy, release, verification, and evidence entrypoints under `scripts/*.sh`.
- YAML/JSON - Kubernetes, observability, OpenAPI, route manifests, package manifests, and release config under `deploy/`, `docs/api/`, and root config files.

## Runtime

**Backend runtime:**
- Go module: `src/server/go.mod`.
- Main HTTP server entrypoint: `src/server/cmd/server/main.go`.
- Service command entrypoints: `src/server/cmd/chat/`, `src/server/cmd/agent/`, `src/server/cmd/relay/`, `src/server/cmd/rag/`, `src/server/cmd/workflow/`, `src/server/cmd/task/`, `src/server/cmd/billing/`, `src/server/cmd/channel/`, `src/server/cmd/gateway/`, `src/server/cmd/marketplace/`, and `src/server/cmd/observability/`.

**Frontend runtime:**
- Node 20+ and pnpm 10.6.0 per `README.md` and root `package.json`.
- Vite React app under `src/web/`.
- Vite dev/preview proxy forwards `/api` and `/v1` to `VITE_API_PROXY_TARGET` in `src/web/vite.config.ts`.

## Package Managers And Lockfiles

- Go modules: `src/server/go.mod` and `src/server/go.sum`.
- pnpm workspace: `pnpm-workspace.yaml`, `pnpm-lock.yaml`, and root `package.json`.
- npm lockfiles are also tracked: root `package-lock.json` and `src/web/package-lock.json`.
- The root pnpm workspace points at `src/web`; backend dependency management is independent through Go modules.

## Frameworks

**Backend:**
- Gin HTTP framework via `github.com/gin-gonic/gin` in `src/server/go.mod`.
- GORM and SQL drivers for persistence, including PostgreSQL `lib/pq` and MySQL driver dependencies in `src/server/go.mod`.
- gRPC/protobuf via `google.golang.org/grpc`, `google.golang.org/protobuf`, and service definitions under `api/proto/*.proto`.
- Redis/Asynq/Kafka integrations through dependencies in `src/server/go.mod`.
- Prometheus and OpenTelemetry packages for metrics and tracing.
- Stripe Go SDK for payment/provider lifecycle paths.

**Frontend:**
- React 18, React Router, Vite, TypeScript, Tailwind CSS 3, SWR, Zustand, Zod, React Hook Form, Radix UI, XYFlow, Recharts, GSAP, Sonner, and cmdk from `src/web/package.json`.
- Vite config and test setup are in `src/web/vite.config.ts` and `src/web/src/test/setup.ts`.

**Testing:**
- Go `testing` packages under `src/server/**/*_test.go`.
- Vitest/jsdom for frontend unit and component tests.
- Playwright for browser E2E under `src/web/e2e/`.
- Shell/Python release fixtures under `scripts/*-fixtures.sh` and `scripts/target_release_fixture_mutations.py`.

## Key Dependencies

**Critical backend dependencies:**
- `github.com/gin-gonic/gin` - HTTP routing and middleware.
- `gorm.io/gorm`, `gorm.io/driver/mysql`, `github.com/lib/pq` - persistence.
- `github.com/redis/go-redis/v9` and `github.com/hibiken/asynq` - Redis-backed queue/runtime support.
- `github.com/segmentio/kafka-go` - Kafka/event integration.
- `github.com/ClickHouse/clickhouse-go/v2` - request-log and analytics surfaces.
- `github.com/prometheus/client_golang` and OpenTelemetry packages - metrics and tracing.
- `github.com/stripe/stripe-go/v82` - Stripe checkout/refund/provider lifecycle.
- `google.golang.org/grpc` and protobuf packages - Agent, Billing, RAG, Relay, Task, and Workflow service contracts.

**Critical frontend dependencies:**
- `react`, `react-dom`, `react-router-dom` - app shell and route surfaces.
- `swr` and `zustand` - frontend data/state patterns.
- `zod`, `react-hook-form`, and `@hookform/resolvers` - forms and validation.
- `@xyflow/react` - workflow/agent visual graph surfaces.
- `recharts` - analytics and dashboard visualization.
- `@testing-library/react`, `vitest`, `jsdom`, and `@playwright/test` - frontend verification.

## Configuration

**Environment:**
- Safe example variables live in `config/.env.example` and `deploy/docker/.env.example`.
- Do not read or commit real `.env` contents.
- DB-backed verification uses `TEST_DATABASE_URL`; strict local DB requirement uses `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- Vite API proxy target defaults to `http://127.0.0.1:8080` through `VITE_API_PROXY_TARGET`.

**Build and runtime config:**
- Root scripts in `package.json` wrap quality gates, commercial verifiers, and target evidence workflows.
- Frontend scripts in `src/web/package.json` define `dev`, `preview`, `build`, `test`, and `test:e2e`.
- Docker builds use root `Dockerfile.server`, `Dockerfile.web`, `Dockerfile.postgres-pgvector`, and service-specific Dockerfiles under `deploy/docker/`.
- Kubernetes resources live under `deploy/kubernetes/`.

## Platform Requirements

**Development:**
- Go 1.25, Node.js 20+, pnpm 10.6.0, PostgreSQL 14+.
- Optional local services through `docker-compose.yml`: PostgreSQL/pgvector, Redis, Qdrant, ClickHouse, Kafka, and microservice containers.

**Production / target release:**
- Docker and Kubernetes manifests under `deploy/`.
- External filled Kubernetes secrets; `deploy/kubernetes/secret.example.yaml` is an example only.
- Target evidence workdir outside git for manifests, logs, artifacts, provider proof, gRPC smoke, workflow telemetry, request-log proof, and secret audit.

## Verification Commands

- Install frontend deps: `pnpm install --frozen-lockfile`.
- General local checks: `bash scripts/check.sh all`.
- General local tests: `bash scripts/test.sh all`.
- Frontend build: `pnpm --dir src/web build`.
- Frontend unit tests: `pnpm --dir src/web test`.
- Browser E2E: `pnpm --dir src/web test:e2e`.
- Backend tests: `cd src/server && go test ./... -count=1`.
- Release docs/gates: `bash scripts/verify-quality-gates.sh`.
- Target evidence validation: `bash scripts/verify-target-release-evidence.sh`.

## Where To Add Stack-Related Code

- Backend domain logic: `src/server/internal/<domain>/`.
- Backend command entrypoints: `src/server/cmd/<service>/`.
- Public proto contracts: `api/proto/*.proto`.
- Frontend feature APIs: `src/web/src/features/<domain>/`.
- Frontend routes: `src/web/src/routes/`.
- Release verifiers and collectors: `scripts/`.
- Deployment resources: `deploy/docker/`, `deploy/kubernetes/`, and `deploy/observability/`.

---

*Stack analysis: 2026-07-13*

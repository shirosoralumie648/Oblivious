<!-- refreshed: 2026-07-13 -->
# Architecture

**Analysis Date:** 2026-07-13

## System Overview

Oblivious is a multi-tenant AI SaaS platform with a React/Vite frontend, Go HTTP/API backend, domain services, Relay-governed provider access, SQL/pgvector persistence, workflow/agent runtime, gRPC/microservice contracts, and release-evidence automation.

## Primary Layers

### Frontend layer
- User-facing route surfaces live under `src/web/src/routes/`.
- Domain API clients and frontend types live under `src/web/src/features/`.
- Browser E2E specs and fixtures live under `src/web/e2e/`.
- Vite proxy and test setup live in `src/web/vite.config.ts`.

### HTTP/API layer
- Main HTTP server entrypoint: `src/server/cmd/server/main.go`.
- Route registration and middleware: `src/server/internal/http/router.go` and related `*_handler.go` files.
- Public API documentation and contract surface: `docs/API.md`, `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`.
- API verification: `scripts/verify-openapi-contract.sh` and `scripts/verify_openapi_contract.py`.

### Domain layer
- Admin operations: `src/server/internal/admin/`.
- Agent runtime, tools, memory, approvals: `src/server/internal/agent/`.
- Auth and tenant identity: `src/server/internal/auth/`, `src/server/internal/tenant/`.
- Chat and Knowledge/RAG: `src/server/internal/chat/`, `src/server/internal/knowledge/`, `src/server/internal/rag/`.
- Billing, quota, payments, marketplace: `src/server/internal/billing/`, `src/server/internal/quota/`, `src/server/internal/payment/`, `src/server/internal/marketplace/`, `src/server/internal/stripe/`.
- Workflow and scheduled task runtime: `src/server/internal/workflow/`, `src/server/internal/schedule/`, `src/server/internal/task/`.
- Observability and metrics: `src/server/internal/observability/`, `src/server/internal/metrics/`, `src/server/pkg/metrics/`.

### Persistence and migration layer
- Migrations live under `src/server/migrations/`.
- Microservice split migration SQL lives under `src/server/migrations/microservices/`.
- ClickHouse migrations live under `src/server/migrations/clickhouse/`.
- Migration commands and service-specific wrappers live under `src/server/cmd/migrate/` and `scripts/migrate-*.sh`.

## Relay Invariant

Relay is the core commercial invariant: AI calls must pass through Relay so billing, rate limiting, audit, monitoring, and request logging remain unified across Chat, Agent, Knowledge RAG, and compatible `/v1/*` endpoints.

Use these locations for Relay-related work:
- Runtime: `src/server/internal/relay/`.
- Channel/provider paths: `src/server/internal/channel/`, `src/server/internal/gateway/`, and `src/server/internal/relay/channel*`.
- Security checks: `scripts/verify-relay-security.sh`.
- Release evidence: `scripts/collect-relay-realtime-evidence.sh`, `scripts/collect-relay-batch-evidence.sh`, and matching Python collectors.

Do not add direct provider calls in frontend pages, HTTP handlers, or agent tools without routing through Relay or a documented outbound policy.

## Tenant And Auth Boundary

- Auth/session behavior belongs under `src/server/internal/auth/` and HTTP middleware/tests under `src/server/internal/http/`.
- Tenant organization lifecycle belongs under `src/server/internal/tenant/`.
- Active organization / tenant scope must be carried through Chat, Knowledge, Agent, MCP, Console, Quota, Marketplace, Admin, and scheduled task surfaces.
- Commercial DB evidence profiles in `scripts/verify-commercial-db-evidence.sh` encode tenant-membership and cross-surface isolation checks.

## Workflow And Agent Runtime

- Workflow definitions, execution, versioning, debugging, failure handling, triggers, sandboxing, and node execution live under `src/server/internal/workflow/`.
- Agent runtime, runners, tools, memory, approvals, and model routing live under `src/server/internal/agent/`.
- Scheduled task runtime lives under `src/server/internal/schedule/` and `src/server/internal/task/`.
- Frontend workflow/agent pages live under `src/web/src/routes/workspace/WorkflowsPage.tsx`, `src/web/src/routes/workspace/AgentsPage.tsx`, and `src/web/src/routes/workspace/AgentPlanStepsPage.tsx`.

## gRPC And Microservice Boundaries

- Proto contracts live under `api/proto/*.proto`.
- Generated clients/servers live in `api/proto/*.pb.go` and internal generated packages.
- Runtime service adapters live under `src/server/internal/grpc/` and `src/server/pkg/`.
- Service command entrypoints live under `src/server/cmd/<service>/`.
- Service-specific Dockerfiles live under `deploy/docker/Dockerfile.<service>`.
- Kubernetes deployments live under `deploy/kubernetes/*-deployment.yaml`.
- Microservice boundary docs live in `docs/architecture/ADR-012-microservices-boundaries.md`.

## Release Evidence Architecture

- Local quality gates: `scripts/check.sh`, `scripts/test.sh`, `scripts/verify-quality-gates.sh`.
- Strict commercial verifier: `scripts/verify-commercial-completion.sh`.
- Target evidence runner: `scripts/run-target-release-evidence.sh`.
- Target manifest validator: `scripts/verify-target-release-evidence.sh` and `scripts/verify_target_release_evidence.py`.
- Evidence assembly: `scripts/assemble-target-release-evidence.sh` and `scripts/assemble_target_release_evidence.py`.
- Operator guidance: `docs/release/rc-checklist.md`, `docs/release/commercial-gates.md`, `docs/release/release-rollback-runbook.md`, and `docs/product/operator-guide.md`.

## Where To Extend Safely

- New backend feature: add service/store under `src/server/internal/<domain>/`, route under `src/server/internal/http/`, tests beside both, and OpenAPI docs if public.
- New frontend page: add route under `src/web/src/routes/`, API client/types under `src/web/src/features/<domain>/`, component tests beside the route, and E2E fixture only when browser flow matters.
- New provider/tool: add policy/Relay integration under `src/server/internal/relay/`, `src/server/internal/outboundpolicy/`, or `src/server/internal/agent/tools/`, with security and billing/quota tests.
- New release proof: add collector, assembler/validator updates, fixture mutation, fixture script, quality gate, and docs together.

---

*Architecture analysis: 2026-07-13*

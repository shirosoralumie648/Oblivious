# Codebase Structure

**Analysis Date:** 2026-07-13

## Top-Level Layout

- `api/proto/` - public protobuf contracts.
- `config/` - environment examples.
- `deploy/` - Docker, Kubernetes, observability assets.
- `docs/` - API, architecture, product, release, audit docs.
- `scripts/` - dev, test, migration, verification, evidence tools.
- `src/server/` - Go backend module.
- `src/web/` - React/Vite frontend workspace.
- `reference/` - nested reference repositories, not product source.
- `.planning/` - GSD project state, phases, orchestration records, and codebase maps.
- Root `Dockerfile.server`, `Dockerfile.web`, and `docker-compose.yml` define mainline local/container surfaces.

## Backend Layout

### Entrypoints
- `src/server/cmd/server/main.go` - main HTTP server.
- `src/server/cmd/migrate/` - migration command and validation.
- `src/server/cmd/grpc-smoke/` - gRPC smoke utility.
- `src/server/cmd/<service>/main.go` - service-specific microservice entrypoints.

### Internal packages
- `src/server/internal/http/` - HTTP routing, handlers, middleware, route tests, commercial journey tests.
- `src/server/internal/admin/` - admin operations, API tokens, billing inspection, channels, routes, pricing, usage logs, users.
- `src/server/internal/agent/` - Agent service, runner, tools, memory, approvals, runtime, model routing, persistence.
- `src/server/internal/workflow/` - workflow service, stores, node execution, versioning, triggers, sandbox, debug, failure handling.
- `src/server/internal/knowledge/` and `src/server/internal/rag/` - knowledge bases, document ingestion, embedding, retrieval, citations, RAG stores.
- `src/server/internal/relay/` - Relay routing, handlers, channels, cache, rate-limit, health, migrations, file mapping, provider boundaries.
- `src/server/internal/marketplace/`, `src/server/internal/billing/`, `src/server/internal/quota/`, `src/server/internal/payment/`, `src/server/internal/stripe/` - monetization and marketplace surfaces.
- `src/server/internal/auth/`, `src/server/internal/tenant/`, `src/server/internal/userprefs/` - identity, organization scope, preferences.
- `src/server/internal/observability/`, `src/server/internal/metrics/`, `src/server/internal/notification/`, `src/server/internal/usage/` - operations, alerting, metrics, request usage.
- `src/server/internal/outboundpolicy/`, `src/server/internal/mcp/`, `src/server/internal/ws/` - outbound/tool/websocket boundaries.

### Public/backend package layer
- `src/server/pkg/agent/`, `src/server/pkg/workflow/`, `src/server/pkg/task/`, `src/server/pkg/billing/`, `src/server/pkg/rag/`, `src/server/pkg/relay/`, `src/server/pkg/events/`, and `src/server/pkg/metrics/` contain package-level adapters and shared contracts.

### Migrations
- `src/server/migrations/` - primary DB migrations.
- `src/server/migrations/microservices/` - service-split migration SQL.
- `src/server/migrations/clickhouse/` - ClickHouse-specific migrations.

## Frontend Layout

- `src/web/src/app/` - app shell, router, providers, router-level tests.
- `src/web/src/routes/workspace/` - customer workspace pages including Chat, Agents, Workflows, Knowledge, Tasks, and settings-like work surfaces.
- `src/web/src/routes/admin/` - admin operations pages.
- `src/web/src/routes/marketing/` - public/auth/onboarding routes.
- `src/web/src/features/` - typed feature APIs, domain types, and feature tests.
- `src/web/src/components/` - shared UI components.
- `src/web/e2e/` - Playwright specs.
- `src/web/e2e/fixtures/` - route fixtures and browser data.
- `src/web/vite.config.ts` - Vite, proxy, alias, Vitest config.

## API And Contract Layout

- `api/proto/*.proto` - protobuf source contracts.
- `api/proto/*.pb.go` and `api/proto/*_grpc.pb.go` - generated Go proto output.
- `docs/api/openapi.yaml` - OpenAPI contract.
- `docs/api/route-surface-manifest.json` - route surface manifest.
- `scripts/verify-openapi-contract.sh` and `scripts/verify_openapi_contract.py` - API contract verification.

## Scripts Layout

- `scripts/check.sh` - main quality gate wrapper.
- `scripts/test.sh` - main test wrapper.
- `scripts/verify-*.sh` / `scripts/verify_*.py` - quality, security, contract, release, and evidence validators.
- `scripts/collect-*-evidence.sh` / `scripts/collect_*.py` - evidence collectors.
- `scripts/assemble-target-release-evidence.sh` / `scripts/assemble_target_release_evidence.py` - target evidence assembly.
- `scripts/*-fixtures.sh` and `scripts/target_release_fixture_mutations.py` - verifier fixture and negative-case coverage.
- `scripts/migrate-*.sh` - service migration wrappers.
- `scripts/deploy-*.sh`, `scripts/k8s-validate.sh`, `scripts/backup-restore-smoke.sh` - deploy/recovery operations.

## Deploy Layout

- `deploy/docker/` - service-specific Dockerfiles and Docker env examples.
- `deploy/kubernetes/` - namespace, configmap, secret example, app/web/service deployments, ingress, HPA, network policy, data services, and microservice deployments.
- `deploy/observability/` - Grafana dashboard and Prometheus alerts.

## Docs And Planning Layout

- `docs/product/` - public overview, onboarding, pricing, operator guide.
- `docs/release/` - commercial gates, RC checklist, release/rollback, backup/restore, incident response, DR, SLOs, evidence pack.
- `docs/architecture/` - current contracts, ADRs, integration paths, system design.
- `docs/audit/` and `docs/reports/` - implementation depth, reference audits, progress reports, gap matrices.
- `docs/superpowers/specs/` and `docs/superpowers/plans/` - design/implementation planning artifacts.
- `.planning/` - GSD project state, milestones, roadmap, phases, orchestration records, and `.planning/codebase/` maps.

## Reference Tree Caution

`reference/` contains nested upstream/reference repositories and their own `.git` and `.planning` folders. Use it only for reference-comparison tasks. Do not treat `reference/` paths as current Oblivious product implementation paths in audits, GSD plans, or completion claims.

## Where To Add New Code

- New HTTP endpoint: handler/test in `src/server/internal/http/`, service/store in `src/server/internal/<domain>/`, OpenAPI update in `docs/api/openapi.yaml`, route manifest update if required.
- New backend domain: package under `src/server/internal/<domain>/`, tests beside code, optional public adapter under `src/server/pkg/<domain>/`.
- New microservice: command under `src/server/cmd/<service>/`, Dockerfile under `deploy/docker/`, Kubernetes deployment under `deploy/kubernetes/`, migration wrapper under `scripts/migrate-<service>.sh`.
- New frontend route: `src/web/src/routes/<area>/`, API client/types under `src/web/src/features/<domain>/`, route tests beside the page.
- New browser journey: spec under `src/web/e2e/`, fixture under `src/web/e2e/fixtures/`, backend/API contract tests for the same contract.
- New release evidence: collector, assembler/validator, fixtures, quality gate, and `docs/release/` update.
- New proto API: edit `api/proto/*.proto`, regenerate Go output, update adapters under `src/server/internal/grpc/` or `src/server/pkg/`, and add smoke/adapter tests.

---

*Structure analysis: 2026-07-13*

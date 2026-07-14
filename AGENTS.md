<!-- GSD:project-start source:PROJECT.md -->

## Project

**Oblivious Commercial Complete & Target Release**

Oblivious 是一个面向组织工作区的 multi-tenant AI SaaS 平台。它在统一产品中提供 Chat、Knowledge RAG、Agent/SOLO、Workflow、Task、MCP 工具、多渠道发布、Admin 和 Marketplace，并通过 Relay 统一所有可计费 AI 操作的 Provider 路由、鉴权、限流、配额、计价、用量、审计和监控。

本项目不是从零重构，也不是把现有仓库缩减成 MVP 或 RC。目标是在保留当前 brownfield 实现和有效技术决策的基础上，补齐真实运行断点、商业与运维闭环、声明部署模式的能力对等，以及目标环境 no-skip 发布证据，最终形成可直接部署、可收费、可运营、可审计、可恢复的商业产品。

**Core Value:** 让组织客户能够可靠地构建、运行并商业化 AI 应用，同时让每一次 AI 操作都可隔离、可计费、可追踪、可审计、可恢复。

### Constraints

- **Mainline**: 产品和发布范围是 `src/server`、`src/web`、`api/proto`、`config`、`scripts`、`deploy`、`.github/workflows` 和产品文档；`reference/` 不属于实现或发布证据。
- **Stack**: 延续当前 Go 1.25、React/TypeScript、PostgreSQL/pgvector 及已采用的 Redis、Qdrant、ClickHouse、Kafka、Docker/Kubernetes 集成，除非迁移有可验证收益和回滚路径。
- **Security**: secrets 和原始 target evidence 保持在仓库外；生产配置不得使用 placeholder、sample、fake 或本地 fallback。
- **Compatibility**: 当前 `/v1/*`、产品 API、OpenAPI、数据库迁移和客户端契约不能被旧设计路径覆盖。
- **Delivery**: 按独立可验证模块拆分；并行工作必须保持接口契约一致。每个切片都运行相关测试、更新必要文档、执行 `git diff` 自查、原子 commit，并在验证通过后 push。
- **Claim discipline**: 任何 readiness 结论必须说明 evidence class、环境、命令、迁移状态、通过/失败、skip 和残余风险；缺少 target/live proof 时只能声明 repository-local progress。
- **Change safety**: 保留无关用户改动、历史规划证据和未明确授权的外部环境状态，不通过清理工作树误删证据或秘密。

<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->

## Technology Stack

## Languages

- Go 1.25.0 - backend services, CLI commands, HTTP handlers, gRPC adapters, persistence, Relay, Agent, Workflow, Knowledge, Marketplace, Billing, and operations code under `src/server/`.
- TypeScript - React frontend routes, feature API clients, unit tests, and Playwright fixtures under `src/web/src/` and `src/web/e2e/`.
- Python 3 - target evidence collectors, manifest validators, OpenAPI validators, and release automation under `scripts/*.py`.
- Bash - migration, deploy, release, verification, and evidence entrypoints under `scripts/*.sh`.
- YAML/JSON - Kubernetes, observability, OpenAPI, route manifests, package manifests, and release config under `deploy/`, `docs/api/`, and root config files.

## Runtime

- Go module: `src/server/go.mod`.
- Main HTTP server entrypoint: `src/server/cmd/server/main.go`.
- Service command entrypoints: `src/server/cmd/chat/`, `src/server/cmd/agent/`, `src/server/cmd/relay/`, `src/server/cmd/rag/`, `src/server/cmd/workflow/`, `src/server/cmd/task/`, `src/server/cmd/billing/`, `src/server/cmd/channel/`, `src/server/cmd/gateway/`, `src/server/cmd/marketplace/`, and `src/server/cmd/observability/`.
- Node 20+ and pnpm 10.6.0 per `README.md` and root `package.json`.
- Vite React app under `src/web/`.
- Vite dev/preview proxy forwards `/api` and `/v1` to `VITE_API_PROXY_TARGET` in `src/web/vite.config.ts`.

## Package Managers And Lockfiles

- Go modules: `src/server/go.mod` and `src/server/go.sum`.
- pnpm workspace: `pnpm-workspace.yaml`, `pnpm-lock.yaml`, and root `package.json`.
- npm lockfiles are also tracked: root `package-lock.json` and `src/web/package-lock.json`.
- The root pnpm workspace points at `src/web`; backend dependency management is independent through Go modules.

## Frameworks

- Gin HTTP framework via `github.com/gin-gonic/gin` in `src/server/go.mod`.
- GORM and SQL drivers for persistence, including PostgreSQL `lib/pq` and MySQL driver dependencies in `src/server/go.mod`.
- gRPC/protobuf via `google.golang.org/grpc`, `google.golang.org/protobuf`, and service definitions under `api/proto/*.proto`.
- Redis/Asynq/Kafka integrations through dependencies in `src/server/go.mod`.
- Prometheus and OpenTelemetry packages for metrics and tracing.
- Stripe Go SDK for payment/provider lifecycle paths.
- React 18, React Router, Vite, TypeScript, Tailwind CSS 3, SWR, Zustand, Zod, React Hook Form, Radix UI, XYFlow, Recharts, GSAP, Sonner, and cmdk from `src/web/package.json`.
- Vite config and test setup are in `src/web/vite.config.ts` and `src/web/src/test/setup.ts`.
- Go `testing` packages under `src/server/**/*_test.go`.
- Vitest/jsdom for frontend unit and component tests.
- Playwright for browser E2E under `src/web/e2e/`.
- Shell/Python release fixtures under `scripts/*-fixtures.sh` and `scripts/target_release_fixture_mutations.py`.

## Key Dependencies

- `github.com/gin-gonic/gin` - HTTP routing and middleware.
- `gorm.io/gorm`, `gorm.io/driver/mysql`, `github.com/lib/pq` - persistence.
- `github.com/redis/go-redis/v9` and `github.com/hibiken/asynq` - Redis-backed queue/runtime support.
- `github.com/segmentio/kafka-go` - Kafka/event integration.
- `github.com/ClickHouse/clickhouse-go/v2` - request-log and analytics surfaces.
- `github.com/prometheus/client_golang` and OpenTelemetry packages - metrics and tracing.
- `github.com/stripe/stripe-go/v82` - Stripe checkout/refund/provider lifecycle.
- `google.golang.org/grpc` and protobuf packages - Agent, Billing, RAG, Relay, Task, and Workflow service contracts.
- `react`, `react-dom`, `react-router-dom` - app shell and route surfaces.
- `swr` and `zustand` - frontend data/state patterns.
- `zod`, `react-hook-form`, and `@hookform/resolvers` - forms and validation.
- `@xyflow/react` - workflow/agent visual graph surfaces.
- `recharts` - analytics and dashboard visualization.
- `@testing-library/react`, `vitest`, `jsdom`, and `@playwright/test` - frontend verification.

## Configuration

- Safe example variables live in `config/.env.example` and `deploy/docker/.env.example`.
- Do not read or commit real `.env` contents.
- DB-backed verification uses `TEST_DATABASE_URL`; strict local DB requirement uses `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- Vite API proxy target defaults to `http://127.0.0.1:8080` through `VITE_API_PROXY_TARGET`.
- Root scripts in `package.json` wrap quality gates, commercial verifiers, and target evidence workflows.
- Frontend scripts in `src/web/package.json` define `dev`, `preview`, `build`, `test`, and `test:e2e`.
- Docker builds use root `Dockerfile.server`, `Dockerfile.web`, `Dockerfile.postgres-pgvector`, and service-specific Dockerfiles under `deploy/docker/`.
- Kubernetes resources live under `deploy/kubernetes/`.

## Platform Requirements

- Go 1.25, Node.js 20+, pnpm 10.6.0, PostgreSQL 14+.
- Optional local services through `docker-compose.yml`: PostgreSQL/pgvector, Redis, Qdrant, ClickHouse, Kafka, and microservice containers.
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

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

## Operating Principle

## Go Backend Conventions

### Package placement

- Put HTTP route wiring and handlers under `src/server/internal/http/`; use focused handler files rather than expanding `src/server/internal/http/router.go` unless adding route registration.
- Put domain services under `src/server/internal/<domain>/`, following existing domains such as `src/server/internal/agent/`, `src/server/internal/workflow/`, `src/server/internal/knowledge/`, `src/server/internal/marketplace/`, `src/server/internal/quota/`, and `src/server/internal/relay/`.
- Put gRPC runtime adapters under `src/server/internal/grpc/<service>v1/` or package-level adapters under `src/server/pkg/<service>/`; keep generated protobuf output under `api/proto/` and `src/server/internal/grpc/*/*.pb.go`.
- Put persistence tests beside the store implementation, for example `src/server/internal/agent/store_test.go`, `src/server/internal/workflow/store.go`, and `src/server/internal/knowledge/store_test.go`.

### Service and store style

- Keep orchestration in service types and persistence in store types. Use files like `src/server/internal/agent/service.go`, `src/server/internal/workflow/service.go`, and `src/server/internal/marketplace/settlement.go` as examples of domain ownership.
- Prefer explicit constructor dependencies over global state. When behavior touches billing, quota, Relay, or tenants, pass the required service/store dependency rather than looking it up indirectly.
- Preserve table-driven tests with `t.Run` in Go packages; most backend coverage follows `*_test.go` files under the same package tree.
- Use `context.Context` through service and store boundaries, especially for SQL, Relay, workflow, and agent execution paths.

### Error and response handling

- Keep HTTP response envelopes and error contracts consistent with existing handlers under `src/server/internal/http/`.
- Add OpenAPI and route-surface updates when changing public API behavior. Contract sources live in `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`, and `scripts/verify_openapi_contract.py`.
- Prefer narrow handler tests in `src/server/internal/http/*_test.go` for route behavior, auth, tenant scoping, query serialization, and response shape.

## Tenant, Auth, And Commercial Boundaries

- Treat active organization / tenant scope as required for customer data. Existing coverage includes tenant-oriented profile names in `scripts/verify-commercial-db-evidence.sh`.
- Use session/auth boundaries from `src/server/internal/auth/` and `src/server/internal/http/auth_middleware_test.go` rather than ad hoc user IDs.
- For tenant-sensitive changes, add tests against the relevant store and HTTP route. Examples include `src/server/internal/tenant/`, `src/server/internal/quota/`, `src/server/internal/http/server_test.go`, and `src/server/internal/http/commercial_journey_test.go`.
- Do not close tenant-isolation work with frontend fixture tests only.

## Relay And Provider Conventions

- All AI/provider execution should route through Relay or an explicit policy boundary. Relay code is concentrated under `src/server/internal/relay/`.
- Outbound URL/tool behavior belongs behind `src/server/internal/outboundpolicy/`, `src/server/internal/mcp/websearch/`, or `src/server/internal/agent/tools/` rather than scattered handler code.
- Provider and payment behavior should use existing provider-specific packages such as `src/server/internal/stripe/`, `src/server/internal/payment/`, `src/server/internal/billing/`, and `src/server/internal/marketplace/`.
- When adding provider behavior, include quota/billing/request-log assertions and update release evidence profiles where appropriate.

## TypeScript / React Conventions

### Frontend structure

- Keep route pages under `src/web/src/routes/`; workspace pages live under `src/web/src/routes/workspace/`, admin pages under `src/web/src/routes/admin/`, and marketing/auth pages under `src/web/src/routes/marketing/`.
- Keep API clients and domain types under `src/web/src/features/<domain>/`, for example `src/web/src/features/admin/api.ts`, `src/web/src/features/agents/agentsApi.ts`, and `src/web/src/features/workflows/workflowsApi.ts`.
- Keep shared app wiring under `src/web/src/app/`; router tests under `src/web/src/app/router.test.tsx` should not become the only proof for backend behavior.
- Use the `@` alias from `src/web/vite.config.ts` for frontend source imports when it improves clarity.

### Component and state style

- Prefer typed API client functions over inline `fetch` calls inside pages.
- Keep user-visible loading, empty, recovery, quota, budget, settlement, and review boundary copy in route components where users interact with it.
- Keep schema validation and form behavior close to the feature page or feature API; frontend dependencies include `zod`, `react-hook-form`, `@hookform/resolvers`, `swr`, `zustand`, `react-router-dom`, and `@xyflow/react`.
- Avoid expanding already-large pages such as `src/web/src/routes/workspace/WorkflowsPage.tsx` without extracting smaller helpers or feature-level API utilities.

## Release Evidence And Docs Conventions

- Put operator-facing release instructions under `docs/release/`; key files include `docs/release/rc-checklist.md`, `docs/release/commercial-completion-audit.md`, and `docs/release/release-rollback-runbook.md`.
- Put architecture contracts under `docs/architecture/`; `docs/architecture/current-system-contracts.md` is the high-level evidence map for runtime contracts.
- Put product-facing docs under `docs/product/`.
- Put verifier implementations under `scripts/` and keep shell wrappers fail-closed with `set -euo pipefail`.
- Keep target/live proof outside git. Use `config/.env.example` and `deploy/kubernetes/secret.example.yaml` as examples only.

## Environment And Tooling Conventions

- Use `scripts/check.sh` for quality checks and `scripts/test.sh` for test entrypoints. Both set `COREPACK_HOME`, `GOCACHE`, and `GOMODCACHE` defaults under `.tmp/`.
- Use `TEST_DATABASE_URL` for DB-backed Go tests. `scripts/test.sh` runs unit tests and explicitly skips integration tests when the variable is absent unless `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- Use `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` when a system Chrome/Chromium install is preferred for browser E2E.
- Keep package-manager updates synchronized across `pnpm-lock.yaml`, root `package-lock.json`, and `src/web/package-lock.json` when the dependency security gate inspects them.

## Naming And Placement Rules

- Go packages use lowercase directory names under `src/server/internal/`; exported symbols should be reserved for cross-package use.
- HTTP route tests should name the route behavior explicitly, as seen in `src/server/internal/http/*_test.go`.
- Frontend test files should sit beside the page/feature when unit-level, or under `src/web/e2e/` when browser-level.
- Release scripts should use verb-object names such as `verify-*.sh`, `collect-*-evidence.sh`, `assemble-*.sh`, and keep paired fixture scripts with the `-fixtures.sh` suffix.

## Anti-Patterns

- Do not bypass Relay for new AI/provider calls.
- Do not treat `reference/` code as product implementation without explicit reference-analysis scope.
- Do not read or commit real `.env`, provider keys, Kubernetes secrets, target manifests, backup dumps, or downloaded target artifacts.
- Do not count local fixture scripts as target/live evidence.
- Do not add frontend-only fixture proof for backend persistence, billing, provider rails, tenant isolation, or release readiness.

<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

## System Overview

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

- Runtime: `src/server/internal/relay/`.
- Channel/provider paths: `src/server/internal/channel/`, `src/server/internal/gateway/`, and `src/server/internal/relay/channel*`.
- Security checks: `scripts/verify-relay-security.sh`.
- Release evidence: `scripts/collect-relay-realtime-evidence.sh`, `scripts/collect-relay-batch-evidence.sh`, and matching Python collectors.

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

<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

| Skill | Description | Path |
|-------|-------------|------|
| "AgentDB Advanced Features" | "Master advanced AgentDB features including QUIC synchronization, multi-database management, custom distance metrics, hybrid search, and distributed systems integration. Use when building distributed AI systems, multi-agent coordination, or advanced vector search applications." | `.claude/skills/agentdb-advanced/SKILL.md` |
| "AgentDB Learning Plugins" | "Create and train AI learning plugins with AgentDB's 9 reinforcement learning algorithms. Includes Decision Transformer, Q-Learning, SARSA, Actor-Critic, and more. Use when building self-learning agents, implementing RL, or optimizing agent behavior through experience." | `.claude/skills/agentdb-learning/SKILL.md` |
| "AgentDB Memory Patterns" | "Implement persistent memory patterns for AI agents using AgentDB. Includes session memory, long-term storage, pattern learning, and context management. Use when building stateful agents, chat systems, or intelligent assistants." | `.claude/skills/agentdb-memory-patterns/SKILL.md` |
| "AgentDB Performance Optimization" | "Optimize AgentDB performance with quantization (4-32x memory reduction), HNSW indexing (150x faster search), caching, and batch operations. Use when optimizing memory usage, improving search speed, or scaling to millions of vectors." | `.claude/skills/agentdb-optimization/SKILL.md` |
| "AgentDB Vector Search" | "Implement semantic vector search with AgentDB for intelligent document retrieval, similarity matching, and context-aware querying. Use when building RAG systems, semantic search engines, or intelligent knowledge bases." | `.claude/skills/agentdb-vector-search/SKILL.md` |
| browser | Web browser automation with AI-optimized snapshots for claude-flow agents | `.claude/skills/browser/SKILL.md` |
| github-code-review | Comprehensive GitHub code review with AI-powered swarm coordination | `.claude/skills/github-code-review/SKILL.md` |
| github-multi-repo | Multi-repository coordination, synchronization, and architecture management with AI swarm orchestration | `.claude/skills/github-multi-repo/SKILL.md` |
| github-project-management | Comprehensive GitHub project management with swarm-coordinated issue tracking, project board automation, and sprint planning | `.claude/skills/github-project-management/SKILL.md` |
| github-release-management | Comprehensive GitHub release orchestration with AI swarm coordination for automated versioning, testing, deployment, and rollback management | `.claude/skills/github-release-management/SKILL.md` |
| github-workflow-automation | Advanced GitHub Actions workflow automation with AI swarm coordination, intelligent CI/CD pipelines, and comprehensive repository management | `.claude/skills/github-workflow-automation/SKILL.md` |
| Hooks Automation | Automated coordination, formatting, and learning from Claude Code operations using intelligent hooks with MCP integration. Includes pre/post task hooks, session management, Git integration, memory coordination, and neural pattern training for enhanced development workflows. | `.claude/skills/hooks-automation/SKILL.md` |
| Pair Programming | AI-assisted pair programming with multiple modes (driver/navigator/switch), real-time verification, quality monitoring, and comprehensive testing. Supports TDD, debugging, refactoring, and learning sessions. Features automatic role switching, continuous code review, security scanning, and performance optimization with truth-score verification. | `.claude/skills/pair-programming/SKILL.md` |
| "ReasoningBank with AgentDB" | "Implement ReasoningBank adaptive learning with AgentDB's 150x faster vector database. Includes trajectory tracking, verdict judgment, memory distillation, and pattern recognition. Use when building self-learning agents, optimizing decision-making, or implementing experience replay systems." | `.claude/skills/reasoningbank-agentdb/SKILL.md` |
| "ReasoningBank Intelligence" | "Implement adaptive learning with ReasoningBank for pattern recognition, strategy optimization, and continuous improvement. Use when building self-learning agents, optimizing workflows, or implementing meta-cognitive systems." | `.claude/skills/reasoningbank-intelligence/SKILL.md` |
| "Skill Builder" | "Create new Claude Code Skills with proper YAML frontmatter, progressive disclosure structure, and complete directory organization. Use when you need to build custom skills for specific workflows, generate skill templates, or understand the Claude Skills specification." | `.claude/skills/skill-builder/SKILL.md` |
| sparc-methodology | SPARC (Specification, Pseudocode, Architecture, Refinement, Completion) comprehensive development methodology with multi-agent orchestration | `.claude/skills/sparc-methodology/SKILL.md` |
| stream-chain | Stream-JSON chaining for multi-agent pipelines, data transformation, and sequential workflows | `.claude/skills/stream-chain/SKILL.md` |
| swarm-advanced | Advanced swarm orchestration patterns for research, development, testing, and complex distributed workflows | `.claude/skills/swarm-advanced/SKILL.md` |
| "Swarm Orchestration" | "Orchestrate multi-agent swarms with agentic-flow for parallel task execution, dynamic topology, and intelligent coordination. Use when scaling beyond single agents, implementing complex workflows, or building distributed AI systems." | `.claude/skills/swarm-orchestration/SKILL.md` |
| "V3 CLI Modernization" | "CLI modernization and hooks system enhancement for claude-flow v3. Implements interactive prompts, command decomposition, enhanced hooks integration, and intelligent workflow automation." | `.claude/skills/v3-cli-modernization/SKILL.md` |
| "V3 Core Implementation" | "Core module implementation for claude-flow v3. Implements DDD domains, clean architecture patterns, dependency injection, and modular TypeScript codebase with comprehensive testing." | `.claude/skills/v3-core-implementation/SKILL.md` |
| "V3 DDD Architecture" | "Domain-Driven Design architecture for claude-flow v3. Implements modular, bounded context architecture with clean separation of concerns and microkernel pattern." | `.claude/skills/v3-ddd-architecture/SKILL.md` |
| "V3 Deep Integration" | "Deep agentic-flow@alpha integration implementing ADR-001. Eliminates 10,000+ duplicate lines by building claude-flow as specialized extension rather than parallel implementation." | `.claude/skills/v3-integration-deep/SKILL.md` |
| "V3 MCP Optimization" | "MCP server optimization and transport layer enhancement for claude-flow v3. Implements connection pooling, load balancing, tool registry optimization, and performance monitoring for sub-100ms response times." | `.claude/skills/v3-mcp-optimization/SKILL.md` |
| "V3 Memory Unification" | "Unify 6+ memory systems into AgentDB with HNSW indexing for 150x-12,500x search improvements. Implements ADR-006 (Unified Memory Service) and ADR-009 (Hybrid Memory Backend)." | `.claude/skills/v3-memory-unification/SKILL.md` |
| "V3 Performance Optimization" | "Achieve aggressive v3 performance targets: 2.49x-7.47x Flash Attention speedup, 150x-12,500x search improvements, 50-75% memory reduction. Comprehensive benchmarking and optimization suite." | `.claude/skills/v3-performance-optimization/SKILL.md` |
| "V3 Security Overhaul" | "Complete security architecture overhaul for claude-flow v3. Addresses critical CVEs (CVE-1, CVE-2, CVE-3) and implements secure-by-default patterns. Use for security-first v3 implementation." | `.claude/skills/v3-security-overhaul/SKILL.md` |
| "V3 Swarm Coordination" | "15-agent hierarchical mesh coordination for v3 implementation. Orchestrates parallel execution across security, core, and integration domains following 10 ADRs with 14-week timeline." | `.claude/skills/v3-swarm-coordination/SKILL.md` |
| "Verification & Quality Assurance" | "Comprehensive truth scoring, code quality verification, and automatic rollback system with 0.95 accuracy threshold for ensuring high-quality agent outputs and codebase reliability." | `.claude/skills/verification-quality/SKILL.md` |
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->

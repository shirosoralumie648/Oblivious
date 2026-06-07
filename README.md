# Oblivious

Oblivious 是一个面向工作区的 multi-tenant AI SaaS 平台，提供 Chat、Agent、Knowledge RAG、Relay、Admin、Marketplace 和商业化运营能力。后端使用 Go 构建，前端基于 React，PostgreSQL 作为数据存储。

核心商业不变量是 Relay：所有 AI 调用必须经过 Relay，让计费、限流、审计和监控在 Chat、Agent workflows、Knowledge RAG 与受支持的 `/v1/*` 端点上保持统一。

## Mainline Boundary

当前主线覆盖：

- `src/server`
- `src/web`
- `config`
- `scripts`
- `.github/workflows`

`lobehub/` 和 `new-api/` 是仓库内参考目录，不属于根 workspace、根 CI 或发布范围。

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25, net/http |
| 前端 | React, TypeScript, pnpm |
| 数据库 | PostgreSQL 14+ |
| 构建工具 | pnpm 10.6.0, Go modules |
| 监控 | Prometheus, Grafana |
| 部署 | Docker, Kubernetes |

## 项目结构

```
src/server   - Go API 服务、数据库迁移、领域服务
src/web      - React 前端应用
config       - 环境变量模板
scripts      - 开发与 CI 脚本
deploy       - Docker、Kubernetes、可观测性配置
docs         - 架构文档、API 文档、发布指南
```

## Product Surfaces

- Chat workspace：模型配置、Knowledge 绑定、SOLO handoff、quota-aware errors、Relay-backed provider access。
- Agent 和 SOLO workflows：durable `agent_runs`、`agent_tool_runs`、approval/reject/retry state、memory evidence、tool boundaries、budget context。
- Knowledge：Relay embeddings、pgvector retrieval、`embedding_rag` metadata、source citations。
- MCP built-ins：`calculator` 与 `datetime` 是默认商业内置工具；`web_search` 与 `http_request` 在配置真实 provider 或 tenant-safe outbound policy 前默认禁用。
- Admin operations：channels、routes、plans、billing inspection、users、audit logs、Marketplace review queues。
- Marketplace：browse、publish、review、install、owner stats、governance、paid-install order handling、settlement、payout、refund-impact evidence。
- Production operations evidence：compose validation、Kubernetes manifest validation path、backup/restore、observability、release/rollback、incident response、disaster recovery。

## Quick Start / 快速开始

### 环境要求

- Go 1.25
- Node.js 20+
- pnpm 10.6.0
- PostgreSQL 14+

### 安装与运行

```bash
# 1. 安装依赖
pnpm install --frozen-lockfile

# 2. 配置环境变量
cp config/.env.example .env
# 编辑 .env 填入数据库连接等配置

# 3. 执行数据库迁移
(cd src/server && go run ./cmd/migrate)

# 4. 启动开发服务器
bash scripts/dev.sh
```

服务启动后访问 `http://localhost:8080`。

## 开发指南

## Quality Gates

运行与 CI 一致的检查命令，确保代码质量：

```bash
bash scripts/check.sh
bash scripts/test.sh
```

`bash scripts/check.sh` 会检查发布资产完整性、文档一致性、前端生产构建、后端单元/契约测试。

`bash scripts/test.sh` 会执行前端 Vitest、后端单元测试与 HTTP 集成测试。如未设置 `TEST_DATABASE_URL`，本地集成测试会按规则显式跳过；CI 设置 `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` 时必须运行数据库集成覆盖。

商业 readiness gate 定义在 [docs/release/commercial-gates.md](docs/release/commercial-gates.md)。当前 HTTP surface 索引在 [docs/API.md](docs/API.md)。历史 RC readiness checklist 见 [docs/release/rc-checklist.md](docs/release/rc-checklist.md)。

### 运行测试

```bash
bash scripts/test.sh
```

该脚本依次执行：前端 Vitest 测试、后端单元测试、HTTP 集成测试。如未设置 `TEST_DATABASE_URL` 环境变量，集成测试将被跳过。

### API 文档

完整的 API 文档参见 [docs/API.md](docs/API.md) 和 [docs/api/README.md](docs/api/README.md)，OpenAPI 3.0 规范文件位于 [docs/api/openapi.yaml](docs/api/openapi.yaml)。

## Commercial Documentation

- [docs/product/public-overview.md](docs/product/public-overview.md): public product overview for Chat, Agent, Knowledge RAG, Relay, Admin, Marketplace, billing, and operations
- [docs/product/onboarding.md](docs/product/onboarding.md): customer, admin, publisher, and operator onboarding paths
- [docs/product/pricing.md](docs/product/pricing.md): subscription, top-up, quota, invoice, refund, and Marketplace settlement model
- [docs/product/operator-guide.md](docs/product/operator-guide.md): deploy, backup, restore, observability, release, rollback, incident, and disaster recovery index
- [docs/API.md](docs/API.md): current routed HTTP API index
- [docs/architecture/current-system-contracts.md](docs/architecture/current-system-contracts.md): current API and runtime contract baseline
- [docs/release/commercial-gates.md](docs/release/commercial-gates.md): commercial-readiness gate contract
- [docs/release/release-rollback-runbook.md](docs/release/release-rollback-runbook.md): release and rollback procedure
- [docs/release/backup-restore-runbook.md](docs/release/backup-restore-runbook.md): PostgreSQL backup and restore procedure
- [docs/release/observability-slos.md](docs/release/observability-slos.md): observability, alert, dashboard, and SLO contract
- [docs/release/incident-response-runbook.md](docs/release/incident-response-runbook.md): incident response procedure
- [docs/release/disaster-recovery-runbook.md](docs/release/disaster-recovery-runbook.md): disaster recovery procedure

## Completion Boundary

Phase 30 已完成当前 v08 Product Completeness 的端到端商业旅程与 `AUDIT-01` 仓库内证据闭环，但 external Prometheus/Grafana/OTel/error-tracking deployment、真实 provider keys 和 live runtime smoke 仍属于环境相关证据。

`no-final-readiness`: 不要在缺少每个商业 gate 的当前仓库证据、自动化验证和适用 runtime smoke 前声称最终商业 readiness。

## 核心功能

- **对话 (Chat)** - 与 AI 模型进行多轮对话，支持模型切换、参数调节、知识库关联
- **知识库 (Knowledge)** - 创建和管理知识库，上传文档，基于语义的检索问答
- **任务 (Task)** - SOLO 任务编排，支持自动/半自动/手动执行模式，预算控制与审批流程
- **控制台 (Console)** - 使用量统计、账单管理、模型概览、访问权限控制
- **用户偏好** - 默认模式、模型策略、引导流程等个性化配置

## 相关文档

- [API 文档](docs/api/README.md)
- [系统架构](docs/architecture/current-system-contracts.md)
- [RC 发布检查清单](docs/release/rc-checklist.md)
- [商业门禁](docs/release/commercial-gates.md)
- [可观测性 SLO](docs/release/observability-slos.md)
- [事故响应手册](docs/release/incident-response-runbook.md)
- [运维指南](docs/product/operator-guide.md)

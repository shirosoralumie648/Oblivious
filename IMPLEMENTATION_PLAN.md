> **⚠️ DEPRECATED (2026-07-26)**
> 本文件是 2026-06 微服务改造计划的历史存档，已与现状脱节。
> 当前规划权威见 `.planning/ROADMAP.md` + `.planning/EXECUTION-STRATEGY.md`，入口见 `goal.md`。
> **请勿据此执行。** 正文保留仅供历史追溯。

---

# Oblivious 完成计划

> 生成于 2026-06-11，基于 goal.md + 4 份设计文档 + 3 路架构侦察
> 执行原则：按模块拆分、每步测试、git diff 自查、commit + push

## 现状总结

### 已完成（Stage A/B/C0/C1/D1）✅
- 内置工具 170 个（15 类）
- 代码沙箱 8 语言
- Web 搜索 15 提供商客户端 + 回退链
- ModelRouter/SkillSelector 纯组件
- deepdoc 结构感知解析
- K8s 基础设施清单（PgBouncer/MinIO/Kafka）
- 服务边界 ADR + proto 契约骨架
- Next.js 迁移指南

### 待完成（本次执行）

**架构改造（C2-C4）：**
- C2: Database per Service（12 服务独立库 + 迁移脚本）
- C3: 12 微服务独立部署（cmd 入口 + Dockerfile + K8s Deployment）
- C4: gRPC 服务间调用 + Kafka 事件总线

**前端改造（D2）：**
- Zustand 替换自定义状态
- SWR 替换手动 fetch
- Recharts 替换手写图表
- React Hook Form + Zod 替换手动表单

**深度集成（Phase 2）：**
- B3: ModelRouter/SkillSelector 进 agent runner 循环 + call_agent 工具
- B2: websearch 动态提供商选择进工具执行
- B4: deepdoc 进知识库上传管线

---

## 执行策略

### 原则
1. **最小改动单元**：每个 commit 是一个可验证的功能片
2. **测试先行**：改动后立即跑相关测试（Go: `go test ./...`，Web: `npm test`）
3. **双模式共存**：架构改造期间保持单体模式可运行（通过环境变量切换）
4. **并行窗口**：独立模块（前端 D2、服务端 C2-C4、Phase 2）可并行，但同一模块内串行

### 风险控制
- 每个 agent 任务 ≤ 5 文件改动
- 复杂任务（如 C3 全 12 服务部署）拆成 12 个串行 agent
- 测试失败立即停止、回滚、分析
- 每完成一个 Stage 推送一次

---

## 阶段拆分

### Stage C2: Database per Service（Week 1-2，估计 15-20 commits）

**目标**：12 个逻辑库 schema + 双写验证器

**任务分解**：
1. **C2.1** 定义 12 服务的数据库 schema 文件（`migrations/microservices/gateway.sql` ... `observability.sql`）
   - 从现有 80 个迁移文件提取表归属（按规格表归属映射）
   - 每个服务一个 SQL 文件，包含 CREATE TABLE + 索引
   - 验证：SQL 语法检查（`psql --dry-run`）

2. **C2.2** 实现双写验证器框架（`internal/migration/validator.go`）
   - `Validator` 接口：`Validate(ctx, legacyDB, newDB) error`
   - 实现 `ConversationValidator`、`BillingValidator` 等 12 个
   - 验证：单测 mock 双写场景

3. **C2.3-C2.14** 逐服务迁移脚本（每个服务 1 commit）
   - 脚本：`scripts/migrate-[service].sh`（读取旧库 → 写新库 → 调用 Validator）
   - 顺序：Gateway（无表）→ Admin → Billing → Task → Schedule → Observability → Chat → Relay → Knowledge → Workflow → Agent → Marketplace → Channel
   - 验证：每个脚本跑在测试 DB，检查行数一致性

4. **C2.15** 集成测试：双写模式下全套 Go 测试通过
   - 环境变量：`OBLIVIOUS_DB_MODE=dual_write`
   - 验证：`go test ./... -tags=integration`

**交付物**：
- `migrations/microservices/*.sql` (12 文件)
- `internal/migration/validator.go` + 12 validator 实现
- `scripts/migrate-*.sh` (12 脚本)
- 更新 `docs/design/FUSION_GAP_CLOSURE_PLAN.md` Stage C2 状态

---

### Stage C3: 12 微服务独立部署（Week 2-3，估计 25-30 commits）

**目标**：每个服务独立进程 + Dockerfile + K8s Deployment

**任务分解**：
1. **C3.0** 重构 `internal/config` 为 per-service config
   - 提取公共 config：`pkg/config/common.go`（DB、Auth、Observability）
   - 每个服务 config：`pkg/config/[service].go`（特定字段）
   - 验证：现有单体 config 仍能工作

2. **C3.1-C3.12** 逐服务创建 cmd 入口 + Dockerfile（每个服务 2 commits）
   - **Gateway Service**:
     - `cmd/gateway/main.go`：启动 HTTP 路由器、认证中间件、限流
     - `deploy/docker/Dockerfile.gateway`：multi-stage build（Go 1.25 alpine）
     - `deploy/kubernetes/gateway-deployment.yaml`：副本数 3、HPA 3-10、资源请求 500m/512Mi
   - **Relay Service**: cmd/relay、Dockerfile.relay、relay-deployment.yaml（副本 5、HPA 5-20、1000m/1Gi）
   - **Chat / Workflow / RAG / Agent Services**: 各 3 副本、HPA 3-10
   - **Billing / Marketplace / Admin / Channel / Task / Observability**: 各 2 副本、HPA 2-5
   - 验证：每个 Dockerfile 能构建、镜像运行起 health check 通过

3. **C3.13** 更新 docker-compose.yml（添加 12 服务定义）
   - profile: `microservices`（默认 profile 仍是单体）
   - 依赖链：gateway → all services → postgres/redis/qdrant
   - 验证：`docker-compose --profile=microservices up` 全服务启动

4. **C3.14** K8s 集成测试
   - 脚本：`scripts/k8s-deploy-test.sh`（kind 创建集群 → apply manifests → 烟雾测试）
   - 验证：12 服务 Pod Running、gateway 能路由到 relay/chat

**交付物**：
- `cmd/gateway/main.go` ... `cmd/observability/main.go` (12 文件)
- `deploy/docker/Dockerfile.*` (12 文件)
- `deploy/kubernetes/*-deployment.yaml` (12 文件，更新现有 server.yaml）
- 更新 `docker-compose.yml`
- `scripts/k8s-deploy-test.sh`

---

### Stage C4: gRPC + Kafka（Week 3-4，估计 20-25 commits）

**目标**：服务间通信从 HTTP 改 gRPC + 异步事件 Kafka

**任务分解**：
1. **C4.1** 完善 proto 定义（6 个核心服务）
   - `api/proto/relay.proto`：`RelayService.Complete/CompleteStream/Embed`
   - `api/proto/agent.proto`：`AgentService.CreateRun/ExecuteReAct/ApproveToolCall`
   - `api/proto/workflow.proto`：`WorkflowService.Execute/TestNode`
   - `api/proto/rag.proto`：`RAGService.CreateKnowledgeBase/UploadDocument/Retrieve`
   - `api/proto/billing.proto`：`BillingService.RecordUsage/GetQuota`
   - `api/proto/task.proto`：`TaskService.Schedule/Cancel`
   - 验证：`buf lint` 或 `protoc --go_out=. *.proto` 编译通过

2. **C4.2** 生成 gRPC 代码 + 添加依赖
   - `go get google.golang.org/grpc@v1.67.0`
   - Makefile target：`make proto-gen`（生成 `*_grpc.pb.go`）
   - 验证：生成文件编译通过

3. **C4.3-C4.8** 实现 gRPC server（每个服务 1 commit）
   - **Relay**: `pkg/relay/grpc_server.go`（实现 RelayServiceServer）
   - **Agent**: `pkg/agent/grpc_server.go`（实现 AgentServiceServer）
   - **Workflow**: `pkg/workflow/grpc_server.go`
   - **RAG**: `pkg/knowledge/grpc_server.go`
   - **Billing**: `pkg/billing/grpc_server.go`
   - **Task**: `pkg/task/grpc_server.go`
   - 验证：每个 gRPC server 单测（grpctest 模拟调用）

4. **C4.9** 改造 HTTP 路由器调用 gRPC client
   - Chat 调用 Relay：`internal/chat/relay_client.go`（gRPC RelayService client）
   - Agent 调用 Relay/RAG：`internal/agent/clients.go`
   - Workflow 调用 Agent/RAG/Relay
   - 验证：集成测试（启动 gRPC server + client 端到端）

5. **C4.10** Kafka 事件总线基础
   - 添加依赖：`go get github.com/segmentio/kafka-go@v0.4.47`
   - `pkg/event/producer.go`：通用事件发布器
   - `pkg/event/consumer.go`：通用事件消费器
   - `pkg/event/proto/*.proto`：7 个事件 schema（billing.events、task.queue 等）
   - 验证：单测发布/消费事件

6. **C4.11-C4.17** 接入 Kafka（每个生产者/消费者 1 commit）
   - **billing.events**: Relay/Chat/Workflow/Agent 发布 → Billing 消费
   - **task.queue**: Task 发布 → All services 消费
   - **observability.logs**: All services 发布 → Observability 消费（ClickHouse）
   - **workflow.events**: Workflow 发布 → Observability/Task 消费
   - **agent.events**: Agent 发布 → Observability/Billing 消费
   - **channel.events**: Channel 发布 → Observability 消费
   - 验证：端到端测试（发事件 → 消费者收到）

7. **C4.18** 更新 K8s manifests（添加 Kafka 3 节点集群）
   - `deploy/kubernetes/kafka.yaml` 已存在，补充 topic 初始化 Job
   - 验证：kind 集群部署 Kafka + 服务能连接

**交付物**：
- `api/proto/*.proto` (6 文件完善)
- `pkg/*/grpc_server.go` (6 gRPC 实现)
- `pkg/event/producer.go` + `consumer.go` + `proto/*.proto`
- Kafka 集成代码（7 个主题的生产/消费逻辑）
- 更新 K8s manifests

---

### Stage D2: 前端技术栈改造（Week 2-4，与 C2-C4 并行，估计 30-35 commits）

**目标**：Zustand + SWR + Recharts + RHF+Zod

**任务分解**：
1. **D2.1** 添加依赖
   - `npm install zustand swr recharts react-hook-form zod @hookform/resolvers`
   - 验证：`npm run build` 通过

2. **D2.2** Auth store 迁移到 Zustand（最低风险入口）
   - 重写 `src/features/auth/store.ts` 为 Zustand store
   - 更新 `src/app/appContext.tsx`（移除 useSyncExternalStore）
   - 验证：6 个 auth store 测试通过

3. **D2.3-D2.5** 创建 Zustand stores（3 commits）
   - `src/store/chat.ts`（conversations, messages, currentConversation）
   - `src/store/workflow.ts`（workflows, nodes, edges, executionState）
   - `src/store/knowledge.ts`（knowledgeBases, documents, retrievalResults）
   - 验证：每个 store 单测

4. **D2.6** SWR 基础封装
   - `src/lib/swr.ts`：配置 SWRConfig、fetcher、错误处理
   - `src/app/App.tsx`：包裹 SWRConfig provider
   - 验证：无破坏性改动，现有页面仍工作

5. **D2.7** AdminHomePage 迁移到 SWR（第一个采用者）
   - `src/routes/admin/AdminHomePage.tsx`：用 `useSWR('/api/v1/admin/stats')` 替换 useReducer
   - 验证：AdminHomePage.test.tsx 通过（需改 mock 为 MSW）

6. **D2.8-D2.10** 其他页面 SWR 迁移（3 批次）
   - Batch 1: ConsoleHomePage, AdminChannelsPage（只读部分）
   - Batch 2: WorkspaceChatPage, WorkspaceAgentsPage
   - Batch 3: MarketplacePage, KnowledgeBasePage
   - 验证：每批跑相关测试

7. **D2.11** Recharts 替换 StatChart
   - `src/components/shared/StatChart.tsx`：用 ResponsiveContainer + BarChart 重写
   - 验证：StatChart.test.tsx 通过（配置 vitest 忽略 Recharts console 警告）

8. **D2.12-D2.15** 仪表盘图表补充（4 commits）
   - AdminHomePage：API 调用趋势（LineChart）、模型占比（PieChart）
   - ConsoleHomePage：使用量统计（AreaChart）
   - WorkflowEditorPage：节点执行时间（BarChart）
   - KnowledgeBasePage：文档分布（ScatterChart）
   - 验证：每个页面测试通过

9. **D2.16** React Hook Form + Zod 基础
   - `src/lib/formSchemas.ts`：定义常用 schema（UserPreferences, LoginForm, ChannelForm）
   - 验证：schema 类型检查通过

10. **D2.17** SettingsPage 迁移到 RHF+Zod（最简单表单）
    - 用 `useForm({ resolver: zodResolver(schema) })` 替换 useState
    - 验证：SettingsPage.test.tsx 通过

11. **D2.18-D2.20** 其他表单迁移（3 批次）
    - Batch 1: LoginPage, RegisterPage（简单认证表单）
    - Batch 2: WorkflowNodeConfigForm（复杂嵌套表单）
    - Batch 3: AdminChannelsPage ChannelForm（最复杂，useReducer → RHF）
    - 验证：每批相关测试通过

12. **D2.21** 全测试套件验证
    - `npm test`：确认 602 测试全绿
    - 修复任何 console 警告导致的失败
    - 验证：CI 通过

**交付物**：
- `src/store/*.ts` (3 Zustand stores)
- `src/lib/swr.ts` + SWR 集成
- `src/components/shared/StatChart.tsx` (Recharts 重写)
- `src/lib/formSchemas.ts` + RHF+Zod 表单迁移
- 更新 package.json + 测试配置

---

### Phase 2: 深度集成（Week 4-5，估计 15-20 commits）

**目标**：B2/B3/B4 组件接入运行时

**任务分解**：
1. **P2.1** B3 - ModelRouter 进 agent runner 循环
   - `internal/agent/runner.go:ExecuteReAct()`：每次迭代前调用 `modelRouter.Select()`
   - Config 读取：`AgentConfig.ModelRoutingRules`
   - 验证：单测模拟多迭代场景，验证模型切换

2. **P2.2** B3 - SkillSelector 进 agent runner
   - `internal/agent/runner.go:ExecuteReAct()`：迭代开始调用 `skillSelector.Score()`
   - 过滤 top-K skills 传给 LLM
   - 验证：测试技能注入到 prompt

3. **P2.3** B3 - call_agent 工具实现
   - `internal/agent/tools/call_agent.go`：新工具类型，递归调用 `AgentService.CreateRun()`
   - 递归深度守卫：检查 `SubAgentMaxDepth`
   - proto：`agent.proto` 添加 `CallAgentToolInput/Output`
   - 验证：测试 sub-agent 执行 + 深度限制

4. **P2.4** B2 - websearch 动态提供商选择
   - `internal/agent/tools/websearch.go`：读取 `AgentConfig.WebsearchProvider` 选择提供商
   - 回退链：`AgentConfig.WebsearchFallback` 顺序尝试
   - 验证：测试首选失败时回退到次选

5. **P2.5** B2 - 补充缺失的 4 个 websearch 提供商实现
   - 当前已有 11 个（Tavily/Brave/Serper/Bing/Exa/You/Kagi/Mojeek/Jina/Bocha/Baidu）
   - 补充：SerpAPI、Google CSE、DuckDuckGo、SearXNG
   - 位置：`internal/mcp/websearch/providers.go`（追加到现有文件）
   - 验证：每个 httptest 单测

6. **P2.6** B4 - deepdoc 进知识库上传管线
   - `internal/knowledge/document_processor.go:Process()`：调用 `deepdocParser.Parse()` 提取结构
   - 分块策略选择：`ChunkStrategy` 字段驱动（fixed_size/semantic/qa_split）
   - 验证：集成测试上传 DOCX → 检查 chunk 质量

7. **P2.7-P2.9** 集成测试（3 commits）
   - Agent 端到端：call_agent + modelRouter + skillSelector + websearch
   - Knowledge 端到端：上传 → deepdoc 解析 → 分块 → Qdrant 存储 → 检索
   - Workflow 端到端：agent 节点调用 sub-agent
   - 验证：所有集成测试通过

**交付物**：
- `internal/agent/runner.go` (ModelRouter/SkillSelector 集成)
- `internal/agent/tools/call_agent.go` (新工具)
- `internal/mcp/websearch/*.go` (14 个提供商)
- `internal/knowledge/document_processor.go` (deepdoc 集成)
- 集成测试

---

## 执行时间线（4 周）

### Week 1
- **Day 1-2**: C2.1-C2.2（schema 定义 + 双写框架）
- **Day 3-5**: C2.3-C2.14（逐服务迁移脚本）
- **并行**: D2.1-D2.3（前端依赖 + Auth store Zustand）

### Week 2
- **Day 1-3**: C2.15 + C3.0-C3.6（双写测试 + 前 6 服务部署）
- **Day 4-5**: C3.7-C3.12（后 6 服务部署）
- **并行**: D2.4-D2.10（Zustand stores + SWR 迁移）

### Week 3
- **Day 1-3**: C3.13-C3.14 + C4.1-C4.5（K8s 集成 + proto + 前 3 gRPC server）
- **Day 4-5**: C4.6-C4.10（后 3 gRPC server + Kafka 基础）
- **并行**: D2.11-D2.17（Recharts + RHF 基础 + SettingsPage）

### Week 4
- **Day 1-3**: C4.11-C4.18（Kafka 7 主题集成 + K8s 更新）
- **Day 4-5**: P2.1-P2.6（B2/B3/B4 深度集成）
- **并行**: D2.18-D2.21（表单迁移批次 + 全测试验证）

### Week 5（收尾）
- **Day 1-2**: P2.7-P2.9（端到端集成测试）
- **Day 3**: 文档更新（FUSION_GAP_CLOSURE_PLAN.md 标记完成）
- **Day 4-5**: 回归测试 + 性能基准 + 最后 push

---

## 质量门控

每个 Stage 完成标准：
1. ✅ 相关单测全绿（Go: `go test ./[package]`，Web: `npm test -- [file]`）
2. ✅ 全套件测试全绿（Go: `go test ./...`，Web: `npm test`）
3. ✅ 静态检查通过（Go: `go vet ./...` + `gofmt -l .`，Web: `npm run build`）
4. ✅ Git diff 自查（无调试代码、无注释代码、无无关改动）
5. ✅ Commit message 规范（`feat(module): description`）
6. ✅ Push 到 origin/main

禁止：
- ❌ 测试未通过直接 commit
- ❌ 一次改动 > 10 文件
- ❌ 跳过某个 Stage 的验证步骤

---

## 成功标准（goal.md 对照）

1. ✅ **核心功能按设计文档落地**：C2-C4 微服务架构 + D2 现代前端栈 + Phase 2 运行时集成
2. ✅ **测试通过**：Go 全套 + Web 602 测试持续绿色
3. ✅ **仓库状态干净**：无未提交改动、无临时文件
4. ✅ **已按阶段提交并 push**：每个 Stage 推送一次（总计 5 次 push）

## 下一步

等待用户确认启动执行。建议顺序：
1. 先执行 D2（前端，风险低，可快速验证）
2. 再执行 C2-C4（后端，复杂度高）
3. 最后 Phase 2（依赖前两者完成）

或用户可指定优先级调整。

# Oblivious 项目文档索引

> **完整的系统设计与实施指南**

---

## 📚 文档导航

### 1️⃣ 功能融合设计方案
**文件**: [`FUSION_DESIGN_PLAN.md`](./FUSION_DESIGN_PLAN.md)

**内容概览**:
- 🎯 总体架构设计（4层架构）
- 📋 12个功能模块的详细抄袭清单
- 🗂️ 从30个参考项目中提取功能
- 🛠️ 技术栈选择（Go + Next.js + PostgreSQL）
- 📁 目标目录结构
- 🚀 4个季度的实施路线图
- 📊 功能优先级矩阵（P0-P3）

**核心价值**: 明确告诉你从哪个项目抄什么功能，以及放到哪里

---

### 2️⃣ 系统设计文档（完整版）

#### Part 1: 架构 & 数据库
**文件**: [`docs/architecture/system-design/SYSTEM_DESIGN.md`](../architecture/system-design/SYSTEM_DESIGN.md)

**内容**:
- 🏗️ **系统架构设计**
  - 总体架构（6层架构图）
  - 微服务划分（10个核心服务）
  - 服务间通信（gRPC + HTTP + Kafka）
  - 服务发现与负载均衡

- 🗄️ **数据库设计（PostgreSQL）**
  - 核心表结构（12张主表）
  - 用户表、API密钥表、对话表
  - 工作流表、知识库表、计费表
  - 索引设计与优化策略

---

#### Part 2: 缓存 & API
**文件**: [`docs/architecture/system-design/SYSTEM_DESIGN_PART2.md`](../architecture/system-design/SYSTEM_DESIGN_PART2.md)

**内容**:
- 💾 **缓存设计（Redis）**
  - 3层缓存策略（L1/L2/L3）
  - Redis 数据结构设计
  - 速率限制、语义缓存
  - 缓存更新策略

- 🔍 **向量数据库（Qdrant）**
  - Collection 设计
  - HNSW 索引参数
  - 分片策略

- 📊 **分析数据库（ClickHouse）**
  - 请求日志表
  - 使用统计表
  - 模型性能表

- 🌐 **API 设计**
  - RESTful API 规范
  - 统一响应格式
  - 认证 API、对话 API（OpenAI 兼容）
  - 工作流 API、知识库 API

---

#### Part 3: 安全 & 性能
**文件**: [`docs/architecture/system-design/SYSTEM_DESIGN_PART3.md`](../architecture/system-design/SYSTEM_DESIGN_PART3.md)

**内容**:
- 🔒 **安全设计**
  - 认证机制（JWT + API Key）
  - RBAC 权限模型
  - 数据加密（TLS 1.3 + AES-256-GCM）
  - 安全防护（SQL注入、XSS、CSRF、DDoS）
  - 审计日志

- ⚡ **性能设计**
  - 性能目标（P95 < 200ms）
  - 数据库优化（连接池、索引、查询）
  - 缓存优化（Cache Aside、语义缓存）
  - 异步处理（Kafka）
  - 负载均衡（多层LB）
  - 性能监控指标

---

#### Part 4: 可观测性 & 部署
**文件**: [`docs/architecture/system-design/SYSTEM_DESIGN_PART4.md`](../architecture/system-design/SYSTEM_DESIGN_PART4.md)

**内容**:
- 📝 **日志系统**
  - 结构化日志（JSON格式）
  - ELK Stack 架构
  - 日志保留策略（7天/30天/365天）

- 📈 **监控系统**
  - Prometheus 指标采集
  - Grafana 仪表盘（4个核心仪表盘）
  - 告警规则（6类关键告警）
  - 多渠道通知（Slack/邮件/PagerDuty）

- 🔍 **分布式追踪**
  - OpenTelemetry 集成
  - Span 设计
  - Jaeger 可视化

- ❤️ **健康检查**
  - 健康检查端点
  - 就绪检查
  - 存活检查

- 🚀 **部署架构**
  - Docker Compose（开发环境）
  - Kubernetes（生产环境）
  - 备份与恢复策略

---

## 📂 项目结构

```
Oblivious/
├── docs/
│   ├── design/
│   │   ├── README.md              # 本文档索引
│   │   └── FUSION_DESIGN_PLAN.md  # 功能融合方案
│   ├── architecture/
│   │   └── system-design/
│   │       ├── SYSTEM_DESIGN.md
│   │       ├── SYSTEM_DESIGN_PART2.md
│   │       ├── SYSTEM_DESIGN_PART3.md
│   │       └── SYSTEM_DESIGN_PART4.md
│   └── reports/
│       └── reference-analysis/
│           └── REFERENCE_PROJECTS_ANALYSIS.md
│
├── src/
│   ├── server/                    # Go 后端
│   │   ├── cmd/
│   │   │   ├── gateway/          # API 网关服务
│   │   │   ├── chat/             # 对话服务
│   │   │   ├── workflow/         # 工作流服务
│   │   │   └── ...
│   │   ├── internal/
│   │   │   ├── gateway/          # 网关核心逻辑
│   │   │   ├── auth/             # 认证授权
│   │   │   ├── billing/          # 计费系统
│   │   │   ├── workflow/         # 工作流引擎
│   │   │   ├── rag/              # RAG 引擎
│   │   │   └── ...
│   │   └── pkg/                  # 公共库
│   │
│   └── web/                       # Next.js 前端
│       ├── app/
│       ├── components/
│       │   ├── chat/
│       │   ├── workflow/
│       │   └── knowledge/
│       └── features/
│
├── database/
│   ├── migrations/                # 数据库迁移
│   ├── schemas/                   # Schema 定义
│   └── clickhouse/                # ClickHouse Schema
│
├── deploy/
│   ├── docker/
│   │   └── docker-compose.yml
│   └── kubernetes/
│       ├── deployments/
│       ├── services/
│       └── configmaps/
│
└── reference/                     # 参考项目（30个）
    ├── new-api/
    ├── dify/
    ├── FastGPT/
    ├── ragflow/
    └── ...
```

---

## 🎯 实施优先级

### **阶段一：核心基础设施 (P0) - Q1**
```
Week 1-2:  ✅ API 网关核心
Week 3-4:  ✅ 认证授权系统
Week 5-6:  ✅ 计费与监控基础
Week 7-8:  ✅ 基础 UI 框架
```

**参考代码**:
- `new-api/relay/` → API 网关
- `sub2api/backend/internal/auth/` → 认证系统
- `helicone/worker/` → 监控系统

---

### **阶段二：应用能力层 (P1) - Q2**
```
Week 9-10:  ✅ 工作流引擎核心
Week 11-12: ✅ 知识库与 RAG
Week 13-14: ✅ 对话管理系统
Week 15-16: ✅ 集成测试
```

**参考代码**:
- `dify/api/core/workflow/` → 工作流引擎
- `ragflow/rag/` → RAG 引擎
- `LibreChat/api/server/` → 对话系统

---

### **阶段三：高级功能 (P2) - Q3**
```
Week 17-18: ✅ Agent 系统
Week 19-20: ✅ MCP 集成
Week 21-22: ✅ 多模态支持
Week 23-24: ✅ 性能优化
```

**参考代码**:
- `dify/api/core/agent/` → Agent 系统
- `bifrost/mcp/` → MCP 集成
- `open-webui/backend/apps/images/` → 多模态

---

### **阶段四：企业功能 (P3) - Q4**
```
Week 25-26: ✅ 多租户 & RBAC
Week 27-28: ✅ 插件系统
Week 29-30: ✅ 安全加固
Week 31-32: ✅ 生产部署
```

**参考代码**:
- `open-webui/backend/apps/webui/models/groups.py` → 多租户
- `lobe-chat/src/plugins/` → 插件系统

---

## 🛠️ 技术栈总览

### **后端**
```yaml
语言: Go 1.22+
框架: Gin / Echo
ORM: GORM
验证: validator
日志: zap
追踪: OpenTelemetry
```

### **前端**
```yaml
框架: Next.js 14 (App Router)
UI: Shadcn/ui + Tailwind CSS
状态: Zustand
流编辑器: React Flow
图表: Recharts
```

### **数据库**
```yaml
主库: PostgreSQL 16
缓存: Redis 7
向量库: Qdrant
分析库: ClickHouse
消息队列: Kafka
对象存储: MinIO
```

### **基础设施**
```yaml
容器化: Docker + Docker Compose
编排: Kubernetes
监控: Prometheus + Grafana
日志: ELK Stack
追踪: Jaeger / Tempo
```

---

## 📊 关键指标

### **性能指标**
- API 响应时间 (P95): **< 200ms**
- LLM 首字延迟 (P95): **< 2s**
- 并发用户数: **10,000+**
- 缓存命中率: **> 80%**

### **业务指标**
- 支持 100+ LLM 模型
- 支持多模态（文本/图像/音频）
- 工作流节点类型: 20+
- 知识库文档格式: 10+

### **可靠性指标**
- 可用性: **99.9%**
- 数据持久性: **99.999%**
- 备份保留: **1 年**

---

## ✅ 快速开始

### **1. 阅读顺序**
```
1. docs/design/FUSION_DESIGN_PLAN.md                         → 了解整体规划
2. docs/architecture/system-design/SYSTEM_DESIGN.md           → 架构与数据库
3. docs/architecture/system-design/SYSTEM_DESIGN_PART2.md     → 缓存与 API
4. docs/architecture/system-design/SYSTEM_DESIGN_PART3.md     → 安全与性能
5. docs/architecture/system-design/SYSTEM_DESIGN_PART4.md     → 可观测性与部署
```

### **2. 项目初始化**
```bash
# 创建项目结构
mkdir -p src/{server,web} database/{migrations,schemas} deploy/{docker,kubernetes}

# 初始化 Go 模块
cd src/server && go mod init github.com/oblivious/server

# 初始化 Next.js 项目
cd src/web && npx create-next-app@latest .
```

### **3. 启动开发环境**
```bash
# 启动基础设施
docker-compose up -d postgres redis qdrant clickhouse

# 运行数据库迁移
cd src/server && go run cmd/migrate/main.go

# 启动后端服务
go run cmd/gateway/main.go

# 启动前端
cd src/web && npm run dev
```

---

## 📞 联系方式

**项目维护**: System Architecture Team  
**文档版本**: v1.0  
**最后更新**: 2026-06-03

---

## 📝 变更日志

### v1.0 - 2026-06-03
- ✅ 初始版本发布
- ✅ 完成功能融合设计方案
- ✅ 完成系统设计文档（4部分）
- ✅ 定义12个核心功能模块
- ✅ 制定4季度实施路线图

---

**现在你拥有了一套完整的系统设计文档！** 🎉

从功能规划、架构设计、数据库设计、API设计、安全设计、性能优化到部署方案，所有细节都已覆盖。

**下一步建议**:
1. 按照 `docs/design/FUSION_DESIGN_PLAN.md` 开始提取参考项目代码
2. 按照 `docs/architecture/system-design/SYSTEM_DESIGN.md` 初始化数据库表结构
3. 按照优先级 P0 → P1 → P2 → P3 逐步实现

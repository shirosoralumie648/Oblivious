# Oblivious 融合设计方案

> **目标**: 从30个参考项目中抽取最佳功能，构建一个完整的企业级 AI 应用平台

---

## 🎯 总体架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    前端层 (Web UI)                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ 对话界面 │  │ 工作流IDE│  │ 知识库   │  │ 管理后台 │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    应用服务层                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Chat API │  │ Workflow │  │ RAG      │  │ Agent    │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    核心引擎层                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ API网关  │  │ 认证授权 │  │ 计费系统 │  │ 监控追踪 │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    数据存储层                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │PostgreSQL│  │  Redis   │  │ Vector DB│  │ ClickHouse│  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 📋 功能抄袭清单

### **第一阶段：核心基础设施 (P0)**

#### 1. API 网关 - 从 `new-api` + `litellm` + `bifrost`

**从 new-api 抄**:
- ✅ `src/server/internal/relay/` - 多模型适配器架构
- ✅ `src/server/internal/relay/channel/` - 渠道管理系统
- ✅ `src/server/internal/relay/loadbalancer.go` - 负载均衡算法
- ✅ `src/server/internal/relay/retry.go` - 重试与故障转移机制
- ✅ `src/server/internal/relay/pricing.go` - 计费规则引擎

**从 litellm 抄**:
- ✅ 统一的 OpenAI 格式转换层
- ✅ 100+ LLM 提供商的适配代码
- ✅ Python SDK 设计模式

**从 bifrost 抄**:
- ✅ 语义缓存实现
- ✅ MCP 网关集成
- ✅ 配置管理（config.json + 环境变量）

**代码位置**:
```
/reference/new-api/relay/           → /src/server/internal/gateway/
/reference/litellm/litellm/         → /src/server/internal/adapters/
/reference/bifrost/core/            → /src/server/internal/cache/
```

---

#### 2. 认证授权 - 从 `sub2api` + `CLIProxyAPI`

**从 sub2api 抄**:
- ✅ `backend/internal/auth/` - JWT 认证系统
- ✅ `backend/internal/account/` - 多账号管理
- ✅ `backend/internal/apikey/` - API Key 生成与验证
- ✅ `backend/internal/middleware/` - 认证中间件
- ✅ `backend/internal/payment/` - 支付系统集成

**从 CLIProxyAPI 抄**:
- ✅ `pkg/oauth/` - OAuth 认证流程
- ✅ `pkg/account/pool.go` - 账号池管理
- ✅ `pkg/router/` - 账号选择策略

**代码位置**:
```
/reference/sub2api/backend/internal/auth/     → /src/server/internal/auth/
/reference/CLIProxyAPI/pkg/oauth/             → /src/server/internal/oauth/
```

---

#### 3. 计费系统 - 从 `new-api` + `sub2api`

**从 new-api 抄**:
- ✅ `src/server/internal/relay/billing.go` - Token 计费逻辑
- ✅ `src/server/internal/relay/pricing.go` - 模型价格配置
- ✅ 实时 Token 统计

**从 sub2api 抄**:
- ✅ `backend/internal/billing/` - 精确计费引擎
- ✅ `backend/internal/quota/` - 配额管理
- ✅ `backend/internal/usage/` - 使用量统计

**代码位置**:
```
/reference/new-api/relay/billing.go           → /src/server/internal/billing/
/reference/sub2api/backend/internal/billing/  → /src/server/internal/billing/
```

---

#### 4. 监控追踪 - 从 `helicone` + `CPA-Manager`

**从 helicone 抄**:
- ✅ `worker/` - 请求日志收集（Cloudflare Workers 模式）
- ✅ `jawn/src/lib/` - 日志处理服务
- ✅ `clickhouse/` - 分析数据库 schema
- ✅ OpenTelemetry 集成

**从 CPA-Manager 抄**:
- ✅ `internal/monitor/` - 请求级监控
- ✅ `internal/statistics/` - 统计分析
- ✅ `internal/alert/` - 异常告警

**代码位置**:
```
/reference/helicone/worker/              → /src/server/internal/collector/
/reference/helicone/clickhouse/          → /database/clickhouse/
/reference/CPA-Manager/internal/monitor/ → /src/server/internal/monitor/
```

---

### **第二阶段：应用能力层 (P1)**

#### 5. 工作流引擎 - 从 `dify` + `FastGPT` + `Flowise`

**从 dify 抄**:
- ✅ `api/core/workflow/` - 工作流核心引擎
- ✅ `api/core/workflow/nodes/` - 节点定义（LLM/知识检索/条件/循环等）
- ✅ `api/core/app/apps/workflow/` - 工作流应用
- ✅ `api/core/tools/` - 工具系统

**从 FastGPT 抄**:
- ✅ `packages/service/core/workflow/` - 工作流运行时
- ✅ `packages/service/core/workflow/dispatch/` - 节点调度
- ✅ `projects/app/src/pages/app/detail/components/WorkflowComponents/` - 可视化编辑器 UI

**从 Flowise 抄**:
- ✅ `packages/components/` - 组件化节点设计
- ✅ `packages/server/src/services/` - 工作流服务
- ✅ 拖拽式 UI 设计模式

**代码位置**:
```
/reference/dify/api/core/workflow/           → /src/server/internal/workflow/
/reference/FastGPT/packages/service/         → /src/server/internal/workflow/
/reference/Flowise/packages/components/      → /src/web/components/workflow/
```

---

#### 6. 知识库与 RAG - 从 `ragflow` + `MaxKB` + `FastGPT`

**从 ragflow 抄**:
- ✅ `rag/` - RAG 核心引擎
- ✅ `deepdoc/` - 深度文档解析
- ✅ `rag/nlp/` - 文本处理（分块/向量化）
- ✅ `rag/app/` - RAG 应用层

**从 MaxKB 抄**:
- ✅ `apps/dataset/` - 数据集管理
- ✅ `apps/embedding/` - Embedding 服务
- ✅ `common/db/search.py` - 混合检索

**从 FastGPT 抄**:
- ✅ `packages/service/core/dataset/` - 知识库服务
- ✅ `packages/service/core/dataset/search/` - 检索服务
- ✅ `packages/service/core/dataset/data/` - 数据管理

**代码位置**:
```
/reference/ragflow/rag/               → /src/server/internal/rag/
/reference/MaxKB/apps/dataset/        → /src/server/internal/dataset/
/reference/FastGPT/packages/service/  → /src/server/internal/dataset/
```

---

#### 7. 对话管理 - 从 `LibreChat` + `lobe-chat` + `open-webui`

**从 LibreChat 抄**:
- ✅ `api/server/services/` - 对话服务
- ✅ `api/models/` - 数据模型
- ✅ `api/server/controllers/` - API 控制器
- ✅ `client/src/components/Chat/` - 聊天 UI 组件

**从 lobe-chat 抄**:
- ✅ `src/server/routers/` - 路由设计
- ✅ `src/store/` - 状态管理（Zustand）
- ✅ `src/features/Conversation/` - 对话特性

**从 open-webui 抄**:
- ✅ `backend/apps/webui/routers/chats.py` - 聊天路由
- ✅ `backend/apps/webui/models/chats.py` - 聊天模型
- ✅ `src/lib/components/chat/` - 聊天组件

**代码位置**:
```
/reference/LibreChat/api/server/     → /src/server/internal/chat/
/reference/lobe-chat/src/            → /src/web/features/chat/
/reference/open-webui/backend/       → /src/server/internal/chat/
```

---

### **第三阶段：高级功能层 (P2)**

#### 8. Agent 系统 - 从 `dify` + `FastGPT` + `open-webui`

**从 dify 抄**:
- ✅ `api/core/agent/` - Agent 引擎
- ✅ `api/core/tools/` - 工具注册与调用
- ✅ `api/core/agent/agent_executor.py` - Agent 执行器

**从 FastGPT 抄**:
- ✅ `packages/service/core/ai/agent/` - Agent 运行时
- ✅ `packages/service/core/workflow/dispatch/agent/` - Agent 调度

**从 open-webui 抄**:
- ✅ `backend/apps/webui/routers/tools.py` - 工具路由
- ✅ `backend/apps/webui/models/tools.py` - 工具模型

**代码位置**:
```
/reference/dify/api/core/agent/      → /src/server/internal/agent/
/reference/FastGPT/packages/service/ → /src/server/internal/agent/
```

---

#### 9. MCP 集成 - 从 `bifrost` + `FastGPT` + `NextChat`

**从 bifrost 抄**:
- ✅ MCP 网关实现
- ✅ MCP 服务器连接管理

**从 FastGPT 抄**:
- ✅ 双向 MCP 支持

**从 NextChat 抄**:
- ✅ MCP 客户端集成

**代码位置**:
```
/reference/bifrost/mcp/           → /src/server/internal/mcp/
/reference/FastGPT/mcp/           → /src/server/internal/mcp/
```

---

#### 10. 多模态支持 - 从 `LibreChat` + `open-webui`

**从 LibreChat 抄**:
- ✅ `api/server/services/Files/` - 文件处理
- ✅ `api/models/File.js` - 文件模型
- ✅ 图像上传与处理

**从 open-webui 抄**:
- ✅ `backend/apps/images/` - 图像生成
- ✅ `backend/apps/audio/` - 音频处理
- ✅ STT/TTS 集成

**代码位置**:
```
/reference/LibreChat/api/server/services/Files/ → /src/server/internal/files/
/reference/open-webui/backend/apps/             → /src/server/internal/media/
```

---

### **第四阶段：企业功能层 (P3)**

#### 11. 用户与权限 - 从 `open-webui` + `sub2api`

**从 open-webui 抄**:
- ✅ `backend/apps/webui/routers/auths.py` - 认证路由
- ✅ `backend/apps/webui/models/users.py` - 用户模型
- ✅ `backend/apps/webui/models/groups.py` - 用户组
- ✅ RBAC 实现

**从 sub2api 抄**:
- ✅ `backend/internal/user/` - 用户管理
- ✅ `backend/internal/team/` - 团队管理

**代码位置**:
```
/reference/open-webui/backend/apps/webui/ → /src/server/internal/users/
/reference/sub2api/backend/internal/      → /src/server/internal/users/
```

---

#### 12. 插件系统 - 从 `open-webui` + `lobe-chat`

**从 open-webui 抄**:
- ✅ `backend/apps/webui/routers/functions.py` - 函数路由
- ✅ Pipelines 框架

**从 lobe-chat 抄**:
- ✅ `src/plugins/` - 插件架构
- ✅ Function Calling 系统

**代码位置**:
```
/reference/open-webui/backend/apps/webui/ → /src/server/internal/plugins/
/reference/lobe-chat/src/plugins/         → /src/web/plugins/
```

---

## 🛠️ 技术栈选择

### **后端**
- **语言**: Go (参考 new-api, bifrost, sub2api)
- **框架**: Gin / Echo
- **ORM**: GORM

### **前端**
- **框架**: Next.js + React (参考 lobe-chat, LibreChat)
- **状态管理**: Zustand
- **UI 组件**: Shadcn/ui
- **工作流编辑器**: React Flow

### **数据库**
- **主数据库**: PostgreSQL
- **缓存**: Redis
- **向量数据库**: Qdrant / pgvector
- **分析数据库**: ClickHouse

### **部署**
- **容器化**: Docker + Docker Compose
- **编排**: Kubernetes (可选)

---

## 📁 目录结构设计

```
Oblivious/
├── src/
│   ├── server/              # Go 后端
│   │   ├── cmd/
│   │   │   ├── api/        # API 服务器
│   │   │   ├── worker/     # 后台任务
│   │   │   └── migrate/    # 数据库迁移
│   │   ├── internal/
│   │   │   ├── gateway/    # API 网关 (from new-api)
│   │   │   ├── auth/       # 认证授权 (from sub2api)
│   │   │   ├── billing/    # 计费系统 (from new-api)
│   │   │   ├── monitor/    # 监控追踪 (from helicone)
│   │   │   ├── workflow/   # 工作流引擎 (from dify)
│   │   │   ├── rag/        # RAG 引擎 (from ragflow)
│   │   │   ├── chat/       # 对话管理 (from LibreChat)
│   │   │   ├── agent/      # Agent 系统 (from dify)
│   │   │   ├── mcp/        # MCP 集成 (from bifrost)
│   │   │   └── plugins/    # 插件系统 (from open-webui)
│   │   ├── pkg/            # 公共库
│   │   └── api/            # API 定义
│   │
│   └── web/                # Next.js 前端
│       ├── app/            # App Router
│       ├── components/     # 组件
│       │   ├── chat/       # 聊天组件 (from lobe-chat)
│       │   ├── workflow/   # 工作流编辑器 (from Flowise)
│       │   └── knowledge/  # 知识库 UI (from FastGPT)
│       ├── features/       # 功能模块
│       ├── hooks/          # Hooks
│       ├── store/          # 状态管理
│       └── lib/            # 工具库
│
├── database/
│   ├── migrations/         # 数据库迁移
│   ├── schemas/            # Schema 定义
│   └── clickhouse/         # ClickHouse Schema
│
├── docs/                   # 文档
├── scripts/                # 脚本
├── docker/                 # Docker 配置
└── deploy/                 # 部署配置
```

---

## 🚀 实施路线图

### **第一季度 (Q1)**
- Week 1-2: 项目初始化 + API 网关核心
- Week 3-4: 认证授权系统
- Week 5-6: 计费与监控基础
- Week 7-8: 基础 UI 框架

### **第二季度 (Q2)**
- Week 9-10: 工作流引擎核心
- Week 11-12: 知识库与 RAG
- Week 13-14: 对话管理系统
- Week 15-16: 集成测试

### **第三季度 (Q3)**
- Week 17-18: Agent 系统
- Week 19-20: MCP 集成
- Week 21-22: 多模态支持
- Week 23-24: 性能优化

### **第四季度 (Q4)**
- Week 25-26: 企业功能（多租户/RBAC）
- Week 27-28: 插件系统
- Week 29-30: 安全加固
- Week 31-32: 生产部署 + 文档

---

## 📊 功能优先级矩阵

| 功能模块 | 优先级 | 复杂度 | 价值 | 参考项目 |
|---------|--------|--------|------|---------|
| API 网关 | P0 | 高 | 极高 | new-api, litellm |
| 认证授权 | P0 | 中 | 极高 | sub2api, CLIProxyAPI |
| 计费系统 | P0 | 中 | 高 | new-api, sub2api |
| 监控追踪 | P0 | 高 | 高 | helicone |
| 工作流引擎 | P1 | 极高 | 极高 | dify, FastGPT |
| 知识库 RAG | P1 | 高 | 极高 | ragflow, MaxKB |
| 对话管理 | P1 | 中 | 高 | LibreChat, lobe-chat |
| Agent 系统 | P2 | 高 | 高 | dify, FastGPT |
| MCP 集成 | P2 | 中 | 中 | bifrost, NextChat |
| 多模态 | P2 | 中 | 中 | LibreChat, open-webui |
| 多租户 | P3 | 中 | 中 | open-webui |
| 插件系统 | P3 | 高 | 中 | open-webui, lobe-chat |

---

## ✅ 下一步行动

1. ✅ **已完成**: 功能分析与设计方案
2. 🔄 **进行中**: 创建详细实施计划
3. 📋 **待办**: 
   - 初始化项目仓库结构
   - 提取 new-api 的 relay 层代码
   - 搭建基础 API 网关框架
   - 设计数据库 Schema

---

**文档版本**: v1.0  
**创建时间**: 2026-06-03  
**预计完成时间**: 2026 Q4

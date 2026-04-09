# Oblivious 整合项目设计文档

日期：2026-04-09
版本：v1.0

## 1. 项目定位与目标

### 1.1 整合背景

Oblivious 现有项目是 LobeHub（B2C）+ New-API（B2B）整合的中间产物（Go 后端 + React 前端工作区框架）。

目标：完成最终整合，形成统一产品。

```
整合前：
  LobeHub (TS/Next.js) + New-API (Go/Gin) + Oblivious (Go/React MVP)

整合后：
  Oblivious (完全体)
  ├── C 端：LobeHub 体验（Chat / Agent / Memory / Marketplace）
  └── B 端：New-API 功能（Relay / 渠道管理 / 计费配额）
```

### 1.2 用户体验设计

```
用户入口（Browser）
    │
    ▼
C 端体验（默认）
    │
    ├── Chat  → Agent Runtime → Relay → OpenAI
    │                        │
    │                        ├─ MCP Tools（工具扩展）
    │                        └─ Memory（RAG 知识库）
    │
    ├── Agent Builder（创建自己的 Agent）
    │
    ├── Marketplace（发布/发现 Agent）
    │
    └── 头像下拉 → "工作台"（跳转到 B 端）
              │
              ▼
B 端体验（工作台）
    │
    ├── API Keys 管理
    ├── 用量明细/账单
    ├── 订阅管理/充值
    ├── 渠道管理（平台管理员）
    ├── 套餐管理（平台管理员）
    └── 用户管理（平台管理员）
```

### 1.3 设计原则

1. **Go 统一后端**：Agent Runtime、MCP Tools、Relay、Admin API 全部 Go 重写
2. **C 端体验走 Relay**：Agent 调用 AI 必须经过 Relay，保持计费/限流统一
3. **用户配额共享**：Agent 消耗的 Token 计入用户自己的配额
4. **向量检索按用户隔离**：pgvector 以 user_id 为 namespace
5. **单一前端项目**：Admin UI 和 C 端放在同一项目，用路由分隔

---

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        React Frontend                                │
│                                                                      │
│   C 端路由 (/chat, /agents, /marketplace, /knowledge)               │
│   B 端路由 (/admin/*, /workspace/*)                                  │
└─────────────────────────────────────────────────────────────────────┘
                               │ HTTP
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Go Backend                                    │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                     API Gateway                               │   │
│  │  Auth Middleware / Session / Rate Limiting / Routing          │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                               │                                       │
│         ┌─────────────────────┼─────────────────────┐               │
│         ▼                     ▼                     ▼               │
│  ┌─────────────┐       ┌─────────────┐       ┌─────────────┐       │
│  │ Agent API   │       │ Relay API   │       │ Admin API   │       │
│  │             │       │             │       │             │       │
│  │ - /agents/* │       │ - /v1/*     │       │ - /admin/*  │       │
│  │ - /tasks/*  │       │   (OpenAI)  │       │ - /channels │       │
│  │ - /memory/* │       │             │       │ - /packages │       │
│  └─────────────┘       └─────────────┘       └─────────────┘       │
│         │                     │                     │               │
│         └─────────────────────┼─────────────────────┘               │
│                               ▼                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    Agent Runtime                              │   │
│  │                                                              │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐       │   │
│  │  │ Task Planner│  │Tool Executor│  │  Memory Engine  │       │   │
│  │  │(multi-step) │  │(MCP+builtin)│  │  (RAG/Knowledge)│       │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────┘       │   │
│  │                               │                               │   │
│  │                    ┌──────────┴──────────┐                    │   │
│  │                    │   MCP Client (Go)   │                    │   │
│  │                    └─────────────────────┘                    │   │
│  └───────────────────────────────┼───────────────────────────────┘   │
│                                  │ 内部调用                           │
│                                  ▼                                   │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                      Relay Layer                              │   │
│  │                                                              │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐    │   │
│  │  │OpenAI Handler│  │Anthropic     │  │ Gemini           │    │   │
│  │  │(15 APIs)    │  │Handler(P2)   │  │Handler(P3)       │    │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘    │   │
│  │                                                              │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐    │   │
│  │  │Router Engine │  │Rate Limiter  │  │Circuit Breaker   │    │   │
│  │  │(weighted LB) │  │(TokenBucket) │  │(health check)    │    │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘    │   │
│  │                                                              │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐    │   │
│  │  │Channel Pool  │  │Billing Hooks │  │Metrics           │    │   │
│  │  │(provider    │  │(cost-plus)   │  │(Prometheus)      │    │   │
│  │  │adapters)    │  │              │  │                  │    │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘    │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                    Shared Services                            │   │
│  │  Auth / Session / User / Quota / Preferences / Marketplace    │   │
│  └───────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PostgreSQL + pgvector                              │
│                                                                      │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐     │
│  │  users  │ │ channels│ │ packages│ │ agents  │ │memory   │     │
│  │sessions │ │model_   │ │subs_    │ │market_  │ │vectors  │     │
│  │ quotas  │ │routes   │ │orders   │ │items    │ │(pgvec)  │     │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘     │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流向

```
C 端用户请求：
  Browser → C 端路由 → Agent API → Agent Runtime → Relay → Provider
                                                      │
                                                      ▼
                                                   Billing Hooks
                                                      │
                                                      ▼
                                                   Quota System

B 端用户请求：
  Browser → B 端路由 → Admin API → Channel/Package Management
```

---

## 3. 模块详细设计

### 3.1 Agent Runtime（Go 重写）

#### 3.1.1 职责

| 职责 | 说明 |
|------|------|
| **任务规划** | 多步推理、工具调用编排 |
| **工具执行** | 调用 MCP 工具和内置工具 |
| **Memory 管理** | 对接 Knowledge RAG 系统 |
| **会话管理** | Agent 对话上下文维护 |

#### 3.1.2 核心接口

```go
type AgentService interface {
    // Agent CRUD
    CreateAgent(ctx context.Context, req *CreateAgentRequest) (*Agent, error)
    GetAgent(ctx context.Context, id string) (*Agent, error)
    ListAgents(ctx context.Context, userID string) ([]*Agent, error)
    UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*Agent, error)
    DeleteAgent(ctx context.Context, id string) error

    // 对话执行
    Chat(ctx context.Context, agentID string, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, agentID string, req *ChatRequest, stream chan *ChatResponse) error

    // 工具管理
    RegisterTool(tool Tool) error
    ListTools(agentID string) ([]Tool, error)
}

type TaskService interface {
    // Agent Task（SOLO 重构）
    CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error)
    GetTask(ctx context.Context, id string) (*Task, error)
    ListTasks(ctx context.Context, userID string) ([]*Task, error)
    ExecuteTask(ctx context.Context, id string) (*TaskResult, error)
    PauseTask(ctx context.Context, id string) error
    ResumeTask(ctx context.Context, id string) error
    CancelTask(ctx context.Context, id string) error
}
```

#### 3.1.3 内置工具（第一期）

| 工具 | 说明 |
|------|------|
| `web_search` | 网页搜索 |
| `calculator` | 计算器 |
| `datetime` | 获取当前时间 |
| `http_request` | HTTP 请求 |
| `code_execution` | 代码执行（沙箱） |

### 3.2 MCP Client（Go 重写）

#### 3.2.1 职责

| 职责 | 说明 |
|------|------|
| **MCP Protocol** | 实现 MCP 客户端协议 |
| **工具发现** | 从 MCP Server 发现可用工具 |
| **请求转发** | 把工具调用转发给 MCP Server |
| **SSE 传输** | 支持 SSE 传输模式 |

#### 3.2.2 核心接口

```go
type MCPClient interface {
    // 连接管理
    Connect(ctx context.Context, serverURL string, authToken string) error
    Disconnect(serverID string) error
    ListServers() []MCPServer

    // 工具
    ListTools(serverID string) ([]ToolDefinition, error)
    CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*ToolResult, error)
}

type MCPServer struct {
    ID       string
    Name     string
    URL      string
    AuthToken string
    Status   string // "connected" | "disconnected" | "error"
}
```

### 3.3 Memory / Knowledge RAG（升级）

#### 3.3.1 架构

```
Agent 需要记忆
    │
    ▼
Memory Engine
    │
    ├─ 查询用户个人 Memory（快速 Key-Value）
    │
    └─ RAG 检索（向量相似度）
            │
            ▼
    ┌──────────────────┐
    │  pgvector        │
    │  user_id=xxx     │  ← 按用户隔离
    │  text embedding  │
    │  metadata        │
    └──────────────────┘
            │
            ▼
    返回相关上下文
            │
            ▼
    注入 Agent Prompt
```

#### 3.3.2 核心接口

```go
type MemoryService interface {
    // 记忆 CRUD
    AddMemory(ctx context.Context, userID string, req *AddMemoryRequest) (*Memory, error)
    GetMemory(ctx context.Context, id string) (*Memory, error)
    ListMemories(ctx context.Context, userID string) ([]*Memory, error)
    DeleteMemory(ctx context.Context, id string) error

    // RAG 检索
    Search(ctx context.Context, userID string, query string, topK int) ([]*Memory, error)

    // 向量化（内部调用 Relay Embeddings）
    EmbedText(ctx context.Context, text string) ([]float32, error)
}
```

#### 3.3.3 数据模型

```sql
-- Memory 表
CREATE TABLE memories (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),  -- pgvector
    metadata JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 向量索引（按用户隔离）
CREATE INDEX idx_memories_user_id ON memories(user_id);
CREATE INDEX idx_memories_embedding ON memories USING ivfflat(embedding cosine_ops) WITH (lists = 100);

-- 搜索时加上 user_id 过滤
SELECT * FROM memories
WHERE user_id = $1
AND embedding <=> $2  -- cosine distance
ORDER BY embedding <=> $2
LIMIT $3;
```

### 3.4 Marketplace（Go 重写）

#### 3.4.1 核心接口

```go
type MarketplaceService interface {
    // Agent 市场
    PublishAgent(ctx context.Context, agentID string) error
    UnpublishAgent(ctx context.Context, agentID string) error
    ListPublishedAgents(ctx context.Context, filter *MarketFilter) ([]*MarketAgent, error)
    InstallAgent(ctx context.Context, userID, agentID string) (*Agent, error)
    ForkAgent(ctx context.Context, userID, agentID string) (*Agent, error)

    // Skills 市场
    ListSkills(ctx context.Context, filter *SkillFilter) ([]*Skill, error)
    InstallSkill(ctx context.Context, userID, skillID string) error
}

type MarketAgent struct {
    ID          string
    Name        string
    Description string
    AuthorID    string
    AuthorName  string
    InstallCount int
    Rating      float64
    Tags        []string
    ThumbnailURL string
}
```

#### 3.4.2 数据模型

```sql
-- Marketplace Items
CREATE TABLE market_items (
    id UUID PRIMARY KEY,
    type VARCHAR(20) NOT NULL,  -- 'agent' | 'skill'
    name VARCHAR(255) NOT NULL,
    description TEXT,
    author_id UUID NOT NULL,
    manifest JSONB NOT NULL,  -- 安装所需信息
    thumbnail_url VARCHAR(500),
    install_count INT DEFAULT 0,
    rating FLOAT DEFAULT 0,
    tags TEXT[],
    status VARCHAR(20) DEFAULT 'draft',  -- 'draft' | 'published' | 'archived'
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 用户安装记录
CREATE TABLE user_installs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    item_id UUID NOT NULL,
    installed_agent_id UUID,  -- 用户安装后在自己账号下的 Agent ID
    installed_at TIMESTAMP,
    UNIQUE(user_id, item_id)
);
```

### 3.5 Admin API

#### 3.5.1 路由设计

| 路由 | 说明 |
|------|------|
| `GET/POST /admin/channels` | 渠道列表/创建 |
| `GET/PUT/DELETE /admin/channels/:id` | 渠道详情/更新/删除 |
| `GET/POST /admin/packages` | 套餐列表/创建 |
| `GET/PUT/DELETE /admin/packages/:id` | 套餐详情/更新/删除 |
| `GET /admin/users` | 用户列表 |
| `PUT /admin/users/:id/quota` | 调整用户配额 |
| `GET /admin/usage` | 全局用量统计 |
| `GET /admin/orders` | 订单/充值记录 |

### 3.6 Relay Layer

详见：`docs/superpowers/specs/2026-04-09-relay-redesign-design.md`

---

## 4. 共享数据模型

### 4.1 ER 图

```
users ──1:N── quotas
  │           │
  │           └── N:1 ─── channels ──N:N── models
  │
  └──1:N── sessions
  │
  └──1:N── agents ──N:N── tools
  │
  └──1:N── memories (vectors)
  │
  └──1:N── market_installs ──N:1── market_items
  │
  └──1:N── subscriptions ──N:1── packages
  │
  └──1:N── topup_orders
```

### 4.2 核心表

```sql
-- 用户表
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    role VARCHAR(20) DEFAULT 'user',  -- 'user' | 'admin'
    group_name VARCHAR(20) DEFAULT 'default',  -- 'default' | 'vip' | 'svip'
    quota_balance DECIMAL(15,6) DEFAULT 0,  -- 余额（Quota）
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 渠道表
CREATE TABLE channels (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- 'openai' | 'anthropic' | 'gemini'
    base_url VARCHAR(500),
    api_key_encrypted TEXT,  -- 加密存储
    models TEXT[],
    rpm_limit INT,
    tpm_limit INT,
    base_cost DECIMAL(10,6),  -- 每1K token 成本(USD)
    markup DECIMAL(5,2) DEFAULT 1.0,
    cb_threshold INT DEFAULT 5,
    cb_timeout INT DEFAULT 30,
    strategy VARCHAR(20) DEFAULT 'weighted',  -- 'weighted' | 'priority'
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 模型路由表
CREATE TABLE model_routes (
    id UUID PRIMARY KEY,
    model VARCHAR(100) NOT NULL,
    strategy VARCHAR(20) DEFAULT 'weighted',
    created_at TIMESTAMP
);

-- 模型-渠道权重
CREATE TABLE model_channel_weights (
    id UUID PRIMARY KEY,
    route_id UUID REFERENCES model_routes(id),
    channel_id UUID REFERENCES channels(id),
    weight INT DEFAULT 100,
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true
);

-- 套餐表
CREATE TABLE packages (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    quota_amount DECIMAL(15,6) NOT NULL,  -- 包含配额
    price DECIMAL(10,2) NOT NULL,
    duration_days INT,  -- NULL 表示永久
    is_active BOOLEAN DEFAULT true,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP
);

-- 订阅表
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    package_id UUID REFERENCES packages(id),
    status VARCHAR(20) DEFAULT 'active',  -- 'active' | 'expired' | 'cancelled'
    started_at TIMESTAMP,
    expires_at TIMESTAMP,
    created_at TIMESTAMP
);

-- 充值订单表
CREATE TABLE topup_orders (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    amount DECIMAL(15,6) NOT NULL,  -- 充值配额
    money DECIMAL(10,2) NOT NULL,  -- 支付金额
    status VARCHAR(20) DEFAULT 'pending',  -- 'pending' | 'paid' | 'cancelled'
    trade_no VARCHAR(100),  -- 第三方交易号
    paid_at TIMESTAMP,
    created_at TIMESTAMP
);

-- Agent 表
CREATE TABLE agents (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    model VARCHAR(100),
    system_prompt TEXT,
    tools JSONB DEFAULT '[]',
    config JSONB DEFAULT '{}',
    is_public BOOLEAN DEFAULT false,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Memory 表
CREATE TABLE memories (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    content TEXT NOT NULL,
    embedding vector(1536),  -- pgvector
    metadata JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- MCP Server 表
CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    url VARCHAR(500) NOT NULL,
    auth_token_encrypted TEXT,
    status VARCHAR(20) DEFAULT 'disconnected',
    created_at TIMESTAMP
);

-- Marketplace Items
CREATE TABLE market_items (
    id UUID PRIMARY KEY,
    type VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    author_id UUID REFERENCES users(id),
    manifest JSONB NOT NULL,
    thumbnail_url VARCHAR(500),
    install_count INT DEFAULT 0,
    rating FLOAT DEFAULT 0,
    tags TEXT[],
    status VARCHAR(20) DEFAULT 'draft',
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- 用户安装记录
CREATE TABLE user_installs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    item_id UUID REFERENCES market_items(id),
    installed_agent_id UUID,
    installed_at TIMESTAMP,
    UNIQUE(user_id, item_id)
);

-- Agent Task 表
CREATE TABLE agent_tasks (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    agent_id UUID REFERENCES agents(id),
    title VARCHAR(255),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',  -- 'pending' | 'running' | 'paused' | 'completed' | 'failed'
    current_step INT DEFAULT 0,
    steps JSONB DEFAULT '[]',
    result JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

---

## 5. 技术选型

### 5.1 后端技术栈

| 技术 | 选择 | 说明 |
|------|------|------|
| 语言 | Go 1.22+ | 统一后端 |
| HTTP 框架 | Gin | 成熟稳定 |
| 数据库 | PostgreSQL 14+ | 已有 pgvector 扩展 |
| 向量 | pgvector | 用户隔离用 user_id namespace |
| ORM | GORM 或 sqlx | 待定 |
| 缓存 | Redis | Session、Rate Limit |
| 指标 | Prometheus + prometheus-client | 监控 |
| 配置 | Viper | 环境变量 + 配置文件 |

### 5.2 前端技术栈

| 技术 | 选择 | 说明 |
|------|------|------|
| 框架 | React 18+ | 现有项目 |
| 路由 | React Router | 路由分隔 C/B 端 |
| 状态 | Zustand + SWR | 已有 |
| UI 库 | @lobehub/ui | LobeHub 组件库 |
| 构建 | Vite | 现有项目 |

### 5.3 依赖库

```go
// go.mod (核心依赖)
require (
    github.com/gin-gonic/gin
    github.com/prometheus/client_golang
    github.com/pgvector/pgvector-go
    github.com/jmoiron/sqlx
    github.com/spf13/viper
    github.com/golang-jwt/jwt/v5
    golang.org/x/crypto  // bcrypt
)
```

---

## 6. 目录结构

```
src/
├── server/                          # Go 后端（重构）
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── api/                    # API 网关
│   │   │   ├── router.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── cors.go
│   │   │   └── response.go         # 统一响应格式
│   │   │
│   │   ├── agent/                   # Agent Runtime
│   │   │   ├── service.go
│   │   │   ├── planner.go           # 任务规划
│   │   │   ├── executor.go          # 工具执行
│   │   │   └── types.go
│   │   │
│   │   ├── task/                    # Agent Task (原 SOLO)
│   │   │   ├── service.go
│   │   │   ├── executor.go          # 任务执行器
│   │   │   └── types.go
│   │   │
│   │   ├── memory/                  # Memory / RAG
│   │   │   ├── service.go
│   │   │   ├── embedder.go          # 调用 Relay Embeddings
│   │   │   └── types.go
│   │   │
│   │   ├── mcp/                     # MCP Client
│   │   │   ├── client.go
│   │   │   ├── protocol.go
│   │   │   ├── sse.go               # SSE 传输
│   │   │   └── tools.go
│   │   │
│   │   ├── relay/                   # Relay Layer（见详细 spec）
│   │   │   ├── handler/
│   │   │   ├── router/
│   │   │   ├── channel/
│   │   │   ├── pool/
│   │   │   ├── billing/
│   │   │   └── metrics/
│   │   │
│   │   ├── admin/                   # Admin API
│   │   │   ├── channel.go
│   │   │   ├── package.go
│   │   │   ├── user.go
│   │   │   └── usage.go
│   │   │
│   │   ├── marketplace/            # Marketplace
│   │   │   ├── service.go
│   │   │   ├── agent.go
│   │   │   └── types.go
│   │   │
│   │   ├── quota/                   # 配额系统
│   │   │   ├── service.go
│   │   │   ├── preconsume.go
│   │   │   └── settle.go
│   │   │
│   │   ├── user/                    # 用户服务
│   │   ├── auth/                    # 认证服务
│   │   └── preferences/             # 偏好设置
│   │
│   ├── pkg/
│   │   ├── database/
│   │   │   ├── postgres.go
│   │   │   └── migrations/
│   │   ├── redis/
│   │   └── config/
│   │
│   └── migrations/
│
├── web/                            # React 前端（重构）
│   └── src/
│       ├── app/
│       │   ├── router.tsx           # 主路由
│       │   │
│       │   ├── (main)/              # C 端页面
│       │   │   ├── chat/
│       │   │   ├── agents/
│       │   │   ├── knowledge/
│       │   │   ├── marketplace/
│       │   │   └── settings/
│       │   │
│       │   └── (admin)/             # B 端页面
│       │       ├── channels/
│       │       ├── packages/
│       │       ├── users/
│       │       ├── usage/
│       │       └── orders/
│       │
│       ├── components/              # 共享组件
│       ├── features/                # LobeHub 提取的组件
│       │   ├── AgentBuilder/
│       │   ├── ChatInput/
│       │   ├── Conversation/
│       │   └── ...
│       ├── services/                # API 调用
│       ├── store/                    # Zustand stores
│       └── types/                    # 共享类型
│
└── config/
    └── .env.example
```

---

## 7. 实施计划

### Phase 1：基础设施（8 周）

| 周 | 任务 |
|----|------|
| 1 | 项目骨架搭建、Go 后端结构、数据库迁移设计 |
| 2 | Auth/User/Quota 共享服务 |
| 3 | Relay Layer 核心：Handler 骨架 + OpenAI Chat Completions |
| 4 | Relay Layer：高可用路由（CB + Rate Limiter + Health Check） |
| 5 | Relay Layer：剩余 OpenAI 接口（Embeddings/Images/Audio 等） |
| 6 | Agent Runtime 核心：Agent CRUD + Chat 对话 |
| 7 | Agent Task（原 SOLO）实现 |
| 8 | MCP Client 核心 + 内置工具 |

### Phase 2：Agent 增强（6 周）

| 周 | 任务 |
|----|------|
| 9 | Memory/RAG 系统 + pgvector 集成 |
| 10 | MCP Server 连接 + 工具发现 |
| 11 | Agent 工具执行串联测试 |
| 12 | Marketplace 核心（Agent 发布/安装） |
| 13 | Marketplace（Skills）|
| 14 | Agent 与 Relay 串联 + 计费集成 |

### Phase 3：Admin + 收尾（6 周）

| 周 | 任务 |
|----|------|
| 15 | Admin API 完整实现 |
| 16 | Admin UI 搭建 |
| 17 | 前端：C 端页面收尾 + B 端页面 |
| 18 | Prometheus Metrics 完整集成 |
| 19 | 端到端测试 + Bug 修复 |
| 20 | 文档 + 部署 |

---

## 8. 未纳入本期功能

| 功能 | 原因 |
|------|------|
| Anthropic Provider | Phase 2 |
| Gemini Provider | Phase 3 |
| Per-User 限流 | 第一期只做 Per-Channel |
| Latency-aware 路由 | 依赖监控数据积累 |
| 支付接入（Stripe/EPay） | 第一期先做余额充值基础 |
| 多语言 i18n | 错误消息国际化 |
| Agent 协作（Multi-Agent） | 未来扩展 |

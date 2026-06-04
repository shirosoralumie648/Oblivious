# Oblivious 完整融合设计文档

**文档版本**: v1.0  
**创建日期**: 2026-06-04  
**设计目标**: 全面重构 - 融合31个参考项目的最佳实践

---

## 1. 执行摘要

### 1.1 设计目标

从当前v08（商业完备单体）全面重构为**领域驱动微服务架构**，融合31个参考项目的160项核心功能，构建下一代多租户AI SaaS平台。

### 1.2 核心价值主张

- **API网关统一**: 100+ LLM提供商，负载均衡，语义缓存
- **工作流编排**: 可视化编排，20+节点类型，完整调试
- **知识库RAG**: 深度文档理解，混合检索，引用溯源
- **Agent系统**: ReAct+规划双引擎，150+工具生态
- **多渠道发布**: 10+渠道原生支持（Web/IM/API）
- **企业级商业**: Token级计费，完整支付，Marketplace生态
- **运维就绪**: 监控/告警/备份/灾难恢复完整

### 1.3 技术栈选型

| 层级 | 技术选择 | 理由 |
|------|---------|------|
| **后端语言** | Go 1.22+ | 保留v08选型，高性能，适合网关/计费 |
| **前端框架** | Next.js 14 (App Router) | lobe-chat最佳实践，现代化 |
| **UI组件** | Shadcn/ui + Tailwind CSS | lobe-chat标准，组件丰富 |
| **状态管理** | Zustand | lobe-chat选型，轻量高效 |
| **工作流编辑器** | React Flow | dify/FastGPT标准 |
| **主数据库** | PostgreSQL 16 + pgvector | 保留v08，向量检索原生支持 |
| **缓存** | Redis 7 | 保留v08，标准选择 |
| **向量数据库** | Qdrant | MaxKB推荐，性能优秀 |
| **分析数据库** | ClickHouse | helicone最佳实践 |
| **消息队列** | Kafka | 微服务标准 |
| **对象存储** | MinIO | helicone选型 |
| **服务通信** | gRPC + HTTP | 微服务标准 |

### 1.4 架构模式

**领域驱动微服务架构（DDD + Microservices）**

- 12个核心微服务，按领域边界划分
- 每个服务独立数据库（Database per Service）
- gRPC内部通信，HTTP外部暴露
- Kafka事件驱动异步通信
- API Gateway统一入口

---

## 2. 总体架构

### 2.1 系统架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         前端层 (Next.js)                             │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │  Chat UI   │  │ Workflow   │  │ Knowledge  │  │   Admin    │   │
│  │  (lobe)    │  │  Editor    │  │    UI      │  │    UI      │   │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                              ▼ HTTP/WebSocket
┌─────────────────────────────────────────────────────────────────────┐
│                    API Gateway (Gateway Service)                     │
│              认证 │ 路由 │ 限流 │ 熔断 │ 监控                        │
└─────────────────────────────────────────────────────────────────────┘
                              ▼ gRPC/HTTP
┌─────────────────────────────────────────────────────────────────────┐
│                         应用服务层                                   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │   Chat     │  │  Workflow  │  │    RAG     │  │   Agent    │   │
│  │  Service   │  │  Service   │  │  Service   │  │  Service   │   │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐   │
│  │  Billing   │  │Marketplace │  │   Admin    │  │  Channel   │   │
│  │  Service   │  │  Service   │  │  Service   │  │  Service   │   │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                              ▼ gRPC
┌─────────────────────────────────────────────────────────────────────┐
│                         核心基础层                                   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                    │
│  │   Relay    │  │    Task    │  │Observability│                   │
│  │  Service   │  │  Service   │  │  Service   │                    │
│  └────────────┘  └────────────┘  └────────────┘                    │
└─────────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         数据存储层                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐│
│  │PostgreSQL│ │  Redis   │ │  Qdrant  │ │ClickHouse│ │  MinIO   ││
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘│
└─────────────────────────────────────────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         消息与事件层                                 │
│                          Kafka Cluster                               │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 微服务详细定义

| 服务名 | 职责 | 参考来源 | 技术栈 | 数据库 |
|-------|------|---------|--------|--------|
| **gateway-service** | API网关、认证、路由、限流 | new-api, bifrost | Go, Envoy | Redis |
| **relay-service** | 统一LLM调用、负载均衡、语义缓存 | new-api, bifrost, litellm | Go | Redis, PostgreSQL |
| **chat-service** | 对话管理、消息流、实时协作 | lobe-chat, LibreChat | Go | PostgreSQL |
| **workflow-service** | 工作流引擎、节点编排、调试 | dify, FastGPT, Coze | Go | PostgreSQL |
| **rag-service** | 文档解析、向量化、混合检索 | ragflow, MaxKB, FastGPT | Go, Python(worker) | PostgreSQL, Qdrant |
| **agent-service** | Agent运行时、工具调用、MCP | dify, FastGPT, open-webui | Go | PostgreSQL |
| **billing-service** | 计费、配额、订阅、支付 | 当前v08, sub2api, new-api | Go | PostgreSQL |
| **marketplace-service** | Agent发布、审核、结算 | 当前v08, Coze | Go | PostgreSQL |
| **admin-service** | 后台管理、监控面板 | 当前v08, open-webui | Go | PostgreSQL |
| **channel-service** | 多渠道适配（飞书/微信等） | Coze | Go | PostgreSQL |
| **task-service** | 异步任务、定时调度 | Coze, anything-llm | Go | PostgreSQL, Redis |
| **observability-service** | 日志、指标、追踪聚合 | 当前v08, helicone | Go | ClickHouse, PostgreSQL |

### 2.3 服务间通信

**通信矩阵**:

| 源服务 | 目标服务 | 协议 | 场景 |
|-------|---------|------|------|
| Gateway | 所有服务 | HTTP/gRPC | 请求路由 |
| Chat | Relay | gRPC | LLM调用 |
| Workflow | Agent, RAG, Relay | gRPC | 节点执行 |
| Agent | Relay, RAG | gRPC | 工具调用 |
| Billing | Relay | Kafka | 计费事件 |
| Task | 所有服务 | Kafka | 异步任务 |
| Observability | 所有服务 | Kafka | 日志/指标收集 |

---

## 3. 核心领域设计

### 3.1 API网关与Relay层

#### 3.1.1 功能融合清单

| 功能 | 实现来源 | 优先级 |
|------|---------|--------|
| 多模型适配器（100+ provider） | new-api | P0 |
| 负载均衡（轮询/权重/自适应） | bifrost + new-api | P0 |
| 语义缓存（降低90%成本） | bifrost | P1 |
| 故障转移与重试 | new-api | P0 |
| 健康检查与自动摘除 | bifrost | P1 |
| 流式响应（SSE） | new-api | P0 |
| OpenAI兼容层 | new-api | P0 |

#### 3.1.2 架构设计

**Relay Service 架构**:

```
┌─────────────────────────────────────────────┐
│           Relay Service                     │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Request Handler                     │ │
│  │   - 请求解析                          │ │
│  │   - 身份验证                          │ │
│  │   - 计费预授权                        │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Semantic Cache (Bifrost)            │ │
│  │   - 向量相似度匹配                    │ │
│  │   - 缓存命中率 > 85%                  │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Load Balancer (Adaptive)            │ │
│  │   - 权重分配                          │ │
│  │   - 健康检查                          │ │
│  │   - 自动故障转移                      │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Provider Adapters (100+)            │ │
│  │   - OpenAI │ Claude │ Gemini │ ...   │ │
│  │   - 统一格式转换                      │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Response Handler                    │ │
│  │   - 流式/非流式                       │ │
│  │   - 计费结算                          │ │
│  │   - 指标上报                          │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

**核心接口**:

```go
// relay-service/internal/relay/service.go
type RelayService interface {
    // 统一LLM调用入口
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    
    // 流式调用
    CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
    
    // Embedding调用
    Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
}

type LoadBalancer interface {
    // 选择最佳channel
    SelectChannel(ctx context.Context, model string) (*Channel, error)
    
    // 健康检查
    HealthCheck(ctx context.Context, channelID string) error
}

type SemanticCache interface {
    // 查询缓存
    Get(ctx context.Context, query string, modelID string) (*CachedResponse, error)
    
    // 写入缓存
    Set(ctx context.Context, query string, modelID string, response *Response) error
}
```

#### 3.1.3 数据模型

```sql
-- relay channels（渠道配置）
CREATE TABLE relay_channels (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- openai, anthropic, gemini...
    type VARCHAR(20) NOT NULL,      -- chat, embedding, image...
    config JSONB NOT NULL,          -- 提供商配置（API key等）
    weight INT DEFAULT 1,           -- 负载均衡权重
    priority INT DEFAULT 0,         -- 优先级
    status VARCHAR(20) DEFAULT 'active',
    health_score FLOAT DEFAULT 1.0, -- 健康评分 0-1
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- relay cache（语义缓存）
CREATE TABLE relay_semantic_cache (
    id UUID PRIMARY KEY,
    query_hash VARCHAR(64) NOT NULL,  -- query的hash
    query_embedding VECTOR(1536),     -- query的embedding向量
    model_id VARCHAR(100) NOT NULL,
    response JSONB NOT NULL,          -- 缓存的响应
    hit_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_semantic_cache_embedding ON relay_semantic_cache 
USING hnsw (query_embedding vector_cosine_ops);

-- relay metrics（实时指标）
CREATE TABLE relay_metrics (
    id UUID PRIMARY KEY,
    channel_id UUID NOT NULL,
    request_count INT DEFAULT 0,
    success_count INT DEFAULT 0,
    error_count INT DEFAULT 0,
    total_tokens BIGINT DEFAULT 0,
    avg_latency_ms INT DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL
);
```

---

### 3.2 工作流引擎层

#### 3.2.1 功能融合清单

| 功能 | 实现来源 | 优先级 |
|------|---------|--------|
| 可视化编排（React Flow） | dify, FastGPT, Coze | P1 |
| 节点类型（20+种） | dify, FastGPT, Coze | P1 |
| LLM节点 | dify | P1 |
| 知识检索节点 | dify, FastGPT | P1 |
| 条件分支节点 | dify | P1 |
| 循环节点 | dify | P1 |
| 代码节点（Python/JS） | dify, Coze | P1 |
| HTTP请求节点 | dify | P1 |
| 数据库节点 | Coze | P2 |
| RPA节点（浏览器自动化） | FastGPT | P2 |
| 用户交互节点 | FastGPT | P2 |
| 调试能力（单点测试+完整链路） | FastGPT | P1 |
| 版本控制 | dify | P2 |

#### 3.2.2 工作流架构设计

**Workflow Service 架构**:

```
┌─────────────────────────────────────────────┐
│       Workflow Service                      │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Workflow Definition Manager         │ │
│  │   - 工作流CRUD                        │ │
│  │   - 版本控制                          │ │
│  │   - 模板管理                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Workflow Executor (Runtime)         │ │
│  │   - DAG解析                           │ │
│  │   - 节点调度                          │ │
│  │   - 状态机管理                        │ │
│  │   - 错误处理与重试                    │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Node Registry (20+ types)           │ │
│  │   - LLM │ Knowledge │ Condition       │ │
│  │   - Loop │ Code │ HTTP │ RPA         │ │
│  │   - Database │ UserInput │ ...       │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Debug & Trace System                │ │
│  │   - 单点测试                          │ │
│  │   - 完整调用链                        │ │
│  │   - 变量检查                          │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

**核心接口**:

```go
// workflow-service/internal/workflow/service.go
type WorkflowService interface {
    // 创建工作流
    Create(ctx context.Context, req *CreateWorkflowRequest) (*Workflow, error)
    
    // 执行工作流
    Execute(ctx context.Context, workflowID string, input map[string]interface{}) (*ExecutionResult, error)
    
    // 单点测试节点
    TestNode(ctx context.Context, nodeID string, input map[string]interface{}) (*NodeResult, error)
}

type Node interface {
    // 节点类型
    Type() NodeType
    
    // 执行节点
    Execute(ctx context.Context, input *NodeInput) (*NodeOutput, error)
    
    // 验证配置
    Validate() error
}

type WorkflowExecutor interface {
    // 执行DAG
    ExecuteDAG(ctx context.Context, dag *DAG, input map[string]interface{}) (*ExecutionResult, error)
    
    // 暂停执行（用户交互节点）
    Pause(ctx context.Context, executionID string) error
    
    // 恢复执行
    Resume(ctx context.Context, executionID string, userInput map[string]interface{}) error
}
```

#### 3.2.3 数据模型

```sql
-- workflows（工作流定义）
CREATE TABLE workflows (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    version INT DEFAULT 1,
    definition JSONB NOT NULL,  -- DAG定义（节点+边）
    variables JSONB,            -- 全局变量定义
    status VARCHAR(20) DEFAULT 'draft',  -- draft, active, archived
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- workflow_executions（工作流执行记录）
CREATE TABLE workflow_executions (
    id UUID PRIMARY KEY,
    workflow_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,  -- running, paused, completed, failed
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    duration_ms INT
);

-- workflow_node_executions（节点执行记录）
CREATE TABLE workflow_node_executions (
    id UUID PRIMARY KEY,
    execution_id UUID NOT NULL,
    node_id VARCHAR(100) NOT NULL,
    node_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    duration_ms INT
);
```

---

### 3.3 知识库与RAG层

#### 3.3.1 功能融合清单

| 功能 | 实现来源 | 优先级 |
|------|---------|--------|
| 文档解析（10+格式） | ragflow | P1 |
| 深度文档理解 | ragflow (deepdoc) | P1 |
| OCR识别 | ragflow | P2 |
| 智能分块（模板策略） | ragflow | P1 |
| QA拆分 | FastGPT | P1 |
| Embedding向量化 | MaxKB | P1 |
| 向量检索（HNSW） | MaxKB, Qdrant | P1 |
| 全文检索 | MaxKB | P1 |
| 混合检索+重排 | MaxKB, FastGPT | P1 |
| 引用溯源 | ragflow | P1 |
| 多库复用 | FastGPT | P2 |
| API知识库 | FastGPT | P2 |
| 增量更新 | FastGPT | P2 |
| 外部数据同步 | ragflow | P2 |

#### 3.3.2 RAG架构设计

**RAG Service 架构**:

```
┌─────────────────────────────────────────────┐
│         RAG Service                         │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Document Processor (ragflow)        │ │
│  │   - 深度文档解析                      │ │
│  │   - OCR识别                           │ │
│  │   - 格式转换                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Chunking Engine                     │ │
│  │   - 模板策略（ragflow）               │ │
│  │   - QA拆分（FastGPT）                 │ │
│  │   - 语义分块                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Embedding Service (MaxKB)           │ │
│  │   - 调用Relay获取embedding            │ │
│  │   - 批量向量化                        │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Vector Store (Qdrant)               │ │
│  │   - HNSW索引                          │ │
│  │   - 向量检索                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Hybrid Retriever (MaxKB)            │ │
│  │   - 向量检索 + 全文检索               │ │
│  │   - Reranker重排                      │ │
│  │   - 引用溯源（ragflow）               │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

**核心接口**:

```go
// rag-service/internal/rag/service.go
type RAGService interface {
    // 创建知识库
    CreateKnowledgeBase(ctx context.Context, req *CreateKBRequest) (*KnowledgeBase, error)
    
    // 上传文档
    UploadDocument(ctx context.Context, kbID string, file io.Reader) (*Document, error)
    
    // 检索
    Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)
}

type DocumentProcessor interface {
    // 解析文档
    Parse(ctx context.Context, file io.Reader, format string) (*ParsedDocument, error)
}

type ChunkingEngine interface {
    // 分块
    Chunk(ctx context.Context, doc *ParsedDocument, strategy ChunkStrategy) ([]*Chunk, error)
}

type HybridRetriever interface {
    // 混合检索
    Search(ctx context.Context, query string, kbIDs []string, topK int) ([]*RetrievalResult, error)
}
```

#### 3.3.3 数据模型

```sql
-- knowledge_bases（知识库）
CREATE TABLE knowledge_bases (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    chunk_strategy VARCHAR(50) DEFAULT 'template',
    chunk_size INT DEFAULT 500,
    chunk_overlap INT DEFAULT 50,
    embedding_model VARCHAR(100) DEFAULT 'text-embedding-3-small',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- documents（文档）
CREATE TABLE documents (
    id UUID PRIMARY KEY,
    kb_id UUID NOT NULL,
    name VARCHAR(500) NOT NULL,
    format VARCHAR(50) NOT NULL,  -- pdf, docx, txt...
    size_bytes BIGINT NOT NULL,
    status VARCHAR(20) DEFAULT 'processing',  -- processing, completed, failed
    storage_path VARCHAR(1000),
    chunk_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- document_chunks（文档块）
CREATE TABLE document_chunks (
    id UUID PRIMARY KEY,
    document_id UUID NOT NULL,
    kb_id UUID NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    embedding_model VARCHAR(100),
    metadata JSONB,  -- 页码、标题等
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Qdrant中的向量存储（通过SDK管理）
-- Collection: kb_{kb_id}
-- Vector dimension: 1536 (text-embedding-3-small)
-- Payload: {chunk_id, document_id, content, metadata}
```

---

## 4. 继续阅读

由于文档较长，已拆分为多个部分：

- **第一部分**（本文档）: 执行摘要、总体架构、核心领域（Gateway/Relay/Workflow/RAG）
- **第二部分**: Agent系统、计费商业化、Marketplace、多渠道
- **第三部分**: 前端设计、数据库完整Schema、API规范
- **第四部分**: 部署架构、监控运维、实施路线图

---

*文档继续编写中...*


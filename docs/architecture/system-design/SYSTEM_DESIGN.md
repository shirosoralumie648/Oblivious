# Oblivious 系统设计文档

> **版本**: v1.0  
> **创建时间**: 2026-06-03  
> **作者**: System Architecture Team

---

## 📑 目录

1. [系统架构设计](#1-系统架构设计)
2. [数据库设计](#2-数据库设计)
3. [API 设计](#3-api-设计)
4. [安全设计](#4-安全设计)
5. [性能设计](#5-性能设计)
6. [可观测性设计](#6-可观测性设计)

---

## 1. 系统架构设计

### 1.1 总体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                          客户端层                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ Web UI   │  │ Mobile   │  │  CLI     │  │  SDK     │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          接入层 (Nginx)                              │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  负载均衡 + SSL 终止 + 限流 + 静态资源                        │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          API 网关层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ 认证网关 │  │ 路由网关 │  │ 协议转换 │  │ 流量控制 │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          业务服务层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ Chat     │  │ Workflow │  │  RAG     │  │  Agent   │           │
│  │ Service  │  │ Service  │  │ Service  │  │ Service  │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
│                                                                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ User     │  │ Billing  │  │  Admin   │  │  Plugin  │           │
│  │ Service  │  │ Service  │  │ Service  │  │ Service  │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          核心引擎层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ LLM      │  │ Embedding│  │ Document │  │ Vector   │           │
│  │ Router   │  │ Engine   │  │ Parser   │  │ Search   │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          数据存储层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │PostgreSQL│  │  Redis   │  │ Qdrant   │  │ClickHouse│           │
│  │  (主库)  │  │  (缓存)  │  │ (向量库) │  │ (分析库) │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
│                                                                      │
│  ┌──────────┐  ┌──────────┐                                        │
│  │  MinIO   │  │  Kafka   │                                        │
│  │(对象存储)│  │(消息队列)│                                        │
│  └──────────┘  └──────────┘                                        │
└─────────────────────────────────────────────────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          外部服务层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ OpenAI   │  │ Claude   │  │ Gemini   │  │  ...     │           │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 微服务架构

#### 1.2.1 服务划分原则
- **单一职责**: 每个服务只负责一个业务域
- **松耦合**: 服务间通过 API 通信，避免共享数据库
- **高内聚**: 相关功能聚合在同一服务内
- **独立部署**: 每个服务可独立部署和扩展

#### 1.2.2 核心服务列表

| 服务名称 | 职责 | 端口 | 技术栈 |
|---------|------|------|--------|
| gateway-service | API 网关、路由、协议转换 | 8080 | Go + Gin |
| auth-service | 认证、授权、用户管理 | 8081 | Go + JWT |
| chat-service | 对话管理、消息存储 | 8082 | Go + WebSocket |
| workflow-service | 工作流编排、执行 | 8083 | Go + GORM |
| rag-service | 文档解析、向量检索 | 8084 | Python + FastAPI |
| agent-service | Agent 调度、工具调用 | 8085 | Go |
| billing-service | 计费、配额、使用统计 | 8086 | Go |
| admin-service | 管理后台、系统配置 | 8087 | Go |
| monitor-service | 日志收集、监控追踪 | 8088 | Go |
| web-service | 前端 Web 应用 | 3000 | Next.js |

### 1.3 服务间通信

#### 1.3.1 同步通信
- **协议**: gRPC (内部服务间) + HTTP/REST (外部 API)
- **格式**: Protocol Buffers (gRPC) + JSON (REST)
- **超时**: 默认 30s，可配置

#### 1.3.2 异步通信
- **消息队列**: Kafka
- **使用场景**:
  - 日志收集
  - 计费事件
  - 工作流任务
  - 通知推送

#### 1.3.3 服务发现
- **方案**: Consul / Kubernetes Service
- **健康检查**: HTTP /health 端点
- **负载均衡**: 客户端负载均衡（gRPC）

---

## 2. 数据库设计

### 2.1 主数据库 (PostgreSQL)

#### 2.1.1 核心表设计

**用户表 (users)**
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user', -- admin, user, guest
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, suspended, deleted
    quota_credits BIGINT NOT NULL DEFAULT 0,
    used_credits BIGINT NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_created_at ON users(created_at);
```

**API 密钥表 (api_keys)**
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    key_prefix VARCHAR(20) NOT NULL, -- 用于显示，如 sk-xxx...
    scopes TEXT[], -- ['chat', 'workflow', 'rag']
    rate_limit_rpm INTEGER, -- 每分钟请求数
    rate_limit_tpm INTEGER, -- 每分钟 Token 数
    expires_at TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_status ON api_keys(status);
```

**对话表 (conversations)**
```sql
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(500),
    model VARCHAR(100),
    system_prompt TEXT,
    temperature DECIMAL(3,2),
    max_tokens INTEGER,
    metadata JSONB,
    message_count INTEGER DEFAULT 0,
    total_tokens BIGINT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversations_user_id ON conversations(user_id);
CREATE INDEX idx_conversations_created_at ON conversations(created_at);
CREATE INDEX idx_conversations_status ON conversations(status);
```

**消息表 (messages)**
```sql
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL, -- system, user, assistant, tool
    content TEXT NOT NULL,
    attachments JSONB, -- 附件信息
    tool_calls JSONB, -- 工具调用记录
    token_count INTEGER,
    model VARCHAR(100),
    finish_reason VARCHAR(50),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_messages_role ON messages(role);
```

**工作流表 (workflows)**
```sql
CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    definition JSONB NOT NULL, -- 工作流 JSON 定义
    version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'draft', -- draft, published, archived
    is_public BOOLEAN DEFAULT FALSE,
    run_count INTEGER DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP
);

CREATE INDEX idx_workflows_user_id ON workflows(user_id);
CREATE INDEX idx_workflows_status ON workflows(status);
CREATE INDEX idx_workflows_created_at ON workflows(created_at);
```

**工作流执行表 (workflow_runs)**
```sql
CREATE TABLE workflow_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL, -- pending, running, completed, failed, cancelled
    input JSONB,
    output JSONB,
    error TEXT,
    execution_time_ms INTEGER,
    token_usage JSONB,
    node_results JSONB, -- 每个节点的执行结果
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_runs_workflow_id ON workflow_runs(workflow_id);
CREATE INDEX idx_workflow_runs_user_id ON workflow_runs(user_id);
CREATE INDEX idx_workflow_runs_status ON workflow_runs(status);
CREATE INDEX idx_workflow_runs_created_at ON workflow_runs(created_at);
```

**知识库表 (knowledge_bases)**
```sql
CREATE TABLE knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    embedding_model VARCHAR(100) NOT NULL,
    chunk_size INTEGER DEFAULT 500,
    chunk_overlap INTEGER DEFAULT 50,
    document_count INTEGER DEFAULT 0,
    total_chunks INTEGER DEFAULT 0,
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_knowledge_bases_user_id ON knowledge_bases(user_id);
CREATE INDEX idx_knowledge_bases_status ON knowledge_bases(status);
```

**文档表 (documents)**
```sql
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    file_type VARCHAR(50), -- pdf, docx, txt, md, etc.
    file_size BIGINT,
    file_url TEXT, -- MinIO 存储路径
    status VARCHAR(50) DEFAULT 'pending', -- pending, processing, completed, failed
    chunk_count INTEGER DEFAULT 0,
    parsed_text TEXT,
    metadata JSONB,
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP
);

CREATE INDEX idx_documents_kb_id ON documents(knowledge_base_id);
CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_created_at ON documents(created_at);
```

**计费记录表 (billing_records)**
```sql
CREATE TABLE billing_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    resource_type VARCHAR(50) NOT NULL, -- chat, workflow, rag, agent
    resource_id UUID,
    model VARCHAR(100),
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    cost_credits BIGINT NOT NULL, -- 消耗的积分（微积分，1积分=1000000微积分）
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_records_user_id ON billing_records(user_id);
CREATE INDEX idx_billing_records_created_at ON billing_records(created_at);
CREATE INDEX idx_billing_records_resource_type ON billing_records(resource_type);
```

继续创建其他表...

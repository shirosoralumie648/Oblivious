# Oblivious 完整融合设计文档 - 第三部分

**接续**: [第一部分](./2026-06-04-complete-fusion-design.md) | [第二部分](./2026-06-04-complete-fusion-design-part2.md)

---

## 7. 完整数据库Schema

### 7.1 数据库分布策略

**Database per Service原则**:

| 服务 | 数据库实例 | 存储内容 |
|------|----------|----------|
| Gateway | Redis | 路由缓存、限流计数器 |
| Relay | PostgreSQL + Redis | 渠道配置、缓存元数据 |
| Chat | PostgreSQL | 对话、消息、分叉 |
| Workflow | PostgreSQL | 工作流定义、执行记录 |
| RAG | PostgreSQL + Qdrant | 文档、分块元数据、向量 |
| Agent | PostgreSQL | Agent运行、工具调用、记忆 |
| Billing | PostgreSQL | 交易、配额、订阅（v08） |
| Marketplace | PostgreSQL | 发布物、结算（v08） |
| Admin | PostgreSQL | 系统配置（v08） |
| Channel | PostgreSQL | 渠道配置、消息日志 |
| Task | PostgreSQL + Redis | 任务定义、队列 |
| Observability | ClickHouse + PostgreSQL | 日志、指标、追踪 |

### 7.2 核心表结构（完整版）

#### 7.2.1 用户与组织（保留v08）

```sql
-- organizations（组织/租户）
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    subscription_tier VARCHAR(50) DEFAULT 'free',
    quota_tokens BIGINT DEFAULT 0,
    used_tokens BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- users（用户）
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(200),
    avatar_url VARCHAR(500),
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- organization_members（组织成员）
CREATE TABLE organization_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(50) NOT NULL,  -- owner, admin, member
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(organization_id, user_id)
);
```

#### 7.2.2 Chat相关表

```sql
-- conversations（对话）
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(500),
    model VARCHAR(100),
    parent_id UUID REFERENCES conversations(id),  -- 对话分叉
    knowledge_base_ids UUID[],
    persona_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- messages（消息）
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    role VARCHAR(20) NOT NULL,  -- user, assistant, system
    content TEXT NOT NULL,
    model VARCHAR(100),
    tokens_used INT,
    metadata JSONB,  -- 图片、文件等
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- personas（人格配置 - from Coze）
CREATE TABLE personas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name VARCHAR(200) NOT NULL,
    role VARCHAR(200),
    style TEXT,
    tone TEXT,
    constraints TEXT[],
    opening_message TEXT,
    suggested_questions TEXT[],
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

#### 7.2.3 Relay相关表（已在第一部分）

```sql
-- relay_channels, relay_semantic_cache, relay_metrics
-- 见第一部分 3.1.3
```

#### 7.2.4 Workflow相关表（已在第一部分）

```sql
-- workflows, workflow_executions, workflow_node_executions
-- 见第一部分 3.2.3
```

#### 7.2.5 Knowledge相关表（已在第一部分）

```sql
-- knowledge_bases, documents, document_chunks
-- 见第一部分 3.3.3
```

#### 7.2.6 Agent相关表（已在第二部分）

```sql
-- agent_runs, agent_tool_runs, agent_memories
-- 见第二部分 3.4.4
```

#### 7.2.7 Billing相关表（保留v08）

```sql
-- 保留当前v08的完整Billing Schema:
-- - subscriptions（订阅）
-- - subscription_items（订阅项）
-- - invoices（发票）
-- - payments（支付）
-- - quota_transactions（配额交易）
-- - usage_records（使用记录）
-- - refunds（退款）

-- 新增：并发控制（from sub2api）
CREATE TABLE concurrency_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID,  -- NULL表示组织级
    max_concurrent_requests INT NOT NULL DEFAULT 5,
    current_concurrent INT DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 新增：Token速率限制（from sub2api）
CREATE TABLE token_rate_limits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID,
    window_seconds INT NOT NULL DEFAULT 60,
    max_tokens_per_window BIGINT NOT NULL,
    current_window_start TIMESTAMPTZ,
    current_window_tokens BIGINT DEFAULT 0
);
```

#### 7.2.8 Marketplace相关表（保留v08）

```sql
-- 保留当前v08的完整Marketplace Schema:
-- - marketplace_agents（发布物）
-- - marketplace_installs（安装记录）
-- - marketplace_reviews（评论）
-- - marketplace_transactions（交易）
-- - marketplace_settlements（结算）

-- 新增：模板市场（from Coze）
CREATE TABLE marketplace_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    type VARCHAR(50) NOT NULL,  -- workflow, bot, plugin
    name VARCHAR(200) NOT NULL,
    description TEXT,
    template_data JSONB NOT NULL,
    category VARCHAR(100),
    tags TEXT[],
    downloads_count INT DEFAULT 0,
    rating_avg DECIMAL(3,2),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

#### 7.2.9 Channel相关表（新增）

```sql
-- channel_configs（渠道配置）
CREATE TABLE channel_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    type VARCHAR(50) NOT NULL,  -- feishu, wechat, discord, slack, web_sdk
    name VARCHAR(200) NOT NULL,
    config JSONB NOT NULL,  -- 渠道专用配置
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- channel_messages（渠道消息日志）
CREATE TABLE channel_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES channel_configs(id),
    conversation_id UUID REFERENCES conversations(id),
    direction VARCHAR(10) NOT NULL,  -- inbound, outbound
    raw_message JSONB NOT NULL,
    transformed_message JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

#### 7.2.10 Task相关表（新增）

```sql
-- scheduled_tasks（定时任务 - from Coze）
CREATE TABLE scheduled_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name VARCHAR(200) NOT NULL,
    type VARCHAR(50) NOT NULL,  -- workflow, agent
    target_id UUID NOT NULL,  -- workflow_id or agent_id
    cron_expression VARCHAR(100) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- task_executions（任务执行记录）
CREATE TABLE task_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES scheduled_tasks(id),
    status VARCHAR(20) NOT NULL,  -- running, completed, failed
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    result JSONB,
    error TEXT
);
```

#### 7.2.11 Observability（ClickHouse）

```sql
-- ClickHouse表（日志聚合 - from helicone）
CREATE TABLE request_logs (
    id UUID,
    timestamp DateTime64(3),
    organization_id UUID,
    user_id UUID,
    service String,  -- relay, chat, workflow...
    endpoint String,
    method String,
    status_code UInt16,
    duration_ms UInt32,
    request_tokens UInt32,
    response_tokens UInt32,
    model String,
    cost_usd Float64,
    error String,
    trace_id UUID,
    metadata String  -- JSON string
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (organization_id, timestamp, service);
```

---

## 8. API规范

### 8.1 API设计原则

- **RESTful风格**：资源命名遵循REST最佳实践
- **统一响应格式**：成功/错误响应格式一致
- **版本管理**：URL包含版本 `/api/v1/`
- **认证**：Bearer Token（JWT）
- **限流**：响应头返回限流信息

### 8.2 统一响应格式

```json
// 成功响应
{
  "success": true,
  "data": { ... },
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}

// 错误响应
{
  "success": false,
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "Token quota exceeded",
    "details": { ... }
  }
}
```

### 8.3 核心API端点

#### 8.3.1 Relay API（OpenAI兼容）

```
POST   /api/v1/relay/chat/completions
POST   /api/v1/relay/embeddings
POST   /api/v1/relay/images/generations
POST   /api/v1/relay/audio/transcriptions
GET    /api/v1/relay/models
```

#### 8.3.2 Chat API

```
GET    /api/v1/conversations
POST   /api/v1/conversations
GET    /api/v1/conversations/:id
PUT    /api/v1/conversations/:id
DELETE /api/v1/conversations/:id
POST   /api/v1/conversations/:id/fork        # 对话分叉
POST   /api/v1/conversations/:id/messages
GET    /api/v1/conversations/:id/messages
```

#### 8.3.3 Workflow API

```
GET    /api/v1/workflows
POST   /api/v1/workflows
GET    /api/v1/workflows/:id
PUT    /api/v1/workflows/:id
DELETE /api/v1/workflows/:id
POST   /api/v1/workflows/:id/execute
POST   /api/v1/workflows/:id/test-node      # 单点测试
GET    /api/v1/workflows/:id/executions
GET    /api/v1/workflows/:id/executions/:exec_id
POST   /api/v1/workflows/:id/executions/:exec_id/pause
POST   /api/v1/workflows/:id/executions/:exec_id/resume
```

#### 8.3.4 Knowledge API

```
GET    /api/v1/knowledge-bases
POST   /api/v1/knowledge-bases
GET    /api/v1/knowledge-bases/:id
PUT    /api/v1/knowledge-bases/:id
DELETE /api/v1/knowledge-bases/:id
POST   /api/v1/knowledge-bases/:id/documents
GET    /api/v1/knowledge-bases/:id/documents
POST   /api/v1/knowledge-bases/:id/retrieve   # 检索测试
DELETE /api/v1/documents/:id
```

#### 8.3.5 Agent API

```
POST   /api/v1/agent/runs
GET    /api/v1/agent/runs/:id
POST   /api/v1/agent/runs/:id/approve-tool   # 审批工具调用
POST   /api/v1/agent/runs/:id/retry-tool
GET    /api/v1/agent/tools                    # 可用工具列表
POST   /api/v1/agent/memories                 # 创建记忆
GET    /api/v1/agent/memories                 # 检索记忆
```

#### 8.3.6 Channel API

```
GET    /api/v1/channels
POST   /api/v1/channels
GET    /api/v1/channels/:id
PUT    /api/v1/channels/:id
DELETE /api/v1/channels/:id
POST   /api/v1/channels/:id/test              # 测试连接
POST   /api/v1/channels/webhook/:channel_id   # 渠道回调
```

---

## 9. 部署架构

### 9.1 生产环境架构（Kubernetes）

```
┌──────────────────────────────────────────────────┐
│              Ingress (NGINX)                     │
│          SSL Termination, Rate Limiting          │
└──────────────────────────────────────────────────┘
                      ▼
┌──────────────────────────────────────────────────┐
│           Gateway Service (3 replicas)           │
│        认证、路由、熔断、监控埋点               │
└──────────────────────────────────────────────────┘
                      ▼
        ┌──────────────┬──────────────┐
        ▼              ▼              ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Relay     │  │    Chat     │  │  Workflow   │
│  (5 pods)   │  │  (3 pods)   │  │  (3 pods)   │
└─────────────┘  └─────────────┘  └─────────────┘
        ▼              ▼              ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│    RAG      │  │   Agent     │  │  Billing    │
│  (3 pods)   │  │  (3 pods)   │  │  (2 pods)   │
└─────────────┘  └─────────────┘  └─────────────┘
        ▼              ▼              ▼
┌──────────────────────────────────────────────────┐
│                Kafka Cluster                     │
│            (3 brokers, replication=3)            │
└──────────────────────────────────────────────────┘
                      ▼
┌──────────────────────────────────────────────────┐
│              Data Layer                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │PostgreSQL│  │  Redis   │  │  Qdrant  │      │
│  │(Primary+ │  │ Cluster  │  │ Cluster  │      │
│  │ Standby) │  │ (6 nodes)│  │ (3 nodes)│      │
│  └──────────┘  └──────────┘  └──────────┘      │
│  ┌──────────┐  ┌──────────┐                     │
│  │ClickHouse│  │  MinIO   │                     │
│  │ Cluster  │  │ Cluster  │                     │
│  │ (3 nodes)│  │ (4 nodes)│                     │
│  └──────────┘  └──────────┘                     │
└──────────────────────────────────────────────────┘
```

### 9.2 服务资源配置

| 服务 | Replicas | CPU Request | Memory Request | 自动扩缩容 |
|------|----------|-------------|----------------|-----------|
| Gateway | 3 | 500m | 512Mi | 3-10 |
| Relay | 5 | 1000m | 1Gi | 5-20 |
| Chat | 3 | 500m | 512Mi | 3-10 |
| Workflow | 3 | 1000m | 1Gi | 3-10 |
| RAG | 3 | 2000m | 2Gi | 3-10 |
| Agent | 3 | 1000m | 1Gi | 3-10 |
| Billing | 2 | 500m | 512Mi | 2-5 |
| Marketplace | 2 | 500m | 512Mi | 2-5 |
| Channel | 2 | 500m | 512Mi | 2-5 |
| Task | 2 | 500m | 512Mi | 2-5 |
| Observability | 2 | 500m | 512Mi | - |

### 9.3 数据库配置

**PostgreSQL**:
- Primary: 4 vCPU, 16GB RAM
- Standby: 4 vCPU, 16GB RAM
- 连接池: PgBouncer (500 max connections)

**Redis Cluster**:
- 6节点 (3 master + 3 replica)
- 每节点: 2 vCPU, 8GB RAM

**Qdrant**:
- 3节点集群
- 每节点: 4 vCPU, 16GB RAM, 500GB SSD

**ClickHouse**:
- 3节点集群
- 每节点: 8 vCPU, 32GB RAM, 1TB SSD

**MinIO**:
- 4节点集群（分布式模式）
- 每节点: 2 vCPU, 4GB RAM, 2TB HDD

---

## 10. 监控与运维

### 10.1 监控指标（保留v08）

**保留当前v08的完整监控体系**:
- Prometheus + Grafana
- 预制Dashboard
- 告警规则

**新增指标**:
```yaml
# Relay Service
- relay_request_total (counter)
- relay_request_duration_seconds (histogram)
- relay_cache_hit_rate (gauge)
- relay_channel_health_score (gauge)

# Workflow Service
- workflow_execution_total (counter)
- workflow_execution_duration_seconds (histogram)
- workflow_node_error_rate (gauge)

# RAG Service
- rag_document_processing_duration_seconds (histogram)
- rag_retrieval_latency_seconds (histogram)
- rag_chunk_count (gauge)

# Agent Service
- agent_run_total (counter)
- agent_tool_call_total (counter)
- agent_iteration_count (histogram)
```

### 10.2 告警规则（增量）

```yaml
# workflow-alerts.yaml
groups:
  - name: workflow
    rules:
      - alert: WorkflowExecutionFailureRate
        expr: rate(workflow_execution_total{status="failed"}[5m]) > 0.1
        for: 5m
        annotations:
          summary: "Workflow execution failure rate > 10%"
      
      - alert: WorkflowExecutionStuck
        expr: workflow_execution_duration_seconds > 3600
        annotations:
          summary: "Workflow execution running for > 1 hour"

# rag-alerts.yaml
groups:
  - name: rag
    rules:
      - alert: RAGRetrievalSlowness
        expr: histogram_quantile(0.95, rag_retrieval_latency_seconds) > 2
        for: 5m
        annotations:
          summary: "RAG retrieval P95 latency > 2s"
      
      - alert: QdrantDown
        expr: up{job="qdrant"} == 0
        for: 1m
        annotations:
          summary: "Qdrant instance down"
```

### 10.3 日志策略

**结构化日志格式**（保留v08）:
```json
{
  "timestamp": "2026-06-04T10:30:00Z",
  "level": "info",
  "service": "relay-service",
  "trace_id": "uuid",
  "span_id": "uuid",
  "organization_id": "uuid",
  "user_id": "uuid",
  "message": "Request completed",
  "metadata": {
    "model": "gpt-4",
    "tokens": 1500,
    "duration_ms": 2300
  }
}
```

**日志聚合**:
- **应用日志** → Loki → Grafana
- **请求日志** → ClickHouse → 分析面板
- **审计日志** → PostgreSQL → 合规报告

---

## 11. 安全设计

### 11.1 认证与授权（保留v08）

**保留当前v08的完整安全机制**:
- JWT认证
- RBAC权限控制
- API Key管理
- 审计日志

**增强**:
- SSO集成（from open-webui）
- SCIM 2.0（from open-webui）

### 11.2 数据安全

| 安全措施 | 实现 |
|---------|------|
| **传输加密** | TLS 1.3 |
| **静态加密** | PostgreSQL透明加密 |
| **密钥管理** | 配置加密存储，环境变量注入 |
| **API Key存储** | bcrypt hash |
| **敏感数据脱敏** | 日志自动脱敏 |

### 11.3 租户隔离（保留v08）

- **数据库行级安全**：所有查询自动添加 `organization_id` 过滤
- **对象存储隔离**：每个组织独立prefix
- **向量存储隔离**：Qdrant collection命名 `kb_{organization_id}_{kb_id}`

---

## 12. 迁移策略

### 12.1 从v08迁移路径

#### 阶段1: 双写模式（Week 1-4）

```
用户请求 → Gateway (新) → Relay (新) → LLM
            ↓
            v08 Billing (保留) ← 计费事件
```

- Gateway和Relay使用新架构
- 计费、用户、组织继续使用v08数据库
- 双写保证数据一致性

#### 阶段2: 灰度发布（Week 5-8）

```
10% 流量 → 新架构（Chat + Workflow + RAG）
90% 流量 → v08
```

- 按组织ID哈希分流
- 新功能（Workflow/RAG）仅在新架构
- 密切监控错误率和性能

#### 阶段3: 全量切换（Week 9-12）

```
100% 流量 → 新架构
```

- 所有用户切换到新架构
- v08保留14天作为回滚备份
- 数据迁移工具批量迁移历史数据

### 12.2 数据迁移工具

```go
// migration-tool/main.go
type Migrator interface {
    // 迁移对话历史
    MigrateConversations(ctx context.Context, orgID string) error
    
    // 迁移计费数据
    MigrateBilling(ctx context.Context, orgID string) error
    
    // 验证数据一致性
    Validate(ctx context.Context, orgID string) (*ValidationReport, error)
}
```

---

## 13. 成本估算

### 13.1 基础设施成本（月度）

| 资源 | 配置 | 月成本（USD） |
|------|------|--------------|
| **Kubernetes节点** | 10 nodes × 8vCPU/32GB | $2,400 |
| **PostgreSQL** | Primary + Standby | $600 |
| **Redis Cluster** | 6 nodes | $480 |
| **Qdrant** | 3 nodes | $720 |
| **ClickHouse** | 3 nodes | $1,200 |
| **MinIO** | 4 nodes × 2TB | $320 |
| **Kafka** | 3 brokers | $360 |
| **负载均衡** | ALB/NLB | $150 |
| **流量** | 10TB egress | $900 |
| **监控** | Prometheus/Grafana | $200 |
| **备份** | S3存储 | $150 |
| **总计** | | **$7,480** |

### 13.2 开发成本估算

| 阶段 | 工作量 | 人员配置 | 成本估算 |
|------|--------|---------|---------|
| Q1: 基础设施 | 12周 | 2 后端 + 1 前端 | $180k |
| Q2: 工作流/RAG | 12周 | 3 后端 + 1 前端 | $240k |
| Q3: Agent/工具 | 12周 | 3 后端 + 1 前端 | $240k |
| Q4: 多渠道/完善 | 12周 | 2 后端 + 1 前端 + 1 QA | $210k |
| **总计** | 48周 | | **$870k** |

---

## 14. 风险与缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|---------|
| **Python代码迁移Go难度高** | 中 | 高 | 阶段性迁移，先保留Python worker |
| **性能不达预期** | 低 | 高 | 提前压测，保留v08回滚路径 |
| **第三方API变更** | 中 | 中 | 适配器模式隔离，版本锁定 |
| **数据迁移失败** | 低 | 高 | 灰度迁移，双写验证 |
| **微服务复杂度** | 高 | 中 | 完善文档，统一脚手架 |
| **团队学习曲线** | 中 | 中 | 提前培训，结对编程 |

---

## 15. 成功标准

### 15.1 技术指标

| 指标 | 目标 | 当前v08 |
|------|------|---------|
| **API P95延迟** | < 500ms | 800ms |
| **Relay P95延迟** | < 100ms | 200ms |
| **工作流执行成功率** | > 99% | N/A |
| **RAG检索P95** | < 2s | N/A |
| **系统可用性** | > 99.9% | 99.5% |
| **缓存命中率** | > 85% | N/A |

### 15.2 业务指标

| 指标 | 目标 | 当前v08 |
|------|------|---------|
| **月活用户增长** | +50% | - |
| **工作流创建量** | 10k+/月 | 0 |
| **知识库文档量** | 1M+ chunks | - |
| **Agent调用量** | 100k+/月 | 5k/月 |
| **Marketplace GMV** | +200% | - |

---

## 16. 总结

### 16.1 核心价值

1. **API网关统一**：100+ LLM提供商，降低90%成本（语义缓存）
2. **工作流编排**：20+节点，完整调试，企业级UX
3. **知识库RAG**：深度文档理解，混合检索，引用溯源
4. **Agent双引擎**：ReAct + 规划，150+工具生态
5. **多渠道发布**：10+渠道原生支持
6. **商业完备**：保留v08全部计费/支付/Marketplace

### 16.2 技术亮点

- **微服务架构**：清晰领域边界，独立扩展
- **博采众长**：31个项目最佳实践融合
- **渐进迁移**：保留v08核心，降低风险
- **生产就绪**：完整监控、运维、灾备

### 16.3 下一步行动

1. **用户审查设计文档**（本周）
2. **技术预研**（Python→Go迁移方案）
3. **团队招募**（后端3人，前端1人，QA1人）
4. **Q1启动**（Week 1: 项目初始化）

---

**文档完成！**

三部分设计文档总计：
- **第一部分**：执行摘要、总体架构、Gateway/Relay/Workflow/RAG
- **第二部分**：Agent、计费、Marketplace、多渠道、前端、实施路线
- **第三部分**：完整Schema、API规范、部署架构、监控运维、迁移策略、成本风险

*设计文档已完整，等待用户审查和反馈。*





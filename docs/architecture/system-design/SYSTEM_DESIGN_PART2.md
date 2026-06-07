# Oblivious 系统设计文档 - 第二部分

## 2.2 缓存设计 (Redis)

### 2.2.1 缓存策略

**缓存层次**:
```
L1: 应用内存缓存 (本地缓存，TTL 60s)
    ↓ (miss)
L2: Redis 缓存 (分布式缓存，TTL 300-3600s)
    ↓ (miss)
L3: 数据库 (持久化存储)
```

### 2.2.2 Redis 数据结构设计

**用户会话 (Session)**
```
Key:   session:{session_id}
Type:  Hash
TTL:   7天
Fields:
  - user_id: UUID
  - email: string
  - role: string
  - expires_at: timestamp
```

**API 密钥缓存**
```
Key:   apikey:{key_hash}
Type:  Hash
TTL:   1小时
Fields:
  - user_id: UUID
  - scopes: JSON
  - rate_limit_rpm: int
  - rate_limit_tpm: int
  - status: string
```

**速率限制 (Rate Limiting)**
```
# 请求频率限制
Key:   ratelimit:rpm:{user_id}:{minute}
Type:  String (计数器)
TTL:   60秒
Value: 请求次数

# Token 频率限制
Key:   ratelimit:tpm:{user_id}:{minute}
Type:  String (计数器)
TTL:   60秒
Value: Token 数量
```

**LLM 响应缓存 (语义缓存)**
```
Key:   llm_cache:{embedding_hash}
Type:  String
TTL:   24小时
Value: JSON {
  "model": "gpt-4",
  "prompt": "...",
  "response": "...",
  "tokens": 150,
  "created_at": "2026-06-03T10:00:00Z"
}
```

**工作流执行队列**
```
Key:   workflow:queue:pending
Type:  List (LPUSH/RPOP)
Value: workflow_run_id (UUID)
```

**在线用户统计**
```
Key:   users:online
Type:  Set
TTL:   无 (定期清理)
Members: user_id (UUID)
```

### 2.2.3 缓存更新策略

| 场景 | 策略 | 说明 |
|------|------|------|
| 用户信息更新 | Write Through | 先写数据库，再删除缓存 |
| API Key 创建 | Cache Aside | 读时缓存，写时删除 |
| 配置更新 | Pub/Sub | 发布配置更新事件，所有节点刷新 |
| 统计数据 | Lazy Load | 过期后重新计算 |

---

## 2.3 向量数据库 (Qdrant)

### 2.3.1 Collection 设计

**文档块向量集合 (document_chunks)**
```json
{
  "collection_name": "document_chunks",
  "vectors": {
    "size": 1536,  // OpenAI text-embedding-3-small
    "distance": "Cosine"
  },
  "payload_schema": {
    "document_id": "uuid",
    "knowledge_base_id": "uuid",
    "chunk_index": "integer",
    "text": "text",
    "metadata": {
      "source": "string",
      "page": "integer",
      "file_type": "string"
    }
  }
}
```

**对话历史向量集合 (conversation_embeddings)**
```json
{
  "collection_name": "conversation_embeddings",
  "vectors": {
    "size": 1536,
    "distance": "Cosine"
  },
  "payload_schema": {
    "conversation_id": "uuid",
    "message_id": "uuid",
    "user_id": "uuid",
    "text": "text",
    "timestamp": "datetime"
  }
}
```

### 2.3.2 索引策略

**HNSW 参数**:
```yaml
hnsw_config:
  m: 16              # 连接数
  ef_construct: 100  # 构建时的搜索宽度
  ef_search: 50      # 搜索时的搜索宽度
```

**分片策略**:
- 按 knowledge_base_id 分片
- 每个分片最大 100万 向量
- 自动扩展分片

---

## 2.4 分析数据库 (ClickHouse)

### 2.4.1 表设计

**请求日志表 (request_logs)**
```sql
CREATE TABLE request_logs (
    timestamp DateTime64(3),
    request_id String,
    user_id UUID,
    api_key_id Nullable(UUID),
    method String,
    path String,
    model String,
    prompt_tokens UInt32,
    completion_tokens UInt32,
    total_tokens UInt32,
    latency_ms UInt32,
    status_code UInt16,
    error Nullable(String),
    metadata String  -- JSON
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (user_id, timestamp)
TTL timestamp + INTERVAL 90 DAY;
```

**使用统计表 (usage_stats)**
```sql
CREATE TABLE usage_stats (
    date Date,
    hour UInt8,
    user_id UUID,
    model String,
    request_count UInt64,
    total_tokens UInt64,
    total_cost_credits UInt64,
    avg_latency_ms UInt32
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, hour, user_id, model);
```

**模型性能表 (model_performance)**
```sql
CREATE TABLE model_performance (
    timestamp DateTime,
    model String,
    provider String,
    avg_latency_ms UInt32,
    p95_latency_ms UInt32,
    p99_latency_ms UInt32,
    success_rate Float32,
    request_count UInt64
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, model, provider);
```

---

## 3. API 设计

### 3.1 API 规范

**基础规范**:
- **协议**: HTTP/1.1, HTTP/2
- **格式**: JSON (application/json)
- **编码**: UTF-8
- **版本**: URL 路径版本 (/v1/, /v2/)
- **认证**: Bearer Token (Authorization: Bearer sk-xxx)

**统一响应格式**:
```json
{
  "success": true,
  "data": { /* 业务数据 */ },
  "error": null,
  "metadata": {
    "request_id": "uuid",
    "timestamp": "2026-06-03T10:00:00Z",
    "version": "v1"
  }
}
```

**错误响应格式**:
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "INVALID_API_KEY",
    "message": "Invalid API key provided",
    "details": {
      "field": "authorization",
      "reason": "expired"
    }
  },
  "metadata": {
    "request_id": "uuid",
    "timestamp": "2026-06-03T10:00:00Z"
  }
}
```

### 3.2 核心 API 端点

#### 3.2.1 认证 API

**注册**
```http
POST /v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "username": "johndoe",
  "password": "SecurePass123!"
}

Response 201:
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "email": "user@example.com",
    "username": "johndoe",
    "access_token": "jwt_token",
    "refresh_token": "jwt_refresh_token",
    "expires_in": 3600
  }
}
```

**登录**
```http
POST /v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "SecurePass123!"
}

Response 200:
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "access_token": "jwt_token",
    "refresh_token": "jwt_refresh_token",
    "expires_in": 3600
  }
}
```

#### 3.2.2 对话 API (OpenAI 兼容)

**创建对话补全**
```http
POST /v1/chat/completions
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 150,
  "stream": false
}

Response 200:
{
  "id": "chatcmpl-xxxxx",
  "object": "chat.completion",
  "created": 1709481600,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 9,
    "total_tokens": 29
  }
}
```

**流式响应**
```http
POST /v1/chat/completions
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "model": "gpt-4",
  "messages": [...],
  "stream": true
}

Response 200 (Server-Sent Events):
data: {"id":"chatcmpl-xxxxx","object":"chat.completion.chunk","created":1709481600,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-xxxxx","object":"chat.completion.chunk","created":1709481600,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: [DONE]
```

#### 3.2.3 工作流 API

**创建工作流**
```http
POST /v1/workflows
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "name": "Customer Support Bot",
  "description": "Automated customer support workflow",
  "definition": {
    "nodes": [
      {
        "id": "start",
        "type": "trigger",
        "config": {}
      },
      {
        "id": "llm1",
        "type": "llm",
        "config": {
          "model": "gpt-4",
          "prompt": "Analyze customer query: {{input}}"
        }
      },
      {
        "id": "condition1",
        "type": "condition",
        "config": {
          "expression": "{{llm1.sentiment}} == 'negative'"
        }
      }
    ],
    "edges": [
      {"from": "start", "to": "llm1"},
      {"from": "llm1", "to": "condition1"}
    ]
  }
}

Response 201:
{
  "success": true,
  "data": {
    "workflow_id": "uuid",
    "name": "Customer Support Bot",
    "version": 1,
    "status": "draft",
    "created_at": "2026-06-03T10:00:00Z"
  }
}
```

**执行工作流**
```http
POST /v1/workflows/{workflow_id}/run
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "input": {
    "customer_message": "I'm not satisfied with the product"
  }
}

Response 200:
{
  "success": true,
  "data": {
    "run_id": "uuid",
    "workflow_id": "uuid",
    "status": "running",
    "started_at": "2026-06-03T10:00:00Z"
  }
}
```

**获取执行结果**
```http
GET /v1/workflows/runs/{run_id}
Authorization: Bearer sk-xxxxx

Response 200:
{
  "success": true,
  "data": {
    "run_id": "uuid",
    "workflow_id": "uuid",
    "status": "completed",
    "input": {...},
    "output": {
      "sentiment": "negative",
      "recommended_action": "escalate_to_human"
    },
    "execution_time_ms": 2500,
    "token_usage": {
      "total_tokens": 150
    },
    "node_results": [...]
  }
}
```

#### 3.2.4 知识库 API

**创建知识库**
```http
POST /v1/knowledge-bases
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "name": "Product Documentation",
  "description": "All product docs and FAQs",
  "embedding_model": "text-embedding-3-small",
  "chunk_size": 500,
  "chunk_overlap": 50
}

Response 201:
{
  "success": true,
  "data": {
    "knowledge_base_id": "uuid",
    "name": "Product Documentation",
    "status": "active",
    "created_at": "2026-06-03T10:00:00Z"
  }
}
```

**上传文档**
```http
POST /v1/knowledge-bases/{kb_id}/documents
Authorization: Bearer sk-xxxxx
Content-Type: multipart/form-data

file: product_manual.pdf
metadata: {"category": "manual", "version": "2.0"}

Response 201:
{
  "success": true,
  "data": {
    "document_id": "uuid",
    "name": "product_manual.pdf",
    "status": "processing",
    "file_size": 2048576,
    "created_at": "2026-06-03T10:00:00Z"
  }
}
```

**检索文档**
```http
POST /v1/knowledge-bases/{kb_id}/retrieve
Authorization: Bearer sk-xxxxx
Content-Type: application/json

{
  "query": "How to reset password?",
  "top_k": 5,
  "min_score": 0.7
}

Response 200:
{
  "success": true,
  "data": {
    "results": [
      {
        "document_id": "uuid",
        "chunk_index": 5,
        "text": "To reset your password, go to Settings...",
        "score": 0.92,
        "metadata": {
          "source": "user_guide.pdf",
          "page": 12
        }
      }
    ]
  }
}
```

继续第三部分...

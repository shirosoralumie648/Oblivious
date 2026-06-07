# Oblivious 完整功能交付计划

**版本**: v1.0
**日期**: 2026-04-22
**基准状态**: 当前 main 分支 (7afaea9)
**预计总工期**: 18 周

---

## 一、当前状态快照

### 1.1 已完成模块

| 模块 | 完成度 | 文件位置 | 说明 |
|------|--------|----------|------|
| Auth/Session | 86% | `internal/auth`, `internal/http/auth_*` | 注册、登录、会话、中间件 |
| Preferences | 82% | `internal/userprefs` | 用户偏好设置 |
| Chat | 78% | `internal/chat` | 会话、消息、配置（走本地 ReplyGenerator） |
| Knowledge | 76% | `internal/knowledge` | 文本检索 Beta，非向量 RAG |
| SOLO Task | 68% | `internal/task` | 受限 runtime MVP |
| Console | 72% | `internal/console` | 使用量、模型、计费摘要 |
| Relay Core | 70% | `internal/relay` | Handler + Router + Billing + Metrics（独立模块） |
| 前端骨架 | 80% | `src/web/src` | 营销页、工作区、控制台页面 |

### 1.2 关键缺口

| 缺口 | 影响 | 优先级 |
|------|------|--------|
| **Relay 未集成到主应用** | Chat 无法通过 Relay 调用真实 LLM | P0 |
| **无 Agent Runtime** | 无法创建和管理 Agent | P0 |
| **无 MCP Client** | 无法调用外部工具 | P1 |
| **Knowledge 非 RAG** | 无向量检索能力 | P1 |
| **无 Admin API** | 无渠道/套餐/用户管理 | P1 |
| **无 Admin UI** | 无 B 端管理界面 | P2 |
| **无 Marketplace** | 无 Agent 市场 | P2 |

### 1.3 数据库现状

已有迁移：
- `0001_phase1_foundation` - 用户、会话、工作区
- `0002_user_preferences` - 用户偏好
- `0003-0004` - 会话配置
- `0005_usage_records` - 使用记录
- `0006-0007_knowledge` - 知识库
- `0008` - 会话-知识绑定
- `0009-0011` - 任务系统
- `0012_knowledge_document_chunks` - 文档分块

---

## 二、交付里程碑总览

```
Phase 1: Relay 集成与基础能力 (4周)
├── M1.1 Relay 挂载到主应用
├── M1.2 Chat 走 Relay 调用 LLM
├── M1.3 Agent Runtime 核心
└── M1.4 MCP Client 骨架

Phase 2: Agent 与 Memory 增强 (6周)
├── M2.1 Memory/RAG 系统 (pgvector)
├── M2.2 Agent 工具执行
├── M2.3 MCP 工具串联
└── M2.4 Agent-Relay-计费集成

Phase 3: Admin 与 Marketplace (5周)
├── M3.1 Admin API (渠道/套餐/用户)
├── M3.2 Admin UI
├── M3.3 Marketplace 核心
└── M3.4 Marketplace UI

Phase 4: 质量与发布 (3周)
├── M4.1 端到端测试
├── M4.2 文档完善
└── M4.3 部署配置
```

---

## 三、Phase 1: Relay 集成与基础能力 (4周)

### M1.1 Relay 挂载到主应用 (Week 1)

**目标**: 将 Relay 模块挂载到主 HTTP server，使 `/v1/*` 路由走 Relay

**任务清单**:

#### 1.1.1 数据库迁移 - 渠道和模型路由表
```sql
-- migrations/0013_channels.sql
CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- 'openai' | 'anthropic' | 'gemini'
    base_url VARCHAR(500) NOT NULL DEFAULT 'https://api.openai.com',
    api_key_encrypted TEXT NOT NULL,
    models TEXT[] NOT NULL,
    rpm_limit INT DEFAULT 1000,
    tpm_limit INT DEFAULT 100000,
    cb_threshold INT DEFAULT 5,
    cb_timeout INT DEFAULT 30,
    health_check_strategy VARCHAR(20) DEFAULT 'models_api',
    probe_model VARCHAR(100),
    probe_prompt VARCHAR(500),
    strategy VARCHAR(20) DEFAULT 'weighted',
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE model_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model VARCHAR(100) NOT NULL UNIQUE,
    strategy VARCHAR(20) DEFAULT 'weighted',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE model_channel_weights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID NOT NULL REFERENCES model_routes(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    weight INT DEFAULT 100,
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    UNIQUE(route_id, channel_id)
);

CREATE INDEX idx_channels_provider ON channels(provider);
CREATE INDEX idx_channels_enabled ON channels(enabled);
CREATE INDEX idx_model_routes_model ON model_routes(model);
```

#### 1.1.2 配置扩展
```go
// internal/config/config.go 新增字段
type Config struct {
    // ... 现有字段 ...
    
    // Relay 配置
    RelayEnabled      bool   `env:"RELAY_ENABLED,default=true"`
    RelayDefaultModel string `env:"RELAY_DEFAULT_MODEL,default=gpt-4o-mini"`
    
    // 默认渠道配置（用于开发环境）
    OpenAIAPIKey      string `env:"OPENAI_API_KEY"`
    OpenAIBaseURL     string `env:"OPENAI_BASE_URL,default=https://api.openai.com"`
}
```

#### 1.1.3 Relay Store (数据库持久化)
```go
// internal/relay/store.go
type RelayStore struct {
    db *sql.DB
}

func (s *RelayStore) ListChannels() ([]*types.Channel, error)
func (s *RelayStore) GetChannel(id string) (*types.Channel, error)
func (s *RelayStore) CreateChannel(ch *types.Channel) error
func (s *RelayStore) UpdateChannel(ch *types.Channel) error
func (s *RelayStore) DeleteChannel(id string) error

func (s *RelayStore) GetModelRoute(model string) (*types.ModelRoute, error)
func (s *RelayStore) SetModelRoute(route *types.ModelRoute) error
```

#### 1.1.4 主应用集成
```go
// internal/http/server.go 修改
func NewServer(cfg config.Config, database *sql.DB) *stdhttp.Server {
    mux := stdhttp.NewServeMux()
    
    // ... 现有路由 ...
    
    // Relay 集成
    if cfg.RelayEnabled {
        relayStore := relay.NewRelayStore(database)
        pool := relay.NewChannelPoolFromStore(relayStore)
        relayInstance, _ := relay.NewRelay(&relay.Config{Pool: pool})
        
        // 挂载 /v1/* 到 Relay
        mux.Handle("/v1/", relayInstance.Engine())
    }
    
    // ...
}
```

#### 1.1.5 开发环境默认渠道
```go
// 如果数据库无渠道，自动创建默认渠道（开发环境）
func ensureDefaultChannel(store *RelayStore, cfg config.Config) {
    channels, _ := store.ListChannels()
    if len(channels) == 0 && cfg.OpenAIAPIKey != "" {
        store.CreateChannel(&types.Channel{
            Name:     "Default OpenAI",
            Provider: "openai",
            BaseURL:  cfg.OpenAIBaseURL,
            APIKey:   cfg.OpenAIAPIKey,
            Models:   []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"},
            Enabled:  true,
        })
    }
}
```

**验收标准**:
- [ ] `GET /v1/models` 返回可用模型列表
- [ ] `POST /v1/chat/completions` 通过 Relay 调用 OpenAI
- [ ] 渠道配置可从数据库读取
- [ ] 无渠道时返回友好错误

---

### M1.2 Chat 走 Relay 调用 LLM (Week 1-2)

**目标**: 修改 Chat 模块，使其通过 Relay 调用 LLM，而非本地 ReplyGenerator

**任务清单**:

#### 1.2.1 Chat Gateway 重构
```go
// internal/chat/gateway.go 重构
type RelayGateway struct {
    httpClient *http.Client
    relayURL   string  // 默认 http://localhost:8080/v1
}

func (g *RelayGateway) Complete(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 构造 OpenAI 格式请求
    openaiReq := map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   req.Stream,
    }
    if req.MaxTokens > 0 {
        openaiReq["max_tokens"] = req.MaxTokens
    }
    
    // 调用本地 Relay
    resp, err := g.httpClient.Post(
        g.relayURL + "/chat/completions",
        "application/json",
        bytes.NewReader(jsonBytes),
    )
    // ...
}
```

#### 1.2.2 Chat Service 配置切换
```go
// internal/chat/service.go
type Service struct {
    store    *SQLStore
    gateway  ChatGateway  // 接口化
    // ...
}

// 工厂函数
func NewServiceWithRelay(store *SQLStore, relayURL string) *Service {
    return &Service{
        store:   store,
        gateway: NewRelayGateway(relayURL),
    }
}

func NewServiceWithLocal(store *SQLStore, generator *HTTPReplyGenerator) *Service {
    return &Service{
        store:   store,
        gateway: NewLocalGateway(generator),
    }
}
```

#### 1.2.3 流式响应支持
```go
// internal/chat/gateway.go
func (g *RelayGateway) CompleteStream(ctx context.Context, req *ChatRequest, stream chan<- ChatStreamChunk) error {
    // SSE 流式读取
    resp, _ := g.httpClient.Post(...)
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            // 解析 SSE chunk
            chunk := parseSSEChunk(line)
            stream <- chunk
        }
    }
    return nil
}
```

**验收标准**:
- [ ] Chat 消息通过 Relay 发送到 OpenAI
- [ ] 流式响应正常工作
- [ ] Token 使用量正确记录
- [ ] 原有测试不回归

---

### M1.3 Agent Runtime 核心 (Week 2-3)

**目标**: 建立 Agent 服务基础，支持创建、管理 Agent 并进行对话

**任务清单**:

#### 1.3.1 数据库迁移
```sql
-- migrations/0014_agents.sql
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    model VARCHAR(100) DEFAULT 'gpt-4o-mini',
    system_prompt TEXT,
    tools JSONB DEFAULT '[]',
    config JSONB DEFAULT '{}',
    is_public BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_agents_user_id ON agents(user_id);
CREATE INDEX idx_agents_is_public ON agents(is_public);
CREATE INDEX idx_agents_tools ON agents USING GIN(tools);

-- Agent 对话历史
CREATE TABLE agent_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE agent_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,  -- 'user' | 'assistant' | 'tool'
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_agent_messages_conversation ON agent_messages(conversation_id);
```

#### 1.3.2 Agent Service
```go
// internal/agent/service.go
package agent

type Service struct {
    store   *SQLStore
    gateway ChatGateway
}

func (s *Service) CreateAgent(ctx context.Context, userID string, req *CreateAgentRequest) (*Agent, error)
func (s *Service) GetAgent(ctx context.Context, id string) (*Agent, error)
func (s *Service) ListAgents(ctx context.Context, userID string) ([]*Agent, error)
func (s *Service) UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*Agent, error)
func (s *Service) DeleteAgent(ctx context.Context, id string) error

// 对话
func (s *Service) CreateConversation(ctx context.Context, agentID, userID string) (*Conversation, error)
func (s *Service) SendMessage(ctx context.Context, conversationID string, content string) (*Message, error)
func (s *Service) SendMessageStream(ctx context.Context, conversationID string, content string, stream chan<- MessageChunk) error
```

#### 1.3.3 Agent HTTP Handler
```go
// internal/http/agent_handler.go
type agentHandler struct {
    service *agent.Service
}

// 路由
// GET    /api/v1/app/agents           - 列出用户的 Agent
// POST   /api/v1/app/agents           - 创建 Agent
// GET    /api/v1/app/agents/:id       - 获取 Agent 详情
// PUT    /api/v1/app/agents/:id       - 更新 Agent
// DELETE /api/v1/app/agents/:id       - 删除 Agent
// GET    /api/v1/app/agents/:id/conversations  - 列出对话
// POST   /api/v1/app/agents/:id/conversations  - 创建对话
// POST   /api/v1/app/agents/conversations/:cid/messages - 发送消息
```

#### 1.3.4 前端 Agent 页面骨架
```tsx
// src/web/src/routes/workspace/AgentsPage.tsx
// src/web/src/routes/workspace/AgentDetailPage.tsx
// src/web/src/features/agents/api.ts
// src/web/src/features/agents/store.ts
```

**验收标准**:
- [ ] Agent CRUD API 正常工作
- [ ] Agent 对话通过 Relay 调用 LLM
- [ ] 前端可创建和管理 Agent
- [ ] 对话历史正确保存

---

### M1.4 MCP Client 骨架 (Week 3-4)

**目标**: 实现 MCP (Model Context Protocol) 客户端，支持工具发现和调用

**任务清单**:

#### 1.4.1 数据库迁移
```sql
-- migrations/0015_mcp_servers.sql
CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    url VARCHAR(500) NOT NULL,
    auth_token_encrypted TEXT,
    status VARCHAR(20) DEFAULT 'disconnected',  -- 'connected' | 'disconnected' | 'error'
    last_connected_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_mcp_servers_user_id ON mcp_servers(user_id);
```

#### 1.4.2 MCP 协议实现
```go
// internal/mcp/client.go
package mcp

type Client struct {
    servers map[string]*ServerConnection
}

type ServerConnection struct {
    id     string
    url    string
    conn   *sse.Connection  // SSE 连接
    tools  []ToolDefinition
}

func (c *Client) Connect(ctx context.Context, serverID, url, authToken string) error
func (c *Client) Disconnect(serverID string) error
func (c *Client) ListTools(serverID string) ([]ToolDefinition, error)
func (c *Client) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*ToolResult, error)
```

#### 1.4.3 MCP 协议消息
```go
// internal/mcp/protocol.go
type InitializeRequest struct {
    ProtocolVersion string `json:"protocolVersion"`
    ClientInfo      ClientInfo `json:"clientInfo"`
}

type ListToolsRequest struct {
    // MCP 协议字段
}

type CallToolRequest struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

type ToolDefinition struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    InputSchema any    `json:"inputSchema"`
}
```

#### 1.4.4 内置工具
```go
// internal/agent/tools/builtin.go
var BuiltinTools = map[string]Tool{
    "web_search":    &WebSearchTool{},
    "calculator":    &CalculatorTool{},
    "datetime":      &DatetimeTool{},
    "http_request":  &HTTPRequestTool{},
}
```

**验收标准**:
- [ ] 可连接外部 MCP Server
- [ ] 可发现 MCP Server 提供的工具
- [ ] 可调用 MCP 工具
- [ ] 内置工具正常工作

---

## 四、Phase 2: Agent 与 Memory 增强 (6周)

### M2.1 Memory/RAG 系统 (Week 5-6)

**目标**: 升级 Knowledge 到真正的向量检索系统

**任务清单**:

#### 2.1.1 pgvector 扩展
```sql
-- migrations/0016_pgvector.sql
CREATE EXTENSION IF NOT EXISTS vector;

-- 重构 memory_documents 和 memory_chunks
CREATE TABLE memory_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255),
    content TEXT NOT NULL,
    source_type VARCHAR(20) DEFAULT 'manual',  -- 'manual' | 'upload' | 'url'
    source_url VARCHAR(500),
    metadata JSONB DEFAULT '{}',
    total_chunks INT DEFAULT 0,
    embedding_model VARCHAR(100) DEFAULT 'text-embedding-3-small',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE memory_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES memory_documents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    chunk_index INT NOT NULL,
    embedding vector(1536),  -- OpenAI text-embedding-3-small 维度
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- 向量索引
CREATE INDEX idx_memory_chunks_embedding ON memory_chunks 
    USING ivfflat(embedding cosine_ops) WITH (lists = 100);
CREATE INDEX idx_memory_chunks_user_id ON memory_chunks(user_id);
CREATE INDEX idx_memory_chunks_document_id ON memory_chunks(document_id);
```

#### 2.1.2 Memory Service
```go
// internal/memory/service.go
package memory

type Service struct {
    store     *SQLStore
    embedder  Embedder  // 调用 Relay Embeddings
}

func (s *Service) AddDocument(ctx context.Context, userID string, req *AddDocumentRequest) (*Document, error) {
    // 1. 创建文档记录
    // 2. 分块
    chunks := s.chunkText(req.Content, ChunkingConfig{
        ChunkSize:    512,
        ChunkOverlap: 64,
    })
    // 3. 批量嵌入
    embeddings := s.embedder.EmbedBatch(ctx, chunks)
    // 4. 存储 chunks + embeddings
}

func (s *Service) Search(ctx context.Context, userID string, query string, topK int) ([]*SearchResult, error) {
    // 1. 嵌入查询
    queryEmbedding := s.embedder.Embed(ctx, query)
    // 2. 向量相似度搜索
    // SELECT ... ORDER BY embedding <=> queryEmbedding LIMIT topK
}

func (s *Service) DeleteDocument(ctx context.Context, id string) error {
    // 级联删除 chunks
}
```

#### 2.1.3 Embedder (调用 Relay)
```go
// internal/memory/embedder.go
type RelayEmbedder struct {
    relayURL string
    model    string  // text-embedding-3-small
}

func (e *RelayEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, _ := http.Post(e.relayURL + "/embeddings", ...)
    // 解析 embedding 向量
}

func (e *RelayEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    // 批量嵌入
}
```

#### 2.1.4 分块策略
```go
// internal/memory/chunker.go
type Chunker struct {
    chunkSize    int  // 512 tokens
    chunkOverlap int  // 64 tokens
}

func (c *Chunker) Chunk(text string) []string {
    // 1. 按段落分割
    // 2. 滑动窗口
    // 3. 返回 chunks
}
```

**验收标准**:
- [ ] 文档上传后自动分块和嵌入
- [ ] 向量相似度搜索正常工作
- [ ] 按用户隔离数据
- [ ] 删除文档级联删除 chunks

---

### M2.2 Agent 工具执行 (Week 6-7)

**目标**: Agent 可调用工具并处理结果

**任务清单**:

#### 2.2.1 工具执行器
```go
// internal/agent/executor.go
type ToolExecutor struct {
    mcpClient   *mcp.Client
    builtinTools map[string]Tool
}

func (e *ToolExecutor) Execute(ctx context.Context, toolCall *ToolCall) (*ToolResult, error) {
    // 判断是内置工具还是 MCP 工具
    if tool, ok := e.builtinTools[toolCall.Name]; ok {
        return tool.Execute(ctx, toolCall.Arguments)
    }
    // MCP 工具
    return e.mcpClient.CallTool(ctx, toolCall.ServerID, toolCall.Name, toolCall.Arguments)
}
```

#### 2.2.2 Agent 执行循环
```go
// internal/agent/runner.go
type AgentRunner struct {
    service    *Service
    executor   *ToolExecutor
    memory     *memory.Service
}

func (r *AgentRunner) Run(ctx context.Context, agent *Agent, conversationID string, userMessage string) error {
    messages := []Message{{Role: "user", Content: userMessage}}
    
    for {
        // 1. 构建上下文（系统提示 + 历史 + 记忆）
        context := r.buildContext(ctx, agent, messages)
        
        // 2. 调用 LLM
        response := r.gateway.Complete(ctx, context)
        
        // 3. 如果有工具调用
        if response.HasToolCalls() {
            for _, tc := range response.ToolCalls {
                result := r.executor.Execute(ctx, tc)
                messages = append(messages, Message{
                    Role:         "tool",
                    Content:      result.Content,
                    ToolCallID:   tc.ID,
                })
            }
            continue  // 继续循环
        }
        
        // 4. 无工具调用，返回最终响应
        return nil
    }
}
```

#### 2.2.3 记忆注入
```go
// internal/agent/runner.go
func (r *AgentRunner) buildContext(ctx context.Context, agent *Agent, messages []Message) []Message {
    // 1. 系统提示
    context := []Message{{Role: "system", Content: agent.SystemPrompt}}
    
    // 2. RAG 检索相关记忆
    if agent.Config.EnableMemory {
        query := messages[len(messages)-1].Content
        memories := r.memory.Search(ctx, agent.UserID, query, 5)
        context = append(context, Message{
            Role:    "system",
            Content: formatMemories(memories),
        })
    }
    
    // 3. 对话历史
    context = append(context, messages...)
    
    return context
}
```

**验收标准**:
- [ ] Agent 可调用内置工具
- [ ] Agent 可调用 MCP 工具
- [ ] 工具结果正确注入对话
- [ ] 多轮工具调用正常

---

### M2.3 MCP 工具串联 (Week 7-8)

**目标**: 完整的 MCP 工具发现、配置、调用流程

**任务清单**:

#### 2.3.1 MCP Server 管理 API
```go
// internal/http/mcp_handler.go
// GET    /api/v1/app/mcp-servers       - 列出用户的 MCP Server
// POST   /api/v1/app/mcp-servers       - 添加 MCP Server
// GET    /api/v1/app/mcp-servers/:id   - 获取详情
// DELETE /api/v1/app/mcp-servers/:id   - 删除
// POST   /api/v1/app/mcp-servers/:id/connect    - 连接
// GET    /api/v1/app/mcp-servers/:id/tools      - 列出工具
```

#### 2.3.2 Agent 工具绑定
```go
// Agent 可绑定 MCP Server 的工具
type AgentToolBinding struct {
    AgentID      string
    MCPServerID  string
    ToolName     string
    Enabled      bool
}
```

#### 2.3.3 前端 MCP 配置页面
```tsx
// src/web/src/routes/workspace/MCPServersPage.tsx
// - 添加 MCP Server (URL + Auth Token)
// - 查看可用工具
// - 绑定工具到 Agent
```

**验收标准**:
- [ ] 用户可添加 MCP Server
- [ ] 自动发现 MCP Server 的工具
- [ ] Agent 可绑定 MCP 工具
- [ ] 工具调用正常

---

### M2.4 Agent-Relay-计费集成 (Week 8-9)

**目标**: Agent 调用走 Relay，Token 消耗计入用户配额

**任务清单**:

#### 2.4.1 配额系统
```sql
-- migrations/0017_quotas.sql
CREATE TABLE quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance DECIMAL(15,6) DEFAULT 0,  -- 余额 (USD)
    used DECIMAL(15,6) DEFAULT 0,     -- 已使用
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE TABLE billing_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    channel_id VARCHAR(100),
    model VARCHAR(100),
    api_type VARCHAR(50),
    idempotency_key VARCHAR(200),
    pre_authorized_amt DECIMAL(15,6),
    settled_amt DECIMAL(15,6),
    status VARCHAR(20) DEFAULT 'preauthorized',  -- 'preauthorized' | 'settled' | 'refunded'
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_billing_sessions_idempotency ON billing_sessions(idempotency_key);
```

#### 2.4.2 配额服务
```go
// internal/quota/service.go
func (s *Service) PreConsume(ctx context.Context, userID string, amount float64) error {
    // 检查余额
    // 预扣
}

func (s *Service) Settle(ctx context.Context, userID string, actualAmount float64) error {
    // 结算差额
}

func (s *Service) Refund(ctx context.Context, userID string, amount float64) error {
    // 退款
}
```

#### 2.4.3 Agent 调用计费
```go
// Agent 调用 Relay 时自动计费
// Relay 的 BillingHook 已实现，Agent 只需调用 Relay
```

**验收标准**:
- [ ] Agent 调用消耗用户配额
- [ ] 配额不足时拒绝调用
- [ ] 计费记录可查询
- [ ] 预扣/结算/退款正常

---

## 五、Phase 3: Admin 与 Marketplace (5周)

### M3.1 Admin API (Week 10-11)

**目标**: 完整的渠道、套餐、用户管理 API

**任务清单**:

#### 3.1.1 数据库迁移
```sql
-- migrations/0018_admin.sql
CREATE TABLE packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    quota_amount DECIMAL(15,6) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    duration_days INT,  -- NULL 表示永久
    is_active BOOLEAN DEFAULT true,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    package_id UUID NOT NULL REFERENCES packages(id),
    status VARCHAR(20) DEFAULT 'active',
    started_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE topup_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(15,6) NOT NULL,
    money DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    trade_no VARCHAR(100),
    paid_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 3.1.2 Admin Handler
```go
// internal/http/admin_handler.go

// 渠道管理
// GET    /api/v1/admin/channels
// POST   /api/v1/admin/channels
// GET    /api/v1/admin/channels/:id
// PUT    /api/v1/admin/channels/:id
// DELETE /api/v1/admin/channels/:id

// 套餐管理
// GET    /api/v1/admin/packages
// POST   /api/v1/admin/packages
// ...

// 用户管理
// GET    /api/v1/admin/users
// PUT    /api/v1/admin/users/:id/quota

// 用量统计
// GET    /api/v1/admin/usage
// GET    /api/v1/admin/orders
```

#### 3.1.3 Admin 中间件
```go
// internal/http/admin_middleware.go
func requireAdmin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := getUserFromContext(r)
        if user.Role != "admin" {
            writeError(w, 403, "forbidden", "admin required")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

**验收标准**:
- [ ] 管理员可管理渠道
- [ ] 管理员可管理套餐
- [ ] 管理员可查看用户和用量
- [ ] 非管理员无法访问

---

### M3.2 Admin UI (Week 11-12)

**目标**: B 端管理界面

**任务清单**:

#### 3.2.1 Admin 路由
```tsx
// src/web/src/app/router.tsx
{
  path: '/admin',
  element: <AdminLayout />,
  children: [
    { index: true, element: <AdminDashboard /> },
    { path: 'channels', element: <ChannelsPage /> },
    { path: 'channels/:id', element: <ChannelDetailPage /> },
    { path: 'packages', element: <PackagesPage /> },
    { path: 'users', element: <UsersPage /> },
    { path: 'usage', element: <AdminUsagePage /> },
    { path: 'orders', element: <OrdersPage /> },
  ]
}
```

#### 3.2.2 渠道管理页面
```tsx
// src/web/src/routes/admin/ChannelsPage.tsx
// - 渠道列表
// - 添加/编辑渠道
// - 测试渠道连接
// - 查看渠道状态（熔断、限流）
```

#### 3.2.3 套餐管理页面
```tsx
// src/web/src/routes/admin/PackagesPage.tsx
// - 套餐列表
// - 添加/编辑套餐
// - 查看订阅情况
```

**验收标准**:
- [ ] 渠道管理 UI 正常
- [ ] 套餐管理 UI 正常
- [ ] 用户管理 UI 正常
- [ ] 用量统计 UI 正常

---

### M3.3 Marketplace 核心 (Week 12-13)

**目标**: Agent 发布、发现、安装

**任务清单**:

#### 3.3.1 数据库迁移
```sql
-- migrations/0019_marketplace.sql
CREATE TABLE market_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,  -- 'agent' | 'skill'
    name VARCHAR(255) NOT NULL,
    description TEXT,
    author_id UUID NOT NULL REFERENCES users(id),
    manifest JSONB NOT NULL,  -- Agent 配置快照
    thumbnail_url VARCHAR(500),
    install_count INT DEFAULT 0,
    rating FLOAT DEFAULT 0,
    tags TEXT[],
    status VARCHAR(20) DEFAULT 'draft',  -- 'draft' | 'published' | 'archived'
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE user_installs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    item_id UUID NOT NULL REFERENCES market_items(id),
    installed_agent_id UUID REFERENCES agents(id),
    installed_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, item_id)
);

CREATE INDEX idx_market_items_status ON market_items(status);
CREATE INDEX idx_market_items_tags ON market_items USING GIN(tags);
```

#### 3.3.2 Marketplace Service
```go
// internal/marketplace/service.go
func (s *Service) PublishAgent(ctx context.Context, agentID string) error
func (s *Service) UnpublishAgent(ctx context.Context, agentID string) error
func (s *Service) ListPublished(ctx context.Context, filter *Filter) ([]*MarketItem, error)
func (s *Service) InstallAgent(ctx context.Context, userID, itemID string) (*Agent, error)
func (s *Service) RateItem(ctx context.Context, userID, itemID string, rating float64) error
```

#### 3.3.3 Marketplace Handler
```go
// internal/http/marketplace_handler.go
// GET    /api/v1/marketplace              - 列出发布的 Agent
// POST   /api/v1/marketplace              - 发布 Agent
// GET    /api/v1/marketplace/:id          - 详情
// POST   /api/v1/marketplace/:id/install  - 安装
// POST   /api/v1/marketplace/:id/rate     - 评分
```

**验收标准**:
- [ ] Agent 可发布到市场
- [ ] 用户可浏览市场
- [ ] 用户可安装 Agent
- [ ] 安装计数正常

---

### M3.4 Marketplace UI (Week 13-14)

**目标**: 市场前端界面

**任务清单**:

#### 3.4.1 Marketplace 页面
```tsx
// src/web/src/routes/marketplace/MarketplacePage.tsx
// - 搜索 Agent
// - 分类浏览
// - Agent 详情
// - 安装按钮
```

#### 3.4.2 Agent 发布流程
```tsx
// src/web/src/routes/workspace/AgentPublishPage.tsx
// - 选择要发布的 Agent
// - 填写描述、标签
// - 上传缩略图
// - 发布确认
```

**验收标准**:
- [ ] 市场浏览 UI 正常
- [ ] 发布流程正常
- [ ] 安装流程正常
- [ ] 评分功能正常

---

## 六、Phase 4: 质量与发布 (3周)

### M4.1 端到端测试 (Week 15-16)

**任务清单**:

#### 4.1.1 集成测试
```go
// tests/integration/agent_test.go
func TestAgentEndToEnd(t *testing.T) {
    // 1. 创建用户
    // 2. 创建 Agent
    // 3. 发送消息
    // 4. 验证响应
    // 5. 验证计费
}

// tests/integration/relay_test.go
func TestRelayWithBilling(t *testing.T) {
    // 1. 配置渠道
    // 2. 发送请求
    // 3. 验证计费
    // 4. 验证熔断
}
```

#### 4.1.2 E2E 测试
```typescript
// tests/e2e/chat.spec.ts
test('chat through relay', async ({ page }) => {
  await page.goto('/chat')
  await page.fill('[data-testid="message-input"]', 'Hello')
  await page.click('[data-testid="send-button"]')
  await expect(page.locator('[data-testid="assistant-message"]')).toBeVisible()
})
```

#### 4.1.3 性能测试
```go
// tests/benchmark/relay_bench_test.go
func BenchmarkRelayChat(b *testing.B) {
    // 并发请求测试
}
```

**验收标准**:
- [ ] 所有集成测试通过
- [ ] E2E 测试通过
- [ ] 性能达标

---

### M4.2 文档完善 (Week 16-17)

**任务清单**:

#### 4.2.1 API 文档
- OpenAPI/Swagger 规范
- 每个 API 的请求/响应示例

#### 4.2.2 用户文档
- 快速开始指南
- Agent 创建教程
- MCP 工具配置指南

#### 4.2.3 运维文档
- 部署指南
- 配置说明
- 监控指标说明

---

### M4.3 部署配置 (Week 17-18)

**任务清单**:

#### 4.3.1 Docker 配置
```dockerfile
# Dockerfile
FROM golang:1.22 AS builder
# ...

FROM node:20 AS frontend-builder
# ...

FROM debian:bookworm-slim
# ...
```

#### 4.3.2 Kubernetes 配置
```yaml
# k8s/deployment.yaml
# k8s/service.yaml
# k8s/configmap.yaml
```

#### 4.3.3 CI/CD 优化
```yaml
# .github/workflows/release.yml
# - 构建
# - 测试
# - 部署
```

---

## 七、风险与缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| Relay 集成复杂度超预期 | 中 | 高 | 预留 buffer，优先完成核心路径 |
| pgvector 性能问题 | 低 | 中 | 预先测试索引配置，准备优化方案 |
| MCP 协议兼容性 | 中 | 中 | 先支持标准 MCP，再扩展 |
| 前端工作量超预期 | 中 | 中 | 优先 API，UI 可迭代 |
| 计费精度问题 | 低 | 高 | 使用精确 decimal 库，充分测试 |

---

## 八、关键依赖

```
M1.1 (Relay 集成)
    ↓
M1.2 (Chat 走 Relay) ──→ M2.4 (计费集成)
    ↓
M1.3 (Agent 核心) ──→ M2.2 (工具执行) ──→ M2.3 (MCP 串联)
    ↓
M2.1 (Memory/RAG)
    ↓
M3.3 (Marketplace) ──→ M3.4 (Marketplace UI)
    ↓
M3.1 (Admin API) ──→ M3.2 (Admin UI)
    ↓
M4.* (质量与发布)
```

---

## 九、里程碑验收命令

每个里程碑完成后执行：

```bash
# 后端测试
cd src/server && go test ./... -count=1

# 前端测试
cd src/web && pnpm test

# 集成测试
bash scripts/test.sh all

# 构建检查
bash scripts/check.sh all
```

---

## 十、总结

**总工期**: 18 周
**关键路径**: Relay 集成 → Agent Runtime → Memory/RAG → Admin → Marketplace
**最高风险**: Relay 集成复杂度、计费精度

**建议执行策略**:
1. **Week 1-4**: 专注 Phase 1，确保 Relay 可用
2. **Week 5-9**: 并行开发 Agent 和 Memory
3. **Week 10-14**: Admin 和 Marketplace 可并行
4. **Week 15-18**: 质量保障，预留 buffer

**下一步行动**: 开始 M1.1 - Relay 挂载到主应用

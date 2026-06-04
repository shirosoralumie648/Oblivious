# Oblivious 完整融合设计文档 - 第二部分

**接续**: [第一部分](./2026-06-04-complete-fusion-design.md)

---

## 3.4 Agent系统层

### 3.4.1 功能融合清单

| 功能 | 实现来源 | 优先级 |
|------|---------|--------|
| **Agent运行时** | | |
| ReAct Agent模式 | dify | P1 |
| 规划Agent模式 | FastGPT | P1 |
| Agent状态持久化 | 当前v08 | P1 |
| 多步推理 | dify, FastGPT | P1 |
| **工具系统** | | |
| 内置工具（150+） | dify (50+) + Coze (100+) | P1 |
| Function Calling | 所有Agent系统 | P1 |
| 自定义工具（API/Python） | dify, open-webui | P1 |
| MCP集成（双向） | FastGPT, bifrost | P2 |
| **高级能力** | | |
| 代码解释器（8语言沙箱） | open-webui, LibreChat | P2 |
| Web搜索（15+提供商） | open-webui, LibreChat | P2 |
| 子代理调用 | open-webui, LibreChat | P2 |
| 记忆系统 | anything-llm | P2 |
| 动态模型路由 | anything-llm | P2 |
| 智能技能选择 | anything-llm | P2 |

### 3.4.2 Agent架构设计

**Agent Service 架构**:

```
┌─────────────────────────────────────────────┐
│         Agent Service                       │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Agent Runtime Manager               │ │
│  │   - ReAct引擎（dify）                 │ │
│  │   - 规划引擎（FastGPT）               │ │
│  │   - 状态持久化（v08）                 │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Tool Registry (150+ tools)          │ │
│  │   - dify 50+ tools                    │ │
│  │   - Coze 100+ plugins                 │ │
│  │   - 自定义工具注册                    │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Tool Executor                       │ │
│  │   - Function Calling                  │ │
│  │   - API调用                           │ │
│  │   - Python沙箱                        │ │
│  │   - MCP Client                        │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Memory System (anything-llm)        │ │
│  │   - 短期记忆（对话内）                │ │
│  │   - 长期记忆（跨对话）                │ │
│  │   - 用户管理记忆                      │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

### 3.4.3 核心接口

```go
// agent-service/internal/agent/service.go
type AgentService interface {
    // 创建Agent运行
    CreateRun(ctx context.Context, req *CreateRunRequest) (*AgentRun, error)
    
    // 执行Agent（ReAct模式）
    ExecuteReAct(ctx context.Context, runID string) (*RunResult, error)
    
    // 执行Agent（规划模式）
    ExecutePlanning(ctx context.Context, runID string) (*RunResult, error)
    
    // 审批工具调用
    ApproveToolCall(ctx context.Context, toolRunID string) error
    
    // 重试失败的工具调用
    RetryToolCall(ctx context.Context, toolRunID string) error
}

type ToolRegistry interface {
    // 注册工具
    Register(tool Tool) error
    
    // 获取可用工具
    GetAvailableTools(ctx context.Context, agentID string) ([]Tool, error)
    
    // 执行工具
    Execute(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error)
}

type MemorySystem interface {
    // 存储记忆
    Store(ctx context.Context, memory *Memory) error
    
    // 检索记忆
    Retrieve(ctx context.Context, query string, limit int) ([]*Memory, error)
    
    // 管理记忆
    Manage(ctx context.Context, memoryID string, action string) error
}
```

### 3.4.4 数据模型

```sql
-- agent_runs（Agent运行记录）
CREATE TABLE agent_runs (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    conversation_id UUID,
    agent_id UUID,
    user_id UUID NOT NULL,
    mode VARCHAR(20) NOT NULL,  -- react, planning
    status VARCHAR(20) NOT NULL,  -- running, paused, completed, failed
    request_text TEXT NOT NULL,
    final_response TEXT,
    memory_used BOOLEAN DEFAULT FALSE,
    memory_results_count INT DEFAULT 0,
    iteration_count INT DEFAULT 0,
    budget_tokens INT,
    used_tokens INT DEFAULT 0,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- agent_tool_runs（工具调用记录）
CREATE TABLE agent_tool_runs (
    id UUID PRIMARY KEY,
    agent_run_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    tool_type VARCHAR(50) NOT NULL,  -- builtin, custom, mcp
    call_id VARCHAR(100),
    arguments JSONB NOT NULL,
    status VARCHAR(20) NOT NULL,  -- pending, approved, executing, completed, failed
    approval_required BOOLEAN DEFAULT FALSE,
    approved_by UUID,
    attempt_count INT DEFAULT 0,
    result JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- agent_memories（Agent记忆）
CREATE TABLE agent_memories (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,
    agent_id UUID,
    type VARCHAR(20) NOT NULL,  -- short_term, long_term, user_managed
    content TEXT NOT NULL,
    embedding VECTOR(1536),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_agent_memories_embedding ON agent_memories 
USING hnsw (embedding vector_cosine_ops);
```

---

## 3.5 计费与商业化层

### 3.5.1 功能融合清单

| 功能 | 实现来源 | 优先级 | 决策 |
|------|---------|--------|------|
| Token级计费 | 当前v08, new-api, sub2api | P0 | 保留v08 |
| 模型定价配置 | new-api, CPA-Manager | P0 | 增强v08 |
| 配额管理 | 当前v08, sub2api | P0 | 保留v08 |
| 并发控制 | sub2api | P1 | 新增 |
| 双重速率限制 | sub2api | P1 | 新增 |
| Stripe集成 | 当前v08 | P0 | 保留v08 |
| 国内支付 | sub2api | P1 | 新增 |
| 订阅系统 | 当前v08 | P0 | 保留v08 |
| 发票系统 | 当前v08 | P0 | 保留v08 |
| 退款处理 | 当前v08 | P0 | 保留v08 |

### 3.5.2 计费架构（保留v08 + 增强）

**决策**: 当前v08的计费系统已完整且经过验证，**全部保留**，仅做增强。

**增强点**:
1. **动态定价配置**（from new-api）
2. **价格同步**（from CPA-Manager的LiteLLM同步）
3. **并发控制**（from sub2api）
4. **双重速率限制**（from sub2api：请求速率 + Token速率）
5. **国内支付集成**（from sub2api：支付宝/微信）

**Billing Service架构**（增强版）:

```
┌─────────────────────────────────────────────┐
│    Billing Service (基于v08增强)            │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Pricing Engine (增强)               │ │
│  │   - v08基础定价                       │ │
│  │   - 动态价格配置（new-api）           │ │
│  │   - LiteLLM价格同步（CPA-Manager）    │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Quota Manager (v08保留)             │ │
│  │   - 配额预授权                        │ │
│  │   - 精确结算                          │ │
│  │   - 退款处理                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Concurrency Control (新增)          │ │
│  │   - 用户级并发限制                    │ │
│  │   - 账号级并发限制                    │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Rate Limiter (增强)                 │ │
│  │   - 请求速率限制                      │ │
│  │   - Token速率限制（新增）             │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Payment Gateway (增强)              │ │
│  │   - Stripe（v08保留）                 │ │
│  │   - 支付宝/微信（新增）               │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

---

## 3.6 Marketplace层

### 3.6.1 功能融合清单

| 功能 | 实现来源 | 优先级 | 决策 |
|------|---------|--------|------|
| Agent发布/审核 | 当前v08, Coze | P1 | 保留v08 + Coze UX |
| 付费机制 | 当前v08 | P1 | 保留v08 |
| 结算系统 | 当前v08 | P1 | 保留v08 |
| 评分评论 | Coze, lobe-chat | P2 | 新增 |
| 版本管理 | 当前v08 | P2 | 保留v08 |
| 举报审核 | 当前v08 | P1 | 保留v08 |
| 模板市场 | Coze, dify | P2 | 新增 |

**决策**: 当前v08的Marketplace核心功能完整，**保留**，增加Coze的UX优化和模板市场。

---

## 3.7 多渠道发布层（新增）

### 3.7.1 功能清单（Coze独有）

| 渠道类型 | 支持平台 | 优先级 |
|---------|---------|--------|
| **即时通讯** | 飞书、微信公众号、Slack、Discord、Telegram | P2 |
| **Web嵌入** | Web SDK、iframe嵌入 | P1 |
| **API** | REST API、Webhook | P0 |

### 3.7.2 Channel Service架构

```
┌─────────────────────────────────────────────┐
│       Channel Service (新增)                │
│                                             │
│  ┌───────────────────────────────────────┐ │
│  │   Channel Manager                     │ │
│  │   - 渠道配置                          │ │
│  │   - 渠道激活/停用                     │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Message Adapter                     │ │
│  │   - 统一消息格式                      │ │
│  │   - 格式转换                          │ │
│  └───────────────────────────────────────┘ │
│                  ▼                          │
│  ┌───────────────────────────────────────┐ │
│  │   Channel Adapters                    │ │
│  │   - 飞书 Adapter                      │ │
│  │   - 微信 Adapter                      │ │
│  │   - Discord Adapter                   │ │
│  │   - Slack Adapter                     │ │
│  │   - Web SDK Adapter                   │ │
│  └───────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

### 3.7.3 核心接口

```go
// channel-service/internal/channel/service.go
type ChannelService interface {
    // 发送消息到渠道
    Send(ctx context.Context, channelID string, message *Message) error
    
    // 接收来自渠道的消息
    Receive(ctx context.Context, channelID string, rawMessage []byte) (*Message, error)
    
    // 配置渠道
    Configure(ctx context.Context, req *ChannelConfig) error
}

type ChannelAdapter interface {
    // 渠道类型
    Type() ChannelType
    
    // 发送消息
    Send(ctx context.Context, message *Message) error
    
    // 消息格式转换
    Transform(ctx context.Context, rawMessage []byte) (*Message, error)
}
```

---

## 4. 前端设计

### 4.1 技术栈（基于lobe-chat）

| 技术 | 选择 | 理由 |
|------|------|------|
| **框架** | Next.js 14 (App Router) | lobe-chat标准，最现代 |
| **UI组件** | Shadcn/ui + Tailwind CSS | lobe-chat标准 |
| **状态管理** | Zustand | 轻量高效 |
| **工作流编辑器** | React Flow | dify/FastGPT标准 |
| **图表** | Recharts | 常用选择 |
| **表单** | React Hook Form + Zod | 类型安全 |
| **请求** | SWR | Next.js官方推荐 |

### 4.2 页面结构

```
src/web/
├── app/                    # Next.js App Router
│   ├── (auth)/            # 认证路由组
│   │   ├── login/
│   │   └── register/
│   ├── (workspace)/       # 工作区路由组
│   │   ├── chat/          # 对话页面（lobe-chat风格）
│   │   ├── workflow/      # 工作流编辑器（dify风格）
│   │   ├── knowledge/     # 知识库管理（ragflow风格）
│   │   ├── agent/         # Agent管理（Coze风格）
│   │   └── marketplace/   # Marketplace浏览
│   ├── (admin)/           # 管理后台路由组
│   │   ├── dashboard/
│   │   ├── channels/      # 渠道管理
│   │   ├── billing/       # 计费统计
│   │   └── review/        # 审核管理
│   └── layout.tsx
├── components/            # 共享组件
│   ├── chat/              # 聊天组件（lobe-chat）
│   │   ├── MessageList.tsx
│   │   ├── ChatInput.tsx
│   │   └── MessageBubble.tsx
│   ├── workflow/          # 工作流组件（React Flow）
│   │   ├── FlowEditor.tsx
│   │   ├── NodePalette.tsx
│   │   └── nodes/         # 20+节点组件
│   ├── knowledge/         # 知识库组件
│   │   ├── DocumentUploader.tsx
│   │   └── RetrievalTest.tsx
│   └── ui/                # Shadcn/ui组件
├── features/              # 功能模块
│   ├── chat/
│   ├── workflow/
│   ├── knowledge/
│   └── agent/
├── hooks/                 # 自定义Hooks
├── store/                 # Zustand状态管理
│   ├── chat.ts
│   ├── workflow.ts
│   └── knowledge.ts
└── lib/                   # 工具库
    ├── api.ts
    └── utils.ts
```

### 4.3 核心页面设计

#### 4.3.1 Chat页面（lobe-chat风格）

**布局**:
```
┌─────────────────────────────────────────┐
│  侧边栏                  │  主对话区    │
│  ┌───────────────┐      │             │
│  │ 对话列表      │      │  消息列表   │
│  │              │      │             │
│  │ + 新对话     │      │  [Message]  │
│  │              │      │  [Message]  │
│  │ 历史对话1    │      │  [Message]  │
│  │ 历史对话2    │      │             │
│  │ ...          │      │             │
│  └───────────────┘      │  输入框     │
│                         │  [________] │
└─────────────────────────────────────────┘
```

**关键功能**:
- 打字机效果
- Markdown渲染 + 代码高亮
- 对话分叉（LibreChat）
- 实时协作（LibreChat）
- 多模态输入（图片/文件）
- 人格选择（Coze）

#### 4.3.2 Workflow页面（dify + FastGPT + Coze UX）

**布局**:
```
┌─────────────────────────────────────────┐
│  工具栏                                 │
│  [保存] [测试] [版本] [发布]           │
├─────────────────────────────────────────┤
│  节点面板  │  画布区域  │  调试面板    │
│  ┌──────┐ │           │              │
│  │LLM节点│ │  ┌────┐  │  变量检查    │
│  │知识库│ │  │节点│  │              │
│  │条件  │ │  └────┘  │  调用链路    │
│  │循环  │ │   ┌────┐ │              │
│  │代码  │ │   │节点│ │  输出结果    │
│  │HTTP  │ │   └────┘ │              │
│  │...   │ │          │              │
│  └──────┘ │          │              │
└─────────────────────────────────────────┘
```

**关键功能**:
- React Flow可视化编排
- 20+节点类型拖拽
- 单点测试（FastGPT）
- 完整调用链（FastGPT）
- 变量检查（Coze）
- 版本控制（dify）

#### 4.3.3 Knowledge页面（ragflow风格）

**布局**:
```
┌─────────────────────────────────────────┐
│  知识库列表                             │
│  [+ 新建] [导入] [同步]                │
├─────────────────────────────────────────┤
│  知识库1  │  文档列表  │  文档预览    │
│  知识库2  │  ┌──────┐ │              │
│  知识库3  │  │文档1 │ │  文档内容    │
│           │  │文档2 │ │              │
│           │  │文档3 │ │  分块可视化  │
│           │  └──────┘ │              │
│           │           │  检索测试    │
│           │           │  [________]  │
└─────────────────────────────────────────┘
```

**关键功能**:
- 文档上传（拖拽）
- 在线抓取（ragflow）
- 分块可视化（ragflow）
- 检索测试（FastGPT + Coze）
- 引用溯源（ragflow）

---

## 5. 实施路线图

### 5.1 阶段划分（4个季度）

#### Q1: 核心基础设施（12周）

**目标**: API网关、Relay、基础前端

| 周次 | 任务 | 交付物 |
|------|------|--------|
| Week 1-2 | 项目初始化、微服务脚手架 | 12个服务骨架 |
| Week 3-4 | Gateway Service | 路由/认证/限流 |
| Week 4-6 | Relay Service | 多模型适配/负载均衡 |
| Week 7-8 | 语义缓存（bifrost） | 缓存功能 |
| Week 9-10 | Chat Service | 对话管理 |
| Week 11-12 | 前端Chat页面（lobe-chat） | 可用对话界面 |

**参考代码提取**:
- `reference/new-api/relay/` → Gateway/Relay
- `reference/bifrost/core/cache/` → 语义缓存
- `reference/lobe-chat/src/` → 前端Chat

#### Q2: 工作流与知识库（12周）

**目标**: 工作流引擎、RAG系统

| 周次 | 任务 | 交付物 |
|------|------|--------|
| Week 13-16 | Workflow Service | 工作流引擎+10个基础节点 |
| Week 17-18 | 工作流前端（React Flow） | 可视化编辑器 |
| Week 19-20 | RAG Service核心 | 文档解析+向量化 |
| Week 21-22 | 混合检索 | 检索引擎 |
| Week 23-24 | 知识库前端 | 知识库管理界面 |

**参考代码提取**:
- `reference/dify/api/core/workflow/` → 工作流引擎（需Go重写）
- `reference/FastGPT/packages/service/` → 节点调试
- `reference/ragflow/rag/` → RAG引擎（需Go重写）
- `reference/MaxKB/apps/` → 检索系统

#### Q3: Agent与高级功能（12周）

**目标**: Agent系统、工具生态

| 周次 | 任务 | 交付物 |
|------|------|--------|
| Week 25-28 | Agent Service | ReAct+规划双引擎 |
| Week 29-30 | 工具注册系统 | 50+内置工具 |
| Week 31-32 | MCP集成 | 双向MCP |
| Week 33-34 | 代码解释器 | 8语言沙箱 |
| Week 35-36 | Agent前端 | Agent管理界面 |

**参考代码提取**:
- `reference/dify/api/core/agent/` → ReAct Agent
- `reference/FastGPT/packages/service/core/ai/agent/` → 规划Agent
- `reference/open-webui/backend/apps/webui/routers/tools.py` → 代码沙箱

#### Q4: 多渠道与商业完整（12周）

**目标**: 多渠道发布、商业功能完善

| 周次 | 任务 | 交付物 |
|------|------|--------|
| Week 37-40 | Channel Service | 5个渠道适配器 |
| Week 41-42 | 计费增强 | 并发控制+国内支付 |
| Week 43-44 | Marketplace UX | Coze风格UI |
| Week 45-46 | 模板市场 | Bot/工作流/插件模板 |
| Week 47-48 | 集成测试+部署 | 生产就绪 |

---

## 6. 保留vs重构决策表

| 模块 | 决策 | 理由 |
|------|------|------|
| **Gateway/Relay** | 重构 | 集成new-api + bifrost，能力大幅提升 |
| **Auth认证** | 保留v08 | 已完整，JWT+OAuth已验证 |
| **Billing计费** | 保留v08 + 增强 | 核心完整，仅增强定价和支付 |
| **Marketplace** | 保留v08 + UX增强 | 核心完整，增加Coze的UX |
| **运维体系** | 完全保留v08 | 已验证，无需重建 |
| **Workflow** | 全新构建 | 当前无，借鉴dify/FastGPT |
| **RAG** | 全新构建 | 当前v08简单，用ragflow/MaxKB |
| **Chat UI** | 重构 | 用lobe-chat的现代框架 |
| **Agent** | 升级v08 | v08有基础，增加双引擎和工具 |
| **Channel** | 全新构建 | 当前无，借鉴Coze |
| **Admin** | 保留v08 + 增强 | 基础完整，增加新模块管理 |

---

*第二部分完成，继续第三部分：数据库完整Schema、API规范、部署架构*




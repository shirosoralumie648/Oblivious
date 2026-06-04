# Oblivious 功能逻辑详细设计

**关联文档**: [完整设计第一部分](./2026-06-04-complete-fusion-design.md) | [第二部分](./2026-06-04-complete-fusion-design-part2.md) | [第三部分](./2026-06-04-complete-fusion-design-part3.md)

**创建日期**: 2026-06-04  
**状态**: 用户审查确认

---

## 说明

本文档记录了与用户深入讨论后明确的各功能域的详细逻辑和行为规则。这些是设计文档的**补充细节**，用于指导后续实现。

---

## 1. API网关与Relay层

### 1.1 负载均衡策略

**需求**: 同时支持加权和自适应两种模式

**详细逻辑**:
```yaml
负载均衡策略:
  - 加权轮询: 
      描述: 根据渠道权重分配流量
      配置: 每个channel有weight字段(1-100)
      适用场景: 渠道性能稳定且已知
      
  - 自适应均衡:
      描述: 根据健康评分、历史延迟、错误率动态调整
      指标:
        - health_score: 0-1，实时健康评分
        - avg_latency_ms: 平均响应延迟
        - error_rate: 错误率 (5分钟窗口)
      权重计算: weight_dynamic = health_score * (1 - error_rate) / log(latency_ms)
      适用场景: 渠道性能波动大
```

### 1.2 渠道亲和性

**需求**: 按对话粘性 + 渠道故障自动切换 + 语义缓存跨渠道共享

**详细逻辑**:
```yaml
渠道亲和策略:
  粘性级别: conversation_id
  实现:
    - 对话创建时通过负载均衡选择channel
    - 存储mapping: conversation_id -> channel_id
    - 同一对话后续请求优先使用该channel
    
  故障切换:
    - 当前channel返回5xx/429/timeout
    - 自动选择备用channel
    - 更新mapping: conversation_id -> new_channel_id
    - 用户无感知
    
  语义缓存:
    - 缓存key: hash(query + model_id)，不包含channel_id
    - 任意channel都可命中缓存
    - 渠道切换不影响缓存命中率
```

### 1.3 RPM/TPM限制处理

**需求**: 混合模式（本地计数 + 429响应）

**详细逻辑**:
```yaml
限制处理策略:
  第一层防护 - 本地计数器:
    - 渠道配置时填写: max_rpm, max_tpm
    - Redis存储计数器: channel:{id}:rpm, channel:{id}:tpm
    - 窗口: 滑动窗口60秒
    - 预判: 接近限制90%时降低该channel权重
    
  第二层防护 - 被动响应:
    - 捕获上游429错误
    - 立即标记该channel状态为rate_limited
    - 从headers解析重试时间(Retry-After)
    - 暂时移除该channel，等待重试时间后恢复
    - 自动切换到其他channel重试
    
  协同工作:
    - 本地计数器防止大部分429
    - 被动响应处理计数不准确的情况
    - 双重保护确保用户请求成功率
```

### 1.4 故障切换规则

**需求**: 5xx/429/网络错误重试，401/403不重试

**详细逻辑**:
```yaml
错误分类与处理:
  自动切换重试:
    - 500/502/503/504: 上游服务器错误 → 立即切换channel重试
    - 429: Rate limit → 标记channel，切换其他channel
    - 网络错误: Timeout/Connection refused/DNS failure → 切换channel
    - 最大重试次数: 3次（跨channel）
    
  不重试直接返回:
    - 401: Unauthorized → API Key失效，标记channel为invalid
    - 403: Forbidden → 权限问题，标记channel为forbidden
    - 400: Bad Request → 请求格式错误，返回给用户修正
    
  重试策略:
    - 首次重试: 立即
    - 第二次重试: 延迟1s
    - 第三次重试: 延迟3s
    - 指数退避: delay = 1s * 3^(retry_count - 1)
```

### 1.5 语义缓存设计

**需求**: 混合模式（公共query全局共享 + 敏感query按组织隔离）

**详细逻辑**:
```yaml
缓存策略:
  公共缓存 (全局共享):
    条件: query不包含敏感信息
    判断规则:
      - 不含组织名/用户名/邮箱/电话
      - 不含"我的"/"我们公司"等代词
      - 通用知识性query (如"1+1=?", "什么是AI")
    命名空间: global:cache:{model}:{query_hash}
    TTL: 24小时
    
  私有缓存 (组织隔离):
    条件: query包含敏感信息或用户指定
    命名空间: org:{org_id}:cache:{model}:{query_hash}
    TTL: 1小时
    
  缓存查询流程:
    1. 对query进行敏感信息检测
    2. 如果是公共query，先查global缓存
    3. 如果是私有query或global未命中，查org缓存
    4. 命中则返回，未命中则调用LLM并缓存
```

---

## 2. 工作流引擎

### 2.1 触发方式

**需求**: 手动/对话/定时/Webhook/语义触发

**详细逻辑**:
```yaml
触发类型定义:
  manual:
    描述: 用户手动点击"执行"按钮
    参数: input (用户输入的变量)
    
  conversation:
    描述: 用户发消息时自动触发
    绑定: conversation_id → workflow_id
    参数: message (用户消息)
    
  schedule:
    描述: 定时任务
    配置: cron表达式
    参数: 无或预设变量
    
  webhook:
    描述: HTTP回调触发
    配置: webhook_url, secret
    参数: webhook payload
    
  semantic:
    描述: 语义匹配触发 (类似Claude Code技能)
    配置:
      - keywords: ["关键词1", "关键词2"]
      - semantic_threshold: 0.85 (向量相似度阈值)
    流程:
      1. 用户消息向量化
      2. 与已配置工作流的触发条件匹配
      3. 相似度 > 阈值时自动触发
    参数: message
```

### 2.2 节点失败处理

**需求**: 四种策略可选，默认暂停等待用户决策

**详细逻辑**:
```yaml
失败处理策略 (节点级配置):
  1. auto_retry:
      描述: 自动重试后失败
      配置:
        - max_retries: 3
        - retry_delay: [1s, 3s, 9s] (指数退避)
      行为: 超过重试次数后标记workflow为failed
      适用: 网络请求、外部API调用
      
  2. pause_on_failure (默认):
      描述: 暂停等待用户决策
      行为:
        - workflow状态: paused
        - 通知用户: 邮件/站内信/Webhook
        - 用户选项: [重试节点, 跳过节点, 编辑输入重试, 终止workflow]
      适用: 重要节点、需人工判断的场景
      
  3. skip_on_failure:
      描述: 自动跳过继续执行
      行为:
        - 节点状态: skipped
        - 该节点输出为null
        - 后续节点继续执行
        - workflow状态: partial_success
      适用: 非关键节点、可选步骤
      
  4. failure_branch:
      描述: 跳转到失败分支
      配置: failure_branch_node_id
      行为:
        - 跳转到指定的错误处理分支
        - 传递错误信息到分支输入
        - 执行错误处理逻辑（如通知/回滚/补偿）
      适用: 复杂业务逻辑、需补偿的场景
```

### 2.3 并发控制

**需求**: 用户可配置上限 + 智能模式 + 站长级资源限制

**详细逻辑**:
```yaml
并发控制层级:
  1. 工作流级配置:
      max_concurrent_executions: 10 (默认)
      用户可编辑: 是
      超限行为: queue (排队) 或 reject (拒绝)
      
  2. 组织级限制:
      org_max_concurrent_workflows: 50
      所有工作流共享此配额
      超限行为: queue (FIFO)
      
  3. 站长级限制 (系统配置):
      system_max_concurrent_workflows: 1000
      global_max_workflow_executions_per_minute: 10000
      目的: 防止系统资源耗尽
      超限行为: rate_limit_error
      
智能模式:
  根据触发类型自动选择:
    - conversation触发: 允许高并发 (用户体验优先)
    - schedule触发: 串行执行 (避免资源竞争)
    - webhook触发: 中等并发 (平衡响应速度和资源)
```

### 2.4 资源限制

**需求**: 组织级并发/超时/Token预算/节点次数

**详细逻辑**:
```yaml
资源限制配置:
  1. 组织级并发限制:
      字段: org.max_concurrent_workflows
      默认: 50
      检查: 工作流启动前
      超限: 排队或拒绝
      
  2. 执行超时限制:
      字段: workflow.max_execution_duration_seconds
      默认: 3600 (1小时)
      检查: 定时检查运行时间
      超限: 自动终止，状态=timeout
      
  3. Token预算限制:
      字段: workflow.max_tokens_budget
      默认: null (无限制)
      检查: 每次LLM调用后累加
      超限: 暂停workflow，通知用户
      用途: 成本控制
      
  4. 节点执行次数限制:
      字段: workflow.max_node_executions
      默认: 1000
      检查: 每执行一个节点+1 (含循环)
      超限: 终止workflow，状态=max_iterations
      用途: 防止死循环
```

### 2.5 变量作用域

**需求**: 分层变量（节点局部 + 工作流全局）

**详细逻辑**:
```yaml
变量系统:
  节点局部变量:
    命名空间: node.{node_id}.{var_name}
    生命周期: 节点执行期间
    可见范围: 仅当前节点
    用途: 节点内部临时计算
    
  工作流全局变量:
    命名空间: workflow.{var_name}
    生命周期: 整个workflow执行期间
    可见范围: 所有节点
    初始化: workflow启动时
    用途: 跨节点数据传递
    
  节点输出:
    命名空间: nodes.{node_id}.output
    可见范围: 该节点之后的所有节点
    引用语法: {{nodes.node_1.output.result}}
    
  系统变量:
    - {{execution.id}}: 执行ID
    - {{execution.started_at}}: 开始时间
    - {{workflow.name}}: 工作流名称
    - {{user.id}}: 触发用户ID
    - {{org.id}}: 组织ID
```

### 2.6 版本管理

**需求**: 版本隔离 + 多版本历史 + 版本分支

**详细逻辑**:
```yaml
版本管理策略:
  版本隔离:
    - 每次保存工作流自动生成版本号 (v1, v2, v3...)
    - 正在执行的实例绑定版本号
    - 新触发使用最新published版本
    - 编辑中的draft不影响运行中的实例
    
  多版本历史:
    - 保留所有历史版本 (可配置保留数量)
    - 用户可查看任意版本的定义
    - 用户可回滚到任意历史版本
    - 回滚操作创建新版本 (v5 rollback to v2 → v6)
    
  版本分支:
    - 从任意版本创建分支: v3 → v3-experiment
    - 分支独立编辑和测试
    - 分支可合并回主线或发布为新workflow
    - 用途: A/B测试、实验性功能
    
  版本状态:
    - draft: 编辑中，不可执行
    - published: 已发布，可执行
    - archived: 已归档，只读
```

---

## 3. 知识库与RAG

### 3.1 检索策略配置

**需求**: 可配置（纯向量/混合/混合+重排）

**详细逻辑**:
```yaml
检索模式 (知识库级配置):
  vector_only:
    描述: 纯向量检索
    流程: query → embedding → vector_search
    优点: 速度快
    缺点: 可能漏掉关键词精确匹配
    适用: 语义相似度为主的场景
    
  hybrid:
    描述: 混合检索
    流程:
      1. vector_search → 获取top_k1结果
      2. bm25_search → 获取top_k2结果
      3. reciprocal_rank_fusion融合排序
    配置:
      - vector_weight: 0.7
      - bm25_weight: 0.3
    优点: 兼顾语义和关键词
    适用: 大多数场景 (推荐)
    
  hybrid_rerank:
    描述: 混合检索 + 重排
    流程:
      1-3同hybrid
      4. reranker模型重新打分排序
    配置:
      - reranker_model: "bge-reranker-large"
      - rerank_top_k: 10 (取前10送reranker)
    优点: 准确性最高
    缺点: 延迟增加200-500ms
    适用: 对准确性要求极高的场景
```

### 3.2 分块策略配置

**需求**: 多种策略可选（固定/语义/QA）

**详细逻辑**:
```yaml
分块策略 (知识库级配置):
  fixed_size:
    描述: 固定大小分块
    参数:
      - chunk_size: 500 (字符数)
      - chunk_overlap: 50
    优点: 简单快速
    缺点: 可能截断语义
    适用: 格式统一的文档
    
  semantic:
    描述: 语义分块
    方法:
      - 按段落/章节/标题自然分割
      - 保持语义完整性
    参数:
      - max_chunk_size: 1000
      - respect_boundaries: [paragraph, section, chapter]
    优点: 语义完整
    适用: 结构化文档 (推荐)
    
  qa_split:
    描述: 问答拆分
    方法:
      - LLM分析文档生成Q&A对
      - 每个Q&A对作为一个chunk
    参数:
      - qa_generation_model: "gpt-4"
    优点: 检索准确度高
    缺点: 处理慢、成本高
    适用: FAQ、知识问答场景
    
  template_based (ragflow):
    描述: 基于模板的智能分块
    方法:
      - 识别文档类型(合同/论文/手册)
      - 应用对应模板规则
    优点: 最智能、准确度最高
    适用: 复杂文档
```

### 3.3 文档更新策略

**需求**: 用户可配置（全量/增量/多版本）

**详细逻辑**:
```yaml
更新策略 (知识库级配置):
  full_replace:
    描述: 全量更新
    流程:
      1. 删除该文档的所有旧chunks
      2. 重新解析、分块、向量化
      3. 写入新chunks
    优点: 实现简单
    缺点: 资源消耗大
    适用: 文档变化大的场景
    
  incremental:
    描述: 增量更新
    流程:
      1. 对比新旧文档内容
      2. 识别变化的部分
      3. 只更新变化的chunks
      4. 保留未变化的chunks
    算法: 文本diff + chunk hash对比
    优点: 高效、节省资源
    适用: 文档局部修改的场景 (推荐)
    
  versioned:
    描述: 多版本共存
    流程:
      1. 新版本作为独立文档存储
      2. 保留旧版本
      3. chunk metadata标记版本号
    检索:
      - 用户可选择检索特定版本
      - 或检索所有版本
    适用: 需要版本历史的场景
```

### 3.4 引用源追溯

**需求**: 可视化（文档名 + 页码 + 高亮）

**详细逻辑**:
```yaml
源追溯实现:
  返回结构:
    - document_id: UUID
    - document_name: "用户手册.pdf"
    - page_number: 15
    - chunk_index: 3
    - original_text: "完整的chunk文本"
    - matched_snippet: "命中的具体片段"
    - highlight_positions: [
        {start: 120, end: 180}
      ]
    - confidence_score: 0.92
    
  可视化展示:
    - 左侧: 检索结果列表
    - 右侧: 文档预览面板
      - 显示文档名和页码
      - 渲染原始chunk文本
      - 高亮matched_snippet
      - 点击可跳转到原文档
      
  链接原文档:
    - PDF: 生成带页码的URL
    - 在线文档: 直接链接
    - 本地文件: 下载链接
```

---

## 4. Agent系统

### 4.1 双引擎执行模式

**需求**: ReAct + 规划双引擎

**详细逻辑**:
```yaml
执行模式 (Agent级配置):
  react:
    描述: 思考-行动-观察循环
    流程:
      1. Thought: LLM分析当前状态，决定下一步
      2. Action: 调用工具
      3. Observation: 获取工具结果
      4. 循环直到完成或达到最大迭代
    优点: 灵活、实时响应
    适用: 简单任务、实时交互
    
  planning:
    描述: 先制定完整计划再执行
    流程:
      1. LLM分析任务，生成完整步骤计划
      2. 用户审核计划(可选)
      3. 按计划顺序执行每个步骤
      4. 执行中可根据结果调整计划
    优点: 结构化、成本可控
    适用: 复杂多步任务
    
  切换逻辑:
    用户可在Agent配置中选择默认模式
    也可在运行时动态切换
```

### 4.2 工具调用审批

**需求**: 用户可配置（默认分级审批）

**详细逻辑**:
```yaml
审批策略 (Agent级配置):
  工具风险分级:
    safe: [search, read, get, list, calculate]
    medium: [write, create, update, post]
    dangerous: [delete, drop, pay, transfer, execute_code]
    
  审批配置:
    - approval_mode: "tiered" | "all" | "none" | "custom"
    
  tiered (分级审批，默认):
    - safe工具: 自动执行
    - medium工具: 首次需审批，后续可自动
    - dangerous工具: 每次都需审批
    
  all (全部审批):
    - 所有工具调用都需要审批
    - 适用: 高安全要求场景
    
  none (无审批):
    - 所有工具自动执行
    - 适用: 可信环境、测试环境
    
  custom (自定义):
    - 用户为每个工具单独配置
    - 工具列表UI，逐个设置审批要求
```

### 4.3 迭代控制

**需求**: 双重限制（次数 + Token预算）

**详细逻辑**:
```yaml
迭代控制配置:
  max_iterations:
    描述: 最大迭代次数
    默认: 10
    用户可配置: 1-100
    检查: 每完成一次思考-行动循环+1
    超限: 状态=max_iterations_reached
    
  token_budget:
    描述: Token预算
    默认: null (无限制)
    用户可配置: 1000-1000000
    检查: 每次LLM调用后累加
    超限: 状态=token_budget_exceeded
    
  双重限制协同:
    - 两个条件同时检查
    - 任一触发即停止
    - 返回触发的具体原因
    - 用户可选择增加预算继续执行
```

### 4.4 分层记忆系统

**需求**: 短期 + 长期 + 用户管理

**详细逻辑**:
```yaml
记忆层级:
  短期记忆:
    范围: 单次对话
    存储: 对话历史messages数组
    生命周期: 对话结束后清空
    容量: 最近50条消息或10k tokens
    
  长期记忆:
    范围: 跨对话
    存储: PostgreSQL + 向量embedding
    生命周期: 永久或用户手动删除
    类型:
      - 用户偏好: "我喜欢简洁回答"
      - 重要事实: "公司名是XX，成立于2020年"
      - 历史交互: 重要的Q&A记录
    检索:
      - 每次对话开始时向量检索相关记忆
      - Top-5记忆注入到system prompt
      
  用户管理:
    - 查看所有长期记忆
    - 手动添加/编辑/删除记忆
    - 标记记忆重要性(1-5星)
    - 导出/导入记忆
```

---

## 5. 多渠道发布

### 5.1 消息格式转换

**需求**: 统一消息格式 + 渠道适配器

**详细逻辑**:
```yaml
统一消息格式:
  InternalMessage:
    id: UUID
    conversation_id: UUID
    role: "user" | "assistant" | "system"
    content: 
      - type: "text" | "image" | "file" | "card"
      - text: string
      - url: string (for image/file)
      - metadata: object
    timestamp: ISO8601
    
渠道适配器:
  FeiShuAdapter:
    inbound: FeiShuMessageCard → InternalMessage
    outbound: InternalMessage → FeiShuMessageCard
    特性: 支持富文本卡片、交互按钮
    
  WeChatAdapter:
    inbound: WeChatXML → InternalMessage
    outbound: InternalMessage → WeChatXML
    特性: 支持文本、图片、链接
    
  DiscordAdapter:
    inbound: DiscordMessage → InternalMessage
    outbound: InternalMessage → DiscordEmbed
    特性: 支持Embed、Emoji、Reaction
    
  转换流程:
    用户消息 → 渠道Webhook → Adapter.transform_inbound()
    → Bot处理 → Adapter.transform_outbound() → 发送到渠道
```

### 5.2 消息日志存储

**需求**: 保存原始 + 转换后

**详细逻辑**:
```yaml
消息日志表结构:
  channel_messages:
    id: UUID
    channel_id: UUID
    conversation_id: UUID
    direction: "inbound" | "outbound"
    
    raw_message: JSONB
      # 渠道原始格式
      # 用于调试和审计
      
    transformed_message: JSONB
      # 统一格式
      # 用于处理和展示
      
    transform_success: boolean
    transform_error: text
    
    created_at: timestamp
    
存储策略:
  - 所有消息都保存原始和转换后格式
  - 保留期: 30天 (可配置)
  - 超期自动归档到对象存储
  - 用途: 调试渠道问题、审计、合规
```

### 5.3 渠道故障处理

**需求**: 队列重发 + 通知管理员 + 手动切换

**详细逻辑**:
```yaml
故障处理流程:
  1. 检测故障:
      - 渠道API返回5xx
      - 连续3次请求失败
      - Webhook回调超时
      → 标记channel.status = "degraded"
      
  2. 消息队列:
      - 失败的消息进入Redis队列
      - 队列key: channel:{id}:failed_messages
      - 保留消息内容、重试次数、失败原因
      
  3. 重试机制:
      - 延迟重试: 1min, 5min, 15min, 30min, 1h
      - 最大重试: 5次
      - 超过5次 → 放弃，标记为permanent_failure
      
  4. 通知管理员:
      - 故障检测后立即通知
      - 通知方式: 邮件 + 站内信 + Webhook
      - 通知内容: 渠道名、故障类型、影响范围
      
  5. 手动切换:
      - 管理员Dashboard显示所有渠道状态
      - 一键切换到备用渠道
      - 队列中的消息自动路由到新渠道
      
  6. 自动恢复:
      - 定期健康检查(每5分钟)
      - 连续3次成功 → 恢复"active"状态
      - 队列中的消息开始重发
```

---

## 6. 计费与商业化

### 6.1 速率限制

**需求**: RPM + TPM + 并发数 + 单次请求

**详细逻辑**:
```yaml
速率限制类型:
  RPM (Requests Per Minute):
    粒度: 用户级 或 组织级
    实现: Redis滑动窗口计数器
    超限: 返回429，Retry-After头
    
  TPM (Tokens Per Minute):
    粒度: 用户级 或 组织级
    实现: Redis累加器，60s窗口
    超限: 返回429，剩余配额显示
    
  并发数限制:
    粒度: 用户级 或 组织级
    实现: Redis计数器，请求开始+1，结束-1
    超限: 返回429或排队
    
  单次请求Token限制:
    粒度: 订阅级别
    实现: 请求前检查input_tokens
    超限: 返回400，提示分批请求
    
配置示例:
  free_tier:
    rpm: 60
    tpm: 10000
    max_concurrent: 3
    max_tokens_per_request: 4000
    
  pro_tier:
    rpm: 600
    tpm: 100000
    max_concurrent: 10
    max_tokens_per_request: 32000
```

### 6.2 配额粒度管理

**需求**: 双层级（组织 + 用户可选）

**详细逻辑**:
```yaml
配额管理模式:
  organization_level:
    描述: 组织统一配额
    字段: organization.quota_tokens
    共享: 组织内所有用户共享
    适用: 小团队、统一管理
    优点: 管理简单
    
  user_level:
    描述: 用户独立配额
    字段: organization_member.quota_tokens
    独立: 每个用户独立配额
    适用: 大团队、需要隔离
    优点: 责任明确
    
  切换逻辑:
    - 组织设置中配置quota_mode
    - quota_mode = "organization" → 使用organization.quota_tokens
    - quota_mode = "user" → 使用member.quota_tokens
    - 创建组织时选择，后续可变更
    
  分配策略:
    - organization模式: 先到先得
    - user模式: 管理员分配或购买
```

### 6.3 费用统计维度

**需求**: 模型/功能/用户/时间四维度

**详细逻辑**:
```yaml
统计维度实现:
  1. 按模型统计:
      group_by: model_name
      metrics: [request_count, total_tokens, total_cost]
      可视化: 饼图 - 各模型占比
      
  2. 按功能统计:
      group_by: feature_type
      feature_type: [chat, workflow, rag, agent, image, audio]
      可视化: 柱状图 - 各功能使用量
      
  3. 按用户统计:
      group_by: user_id
      metrics: [request_count, total_tokens, total_cost]
      可视化: 表格 - Top消费用户
      排序: 按cost降序
      
  4. 按时间统计:
      group_by: date/hour
      metrics: [request_count, total_tokens, total_cost]
      可视化: 折线图 - 趋势
      粒度: 小时/天/周/月
      
多维度交叉:
  - 模型 × 时间: 各模型每天消耗趋势
  - 用户 × 功能: 用户使用偏好分析
  - 功能 × 时间: 功能使用趋势
  
数据存储:
  - 实时: ClickHouse (秒级查询)
  - 聚合: PostgreSQL (每日聚合)
  - 展示: Grafana Dashboard
```

---

## 7. 总结

本文档详细记录了6大核心功能域的业务逻辑规则，所有决策都经过用户确认。这些细节将直接指导后续的详细设计和代码实现。

**下一步**:
1. 用户最终审查本文档
2. 技术预研（Python→Go迁移）
3. 启动实施（Q1 Week 1）

---

## 7. 前端UI设计

### 7.1 聊天界面布局

**需求**: lobe-chat侧边栏布局

**详细逻辑**:
```yaml
布局结构 (参考lobe-chat):
  侧边栏 (左侧, 宽度300px):
    - 对话列表
    - 新建对话按钮
    - 搜索框
    - 过滤器（全部/已标记/已归档）
    
  主对话区 (右侧, 自适应):
    - 顶部: 模型选择 + 对话设置
    - 中部: 消息列表 (滚动区域)
    - 底部: 输入框 + 工具栏
    
  响应式:
    - 移动端: 侧边栏折叠，点击展开
    - 平板: 侧边栏窄化到200px
    - 桌面: 完整布局
```

### 7.2 消息操作功能

**需求**: 操作菜单/对话分叉/消息书签/消息分享

**详细逻辑**:
```yaml
消息操作菜单:
  触发: 鼠标悬停消息显示操作按钮
  功能:
    - 复制: 复制消息文本到剪贴板
    - 编辑: 修改用户消息并重新生成
    - 删除: 删除消息（确认弹窗）
    - 重新生成: 针对AI回复，重新生成响应
    
对话分叉 (LibreChat特性):
  触发: 消息操作菜单 → "从这里分叉"
  行为:
    - 创建新对话
    - 复制从对话开始到该消息的所有历史
    - 跳转到新对话，可以继续不同方向的探索
  用途: 探索不同回答路径
  
消息书签:
  触发: 消息操作菜单 → "添加书签"
  存储: bookmark表 (user_id + message_id)
  查看: 侧边栏 → 书签标签页
  用途: 收藏重要消息/精彩回答
  
消息分享:
  触发: 消息操作菜单 → "分享"
  方式:
    - 生成分享链接 (可设置过期时间)
    - 生成图片 (美化的对话截图)
    - 复制为Markdown
  隐私: 可选择分享单条或整个对话
```

### 7.3 工作流编辑器

**需求**: 智能吸附（网格 + 自由微调）

**详细逻辑**:
```yaml
节点布局策略:
  网格吸附:
    - 网格大小: 20px × 20px
    - 拖动节点时自动对齐到网格
    - 可通过设置禁用吸附
    
  自由微调:
    - 按住Alt/Option键禁用吸附
    - 可精确放置到任意位置
    - 用于微调节点间距
    
  智能对齐辅助线 (参考Figma):
    - 拖动节点时显示对齐辅助线
    - 自动对齐到其他节点的边缘/中心
    - 等距分布提示
    
  自动排列:
    - 右键 → "自动排列"
    - 算法: 分层布局（DAG）
    - 横向/纵向可选
```

### 7.4 工作流调试

**需求**: 单节点测试/实时高亮/调试面板/性能分析

**详细逻辑**:
```yaml
单节点测试 (FastGPT特性):
  触发: 右键节点 → "测试此节点"
  流程:
    1. 弹窗输入测试数据
    2. 模拟执行该节点
    3. 显示输入/输出
  用途: 快速验证单个节点逻辑
  
实时高亮执行节点:
  执行状态显示:
    - pending: 灰色边框
    - running: 蓝色边框 + 脉冲动画
    - completed: 绿色边框
    - failed: 红色边框
  进度指示: 节点右上角显示执行时间
  
调试面板 (右侧面板):
  标签页:
    - 变量: 显示当前所有变量及其值
    - 调用链: 树形显示节点执行顺序
    - 输出: 每个节点的输出结果
    - 日志: 执行日志流
  实时更新: 执行过程中实时刷新
  
性能分析:
  执行后显示:
    - 总执行时间
    - 每个节点执行时间 (柱状图)
    - Token消耗统计
    - 成本估算
  瓶颈识别: 高亮执行时间最长的节点
```

### 7.5 知识库管理

**需求**: 分块可视化（ragflow风格）

**详细逻辑**:
```yaml
文档预览布局:
  左侧: 文档列表
    - 文档名 + 状态 (处理中/已完成/失败)
    - 分块数量、大小
    - 上传时间
    
  右侧: 文档预览区
    - PDF/Word原文渲染
    - 分块边界高亮显示
    - 鼠标悬停chunk显示详情
    
分块可视化 (ragflow特性):
  显示方式:
    - 在原文上用虚线框标注chunk边界
    - 不同chunk使用不同颜色区分
    - chunk编号标注
    
  交互:
    - 点击chunk: 高亮显示，右侧显示详情
    - chunk详情:
      - chunk_id
      - 文本内容
      - 字符数/Token数
      - embedding向量 (可选显示)
      - 元数据 (页码、标题等)
    - 编辑chunk: 手动修改文本或拆分/合并
```

### 7.6 检索测试

**需求**: 增强搜索结果（高亮 + 分数 + 链接）

**详细逻辑**:
```yaml
检索测试界面:
  输入区:
    - 查询输入框
    - 参数配置:
      - top_k: 1-20
      - 相似度阈值: 0-1
      - 检索模式: vector_only / hybrid / hybrid_rerank
      
  结果展示:
    每条结果卡片显示:
      - 相似度分数 (0.92) + 进度条
      - 来源: 文档名 + 页码
      - 匹配文本 (高亮query关键词)
      - 操作:
        - "查看原文": 跳转到文档预览
        - "查看chunk": 显示完整chunk
        - "添加到测试集": 保存为测试case
        
  性能指标:
    - 检索耗时
    - 命中chunk数
    - 平均相似度
```

---

## 8. Marketplace与生态

### 8.1 发布审核流程

**需求**: AI初审 + 人工复审

**详细逻辑**:
```yaml
审核流程:
  步骤1 - AI自动初审:
    检测项:
      - 违规内容: 暴力/色情/政治敏感
      - 恶意代码: 代码静态扫描
      - 数据窃取: 检测是否访问敏感API
      - Prompt注入: 检测Prompt注入风险
    工具:
      - LLM内容审核
      - 代码静态分析工具
    结果:
      - 通过 → 进入人工复审队列
      - 拒绝 → 通知发行商原因，可修改重新提交
      
  步骤2 - 人工复审:
    审核项:
      - 功能完整性
      - 描述准确性
      - 使用体验
      - 定价合理性
    审核时限: 3个工作日
    结果:
      - 通过 → 上架
      - 拒绝 → 说明理由
      - 待补充 → 要求发行商补充材料
      
  审核SLA:
    - AI初审: 5分钟内
    - 人工复审: 72小时内
    - 紧急: VIP发行商24小时内
```

### 8.2 结算周期配置

**需求**: 用户可配置（周结/月结等）

**详细逻辑**:
```yaml
结算周期选项:
  weekly:
    描述: 每周一结算上周收入
    到账: 结算后3个工作日
    手续费: 2%
    适用: 中小发行商，现金流需求高
    
  monthly:
    描述: 每月1号结算上月收入
    到账: 结算后5个工作日
    手续费: 1%
    适用: 大多数发行商 (默认)
    
  quarterly:
    描述: 每季度首日结算上季度收入
    到账: 结算后5个工作日
    手续费: 0.5%
    适用: 大型发行商，可接受延迟
    
配置方式:
  - 发行商在后台选择结算周期
  - 变更生效: 下个结算周期
  - 不可跨周期变更
  
最低结算金额:
  - 低于$100不结算，累计到下期
  - 避免小额多次手续费损失
```

### 8.3 阶梯分成比例

**需求**: 阶梯分成（销量越高分成越低）

**详细逻辑**:
```yaml
分成阶梯 (月度销售额):
  Tier 1 (0 - $1,000):
    平台抽成: 30%
    发行商得: 70%
    说明: 新发行商起步阶段
    
  Tier 2 ($1,000 - $10,000):
    平台抽成: 20%
    发行商得: 80%
    说明: 中等规模发行商
    
  Tier 3 ($10,000 - $100,000):
    平台抽成: 15%
    发行商得: 85%
    说明: 头部发行商
    
  Tier 4 ($100,000+):
    平台抽成: 10%
    发行商得: 90%
    说明: 超级发行商，平台激励
    
计算方式:
  - 分段计算，非整体折扣
  - 例: 销售额$15,000
    - 前$1,000: 抽成30% = $300
    - 中$9,000: 抽成20% = $1,800
    - 后$5,000: 抽成15% = $750
    - 总抽成: $2,850 (19%)
    
透明展示:
  - 发行商后台实时显示当前阶梯
  - 距离下一阶梯的销售额差距
  - 预估到下一阶梯的收益增加
```

### 8.4 混合推荐算法

**需求**: 热门 + 协同过滤 + 内容推荐 + 随机探索

**详细逻辑**:
```yaml
推荐算法组成:
  1. 热门推荐 (30%权重):
      指标: 下载量 + 评分 + 活跃用户数
      计算: hot_score = downloads * 0.4 + rating * 0.3 + active_users * 0.3
      排序: 7天滚动窗口
      目的: 新用户快速发现优质内容
      
  2. 协同过滤 (30%权重):
      算法: User-based CF
      相似度: Cosine similarity on user-agent matrix
      推荐: "使用过A、B的用户还使用了C"
      目的: 个性化推荐
      
  3. 内容推荐 (30%权重):
      算法: Content-based filtering
      特征: Agent功能标签、分类、描述
      匹配: 用户浏览历史 + 当前查询
      目的: 精准匹配需求
      
  4. 随机探索 (10%权重):
      算法: Multi-armed bandit (ε-greedy)
      ε = 0.1: 10%概率随机推荐
      目的: 帮助新Agent冷启动
      
混合策略:
  - 首页: 热门50% + 协同过滤30% + 探索20%
  - 搜索结果: 内容推荐70% + 协同过滤20% + 探索10%
  - 个人推荐: 协同过滤50% + 内容推荐40% + 探索10%
```

---

## 9. 监控与运维

### 9.1 告警通知渠道

**需求**: 邮件/IM机器人/短信电话/第三方平台

**详细逻辑**:
```yaml
通知渠道配置:
  邮件:
    协议: SMTP
    配置: smtp_host, smtp_port, username, password
    模板: HTML + Plain text
    适用: info/warning级别
    
  IM机器人:
    支持:
      - Slack Webhook
      - 飞书机器人
      - 钉钉机器人
      - 企业微信群机器人
    格式: Markdown rich message
    适用: 所有级别
    
  短信/电话:
    提供商: Twilio / 阿里云短信
    触发: critical级别
    限流: 每人每小时最多5条短信/1次电话
    避免骚扰: 有升级机制
    
  第三方平台:
    支持:
      - PagerDuty
      - Opsgenie
      - 阿里云监控
      - 腾讯云监控
    集成: API/Webhook
    
通知路由规则:
  debug级别: 仅日志，不通知
  info级别: 邮件
  warning级别: 邮件 + IM
  critical级别: 邮件 + IM + 短信 + 第三方平台
```

### 9.2 四级告警系统

**需求**: debug/info/warning/critical

**详细逻辑**:
```yaml
告警级别定义:
  debug:
    描述: 调试信息
    示例: "慢查询检测: query耗时1.2s"
    处理: 仅记录日志
    通知: 无
    
  info:
    描述: 一般信息性事件
    示例: "服务启动", "定时任务完成"
    处理: 记录日志
    通知: 邮件 (批量发送，1小时汇总)
    
  warning:
    描述: 需要关注的异常
    示例: "错误率超过5%", "队列积压"
    处理: 记录 + 自动尝试恢复
    通知: 邮件 + IM (15分钟内最多1次)
    
  critical:
    描述: 严重故障
    示例: "服务down", "数据库连接失败"
    处理: 立即自动恢复 + 上报
    通知: 所有渠道 + 电话 (立即)
    
告警升级:
  - warning持续30分钟 → 升级为critical
  - 同一告警5分钟内重复3次 → 升级一级
```

### 9.3 全自动恢复

**需求**: 自动重启 + 自动扩容 + 自动故障转移

**详细逻辑**:
```yaml
自动重启:
  触发条件:
    - 服务health check连续3次失败
    - OOM (Out of Memory)
    - Panic/Crash
  重启策略:
    - 最大重启次数: 5次/10分钟
    - 延迟: 10s, 30s, 60s, 120s, 300s (指数退避)
    - 超过上限 → 标记为故障，人工介入
  K8s实现: restartPolicy: Always + readinessProbe
  
自动扩容:
  触发条件:
    - CPU使用率 > 80% 持续5分钟
    - 内存使用率 > 85% 持续5分钟
    - 请求队列积压 > 100
  扩容策略:
    - 每次增加当前实例数的50% (最少1个)
    - 最大实例数: 配置上限
    - 冷却期: 5分钟 (避免频繁扩缩)
  K8s实现: HorizontalPodAutoscaler
  
自动缩容:
  触发条件:
    - CPU/内存使用率 < 30% 持续15分钟
  缩容策略:
    - 每次减少20%实例
    - 最小实例数: 配置下限 (通常3)
    - 冷却期: 15分钟
    
自动故障转移:
  数据库:
    - PostgreSQL: 主从自动切换 (Patroni)
    - Redis: Sentinel自动故障转移
  消息队列:
    - Kafka: 自动选举新leader
  负载均衡:
    - 自动摘除不健康节点
    - 健康恢复后自动加回
```

---

## 10. 总结更新

本文档详细记录了**9大核心功能域**的业务逻辑规则，所有决策都经过用户确认。

**新增功能域** (本次更新):
7. 前端UI设计 - 聊天/工作流/知识库界面交互细节
8. Marketplace - 审核/结算/分成/推荐算法
9. 监控运维 - 告警/通知/自动恢复

**完整覆盖**:
1. API网关与Relay
2. 工作流引擎
3. 知识库与RAG
4. Agent系统
5. 多渠道发布
6. 计费与商业化
7. 前端UI设计
8. Marketplace与生态
9. 监控与运维

**下一步**:
1. 用户最终审查本文档
2. 技术预研（Python→Go迁移）
3. 启动实施（Q1 Week 1）

---

*文档完成日期: 2026-06-04*  
*最后更新: 2026-06-04 - 新增前端UI、Marketplace、监控运维3个功能域*



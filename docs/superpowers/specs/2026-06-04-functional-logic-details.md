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

*文档完成日期: 2026-06-04*


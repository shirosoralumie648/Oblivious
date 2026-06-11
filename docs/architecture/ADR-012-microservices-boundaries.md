# ADR-012: 微服务边界与通信契约

日期: 2026-06-11  
状态: ACCEPTED  
相关: fusion spec part1 §1-2

## 背景

当前 v08 为单体架构（所有服务在一个进程、共享数据库）。融合规格要求 DDD 微服务架构：12 个独立服务，database-per-service，gRPC 内部通信。

## 决策

采用以下 12 服务边界（严格按 spec §2.2）：

| 服务 | 职责 | 数据库 | 依赖服务 |
|---|---|---|---|
| **gateway** | API 网关、路由、认证 | gateway_db | - |
| **relay** | LLM 代理、负载均衡、语义缓存 | relay_db | - |
| **chat** | 对话管理、消息持久化 | chat_db | relay (gRPC) |
| **workflow** | 工作流引擎、节点执行 | workflow_db | agent, rag, relay |
| **rag** | 知识库、检索增强 | rag_db | relay (embedding) |
| **agent** | Agent 执行、工具调用 | agent_db | relay, rag |
| **billing** | 计费、配额、限流 | billing_db | - |
| **marketplace** | 模板市场、发布审核 | marketplace_db | - |
| **admin** | 租户管理、系统配置 | admin_db | - |
| **channel** | 多渠道适配器 | channel_db | chat, agent |
| **task** | 定时任务调度 | task_db | workflow, agent |
| **observability** | 指标、告警、日志 | observability_db (ClickHouse) | - |

## 通信模式

- **内部同步调用:** gRPC (proto 定义在 `api/proto/`)
- **异步事件:** Kafka topics (`workflow.executed`, `agent.run.completed`, etc.)
- **外部 API:** HTTP/JSON (gateway 统一入口)

## 数据库策略

- PostgreSQL: 11 个业务服务各一个逻辑数据库
- ClickHouse: observability 服务专用
- 服务间**禁止**跨库 JOIN，通过 gRPC API 聚合

## 迁移路径

**Phase 1 (当前):** 单体模式保持，schema 添加服务边界注释。  
**Phase 2:** Database-per-service 逻辑隔离（同一 PG 实例的独立 schema）。  
**Phase 3:** 独立进程部署（每服务一个 cmd/），保持单体模式可选。  
**Phase 4:** 移除单体模式，强制微服务通信。

## 后果

**优点:** 按规格实现、独立扩展、故障隔离  
**缺点:** 复杂度增加、跨服务事务需分布式协调  
**风险:** 迁移期间双模式维护成本

## 实施

1. 定义 12 个 proto 服务接口
2. 数据库迁移拆分脚本
3. 每服务独立 cmd 入口
4. 测试覆盖单体模式和微服务模式

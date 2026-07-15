# Phase 31: 发布合同与当前基线 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md; this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 31-release-contract-current-baseline
**Areas discussed:** 能力粒度与状态, 首发部署模式, 权威合同与漂移阻断, 未承诺能力的呈现方式

---

## 能力粒度与状态

### 能力声明粒度

| Option | Description | Selected |
|--------|-------------|----------|
| 领域 + 生命周期切片 | 领域用于稳定导航，streaming、cancel、replay、refund 等独立行为分别声明。 | ✓ |
| 模块级 | 每个模块只有一个整体状态，简单但会掩盖生命周期缺口。 | |
| API route 级 | 逐 route 声明，精确但会把 route 存在误当成能力完整。 | |

**User's choice:** `1`
**Notes:** 用户选择推荐项。

### 状态模型

| Option | Description | Selected |
|--------|-------------|----------|
| 承诺与可用性双轴 | 分开表达商业承诺和 profile/runtime 可用性。 | ✓ |
| 单一四态 | `committed / conditional / disabled / unsupported`，简单但语义交叉。 | |
| 仅启用布尔值 | `enabled: true/false`，无法表达商业边界。 | |

**User's choice:** `1`
**Notes:** 用户选择推荐项。

### 动态 readiness

| Option | Description | Selected |
|--------|-------------|----------|
| 合同与观测分离 | release manifest 固定承诺，runtime readiness report 动态记录环境状态。 | ✓ |
| 单一动态 manifest | runtime 改写 availability，承诺会随环境漂移。 | |
| 仅发布时静态快照 | 无法反映发布后的依赖故障。 | |

**User's choice:** `1`
**Notes:** 用户选择推荐项。

### 固定状态集合

| Option | Description | Selected |
|--------|-------------|----------|
| 严格三态 | `commitment: committed|conditional|excluded`，`availability: enabled|disabled|blocked`。 | ✓ |
| 加入预览与降级态 | 表达丰富，但容易让不完整能力进入正式承诺。 | |
| 开放字符串状态 | 灵活但不可稳定校验或跨模块比较。 | |

**User's choice:** `1`
**Notes:** `blocked` 必须阻断发布；所有非 enabled 状态需要结构化 reasonCode。

---

## 首发部署模式

用户在完成上一区域后回复“都按照你推荐的来吧”。以下选择均依据该明确授权采用推荐项，没有再逐题等待。

| Decision | Recommended option selected | Alternatives considered |
|----------|-----------------------------|-------------------------|
| 当前 committed profile | 只承诺 `monolith`；microservices/dual/split 为 excluded + disabled。 | 同时承诺 monolith 和 microservices；按资产存在自动承诺。 |
| 默认 profile | 显式且唯一的 `monolith`；未知/缺失 profile fail closed。 | 自动探测；允许多个默认 profile。 |
| profile 合同内容 | 声明拓扑、依赖、状态存储、capability override、migration/deploy/rollback 和 readiness。 | 只声明 profile 名称；只记录容器列表。 |
| profile 晋级 | Phase 38 通过同能力、tenant-denial 和运维 parity 后显式修改合同。 | 环境变量临时启用；已有 Kubernetes asset 即视为晋级。 |

**User's choice:** 全部采用推荐项。
**Notes:** live scout 显示 `docker-compose.yml` 默认运行 monolith，microservices 只是显式 profile；旧 ADR 和部署资产不能替代 parity evidence。

---

## 权威合同与漂移阻断

| Decision | Recommended option selected | Alternatives considered |
|----------|-----------------------------|-------------------------|
| 能力/部署权威 | 一个 schema-validated、versioned、authored release contract；其他文档和制品从它生成或验证。 | OpenAPI route manifest 兼任所有权威；多个手写 manifest 并存。 |
| 各 surface owner | OpenAPI、`.proto`、migration SQL/ledger 各自保持 canonical，release contract 引用其 digest/version。 | release contract 复制全部 schema；runtime 静默覆盖文档。 |
| 漂移方向 | source 与 consumer 双向检查，缺失、额外和不兼容都失败。 | 只检查文档缺失；自动重写一边后继续发布。 |
| 门禁输出 | CI/release 阻断并生成包含 commit、digest、差异和 skip 的机器报告。 | 只输出人类日志；warning-only。 |

**User's choice:** 全部采用推荐项。
**Notes:** 初次 baseline reconciliation 时 live runtime 优先于旧设计拓扑；建立 canonical contract 后，任何冲突都必须显式解决。

---

## 未承诺能力的呈现方式

| Decision | Recommended option selected | Alternatives considered |
|----------|-----------------------------|-------------------------|
| 用户侧曝光 | excluded 能力从默认 UI/docs/client 隐藏；conditional 只在 profile + readiness enabled 时显示。 | 所有入口常驻并显示 coming soon；只靠文案说明不可用。 |
| API 行为 | excluded 不注册/不广告并返回 404；disabled/blocked 使用稳定结构化错误，blocked 返回 503 并阻断发布。 | 所有状态返回通用 500；返回成功但不执行。 |
| 强制执行层 | frontend、HTTP/gRPC、service/worker、outbound 和 financial side effect 全层使用同一 capability ID。 | 仅隐藏 UI；仅在最外层检查。 |
| Operator 视图 | 展示完整 inventory、profile、reasonCode、依赖、时间、remediation 和 evidence refs，并脱敏。 | 运营者也只看到用户视图；暴露原始配置和内部地址。 |

**User's choice:** 全部采用推荐项。
**Notes:** manifest 缺失、capability 未知、profile 不匹配或 readiness 不可读时一律 fail closed。

---

## the agent's Discretion

- canonical manifest 的确切路径、JSON/YAML 序列化和 schema validator 工具。
- capability ID 分隔符、reasonCode 命名表和 operator 输出格式。
- 前端 client 使用 code generation 或 fingerprint parity，只要覆盖实际 feature API consumers。

## Deferred Ideas

- Microservices/dual/split parity 与晋级由 Phase 38 负责。
- 业务 lifecycle、真实客户旅程、E3/E4 和供应链证明继续留在 Phase 32-39。

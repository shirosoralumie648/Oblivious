# M0 Baseline Freeze Closeout Design

日期：2026-04-06

## 1. Background

当前仓库已经存在两条并行现实：

- 主线实际交付链路集中在 `src/server`、`src/web`、`config`、`scripts`、`.github/workflows`
- 仓库根级 `pnpm-workspace.yaml` 仍把 `lobehub` 与 `new-api` 纳入 workspace，导致主线边界、lockfile、安装范围、CI 认知出现漂移

同时，M0 的原始目标不是继续扩展产品能力，而是冻结后续里程碑赖以执行的基线，包括：

- 主线边界
- 根级验证入口
- 配置与契约文档
- owner 映射与周报/阻塞升级机制

本设计以“当前代码现实”为唯一事实来源，不为追平历史设计而回改当前主线实现。

## 2. Goal

完成一个可持续执行的 M0 基线冻结闭环，使后续里程碑能够基于同一套主线边界、验证入口、配置契约和治理模板推进。

## 3. Non-Goals

本里程碑明确不做以下内容：

- 不新增任何产品功能
- 不扩展 `Console`、`Chat`、`Knowledge`、`SOLO` 的业务能力
- 不重构 `src/web` 或 `src/server` 的业务实现
- 不把 `lobehub` 或 `new-api` 重新纳入主线治理
- 不在治理文档中写入真实负责人姓名

## 4. Decisions

### 4.1 Source Of Truth

当历史文档与当前代码现实冲突时，以当前代码现实为准，更新文档去追平实现，而不是为了对齐旧文档去回改主线代码。

### 4.2 Workspace Strategy

`lobehub` 与 `new-api` 继续保留在仓库中，但它们的角色被明确调整为“仓内参考目录”：

- 不再属于 root workspace
- 不再进入 root lockfile 的主线维护范围
- 不再纳入 root check/test/CI 的覆盖边界
- 不再作为主线交付链的一部分描述

### 4.3 Completion Level

本里程碑的退出标准采用“流程冻结”级别：

- 本地验证入口冻结
- CI 边界冻结
- owner / 周报 / 阻塞升级机制冻结

### 4.4 Governance Writing Style

治理文档采用混合写法：

- 主文档只写角色，不写真实人名
- 模板中预留负责人字段，供实际执行时填写

## 5. Mainline Boundary After Freeze

冻结后的主线交付范围定义为：

- `src/server`
- `src/web`
- `config`
- `scripts`
- `.github/workflows`
- 主线治理与架构文档

冻结后的参考范围定义为：

- `lobehub`
- `new-api`

这些参考目录可以继续作为研究、比对或迁移输入保留在仓库中，但不能继续影响：

- root workspace
- root install
- root check/test
- root CI
- 主线契约文档

## 6. Deliverables

本里程碑交付物收敛为 4 组。

### 6.1 Mainline Boundary Freeze

需要交付：

- root `pnpm-workspace.yaml` 收缩为主线范围
- 文档中明确 `lobehub` 与 `new-api` 的参考目录角色
- root workspace 与主线文档边界一致

### 6.2 Execution Entry Freeze

需要交付：

- root `package.json` 的 `check` / `test` / `dev` 入口语义清晰
- `scripts/check.sh` 与 `scripts/test.sh` 明确只验证主线
- `.github/workflows/ci.yml` 与 root 入口表达相同边界

目标不是增加更多门禁，而是确保“本地入口”和“CI 入口”表达的是同一套主线现实。

### 6.3 Contract And Config Freeze

需要交付：

- `docs/architecture/current-system-contracts.md` 增补主线边界、workspace 策略、验证入口说明
- `config/.env.example` 与当前 `config.Load()` 真实消费字段保持一致
- 如有必要，补一份专门说明 M0 基线冻结结果的治理文档

### 6.4 Governance Freeze

需要交付：

- 角色级 owner 映射文档
- 周报模板
- 阻塞升级机制说明

这些文档只服务后续里程碑执行，不尝试建立完整项目管理体系。

## 7. Verification Model

本里程碑采用三层验证模型。

### 7.1 Local Entry Verification

必须有一组 root 级主线入口，可以作为本地冻结证据：

- `bash scripts/check.sh`
- `bash scripts/test.sh`

这两条命令的语义必须稳定，且不隐式依赖参考目录。

### 7.2 CI Alignment Verification

`.github/workflows/ci.yml` 必须与本地主线边界一致：

- CI 不扫描 `lobehub` / `new-api`
- CI 中的安装与验证范围与 root 入口等价
- CI 文本与本地脚本的主线认知一致

### 7.3 Documentation Consistency Verification

下列资产必须相互一致：

- `pnpm-workspace.yaml`
- root scripts
- CI workflow
- `config/.env.example`
- `docs/architecture/current-system-contracts.md`
- owner / 周报 / 阻塞升级文档

## 8. Risks And Mitigations

### 8.1 Workspace Shrink Risk

风险：

- 移出 `lobehub` / `new-api` 后，任何对它们的隐式依赖都会暴露

处理：

- 只修主线真实入口，不保留对参考目录的隐式兼容

### 8.2 Historical Document Drift Risk

风险：

- 历史报告仍可能记录已经过时的阶段判断

处理：

- 只更新当前仍作为主线基线引用的文档
- 不把本里程碑扩展成历史报告全面重写

### 8.3 CI And Local Drift Risk

风险：

- root scripts、子包 scripts 和 CI 可能再次漂移

处理：

- 以 root 入口为统一门面
- CI 只调用与 root 入口等价的步骤

### 8.4 Governance Overdesign Risk

风险：

- owner / 周报 / 升级机制容易写成大而空的流程文档

处理：

- 只交付最小可执行模板
- 目标是支持后续里程碑执行，而不是建立完整 PM 系统

## 9. Implementation Shape

本里程碑实现上采用“单一基线源收敛”方案，而不是“文档先行”或“工程先行”：

- 同时收敛 workspace、automation、contracts、governance
- 所有修改都围绕“主线真实可执行范围”展开
- 任何不直接服务于后续里程碑稳定执行的内容都不纳入范围

原因是：

- 仅做文档先行会制造“文档已冻结，入口未冻结”的短期失真
- 仅做工程先行会延续“入口已变，文档未追平”的基线断裂
- 单一基线源收敛能在一次里程碑内结束两套真相并存的问题

## 10. Milestone Completion Criteria

当以下条件同时满足时，本里程碑完成：

1. root workspace 不再纳管 `lobehub` 与 `new-api`
2. root `check/test` 与 CI 只覆盖主线范围
3. `current-system-contracts` 与 `.env.example` 明确反映当前代码现实
4. owner 映射、周报模板、阻塞升级机制文档齐备
5. 至少一轮 root 主线检查命令可作为冻结证据稳定跑通

## 11. Recommended Outputs

本设计推荐在实现阶段优先落地以下输出：

1. workspace 边界收敛
2. root/CI 验证入口对齐
3. 契约与配置文档冻结
4. owner / 周报 / 阻塞升级模板补齐

这样可以先消除主线执行歧义，再把治理文档补到足以支撑后续里程碑的程度。

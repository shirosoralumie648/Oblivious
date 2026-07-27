# Oblivious 项目宪章 (Governance Charter)

> 本文件是进入本项目的唯一入口。它不重复需求与阶段细节，只提供导航与不可协商的不变量。
> 历史实现计划见 IMPLEMENTATION_PLAN.md（已弃用）。

## 唯一事实源 (Source of Truth)

| 问题 | 文件 |
|------|------|
| 这是什么产品、给谁用、关键旅程 | `.planning/PROJECT.md` |
| 做到什么才算商业化完成（含证据标准） | `.planning/REQUIREMENTS.md` |
| 分几个阶段、依赖与成功标准 | `.planning/ROADMAP.md` |
| 现在进行到哪 | `.planning/STATE.md` |
| 如何并发推进、如何锁契约 | `.planning/EXECUTION-STRATEGY.md` |

## 什么才算「商业化完成」

完成的唯一定义见 `.planning/REQUIREMENTS.md` 的 Definition of Done 与 E1–E4 证据模型：

- **E1** 单元/契约/fixture → **E2** 仓库运行时 → **E3** 目标环境 → **E4** 商业发布（同 commit/digest、无 skip）。
- route、page、schema、proto、health endpoint、测试数量或文档存在，都**不**单独证明旅程完成。
- 低等级证据不能关闭要求更高等级证据的需求。

## 核心不变量 (Invariants)

1. **Relay 唯一权威**：所有可计费 AI 调用只能经 Relay 到达 Provider；不得有第二套 usage/billing 权威。
2. **租户贯穿**：可信 actor/organization 上下文贯穿 HTTP、gRPC、job、retry、vector、analytics；跨租户访问一律拒绝。
3. **Fail closed**：未声明或未证明的能力必须禁用、隐藏并安全失败。
4. **证据优先于数量**：生命周期完整度和证据优先于 Provider/tool/channel 的目录数量。

## 如何推进 (Execution)

- 所有改动通过 GSD 命令进行（`/gsd-quick`、`/gsd-execute-phase` 等），不绕过 GSD 直接编辑 planning 文件。
- 并发推进遵循 `.planning/EXECUTION-STRATEGY.md` 的波段模型（地基先行 → 多轨并发 → 串行收口）与并发协作机制（契约先锁 / 轨道边界 / 集成门 / 修订协议）。

## 停止条件 (no-final-readiness)

在缺少每个商业 gate 的当前证据、自动化验证和适用 runtime smoke 之前，不得声称最终商业 readiness。

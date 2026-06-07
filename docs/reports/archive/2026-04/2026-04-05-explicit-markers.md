# Historical Material Notice

> 本文件属于历史阶段材料，不再作为当前现状、优先级判断或里程碑规划依据。
> 当前唯一执行基线请改看：`CURRENT_STATUS.md`、`ROADMAP.md`、`docs/architecture/current-system-contracts.md`

# Oblivious 显式标记复核

日期：2026-04-05

本次复核口径：

- 扫描范围：`src/server`、`src/web`、`docs/superpowers/specs`、`new-api`、`lobehub`
- 关键字：`TODO` / `FIXME` / `TBD`
- 过滤项：
  - `context.TODO()`
  - `docs/reports` 下历史报告的自引用
  - 纯字符串字面量中的误命中

## 1. 汇总

| 范围 | TODO | FIXME | TBD | 合计 |
| --- | ---: | ---: | ---: | ---: |
| `src/server` | 0 | 0 | 0 | 0 |
| `src/web` | 0 | 0 | 0 | 0 |
| `docs/superpowers/specs` | 0 | 0 | 0 | 0 |
| `new-api` | 123 | 1 | 0 | 124 |
| `lobehub` | 112 | 5 | 1 | 118 |

结论：

- 主线仓没有显式 marker。
- `new-api` 的 marker 主要集中在 provider adaptor 的 `implement me`。
- `lobehub` 的 marker 分布更广，既有暂未实现能力，也有明确待重构点。

## 2. 高密度文件

| 文件 | 数量 | 说明 |
| --- | ---: | --- |
| `new-api/relay/channel/cohere/adaptor.go` | 7 | provider adaptor 未完工 |
| `new-api/relay/channel/dify/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/mistral/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/mokaai/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/palm/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/tencent/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/xunfei/adaptor.go` | 6 | provider adaptor 未完工 |
| `new-api/relay/channel/zhipu/adaptor.go` | 6 | provider adaptor 未完工 |
| `lobehub/src/server/services/memory/userMemory/extract.ts` | 5 | memory 提取策略待完善 |
| `lobehub/src/server/services/discover/index.ts` | 4 | SDK fallback 与未开放能力 |

## 3. 代表性条目

### 3.1 `new-api`

- `new-api/controller/channel-billing.go:466`：`// TODO: support Azure`
- `new-api/controller/channel-billing.go:485`：`// TODO: make it async`
- `new-api/service/billing_session.go:178`：哨兵错误与 `errors.Is` 重构
- `new-api/relay/channel/task/gemini/image.go:56`：支持 HTTP URL 图片下载并转 base64
- `new-api/relay/channel/task/gemini/dto.go:14-15`：补 `referenceImages` / `lastFrame`
- `new-api/relay/channel/claude/relay-claude.go:434`：`// FIXME`

### 3.2 `lobehub`

- `lobehub/src/store/home/slices/homeInput/action.ts:161`：`DeepResearch mode`
- `lobehub/src/server/services/agentRuntime/AgentRuntimeService.ts:1638-1644`：`approveToolCall` / `rejectToolCall` / `processHumanInput`
- `lobehub/src/server/services/memory/userMemory/extract.ts:2018-2019`：缓存淘汰与 cache size 配置
- `lobehub/src/server/routers/async/image.ts:151`：`401` 错误处理方式待重构
- `lobehub/packages/builtin-tool-group-management/src/executor.ts:234-259`：summarization / workflow / voting 机制未实现
- `lobehub/packages/types/src/index.ts:42`：OpenAI types 待重构

## 4. 对主线的意义

- 这些显式标记目前不应直接并入 `src/server` / `src/web` 主线执行清单。
- 但它们说明：当前 root workspace 挂入的两个嵌入仓，本身都仍在积极演进中，不适合在角色不清晰的情况下继续和主线共用 workspace。

## 5. 历史详细清单

如果需要逐条原始行号清单，仓库内已有上一轮生成的详单可作为补充参考：

- `docs/reports/2026-04-04-explicit-markers.md`

本次复核与上一轮相比，主线仓结论不变：`src/server` / `src/web` 仍然没有显式 TODO/FIXME/TBD，主风险仍在“实现断裂”和“设计滞后”。


# Blocker Escalation Policy

## Severity Matrix

| Severity | Definition | Response Window | Escalate To |
| --- | --- | --- | --- |
| P0 | 主线 `check/test/build` 无法执行，或主线交付链完全阻断 | 同工作日 | TL |
| P1 | 当前里程碑关键路径阻断，24 小时内无法自行解除 | 1 个工作日内 | TL + 对应 owner |
| P2 | 有绕行方案但会影响计划完成度 | 本周周报中升级 | 对应 owner |

## Trigger Rules

- 同一阻塞持续超过 1 个工作日且没有明确 owner 时，升级到 TL。
- 任一阻塞导致 root `bash scripts/check.sh` 或 `bash scripts/test.sh` 失效时，按 `P0` 处理。
- 任何跨 FE / BE / OPS 的边界问题，先记录在周报，再按严重级别升级。

## Escalation Payload

升级时必须包含以下字段：

- Severity
- Impacted milestone
- Exact failing command or asset
- First observed date
- Current workaround
- Needed decision

# Contracts Lock — Band 0 冻结清单

> 这是 Band 1 并发的**准入门**。每类契约在 Band 0 完成时由地基 owner 填实并置为 `LOCKED`。
> 全部 LOCKED 前，Band 1 产品轨不得启动。
> 修改已 LOCKED 项须走 EXECUTION-STRATEGY §3.4 修订协议。

| # | 契约类别 | Owner | 冻结引用（文件 / 符号） | 状态 |
|---|---------|-------|-------------------------|------|
| 1 | HTTP 契约（OpenAPI operation id / 路径 / 请求响应 schema） | 地基 | 待填（沿用 Phase 31.2 surface parity 报告） | ☐ PENDING |
| 2 | gRPC / proto 接口与消息 | 地基 | 待填（沿用 31.2 pinned protoc + 唯一归属清单） | ☐ PENDING |
| 3 | Tenant 上下文签名（actor / org 在 HTTP / gRPC / job / retry / vector / analytics 的传递类型） | 地基 | 待填（Go 类型路径 + 函数签名） | ☐ PENDING |
| 4 | Relay client 接口（Complete / Stream / Embed + usage / quota / 价格快照语义） | 地基 | 待填（Go interface 全路径） | ☐ PENDING |
| 5 | 事件 schema（topic / payload，如使用异步事件） | 地基 | 待填（若无异步事件填「不适用」） | ☐ PENDING |
| 6 | DB schema 归属（表 → track 映射） | 地基 | 待填（表清单 + 归属 track） | ☐ PENDING |

## 冻结判据

- 每项须指向具体文件 / 符号（不是描述），并有对应 E1 / E2 证据。
- 第 3、4 项须被至少一条产品轨的最小竖切实际消费一次，证明接口可用。
- 全部 6 项 `LOCKED` 后，在本文件顶部记录：

```
Contract Lock #1 achieved: <日期> / commit <sha>
```

## 修订记录

> 每次走 §3.4 修订协议后在此追加一行：`<日期> <修改项#> <变更摘要> <影响轨道>`

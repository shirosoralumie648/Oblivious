# M2-A Knowledge Beta Closeout Design

日期：2026-04-07

## 1. Background

`M0 Baseline Freeze Closeout` 与 `M1 Mainline Runnable Closeout` 已经完成，当前主线具备以下事实：

- root `check` / `test` / CI 入口已冻结并可稳定运行
- 工作区主路径与 Console 主路径已达到可构建、可回归、可验收状态
- Knowledge 页面已经具备知识库 CRUD、文档 CRUD 与 `/retrieve` 检索入口
- 后端 `knowledge` store / service / handler 已具备最小 retrieval MVP

但当前 Knowledge 模块仍停留在“主线可运行”而不是“工作区 Beta 可依赖”的状态，主要原因是：

- retrieval 结果质量尚未正式收口
- snippet 呈现和命中排序仍属于实现细节驱动，而不是用户体验驱动
- CRUD 与 retrieval 的状态切换仍可能相互干扰
- 文档尚未把 Knowledge 的当前 Beta 边界写清楚

因此，`M2` 不能整体一口气推进，而应先按子项目顺序执行。第一个子项目定义为：

> **M2-A：Knowledge Beta Closeout**

## 2. Goal

在不改变公开接口 shape 的前提下，把当前 Knowledge CRUD 与 retrieval 链路收口为一个用户可见、可回归、可演示、可文档化的 Beta 子模块。

## 3. Non-Goals

本子项目明确不做以下内容：

- 不新增任何 Knowledge 公开接口
- 不改变 `/api/v1/app/knowledge-bases/...` 现有请求/响应 shape
- 不引入向量检索、embedding、多源导入或异步 ingestion pipeline
- 不把 retrieval 升级为 `M3` 级能力平台
- 不推进 SOLO runtime、Chat gateway、streaming 等非 Knowledge 能力
- 不把 `KnowledgePage` 改造成大规模前端重构项目

## 4. Scope

本子项目范围收敛为四组交付。

### 4.1 Backend Retrieval Quality Closeout

需要交付：

- retrieval 结果排序更符合当前文本命中直觉
- snippet 生成更稳定、更可解释
- 空结果与错误路径语义统一

### 4.2 Knowledge Workspace Beta Closeout

需要交付：

- knowledge base CRUD 用户流稳定
- document CRUD 用户流稳定
- retrieval 区与 CRUD 区切换不互相打架
- 与 Chat 的回跳关系清晰

### 4.3 Beta Regression Closeout

需要交付：

- CRUD 与 retrieval 关键路径测试补齐
- 前后端两侧都具备可支撑 Beta 结论的回归用例

### 4.4 Documentation Closeout

需要交付：

- 当前 Knowledge Beta 能力边界说明
- 当前 retrieval 的真实定位说明
- 与 `M3` 能力边界的清晰分隔

## 5. Technical Decisions

### 5.1 Keep Public Interface Stable

本子项目允许改动：

- `knowledge` store
- `knowledge` service
- `http/knowledge_handler` 的内部处理细节
- `KnowledgePage` 的页面组织与状态收口

但不允许改动：

- path
- request/response JSON shape
- Knowledge API 的前端调用签名

### 5.2 Retrieval Optimization Is Internal, Not Architectural

本次 retrieval 质量优化只允许发生在现有实现边界内：

- chunk 命中排序
- snippet 选取策略
- 文本命中优先级
- 空结果 / 错误结果的表达

目标是让当前 `/retrieve` 更像一个可依赖的 Beta 能力，而不是顺势扩展成新的检索平台。

### 5.3 Page Closeout Over Page Rewrite

前端这次应优先收口 `KnowledgePage` 的用户流，而不是追求组件重构完整性。

允许：

- 抽出少量 section / helper / presenter
- 调整局部状态组合方式
- 优化 CRUD 与 retrieval 间的页面状态切换

不建议：

- 围绕“代码洁癖”做整页重构
- 把当前页面拆成新的复杂前端系统

### 5.4 Beta Quality Is User-Facing Quality

这次“结果质量优化”的判断标准不是更复杂的算法，而是用户是否能理解和依赖结果：

- 更相关的结果排前面
- snippet 能解释为什么命中
- 无结果时反馈明确
- 查询失败时状态稳定
- CRUD 后再次检索，结果与页面状态保持一致

## 6. File Shape

### 6.1 Backend Knowledge Slice

预计涉及：

- `src/server/internal/knowledge/store.go`
- `src/server/internal/knowledge/service.go`
- `src/server/internal/http/knowledge_handler.go`
- `src/server/internal/knowledge/service_test.go`
- `src/server/internal/http/knowledge_handler_test.go`

### 6.2 Frontend Knowledge Slice

预计涉及：

- `src/web/src/routes/workspace/KnowledgePage.tsx`
- `src/web/src/routes/workspace/KnowledgePage.test.tsx`
- 如确有必要，少量 `src/web/src/features/knowledge/*` 文件

### 6.3 Documentation Slice

预计涉及：

- `docs/architecture/current-system-contracts.md`
- 如有必要，一份单独的 Knowledge Beta 说明文档

## 7. Implementation Shape

本子项目实现顺序固定为：

1. 先后端 retrieval 质量收敛
2. 再前端 Knowledge Beta 页面收口
3. 再补 CRUD + retrieval 回归
4. 最后更新文档

这样可以确保前端不是在不稳定的 retrieval 结果上反复调 UI，也避免文档先于真实实现给出错误承诺。

## 8. Risks And Mitigations

### 8.1 M3 Scope Leakage Risk

风险：

- retrieval 优化过程中顺手引入更重的检索能力，导致 `M2-A` 穿透到 `M3`

处理：

- 严格限制为现有接口 shape 下的内部优化
- 不新增新接口，不引入新基础设施依赖

### 8.2 State Interference Risk

风险：

- CRUD、详情、编辑器与 retrieval 状态互相污染

处理：

- 明确把“CRUD 后再检索”“切换知识库后检索”“从 Chat 回跳后继续操作”纳入回归范围

### 8.3 Over-Refactor Risk

风险：

- 为了把 `KnowledgePage` 写得更优雅，最终把收口任务做成页面重构

处理：

- 只允许做支撑 Beta 闭环所需的最小结构调整
- 若拆分组件，必须服务于当前复杂状态收口，而不是服务于抽象本身

### 8.4 Documentation Drift Risk

风险：

- Knowledge 代码已经进入 Beta，文档还停留在“可运行但未定型”

处理：

- 把文档更新纳入完成条件，而不是作为后续补充项

## 9. Verification Model

本子项目采用三层验证。

### 9.1 Backend Verification

必须证明：

- Knowledge CRUD 测试继续通过
- retrieval 相关测试覆盖排序、snippet、空结果或失败语义

### 9.2 Frontend Verification

必须证明：

- Knowledge 页面相关前端测试通过
- CRUD + retrieval 用户流可在页面层回归
- 与 Chat 回跳相关路径不被破坏

### 9.3 Root Verification

必须通过：

- `bash scripts/check.sh`
- `bash scripts/test.sh`

## 10. Completion Criteria

当以下条件同时满足时，本子项目完成：

1. Knowledge CRUD 用户流完整跑通
2. retrieval 用户流完整跑通
3. retrieval 结果质量优化可被测试和演示证明
4. 前端相关测试通过
5. 后端相关测试通过
6. root `check/test` 继续通过
7. 文档对当前 Knowledge Beta 现实有清晰说明
8. 可以明确把 Knowledge 模块从“主线可运行”升级到“Beta 可依赖”

## 11. Recommended Outputs

本设计推荐在实现阶段优先落地以下输出：

1. 后端 retrieval 质量优化
2. Knowledge 页面 Beta 收口
3. CRUD + retrieval 回归补齐
4. Knowledge Beta 文档说明

这样可以先把返回质量收稳，再把页面和文档收成真正可依赖的 Beta 状态。

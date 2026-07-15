# Phase 31 发布合同、动态 Readiness 与契约一致性设计

**状态:** 用户确认
**日期:** 2026-07-15
**适用需求:** RELS-01、RELS-02
**证据上限:** repository-local E1/E2

## 1. 目标

Phase 31 系列建立一个可执行的发布合同，使发布运营者能够准确回答：

1. 当前版本承诺哪些 capability 和 deployment profile。
2. 当前运行环境中哪些适用 capability 实际可用、禁用或阻断。
3. OpenAPI、runtime routes、frontend transports、protobuf、migration 与当前代码是否一致。
4. 每份报告是否来自同一个 clean source commit、contract digest 和显式 deployment profile。

本设计保留 `31-CONTEXT.md` 中 D-01 至 D-16 的全部语义。它不证明 E3 target evidence、E4 same-commit release、microservices parity、供应链签名或后续业务生命周期完整性。

## 2. 阶段拆分

原 Phase 31 横跨构建身份、运行时授权、前端、protobuf、migration、部署和 CI，已经膨胀为 22 个相互耦合的计划。为减少跨计划矛盾，将其拆成三个连续阶段：

| 阶段 | 名称 | 主要交付 | 需求映射 | 目标计划数 |
|---|---|---|---|---:|
| 31 | 发布合同与可信构建身份 | authored contract、derived build identity、统一报告协议、excluded profile 操作合同 | RELS-01 foundation | 5-6 |
| 31.1 | 动态 Readiness 与持续 Fail-Closed | in-process manager、周期刷新、side-effect guards、Admin/app views、部署接线 | RELS-01 completion | 6-8 |
| 31.2 | 契约表面一致性与聚合门禁 | HTTP/frontend/protobuf/migration parity、aggregate、CI 和 operator docs | RELS-02 completion | 8-10 |

依赖顺序固定为：

```text
31 -> 31.1 -> 31.2 -> 32
```

Phase 32 不得在 31.2 完成前启动。拆分只改变交付单元，不删除或推迟 D-01 至 D-16。

## 3. 权威与身份

### 3.1 Authored contract

`config/release/contract.v1.json` 是唯一 authored machine authority，schema 位于 `config/release/contract.schema.json`。合同包含：

- capability domain 和 lifecycle slice IDs；
- commitment、profile default policy、reason codes；
- deployment profiles、topology、entrypoints、dependencies、state stores；
- capability overrides；
- typed migrate/deploy/rollback refs；
- catalog bindings；
- surface references；
- readiness requirements。

合同不得保存自身所属的 Git commit，也不得保存自己的最终 digest。`releaseCommit` 和 `contractDigest` 都是 derived identity，防止自引用。

### 3.2 BuildIdentityV1

可信构建身份定义为：

```json
{
  "schemaVersion": "build-identity/v1",
  "releaseCommit": "<40-hex Git commit>",
  "sourceTree": "<40-hex Git tree>",
  "contractDigest": "sha256:<hex>",
  "dirty": false,
  "evidenceClass": "repository-local"
}
```

身份来源规则：

- 本地和 CI 均以 `git rev-parse HEAD^{commit}` 与 `HEAD^{tree}` 为源码权威。
- CI 的 `GITHUB_SHA` 只做一致性检查，不能覆盖 Git 结果。
- clean build 将 commit、tree 和 contract digest 注入所有 Go binaries，并写入 OCI labels。
- runtime 重新计算镜像内合同 digest，并与 binary identity、OCI label 比较。
- caller、环境变量或普通 CLI 参数不能提供或覆盖 release commit。
- dirty checkout 可以运行单元测试和诊断，但 identity-bearing gate 返回 `source_worktree_dirty`，不能形成发布通过结论。
- production 中缺失、unknown、dirty 或不匹配身份必须在 DB/migration 前 fail closed。

Phase 39 仍负责 signature、provenance、immutable image digest 和 E4 same-commit proof。

## 4. SurfaceReportV1

所有 drift producers 使用同一个嵌套 envelope，不再使用扁平十四字段结构：

```json
{
  "schemaVersion": "surface-report/v1",
  "releaseIdentity": {
    "releaseCommit": "<commit>",
    "sourceTree": "<tree>",
    "contractDigest": "sha256:<hex>",
    "deploymentProfile": "monolith",
    "dirty": false,
    "evidenceClass": "repository-local"
  },
  "surfaceIdentity": {
    "surface": "http",
    "canonicalSource": "docs/api/openapi.yaml",
    "consumer": "runtime-route-registry",
    "version": "v1",
    "sourceDigest": "sha256:<hex>",
    "consumerDigest": "sha256:<hex>"
  },
  "drift": {
    "missing": [],
    "extra": [],
    "incompatible": []
  },
  "evidence": {
    "class": "repository-local",
    "environment": "ci",
    "mode": "static",
    "checkedAt": "<RFC3339Nano>",
    "toolVersions": {},
    "details": {}
  },
  "outcome": {
    "result": "pass",
    "errorCodes": [],
    "skippedChecks": []
  }
}
```

约束如下：

- `releaseIdentity` 只能由 trusted identity provider 生成。
- `drift` 只描述 missing、extra 和 incompatible，不承载环境或执行模式。
- migration environment、migration mode、ledger digest 和工具版本进入 typed `evidence`。
- `evidence.details` 按 surface-specific allowlist/schema 验证，不接受任意未声明字段。
- surface schema registry 至少固定以下 details：HTTP/frontend 的 operation 与 transport counts，protobuf 的 toolchain manifest digest，migration 的 ledger digest、database kind 和 replay mode，readiness 的 generation、checkedAt 和 validUntil，deployment 的 binary/OCI/workload identity。adapter 不得自行增加未注册字段。
- committed surface 的非空 `skippedChecks` 一律失败。
- producer 失败时尽可能原子写入带可信身份的 failure report，并保留原始 producer exit status。
- shared writer 创建缺失的 output parent，使用同目录临时文件、flush/fsync 和 atomic replace；不可写路径返回 `report_output_unwritable`，不得留下 partial/temp 文件。

## 5. 动态 Readiness

### 5.1 唯一运行授权源

`ReadinessManager` 是 runtime 唯一授权源。JSON 文件只是只写审计快照，server 启动和 side-effect guards 不读取旧快照做授权。

核心接口：

```go
type ReadinessManager interface {
    Bootstrap(context.Context) error
    StartRefresh(context.Context)
    Require(capabilityID string) error
    Evaluate() Evaluation
    ExportAudit(path string) error
}
```

`Require` 内部使用一个 Go evaluator 完成 build identity、deployment profile、snapshot generation、freshness、availability 和 reason code 判断。HTTP、worker 或 CLI 不得实现自己的时间规则。

### 5.2 时间合同

monolith 首发 profile 的 authored readiness 参数为：

```json
{
  "refreshIntervalSeconds": 30,
  "maxAgeSeconds": 120,
  "allowedFutureSkewSeconds": 30
}
```

- bootstrap 后每 30 秒执行 bounded probes。
- `validUntil` 从最老适用 observation 加 120 秒推导，不能由文件、环境或 caller 提供。
- 时间比较使用 UTC epoch nanoseconds；边界相等通过，超过一纳秒失败。
- refresh 失败时可以保留旧 generation，但不能延长 `validUntil`。
- 审计聚合器调用 Go inspector 验证 snapshot 并生成 `SurfaceReportV1`。Python 不重复实现 freshness，因此不存在 Go/Python 时间精度分叉。

### 5.3 启动顺序

固定启动顺序：

```text
contract/profile/build validation
  -> DB open and ping
  -> migration apply/ledger validation
  -> synchronous readiness bootstrap probes
  -> publish generation 1
  -> listener and control plane
  -> workers and periodic refresh
```

profile 必须显式提供。`defaultProfile` 仅是 authored manifest metadata，不能替代缺失的 runtime/deploy input。

DB 或 migration 失败仍是 fatal。其他 dependency 失败生成 `blocked` observation；server 可以保留 `/livez` 和 operator inspection，但 `/readyz`、release gate 和对应的新 side effect 必须失败。

### 5.4 持续 enforcement

同一个 manager 必须在下列边界执行 `Require(capabilityID)`：

- HTTP 和 gRPC dispatch；
- worker claim 前及每个 job 的不可逆 effect 前；
- Provider、model、tool 和外部 channel 调用前；
- checkout、refund、payout、settlement 等财务 dispatch 前；
- standalone service entrypoint 和 operation tool 执行前。

snapshot 过期后，已经开始且可安全收尾的工作按现有幂等/取消语义完成，但不得开始新的 side effect。`excluded` 不注册、不 probe；`disabled` 和 `blocked` 都拒绝。任何适用 capability 的 `blocked` 状态都阻断发布。

## 6. Catalog capability authority

合同维护 `catalogBindings`，将固定 model ID、builtin tool ID 及严格的 custom/MCP runtime class 映射到 capability ID。

服务端接口语义：

```go
Resolve(CatalogSubject{Kind, ID, Runtime}) (CapabilityID, error)
RequireEnabled(ctx, CapabilityID, Boundary) error
```

- response DTO 可以返回只读 `capabilityId`。
- create/update 请求不得接受 caller-supplied capability ID。
- Chat config、Agent model/tool、routing target、持久化旧数据和 execution path 均重新解析并授权。
- frontend selector 只负责呈现，server mutation/execution 才是最终安全边界。

## 7. Contract surfaces

### 7.1 HTTP/OpenAPI/runtime

- 每个 OpenAPI operation 携带稳定 capability ID。
- method、path、security、request/response schema、media type 与 runtime registry 双向比较。
- excluded surface 不注册，disabled 返回 `capability_disabled`，blocked 返回 `capability_blocked`。
- Admin full inventory 与 authenticated app-safe projection 使用不同 DTO 和权限边界。

### 7.2 Frontend AST inventory

一个 TypeScript compiler extractor 递归扫描 `src/web/src/**/*.{ts,tsx}` 的生产模块并输出结构化 sidecar。明确排除：

- `*.test.*`、`*.spec.*`；
- `__tests__`、test support、fixtures、mocks、snapshots；
- generated 文件作为 stale consumer 检查，但不作为 transport call source。

fingerprint 至少包含：

- protocol 和 transport kind；
- method/path 与 capability ID；
- request media type、encoder 和 schema；
- response media type、decoder 和 schema；
- SSE/WebSocket event identity。

transport verifier 与 navigation/product exposure verifier 共用该 sidecar。新增、动态不可解析或未分类 transport 必须失败。负向调用样例位于独立 fixture source root，不污染 production scan。

### 7.3 Markdown decoder

`/api/v1/app/conversations/{conversationId}/export.md` 保持 `text/markdown`。frontend shared client 新增 typed text decoder：

- 发送 `Accept: text/markdown`；
- 接受带 charset 的 `Content-Type`；
- 调用 `response.text()`；
- 不通过 JSON envelope parser。

测试必须经过真实 shared client decoder，不能只 mock `get()`。

### 7.4 Protobuf

仓库维护固定工具链 manifest 和 checksum：

- protoc 25.1；
- protoc-gen-go 1.36.11；
- protoc-gen-go-grpc 1.6.2。

bootstrap 安装到 `.tmp/protobuf-tools/bin`，CI cache key 包含 tool manifest digest。release-gates job 在执行 protobuf drift gate 前验证精确版本。每个 tracked proto source 和 generated output 必须在 disposition map 中唯一归属。

### 7.5 Migration

migration 分成三个独立事实：

- numbered SQL immutable inventory/checksum；
- runtime ledger identity；
- committed monolith replay result。

replay 无可用 DB/Docker 时返回明确失败，不得将 skip 计为通过。environment、mode 和 ledger 信息进入 `evidence`。

## 8. Deployment and local harness

- `Dockerfile.server` 打包 release contract，并向所有 active Go binaries 注入相同 build identity。
- OCI labels 与 binary identity、packaged contract digest 必须一致。
- Docker Compose 显式设置 `monolith`，不预挂载授权 snapshot；仅为 audit export 提供可写目录。
- `deploy/kubernetes/app-deployment.yaml` 是本阶段唯一 canonical Kubernetes workload，显式设置 profile，挂载 audit `emptyDir`，并区分 `/livez` 与 `/readyz`。`deploy/kubernetes/server.yaml` 必须改为由 canonical workload 生成的兼容产物，或从 release validation inventory 移除，不能继续作为第二份权威。
- `cmd/release-readiness` 属于 operation/inspection tool，在动态 command inventory 中单独分类；它不能作为普通 runtime service，也不依赖预存在 readiness 文件。
- repository-local disposable harness 构建镜像、迁移、启动、等待 live/ready、读取 runtime audit、核对 binary/OCI/contract identity，并强制清理临时 Compose project 和 volume。

该 harness 只形成 E2 repository-local evidence，不冒充 target deployment proof。

## 9. Aggregate ownership

`scripts/verify-quality-gates.sh` 是唯一直接调用 aggregate 的父 gate：

```text
verify-quality-gates.sh -> verify-release-contract.sh
check.sh docs/all -> verify-quality-gates.sh
verify-commercial-completion.sh -> check.sh docs
```

其他父脚本只做传递接线和 wiring assertion，不能再次直接调用 aggregate。aggregate 本身不得调用 check、test、quality 或 commercial，避免 recursion 和 migration replay 重复执行。

最终 identity-bearing gate 只能在提交后的 clean checkout 或 CI 中运行。tracked docs 不记录当前 exact commit 作为“最新通过证据”，避免提交后身份再次变化。完整机器报告保存在 `.tmp` 或 CI artifacts。

## 10. 稳定错误类

至少固定以下错误码：

- `build_identity_missing`
- `build_identity_mismatch`
- `source_worktree_dirty`
- `contract_digest_mismatch`
- `profile_required`
- `profile_excluded`
- `readiness_unavailable`
- `readiness_stale`
- `capability_unknown`
- `capability_disabled`
- `capability_blocked`
- `surface_schema_invalid`
- `surface_drift`
- `report_output_unwritable`
- `toolchain_mismatch`
- `migration_replay_unavailable`

公开错误不得泄露 secrets、credentials、内部地址或原始 target evidence。

## 11. 验证策略

每个计划保持 2-3 个任务，目标修改 5-8 个文件，硬上限 9 个文件；单任务不得达到 10 个文件。同 wave 不得存在文件写重叠。

核心验证矩阵：

| 风险 | 必需证明 |
|---|---|
| 身份拼接 | clean/dirty、commit/tree/digest/profile substitution、OCI/binary/contract mismatch |
| Readiness | bootstrap、refresh、expiry、future skew、generation、blocked transitions、audit export |
| Side effects | HTTP/gRPC、worker claim/effect、Provider/tool、financial zero-call denial |
| Frontend | production source scope、all transports、media/decoder drift、negative fixture root |
| Catalog | server-derived capability ID、mutation rejection、old persisted data execution denial |
| Protobuf | pinned bootstrap、version mismatch、complete source/output map、stale generation |
| Migration | immutable SQL、ledger mismatch、real replay、unavailable environment failure |
| Aggregation | nested schema、identity splice、skip rejection、single owner、atomic output |

每个切片执行 TDD、相关测试、`git diff --check`、原子 commit，并在验证通过后 push。并行实现只能使用隔离 worktree；共享分支上的 merge 和 push 必须串行。

identity-bearing 切片采用固定的两段式验证：

1. 修改期间在临时 clean Git fixture 中验证 resolver、schema 和 producer 行为，并运行不依赖真实 commit identity 的功能测试。
2. 原子 commit 后确认工作树 clean，再针对真实 `HEAD` 运行 build/runtime identity gate；通过后才 push。失败则继续修复并生成新的本地 commit，未通过的 commit 不 push。

这避免开发中的正常 dirty 状态被误当成发布身份，同时确保最终报告确实绑定被 push 的 commit。

## 12. 现有规划迁移

当前 22 个 `31-*-PLAN.md` 尚未提交且未通过独立 checker，不作为后续计划基础。设计审查完成后：

1. 删除这些未提交的旧计划产物。
2. 恢复其对 ROADMAP、VALIDATION 和临时 auto-chain 配置的修改。
3. 在 ROADMAP 中插入 31.1 和 31.2，并更新 Phase 32 dependency。
4. 为三个阶段分别生成 CONTEXT/VALIDATION/PLAN，所有计划引用本设计。
5. 每个阶段独立通过 requirements、decision coverage、Nyquist 和 plan checker 后才执行。

不得删除已提交的 `31-CONTEXT.md`、`31-RESEARCH.md`、`31-PATTERNS.md` 或历史 Phase 01-30 证据。

## 13. 备选方案与拒绝原因

### 保持单个 Phase 31

优点是无需修改 roadmap；缺点是约 20 个计划同时共享 identity、readiness 和 surface contracts，已经连续造成 checker finding 增长。拒绝。

### 缩成静态 manifest

实现较快，但违反 D-02、D-04、D-15 和 RELS-02，不能保证 runtime 或 client drift 被阻断。拒绝。

### 外置签发 release envelope

可以解决自引用，但增加签发、挂载和 provenance 信任边界。更适合 Phase 39，不作为 Phase 31 基础。

## 14. 完成定义

Phase 31 系列完成时必须同时满足：

1. RELS-01 在 31.1 关闭，RELS-02 在 31.2 关闭。
2. 任何 identity、readiness、surface、toolchain 或 migration drift 都使 gate 非零退出。
3. excluded capability/profile 不被默认广告或执行。
4. blocked/expired capability 不能开始新的 side effect。
5. release operator 可以从机器和人类视图区分 authored commitment、current availability 和 evidence class。
6. 所有结论明确限定为 E1/E2，后续 Phase 32-39 范围保持不变。

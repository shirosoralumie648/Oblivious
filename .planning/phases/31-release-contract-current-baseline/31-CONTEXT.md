# Phase 31: 发布合同与可信构建身份 - Context

**Gathered:** 2026-07-15
**Refined after phase split:** 2026-07-16
**Status:** Ready for foundation planning

<domain>
## Phase Boundary

Phase 31 只交付拆分设计中的 foundation：一个 schema-validated authored release contract、由 clean Git source 和 contract digest 派生的 `BuildIdentityV1`、供后续 producer 共用的嵌套 `SurfaceReportV1` 与可信原子写入协议，以及 excluded deployment profile 的无副作用 operation contract。

本阶段是 `RELS-01` 的 foundation contribution，不能单独关闭 `RELS-01`，也不承担 `RELS-02`。它不实现动态 `ReadinessManager`、runtime side-effect guards、Admin/app projection、HTTP/frontend/protobuf/migration parity、aggregate gate wiring、target evidence 或供应链签名。

### Umbrella Discussion And Split History

- 原 Phase 31 讨论覆盖了 authored commitment、动态 availability、surface parity 和 release evidence；批准设计确认这些语义继续有效，但按依赖拆成 `31 -> 31.1 -> 31.2`。
- `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md` 是拆分后的批准设计。当前阶段只实现其中第 3、4、8、9、11 节与 foundation 直接相关的部分。
- `31-DISCUSSION-LOG.md` 是原 umbrella discussion 的审计记录，必须原样保留；它不作为扩大当前 Phase 31 实现范围的依据。
- Phase 31.1 和 31.2 的行为只记录在本文的 routing/deferred 部分，不得混入本阶段的 trackable decisions。

</domain>

<decisions>
## Implementation Decisions

### Authored Contract Authority

- **D-FND-01:** `config/release/contract.v1.json` 是唯一 authored machine authority，`config/release/contract.schema.json` 是其 strict schema。loader 必须拒绝 unknown fields、trailing JSON、未知 enum、重复 ID、断裂 reference 和不满足语义约束的 profile；文档、环境变量、部署资产或派生报告都不能覆盖它。
- **D-FND-02:** capability ID 采用稳定的“domain + lifecycle slice”语义。合同分别表达 `commitment: committed | conditional | excluded`、profile default policy 和稳定 reason-code catalog；动态 `availability` 不写回 authored contract，也不在本阶段计算。
- **D-FND-03:** runtime deployment profile 必须由调用方显式选择；authored `defaultProfile` 只是发布元数据，不能替代缺失输入。当前只有 `monolith` 是 committed/default，`microservices`、`dual` 和 `split` 保持 excluded，仓库中已有资产不能隐式晋级 profile。
- **D-FND-04:** 每个 profile 必须声明 topology、entrypoints、dependencies、state stores、capability overrides、typed operations、catalog bindings、surface references 和 readiness requirements。Phase 31 校验这些字段及引用完整性，但不执行 readiness probe、catalog runtime enforcement 或 surface parity。
- **D-FND-05:** `migrate`、`deploy`、`rollback` 都使用 profile-bound `OperationRef { profileId, path, argv }`。`path` 必须是 allowlisted repo-relative path，规范化后不得为 absolute、不得逃逸仓库；`argv` 是参数数组，禁止 shell 字符串拼接。excluded profile 的 dispatch 必须在启动子进程前以稳定非零错误拒绝，并证明零副作用。
- **D-FND-06:** authored contract 不保存自身 Git commit、source tree 或最终 digest。`contractDigest` 只能由严格校验后的 canonical contract bytes 计算为 `sha256:<hex>`，避免自引用和格式相关歧义。

### Trusted Build And Report Identity

- **D-FND-07:** `BuildIdentityV1` 只由 clean repository 的 `HEAD^{commit}`、`HEAD^{tree}` 和 canonical contract digest 派生；commit/tree 必须为 40 位 lowercase hex，`dirty` 必须为 `false`，evidence class 固定为 `repository-local`。dirty、missing、unknown 或 mismatch identity 都必须稳定失败。
- **D-FND-08:** runtime caller、环境变量和普通 CLI 参数不得提供或覆盖 release identity。所有 active Go binaries、OCI labels 和 packaged contract 必须绑定同一 identity，并由 foundation build/inspection proof 检测任一不一致。
- **D-FND-09:** 所有后续 build/readiness/deployment/surface producer 必须共用嵌套 `SurfaceReportV1`：顶层只包含 `schemaVersion`、`releaseIdentity`、`surfaceIdentity`、`drift`、`evidence` 和 `outcome`。`drift` 只含 `missing`、`extra`、`incompatible`；环境、模式、工具版本属于 `evidence`，错误码和 skipped checks 属于 `outcome`。Phase 31 交付 typed `evidence.details` registry API，并注册/产出 surface ID 为 `build-identity` 的 foundation report；后续阶段只注册自己拥有的 details schema，不能重建 registry 或 envelope。
- **D-FND-10:** `releaseIdentity` 只能来自 shared trusted identity provider，producer 不得接受 caller-supplied identity fields。shared writer 必须先验证 report，再以同目录临时文件、flush/fsync 和 atomic replace 写入；producer 失败时保留原 exit status，现有有效报告不得被 partial output 或失败写入破坏。

### Verification And Claim Boundary

- **D-FND-11:** identity-bearing work 固定采用两段验证：实现期间在临时 clean Git fixture 中验证 schema/resolver/producer；原子 commit 后只在真实 clean `HEAD` 上运行 binary/OCI/packaged-contract identity gate。最终 clean gate 通过前不得 push，开发中 dirty worktree 不能伪装成发布通过。
- **D-FND-12:** Phase 31 的证据上限是 repository-local E1/E2。它只形成 `RELS-01 foundation contribution`，不得声明 runtime readiness、`RELS-01` completion、`RELS-02` parity、target/live proof、immutable image、signature、provenance 或 E3/E4 release readiness。

### the agent's Discretion

- 在不改变上述字段语义和 ownership 的前提下，planner 可以决定 Go 文件的内部拆分、canonical encoder 的具体实现及 fixture 数据组织。
- 稳定错误码可在批准设计第 10 节的 catalog 内细化，但 identity/profile/contract/report failure 不得退化为自由文本或零退出。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Approved Scope And Requirements

- `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md` - 用户批准的三阶段拆分、identity/report contracts、验证边界和稳定错误类。
- `.planning/ROADMAP.md` - Phase 31 的四项 success criteria，以及 31.1、31.2 和 39 的固定职责边界。
- `.planning/REQUIREMENTS.md` - `RELS-01` 由 Phase 31 与 31.1 共同承担，`RELS-02` 只属于 Phase 31.2。
- `.planning/PROJECT.md` - mainline、security、delivery 和 claim-discipline 约束。
- `.planning/STATE.md` - 当前阶段、拆分历史和未完成 blocker。
- `.planning/phases/31-release-contract-current-baseline/31-DISCUSSION-LOG.md` - 原 umbrella discussion 的保留审计记录；只用于追溯，不改变拆分后的 scope。

### Foundation Runtime And Build Anchors

- `Dockerfile.server` - 当前三个 active Go binaries 只使用 `-ldflags="-s -w"`，尚未注入 identity 或打包 release contract。
- `.dockerignore` - `.git` 被排除，build identity 必须在 Docker build 外从 clean Git 派生，再作为受验证 build input 注入。
- `src/server/cmd/server/main.go` - 当前 `config.Load()` 后立即打开 DB；Phase 31 提供 trusted contract/build primitives，实际 startup ordering 属于 Phase 31.1。
- `src/server/internal/migrations/migrations.go` - deterministic sorting 与 SHA-256 checksum 的可复用实现类比。
- `scripts/verify-target-release-evidence.sh` - 可参考 live Git commit 读取，但其 mismatch escape hatch 不得进入 foundation identity path。
- `docker-compose.yml` - 默认 monolith 与显式 microservices profile 的当前资产事实；资产存在不代表 commitment。

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `src/server/internal/migrations.Checksum` 与 `LoadFiles`: 已有 SHA-256、稳定排序和 checksum mismatch fail-closed 模式，可用于设计 contract digest 的测试类比，但不能直接把 migration checksum 格式冒充 JSON canonicalization。
- `scripts/verify-target-release-evidence.sh`: 已有 `git rev-parse HEAD` 和 clean/source comparison 经验；foundation resolver 必须移除 caller override 与 mismatch bypass。
- 现有 shell verifier 普遍使用 `set -euo pipefail`，适合作为窄 wrapper；结构化校验和 identity derivation 应由 typed Go code 负责。

### Missing Foundation Assets

- `config/release/`、`src/server/internal/releasecontract/` 和 `src/server/internal/buildinfo/` 当前不存在。
- 仓库当前没有 shared `SurfaceReportV1` schema/trusted writer，也没有 profile-safe `OperationRef` dispatcher。
- `scripts/run-go-tests-matched.sh` 当前不存在；直接使用可能零匹配仍成功的 `go test -run` 不能成为 Phase 31 证据。

### Integration Points

- `Dockerfile.server` 需要让 server、migrate 和 grpc-smoke 共享同一 injected identity，并把 canonical contract 带入 runtime image。
- foundation CLI/verifier 负责 contract validation、identity derivation/inspection、operation dispatch 和 report validation；它不得提前承担 31.1 的 runtime manager 或 31.2 的 surface aggregate。
- Phase 31.1 将 trusted contract/build provider 接入 `cmd/server` startup 和 side-effect guards；Phase 31.2 将 shared report contract 接入各 surface producer 与唯一 aggregate owner。

</code_context>

<specifics>
## Specific Interfaces

### BuildIdentityV1

```json
{
  "schemaVersion": "build-identity/v1",
  "releaseCommit": "<40-hex>",
  "sourceTree": "<40-hex>",
  "contractDigest": "sha256:<hex>",
  "dirty": false,
  "evidenceClass": "repository-local"
}
```

### SurfaceReportV1 Top Level

```text
schemaVersion
releaseIdentity
surfaceIdentity
drift { missing, extra, incompatible }
evidence
outcome { result, errorCodes, skippedChecks }
```

### OperationRef

```text
{ profileId, path, argv }
```

</specifics>

<deferred>
## Deferred And Routing

### Phase 31.1 - Dynamic Readiness And Continuous Fail-Closed

- `ReadinessManager`, bootstrap/refresh/freshness/generation, dependency probes and audit snapshot authorization.
- Startup ordering, side-effect guards, Admin full inventory, app-safe projection, catalog runtime enforcement, `/livez`, `/readyz`, Compose and Kubernetes runtime wiring.

### Phase 31.2 - Surface Parity And Aggregate Gates

- HTTP/OpenAPI/runtime route parity, frontend AST transport inventory and markdown decoder, protobuf generation parity, migration inventory/ledger/replay evidence.
- Surface-specific producers, aggregate/gate wiring, identity-splice/skip rejection and operator-facing contract documentation.

### Phase 39 - Supply Chain And Target Release

- Signature, SBOM, provenance, reproducible immutable image digest, target E3 evidence and same-commit E4 no-skip proof.

</deferred>

---

*Phase: 31-release-contract-current-baseline*
*Context refined: 2026-07-16*

# Phase 31: 发布合同与当前基线 - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

本阶段只交付 `RELS-01` 和 `RELS-02`：建立一个版本化、可校验、机器可读的发布合同，使发布运营者能够确认本次发布承诺的能力切片、集成、部署 profile 和当前 runtime 契约；同时建立 OpenAPI、protobuf、migration、前端 client 与 runtime 的漂移阻断机制。

本阶段不补齐各业务能力生命周期，不证明真实客户旅程、部署 profile parity、E3 target evidence、E4 same-commit no-skip 证据或供应链制品。这些工作仍由 Phase 32-39 负责。历史 Phase 01-30 产物只能作为 E1/E2 基线，不能自动把本阶段的承诺或 readiness 标记为已证明。

</domain>

<decisions>
## Implementation Decisions

### 能力粒度与状态

- **D-01:** capability manifest 使用“领域 + 生命周期切片”两层模型。顶层领域保持稳定，例如 Identity、Relay、Chat、Knowledge、Automation、Billing、Marketplace、Admin、Operations；可独立失败、禁用或验收的生命周期行为必须拆成稳定 capability ID，不能用整个模块的单一状态掩盖 streaming、cancel、replay、refund、payout 等缺口，也不能把逐 route 存在当作能力完整。
- **D-02:** 每个 capability 分开表达商业承诺与运行可用性。发布合同记录 `commitment` 和 profile 下的默认可用策略；runtime readiness report 记录当前 `availability`，两者不得合并成一个含义模糊的状态。
- **D-03:** 固定状态集合为 `commitment: committed | conditional | excluded` 和 `availability: enabled | disabled | blocked`。`blocked` 表示按合同或当前 profile 本应可用，但必要依赖或 readiness 未通过，必须阻断发布。任何非 `enabled` 状态都必须带稳定、结构化的 `reasonCode`；未知状态和值必须校验失败。
- **D-04:** release manifest 是随版本固定的承诺快照，不由运行时健康变化改写。runtime readiness report 是动态观测，两者通过 release identity、deployment profile 和 capability ID 关联。运营者必须能同时看到“本版本承诺什么”和“当前环境实际是否就绪”。

### 首发部署模式

- **D-05:** 当前发布合同只把 `monolith` 作为 committed deployment profile。`docker-compose.yml` 的默认 `oblivious-server` 和当前主线 runtime 支持这一结论。`microservices`、`dual`、`split` 以及固定 12 服务拓扑在本阶段均为 `excluded` 且默认 `disabled`；已有 Dockerfile、Kubernetes manifest、独立 `cmd/*` 或 accepted ADR 只能作为候选资产，不能自动成为发布承诺。
- **D-06:** 每个 release contract 必须且只能声明一个显式默认 profile；当前默认值为 `monolith`。禁止根据存在的 manifest、环境变量或可执行文件隐式推断 profile。缺失、未知或拼写错误的 profile 必须在启动或部署校验阶段 fail closed。
- **D-07:** 每个 deployment profile 必须声明拓扑/entrypoint、必需依赖和状态存储、适用 capability 集合及 override、migration/deploy/rollback 入口和 readiness 要求。profile readiness 必须独立计算，不能用另一个 profile 的证据替代。
- **D-08:** 非 monolith profile 只有在 Phase 38 通过与 committed capability 集合相同的正向旅程、tenant-denial、migration、deploy、rollback 和必要恢复验证后，才能通过一次显式合同变更晋级为 `committed`。环境级临时开关不得绕过晋级规则。

### 权威合同与漂移阻断

- **D-09:** 新增一个 schema-validated、版本化、仓库内唯一的 authored release contract 作为能力承诺和部署 profile 的机器权威。公开文档、operator 输出和随制品发布的 manifest 都必须从该权威源生成或验证；`docs/api/route-surface-manifest.json` 继续作为 OpenAPI 派生 route inventory，不能承担能力成熟度或部署承诺权威。
- **D-10:** 各契约表面保持清晰的 canonical owner：HTTP 公共契约由 `docs/api/openapi.yaml` 定义并与 runtime route registry 双向校验；protobuf 由 `api/proto/*.proto` 定义并与生成 stub/client 校验；migration 由已编号 SQL、不可变内容/checksum 和 runtime migration ledger 共同约束；前端 client 必须由 OpenAPI 生成，或通过等价的 path/method/request/response fingerprint 与 OpenAPI/runtime 校验。release contract 引用这些表面的 schema/version/digest，不复制另一份手写 route/schema 真相。
- **D-11:** 初次建立基线时，当前可运行代码和实际注册表面优先于旧的设计拓扑或完成措辞；发现冲突后必须显式修改权威契约或 runtime，禁止静默选择一边或自动覆盖。基线建立后执行双向漂移检查：缺失、额外、method/security/capability ID 不符、protobuf 生成物过期、migration 内容漂移或前端 client 不兼容都阻断 CI 和发布。
- **D-12:** 漂移门禁必须输出机器可读报告，至少包含 release commit、surface、canonical source、consumer、digest/version、missing/extra/incompatible 明细、pass/fail 和 skipped checks。committed surface 的任何 skip 都按失败处理；报告应能被现有 quality/commercial verifier 汇总。

### 未承诺能力的呈现与 fail-closed

- **D-13:** `excluded` capability 不出现在默认用户导航、公开产品文档、模型/工具选择器或默认生成 client 中；`conditional` capability 只在当前 profile 声明且 runtime readiness 为 `enabled` 时显示。Operator/Admin 可以查看包含 excluded 项在内的完整 inventory，但这不等于对外承诺。
- **D-14:** fail-closed 行为按状态区分：`excluded` surface 默认不注册、不广告，直接探测表现为 `404`；已进入稳定条件契约但为 `disabled` 的 surface 在任何副作用前返回标准 envelope 和 `capability_disabled`；应当可用但依赖失败的 `blocked` surface 返回 `503` 和 `capability_blocked`，同时成为 release blocker。具体非 404 状态码可由 planner 与现有 envelope 约定对齐，但错误 code 和语义必须稳定。
- **D-15:** 禁用不能只靠 UI。相同 capability ID 必须约束 frontend exposure、HTTP/gRPC ingress、service/worker execution、outbound Provider/tool 调用和财务副作用入口。manifest 缺失、capability 未知、profile 不匹配或 readiness 无法读取时一律按未启用处理；任何环境变量都不能单独启用 `excluded` capability。
- **D-16:** 运营者的机器/人类可读视图至少显示 capability ID、commitment、current availability、deployment profile、reasonCode、必要依赖、最后检查时间、修复提示和 evidence/contract refs。公开错误必须保留可操作性但隐藏 secrets、内部地址和敏感配置；禁止用笼统的“coming soon”或存在 route/page 来暗示承诺。

### the agent's Discretion

- canonical release contract 的确切文件名和目录、JSON 或 YAML 序列化、schema validator 库及生成命令由 researcher/planner 根据现有工具链决定，但必须保持单一 authored authority、稳定 schema version、确定性输出和 runtime/release artifact 可消费性。
- capability ID 的具体分隔符、`reasonCode` 命名表和 operator 输出的版式可在不改变上述语义的前提下决定。
- 前端 client 采用完整 code generation 还是 fingerprint-based parity gate，由研究结果决定；无论选择哪种方式，都必须覆盖实际被调用的 feature API modules，不能只检查一个示例 client。

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project scope and evidence boundary

- `.planning/PROJECT.md` - 当前产品目标、Relay invariant、E1-E4 证据边界和已锁定项目决策。
- `.planning/REQUIREMENTS.md` - `RELS-01`、`RELS-02` 及 deployment parity、后续 release evidence 的需求边界。
- `.planning/ROADMAP.md` - Phase 31 success criteria，以及 Phase 32-39 的明确职责边界。
- `.planning/STATE.md` - 当前 milestone、Phase 31 blocker 和“只保留已证明 profile”的累积决策。

### Current contracts and drift checks

- `docs/api/openapi.yaml` - 当前 HTTP 公共 schema 和 operation contract。
- `docs/api/route-surface-manifest.json` - 由 OpenAPI 派生的 route inventory；当前仅覆盖 route/security/operation metadata。
- `scripts/verify-openapi-contract.sh` - 当前 OpenAPI gate 入口。
- `scripts/verify_openapi_contract.py` - 当前 route manifest parity、security、envelope 和领域 schema 检查实现。
- `api/proto/agent.proto` - Agent gRPC canonical source。
- `api/proto/rag.proto` - RAG gRPC canonical source。
- `api/proto/workflow.proto` - Workflow gRPC canonical source。
- `api/proto/task.proto` - Task gRPC canonical source。
- `api/proto/relay.proto` - Relay gRPC canonical source。
- `api/proto/billing.proto` - Billing gRPC canonical source。
- `scripts/verify-migration-contract.sh` - migration 命名、历史重复 prefix 和静态完整性基线。
- `scripts/verify-migration-replay.sh` - migration ledger replay/skip 的现有验证入口。
- `src/web/src/services/http/client.ts` - 前端共享 HTTP transport 与错误 envelope 消费入口。
- `src/web/src/types/api.ts` - 当前手写前端 API 类型基线。

### Runtime and deployment baseline

- `docs/architecture/current-system-contracts.md` - 当前 mainline/runtime 合同说明；其中完成措辞必须以 live runtime 和新 manifest 校正。
- `docs/architecture/ADR-012-microservices-boundaries.md` - 12 服务目标拓扑和迁移路径；本阶段不得把 accepted target 误当成 committed current profile。
- `docker-compose.yml` - `monolith` 默认服务与显式 `microservices` profile 的当前配置事实。
- `src/server/cmd/server/main.go` - monolith runtime entrypoint。
- `src/server/cmd/gateway/main.go` - microservices 候选 gateway entrypoint。
- `deploy/kubernetes/server.yaml` - 当前 monolith Kubernetes server asset。
- `deploy/kubernetes/gateway-deployment.yaml` - microservices 候选 gateway asset。
- `deploy/kubernetes/relay-deployment.yaml` - microservices 候选 Relay asset。
- `scripts/verify-deployment-operations-contract.sh` - 现有部署/运行合同静态 gate 入口。

### Release policy

- `docs/release/rc-checklist.md` - 当前 RC、target evidence 和 no-skip 运行边界；不得把脚本存在当作 target proof。
- `docs/release/commercial-gates.md` - 当前商业 gate 分类和 residual-risk 边界。
- `scripts/verify-commercial-completion.sh` - 现有 release verifier 汇总入口，Phase 31 drift gate 需要接入。
- `scripts/verify-target-release-evidence.sh` - 后续 Phase 39 的 target manifest/evidence 边界；Phase 31 只建立 contract identity 和引用关系。

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `docs/api/route-surface-manifest.json` + `scripts/verify_openapi_contract.py`: 已有确定性的 OpenAPI route inventory 和双向 manifest parity 逻辑，可扩展 capability ID/ref，但不能直接充当完整 release contract。
- `scripts/verify-migration-contract.sh`、`scripts/verify-migration-replay.sh` 和 `src/server/internal/migrations`: 已有 migration naming、历史例外、ledger replay 基线，可加入 digest/runtime inventory 对齐。
- `api/proto/*.proto` 与部分 `*.pb.go`/`*_grpc.pb.go`: 已有 protobuf source/generated-output 组合，可建立 stale generation gate；当前生成覆盖不均正是需要显式报告的 drift。
- `src/web/src/features/*/*Api.ts`、`src/web/src/services/http/client.ts`、`src/web/src/types/api.ts`: 实际前端 client 分散在 feature modules，contract gate 必须覆盖这些真实消费者。
- `docker-compose.yml`、`deploy/docker/`、`deploy/kubernetes/` 和 `src/server/cmd/*`: 已有多种拓扑资产，可供 profile inventory 使用，但资产存在不代表 profile parity。

### Established Patterns

- Release gate 通常采用小型 shell wrapper 加 Python/Node structured validator，并使用 `set -euo pipefail` fail fast。
- `route-surface-manifest.json` 已采用确定性 JSON inventory；新 release contract 应延续 schema validation、稳定排序和机器可读 diagnostics。
- 目标证据、artifact body 和 secrets 保持在 git 外；仓库内 manifest 只能记录合同和引用，不能伪造 E3/E4 结果。
- `docker-compose.yml` 当前默认启动 monolith，microservices 通过显式 profile opt-in；这比旧 ADR 的最终拓扑措辞更能代表当前 baseline。

### Integration Points

- 将 capability/deployment contract validator 接入 `scripts/check.sh`、`scripts/verify-quality-gates.sh` 和 `scripts/verify-commercial-completion.sh`，使 drift 成为 release blocker。
- 在 runtime startup/router/worker registration 处消费同一 capability/profile identity，避免只有文档门禁而没有运行时 fail-closed。
- 在 OpenAPI operations、protobuf services、migration inventory 和 frontend client parity 报告中引用稳定 capability ID，再由 release contract 聚合。
- 通过只读 operator surface 或 CLI 输出 manifest + runtime readiness join；不得从健康探针反向改写 authored release contract。

</code_context>

<specifics>
## Specific Ideas

- 推荐在 OpenAPI operation 和其他契约表面携带稳定 capability ID，再由 verifier 校验这些 ID 必须存在于 release contract。
- 推荐让 release artifact 携带 canonical contract digest；动态 readiness report 同时记录 release commit、contract digest 和 deployment profile，防止跨版本拼接状态。
- 用户明确授权剩余灰区全部采用推荐方案，因此以上部署、权威来源和 fail-closed 选择均为锁定决策，不是 planner 的待选项。

</specifics>

<deferred>
## Deferred Ideas

- Microservices、dual 或 split profile 的 capability/tenant parity、migration、deploy、rollback 和恢复证明属于 Phase 38。
- Identity、安全、durable execution、Relay、客户旅程、财务、观测和恢复缺口分别属于 Phase 32-37；本阶段只准确声明其当前承诺/禁用状态。
- Immutable image、SBOM、provenance、signature、target E3 evidence 和 same-commit E4 no-skip release 属于 Phase 39。

</deferred>

---

*Phase: 31-release-contract-current-baseline*
*Context gathered: 2026-07-15*

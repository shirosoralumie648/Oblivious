# Phase 31: 发布合同与当前基线 - Research

**Researched:** 2026-07-15
**Domain:** versioned release contract, deployment profile enforcement, and multi-surface contract drift
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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

### Deferred Ideas (OUT OF SCOPE)

- Microservices、dual 或 split profile 的 capability/tenant parity、migration、deploy、rollback 和恢复证明属于 Phase 38。
- Identity、安全、durable execution、Relay、客户旅程、财务、观测和恢复缺口分别属于 Phase 32-37；本阶段只准确声明其当前承诺/禁用状态。
- Immutable image、SBOM、provenance、signature、target E3 evidence 和 same-commit E4 no-skip release 属于 Phase 39。
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|---|---|---|
| RELS-01 | capability/deployment manifest 明确列出承诺的模块、集成和运行模式，未承诺能力保持禁用。 | 单一 authored JSON、monolith-only profile、严格 runtime loader、capability policy 与 operator join。 |
| RELS-02 | OpenAPI、protobuf、migration、前端 client 和当前 runtime 一致，contract drift 会阻断发布。 | 复用 route parity/migration checksum/protoc 生成链，新增统一 fingerprint report 与 fail-closed gate。 |
</phase_requirements>

## Summary

当前仓库已有可复用的合同基础：OpenAPI 与 370 条派生 route inventory、Go runtime route 双向测试、112 个主 PostgreSQL migration 的 SHA-256 ledger、六个主要 proto 的 `protoc` 生成目标，以及 shell-wrapper + structured validator 的 release gate 形态。[VERIFIED: docs/api/route-surface-manifest.json, src/server/internal/http/route_surface_test.go, src/server/internal/migrations/migrations.go, src/server/Makefile]

缺口不是再造业务功能，而是缺少一个能把这些表面与“发布承诺、部署 profile、当前 readiness”关联的唯一机器权威。现有 deployment verifier 还会静态检查 microservice 候选资产，旧 ADR 也描述 12 服务目标；两者都不能被解释为 committed profile。[VERIFIED: scripts/verify_deployment_operations_contract.py, docs/architecture/ADR-012-microservices-boundaries.md]

**Primary recommendation:** 以 `config/release/contract.v1.json` + `config/release/contract.schema.json` 为唯一 authored authority，使用现有 Go/Python/TypeScript 工具链消费和校验；只把 `monolith` 标为 committed/default，把其他 profile 明确标为 excluded/disabled，并由统一 JSON drift report 阻断 quality/commercial gates。

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| Release promise/profile authority | Repository config | Release tooling | 不由动态 health 或部署资产反向改写。[VERIFIED: 31-CONTEXT.md D-04/D-09] |
| Runtime profile/readiness | Go backend | Deployment config | `cmd/server` 是当前 monolith 启动入口，启动应先校验 profile 再产生副作用。[VERIFIED: src/server/cmd/server/main.go] |
| HTTP/runtime parity | OpenAPI + Go HTTP | Python gate | OpenAPI/manifest parity和 runtime route 双向测试已存在。[VERIFIED: scripts/verify_openapi_contract.py, src/server/internal/http/route_surface_test.go] |
| Protobuf parity | `api/proto` | generated Go consumers | canonical sources 与生成目标已存在但覆盖位置不统一。[VERIFIED: api/proto, src/server/Makefile] |
| Migration parity | numbered SQL + ledger | release report | runtime 已按文件内容 SHA-256 fail closed。[VERIFIED: src/server/internal/migrations/migrations.go] |
| Frontend exposure/client parity | feature API modules | generated fingerprints | 真实消费者分散在 15 个 feature API 文件，不能以单个示例代表。[VERIFIED: src/web/src/features, src/web/src/services/http/client.ts] |

## Standard Stack

不新增第三方包。JSON 适合确定性排序、Go `encoding/json` 严格解析、Python `json` gate 和 release artifact 直接消费；现有 PyYAML 继续只负责 OpenAPI/YAML 输入。[VERIFIED: src/server/go.mod, scripts/verify_openapi_contract.py]

| Tool | Current source/version | Phase use |
|---|---|---|
| Go stdlib | project Go 1.25; host 1.26.2 | strict contract loader, SHA-256 digest, runtime policy/readiness tests |
| Python 3 + PyYAML | host Python 3.13.12; already imported | deterministic cross-surface aggregator and fixture mutations |
| TypeScript compiler | existing frontend toolchain | AST/fingerprint coverage of all actual feature clients, not regex parsing |
| `protoc` toolchain | protoc 25.1, `protoc-gen-go` 1.36.11, gRPC 1.6.2 | regenerate to temp and byte/digest compare tracked outputs |

## Recommended Files And Reusable Patterns

| Concern | Recommended files | Reuse |
|---|---|---|
| Authored authority | `config/release/contract.v1.json`, `config/release/contract.schema.json` | deterministic JSON and explicit `schemaVersion`; no route/schema duplication |
| Shared runtime policy | `src/server/internal/releasecontract/{contract.go,readiness.go,contract_test.go}` | strict enum/unknown-field rejection and `crypto/sha256` pattern from migrations |
| Startup/operator view | `src/server/cmd/server/main.go`, `src/server/cmd/release-contract/main.go`, `src/server/internal/http/routes_release_contract.go` | validate before DB/migration; expose read-only Admin/operator join with existing envelope |
| Profile declaration | `docker-compose.yml`, `deploy/kubernetes/server.yaml`, `deploy/kubernetes/configmap.yaml` | make selected `monolith` explicit; preserve candidate assets as excluded inventory |
| HTTP drift | `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`, `scripts/verify_openapi_contract.py`, `src/server/internal/http/route_surface_test.go` | add stable capability ID while retaining current OpenAPI -> manifest -> runtime checks |
| Frontend drift | `scripts/generate_frontend_api_fingerprints.py`, `scripts/verify_frontend_client_contract.mjs`, `src/web/src/generated/api-contract.ts`, all `src/web/src/features/**/*Api.ts` | generate operation fingerprints, then AST-check every real consumer |
| Proto drift | `api/proto/capability.proto`, existing `api/proto/*.proto`, `src/server/Makefile`, `scripts/verify-protobuf-contract.sh` | service/method capability options; temp regeneration and tracked-output comparison |
| Migration drift | `scripts/verify-migration-contract.sh`, `scripts/verify-migration-replay.sh`, `src/server/internal/migrations` | keep historical SQL immutable; report file-set digest plus runtime ledger checksum parity |
| Unified gate/report | `scripts/verify-release-contract.sh`, `scripts/verify_release_contract.py`, `scripts/verify-release-contract-fixtures.sh` | shell + Python fail-fast/negative-fixture pattern; `--output` JSON for aggregation |
| Gate wiring/docs | `scripts/check.sh`, `scripts/verify-quality-gates.sh`, `scripts/verify-commercial-completion.sh`, `docs/release/rc-checklist.md`, `docs/architecture/current-system-contracts.md` | one gate invoked by all aggregators; repository-local claim language only |

## Architecture Patterns

### Contract Shape And Identity

Use stable lowercase dotted capability IDs such as `chat.response.stream` and `billing.refund.execute`; IDs are never route names. The contract owns `schemaVersion`, release version, exactly one `defaultProfile`, profiles, capabilities, reason-code catalog, and digests/versions of canonical surfaces. Dynamic reports add git commit, contract digest, selected profile, checked time and evidence class; they never rewrite the contract.

`monolith` references `src/server/cmd/server/main.go`, `oblivious-server`, required state/dependencies and migration/deploy/rollback/readiness commands. `microservices`, `dual`, and `split` remain present only as excluded/disabled inventory with `profile_parity_unproven`; `OBLIVIOUS_DB_MODE=dual_write` is a storage migration mode, not a deployment commitment.[VERIFIED: docker-compose.yml, docs/architecture/current-system-contracts.md]

### One Policy Across Exposure And Execution

Load and validate the selected profile before migrations or server construction. Pass an immutable policy object into route registration, workers and side-effect services. Excluded ingress is not registered (404); conditional-disabled and blocked checks run before side effects and return the existing envelope with stable codes. Frontend navigation and generated client exports consume the same operator/API projection, not independent feature flags.

### Canonical Surface Pipeline

`OpenAPI -> derived route manifest/fingerprints -> runtime/client consumers -> unified report`. Protobuf and migrations run parallel source-to-consumer comparisons, then the aggregator joins every finding by capability ID and contract digest. A committed surface with a skipped checker makes the aggregate result fail; an excluded surface appearing in runtime/client inventory is reported as incompatible.

### Contract Shape Example

```json
{
  "schemaVersion": 1,
  "defaultProfile": "monolith",
  "profiles": [
    {"id": "monolith", "commitment": "committed", "default": true},
    {
      "id": "microservices",
      "commitment": "excluded",
      "default": false,
      "defaultAvailability": "disabled",
      "reasonCode": "profile_parity_unproven"
    }
  ],
  "surfaceRefs": {
    "openapi": {"source": "docs/api/openapi.yaml", "digestAlgorithm": "sha256"},
    "protobuf": {"source": "api/proto/*.proto", "digestAlgorithm": "sha256"},
    "migrations": {"source": "src/server/migrations/*.sql", "digestAlgorithm": "sha256"}
  }
}
```

This is a structural planning example, not the completed baseline. The authored file must also contain the full profile requirements, lifecycle capability entries, reason-code catalog and deterministic surface digests; dynamic `availability` belongs only in the readiness report.

## Don't Hand-Roll

| Problem | Avoid | Use instead |
|---|---|---|
| Source parsing | regex over YAML/Go/TypeScript/proto | PyYAML, Go AST/current route tests, TypeScript compiler AST, `protoc` |
| Migration identity | a second checksum algorithm/ledger | `migrations.Checksum` and `schema_migrations` |
| Route maturity | infer from route/page existence or tags | explicit capability IDs + release commitment/readiness |
| Profile selection | infer from files or `OBLIVIOUS_DB_MODE` | explicit contract default plus selected deployment profile |
| Readiness evidence | write health changes back to manifest | immutable contract + separately timestamped readiness report |

## Common Pitfalls

- **Candidate topology promoted by accident:** current compose/Kubernetes/ADR files contain microservice assets, and the existing operations verifier checks them. Keep those checks as asset hygiene, but label their evidence as excluded-candidate E1/E2, never profile parity.[VERIFIED: docker-compose.yml, scripts/verify_deployment_operations_contract.py]
- **A second handwritten API truth:** route manifest remains OpenAPI-derived; release contract stores capability associations and digests, not copied request/response schemas.[VERIFIED: scripts/verify_openapi_contract.py]
- **Frontend sample bias:** a gate covering only `src/web/src/types/api.ts` misses 15 real feature client modules. Require full module discovery and fail when a client is unclassified.[VERIFIED: src/web/src/features]
- **Proto false green:** generated files exist in top-level, internal and pkg locations, while `proto-gen` covers six internal targets and not every duplicate. Define one explicit source-to-output map and flag all unmanaged generated copies.[VERIFIED: api/proto, src/server/internal/grpc, src/server/pkg, src/server/Makefile]
- **Historical migration rewrite:** never add metadata by editing applied SQL. Associate capability metadata in the release contract and compare immutable file digests to the existing ledger.[VERIFIED: src/server/internal/migrations/migrations.go]
- **Readiness claim inflation:** Phase 31 can prove repository contract enforcement (E1/E2), not target runtime (E3) or same-commit no-skip release (E4).[VERIFIED: .planning/PROJECT.md, .planning/ROADMAP.md]

## Plan-Oriented Decomposition

1. **31-01 Contract authority and conservative baseline (RELS-01):** add schema/contract, stable ID and reason catalogs, monolith-only profile inventory, deterministic digest/validate CLI, and negative schema fixtures.
2. **31-02 Runtime enforcement and operator join (RELS-01):** validate explicit profile before startup side effects, thread policy through HTTP/worker/side-effect boundaries, add read-only operator report, and prove 404/disabled/blocked semantics.
3. **31-03 HTTP and frontend parity (RELS-02):** add OpenAPI capability IDs, regenerate route/fingerprint outputs, preserve bidirectional runtime checks, cover every feature API module and excluded UI/client exposure.
4. **31-04 Protobuf and migration parity (RELS-02):** annotate proto services/methods, standardize the generation map, compare temp-generated artifacts, report migration set/runtime ledger digests without changing historical SQL.
5. **31-05 Unified blocker and claim alignment (RELS-01/02):** aggregate structured reports, add one-field negative fixtures, wire check/quality/commercial gates, and update operator/current-contract docs with explicit E1/E2 residual risk.

## Validation Architecture

### Test Framework

| Property | Value |
|---|---|
| Frameworks | Go `testing`, Python/shell fixture gates, Vitest/TypeScript compiler checks |
| Existing anchors | `src/server/internal/http/route_surface_test.go`, `scripts/verify-openapi-contract.sh`, `scripts/verify-migration-contract.sh` |
| Quick command | `bash scripts/verify-release-contract-fixtures.sh` |
| Full repository-local gate | `bash scripts/check.sh all && bash scripts/test.sh server` |

### Phase Requirements -> Test Map

| Req | Behavior | Test type | Automated command | Gap |
|---|---|---|---|---|
| RELS-01 | schema/status/profile invariants and monolith-only commitment | fixture + Go unit | `bash scripts/verify-release-contract-fixtures.sh && (cd src/server && go test ./internal/releasecontract -count=1)` | Wave 0: files absent |
| RELS-01 | excluded/disabled/blocked enforcement before side effects | HTTP/unit | `(cd src/server && go test ./internal/http -run 'TestReleaseContract|TestRouteSurface' -count=1)` | Wave 0: policy tests absent |
| RELS-02 | OpenAPI/manifest/runtime/client bidirectional parity | contract + Go + Node | `bash scripts/verify-openapi-contract.sh && bash scripts/verify-frontend-client-contract.sh && (cd src/server && go test ./internal/http -run TestRouteSurface -count=1)` | Wave 0: client gate absent |
| RELS-02 | proto source/generated parity | generation fixture | `bash scripts/verify-protobuf-contract.sh` | Wave 0: stale gate absent |
| RELS-02 | migration filename/content/ledger parity | static + integration | `bash scripts/verify-migration-contract.sh && bash scripts/verify-migration-replay.sh` | replay needs Docker or DB |
| RELS-01/02 | committed skip/drift blocks release aggregation | negative fixture + quality gate | `bash scripts/verify-release-contract-fixtures.sh && bash scripts/verify-quality-gates.sh` | Wave 0: integration absent |

### Required Negative Fixtures

Unknown schema key/status/profile; zero or multiple defaults; environment attempts to enable excluded capability; route missing/extra/wrong method/security/capability ID; unclassified frontend client; stale proto output; edited migration checksum; cross-commit/digest readiness report; committed checker marked skipped. Each mutation must assert its stable machine error class.

### Sampling And Evidence Boundary

- Per task: narrow fixture/unit command plus `git diff --check`.
- Per plan: relevant surface gate and `bash scripts/check.sh docs`.
- Phase gate: all contract fixtures, full server tests, `bash scripts/check.sh all`, and a saved repository-local JSON report with no committed skips.
- VALIDATION.md must record E1/E2, environment/tool versions, migration replay mode and skips/residual risks. It must not claim E3 target parity or E4 commercial readiness.

## Security Domain

| ASVS area | Applies | Required control |
|---|---|---|
| V2/V3 auth/session | indirect | operator surface reuses current authenticated Admin boundary |
| V4 access control | yes | excluded ingress absent; conditional/blocked checks before side effects; Admin-only full inventory |
| V5 validation | yes | strict schema/enums/unknown-field rejection and fail-closed missing contract/profile |
| V6 cryptography | yes | standard SHA-256 digest only; no secrets or custom signing in this phase |

Threat checks must cover manifest tampering, cross-version report splicing, environment override bypass, stale generated consumers, and secret/internal-address leakage in public errors. Supply-chain signing remains Phase 39.

## Environment Availability

Go 1.26.2, Node 24.15.0, pnpm 10.6.0, Python 3.13.12, protoc 25.1, `protoc-gen-go` 1.36.11 and `protoc-gen-go-grpc` 1.6.2 are available locally. Migration replay still requires reachable Docker or an explicit database URL.[VERIFIED: local command probes, scripts/verify-migration-replay.sh]

## Sources And Confidence

- **HIGH:** Phase context, requirements, roadmap/project/state, codebase maps, current OpenAPI/runtime tests, migration implementation, deployment manifests and release gates.
- **No external sources used:** this is codebase-only research; no package addition or package-legitimacy audit is required.
- **Assumptions log:** empty. Recommendations are derived from locked decisions and verified repository patterns; exact baseline capability classifications must be authored and reviewed during 31-01 rather than inferred automatically.

**Valid until:** the release contract or any canonical surface owner changes.

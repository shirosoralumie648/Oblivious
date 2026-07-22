---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 31.2
current_phase_name: contract-surface-parity-gate
status: executing
stopped_at: Completed 31.2-10-PLAN.md
last_updated: "2026-07-22T05:43:57.421Z"
last_activity: 2026-07-22
last_activity_desc: Plan 31.2-10 authenticated frontend release projection closure
progress:
  total_phases: 11
  completed_phases: 2
  total_plans: 44
  completed_plans: 39
  percent: 89
---

# Project State

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-07-14)

**Core value:** 让组织客户能够可靠地构建、运行并商业化 AI 应用，同时让每一次 AI 操作都可隔离、可计费、可追踪、可审计、可恢复。
**Current focus:** Phase 31.2 — contract-surface-parity-gate

## Current Position

Phase: 31.2 (contract-surface-parity-gate) — EXECUTING
Plan: 14 of 15
Status: Ready to execute
Last activity: 2026-07-22 — Plan 31.2-10 authenticated frontend release projection closure

Progress: [█████████░] 89%

## Performance Metrics

**Velocity:**

- Total plans completed: 19
- Average duration: 33 min
- Total execution time: 10.3 hours

**By Phase:** Phase 31 completed 7 plans in 249 minutes; Phase 31.1 completed 19 plans in 619 minutes.

**Recent Trend:** Plan 31.2-10 wired authenticated release projection state into routes, navigation, marketing, Chat models, and Agent tools with 127/127 focused tests passing.

## Accumulated Context

### Decisions

Decisions are logged in `PROJECT.md` Key Decisions.

- [Roadmap] Existing code and historical completion are E1/E2 baseline only; all v1 requirements start pending.
- [Roadmap] User approved the standard Horizontal Layers structure for the current milestone.
- [Roadmap] Current milestone continues after historical Phase 30 and uses sequential Phase 31-39 IDs.
- [Roadmap] Phases follow capability and evidence dependencies, not historical module counts or fixed service topology.
- [Roadmap] Only deployment profiles that prove parity remain in the release contract.
- [Workflow] TDD is disabled; implementation may precede tests, while implementation-complete automated verification and regression coverage remain mandatory.
- [Phase 31]: contract.v1.json is the sole authored authority; source identity, dynamic availability, observations, and contract digest remain derived or deferred. — Prevents documents, environment variables, assets, and derived reports from overriding release commitments.
- [Phase 31]: monolith is the only committed/default profile; microservices, dual, and split remain excluded with profile_parity_unproven. — Repository assets do not prove profile parity or promote candidate deployment modes.
- [Phase 31]: RELS-01 remains pending after plan 31-01. — This plan provides repository-local foundation only; runtime readiness and target/live evidence are owned by later phases.
- [Phase 31]: Canonical JSON sorts object keys and set-like collections, preserves argv order, and has no trailing newline. — Keeps the contract digest independent of authored formatting without changing operation semantics.
- [Phase 31]: Operation dispatch inherits only PATH and never release identity environment fields. — Prevents caller or ambient environment values from becoming release authority.
- [Phase 31]: GitProvider derives identity only from clean explicit-root HEAD objects and recomputed contract digest. — Prevents environment, cwd, or caller values from becoming release authority.
- [Phase 31]: Foundation CLI source-bound commands require explicit repo, contract, and schema inputs. — Keeps validation, digest, identity, operation, and inspection on one explicit authority boundary.
- [Phase 31]: Surface reports use one strict six-block envelope and typed details registry; build identity and committed profile remain resolver-owned. — Prevents producers from injecting release authority or inventing parallel report schemas.
- [Phase 31]: Atomic report failures preserve and byte-verify prior destination bytes or absence before returning after rename. — Prevents partial or failed evidence writes from replacing last-known valid evidence.
- [Phase 31]: Active binary identity inspection precedes server/migrate startup and grpc-smoke parsing/network effects. — Makes the shipped tuple observable without config, DB, migration, listener, dial, or RPC side effects.
- [Phase 31]: One clean-Git tuple must agree across three image binaries, OCI labels, and strict packaged-contract digest. — Prevents build args or one artifact surface from becoming independent identity authority.
- [Phase 31]: Stage A uses disposable clean fixtures; Stage B requires an exact clean real commit, explicit monolith, real Docker artifacts, typed report read-back, and zero skips. — Separates development proof from push-eligible repository-local identity evidence.
- [Phase 31]: RELS-01 remains pending after all six plans. — Phase 31 proves only the authored-contract and trusted-build foundation; dynamic readiness, surface parity, and target/live proof remain later phases.
- [Phase 31]: The user transferred ownership of the four timing-contract files to the Phase 31 gap closure. — Allows the clean-HEAD defect to be fixed coherently without treating the former Phase 31.1 overlay as evidence.
- [Phase 31]: Repository-local verification passes with Stage A, full Go regression, docs aggregate, 42/42 structural checks, clean review, Nyquist compliance, zero open threats, and a zero-skip pre-closeout Stage B tuple. — Completes the foundation without closing RELS-01 or claiming target/live readiness.
- [Phase 31.1]: ReadinessManager is the sole runtime capability authority; audit snapshots remain write-only evidence. — Prevents disk state or offline inspection from authorizing runtime effects.
- [Phase 31.1]: The evaluator accepts only the resolver-confirmed monolith profile and exact authored 30s/120s/30s timing. — Keeps every freshness and future-skew verdict on one UTC nanosecond contract.
- [Phase 31.1]: RuntimeAuthorities compiles digest-bound catalog keys and profile-applicable typed effect mappings once at startup. — Prevents caller, persisted, global, or stale capability selection from becoming dispatch authority.
- [Phase 31.1]: RELS-01 remains pending after Plan 31.1-01. — Consumer guards, lifecycle wiring, deployment proof, and the remaining Phase 31.1 plans are not complete.
- [Phase 31.1]: Worker constructors resolve typed capabilities from the startup RuntimeAuthorities carrier and re-read the same manager-backed Guard before each claim and irreversible effect.
- [Phase 31.1]: Readiness denials persist only stable readiness codes through bounded retry bookkeeping and never authorize provider settlement, refund, dispatch, or terminal transition.
- [Phase 31.1]: Relay separates per-attempt model catalog authorization from the immediate Provider transport guard.
- [Phase 31.1]: Chat fallback uses a distinct descriptor and cannot start after cancellation or a stale readiness generation.
- [Phase 31.1]: Model capability metadata is handler-owned response data; Chat and Admin mutations never accept or persist caller capability authority.
- [Phase 31.1]: Admin validates raw model lists before normalization to reject ambiguous or stale persisted subjects.
- [Phase 31.1]: MCP, Registry, and web-search effects resolve current server-owned catalog subjects immediately before each attempt; capability-like caller metadata remains non-authoritative.
- [Phase 31.1]: Financial consumers use startup-bound typed effects and current-generation guards. — Prevents stale or caller-owned authority.
- [Phase 31.1]: Signed provider events use reconciliation-only verbs. — Prevents inbound events from authorizing new outbound effects.
- [Phase 31.1]: Bootstrap serialization spans the pre-publication state check through generation-one publication. — This makes the first generation linearizable without changing ordinary refresh publication semantics.
- [Phase 31.1]: Each managed dependency owns one capacity-one in-flight probe lane. — Occupied lanes publish dependency_unproven candidates without spawning unbounded context-ignoring work.
- [Phase 31.1]: Strict Router composition rejects caller-supplied Admin services and constructs Admin only from the startup authority carrier. — Prevents compatibility injection from bypassing model mutation readiness.
- [Phase 31.1]: Admin sync, detect, and apply guard once before their first read/probe and still resolve current model subjects before persistence. — Separates generic mutation admission from current catalog authorization without pinning a generation.
- [Phase 31.1]: Marketplace HTTP checkout and local settlement consume the same startup financial carrier but guard their own effects independently. — Denial precedes local order intent and Provider checkout.
- [Phase 31.1]: Strict BuildRuntime constructs the authority-aware Agent web-search provider before the authorized executor and passes the exact instance through ToolRuntimeOptions. — Compatibility setters remain outside strict composition; Plan 19 owns global descriptor exactness.
- [Phase 31.1]: Compatibility ToolExecutor construction is deny-only; effect-capable behavior requires NewAuthorizedToolExecutor or NewServiceWithRuntimeOptions. — Prevents missing runtime authority from becoming implicit allow-all.
- [Phase 31.1]: Admin and App reject any non-empty Evaluation.ErrorCode before sorting or projecting last-known capability state. — Keeps evaluator-owned freshness exact and prevents raw diagnostic or enabled-row leakage.
- [Phase 31.1]: Normal monolith readiness constructs five typed probes only from deployment config plus the existing PostgreSQL handle. — Removes the synthetic probe-base authority while keeping missing dependencies fail closed.
- [Phase 31.1]: Redis readiness configuration survives non-Redis rate limiting, and Kafka brokers are strict typed host:port values. — Keeps feature toggles from erasing deployment readiness addressability.
- [Phase 31.1]: One exact runtimeDescriptorSpecs map owns descriptor identity, EffectID, owner, boundary, disposition, and configuration selection; structural AST proof plus matching RuntimeAuthorities are mandatory. — Prevents text markers, prefix or owner inference, independent APIs, and missing authority from certifying monolith runtime effects.
- [Phase 31.1]: Chat mutation bodies contain exactly one strict JSON object and only JSON whitespace may precede EOF. — Prevents trailing values or malformed tokens from reaching catalog, persistence, Relay, usage, or response reflection.
- [Phase 31.1]: Production runtime construction uses the strict duplicate-rejecting EffectRegistry and exposes only a defensive post-construction descriptor snapshot.
- [Phase 31.1]: Shared Chat consumers retain current dispatch guards while the router remains the single descriptor registration owner.
- [Phase 31.1]: Both readiness gate stages run the real BuildRuntime exact-join selector before deployment or identity-bearing harness work.
- [Phase 31.2]: OpenAPI remains the sole HTTP schema authority; manifest and TypeScript artifacts are deterministic compare-only projections.
- [Phase 31.2]: Runtime parity and complete frontend caller discovery remain owned by Plans 02 and 03; Plan 01 evidence is repository-local E1 only.
- [Phase 31.2]: The protobuf manifest is the only tool and disposition authority and pins exact versions, integrity values, inventories, and the canonical manifest digest. — Prevents host tools, partial ownership, or caller observations from becoming release authority.
- [Phase 31.2]: Protobuf generation is verification-only in a fresh temporary tree and never repairs tracked source or generated output bytes. — Any missing, extra, or byte-drifted generated consumer remains a blocking contract failure.
- [Phase 31.2]: The protobuf report accepts only the validator's closed observation while release identity, committed profile, evidence class, and outcome remain trusted-producer owned. — Keeps protobuf evidence repository-local, no-skip, and resistant to identity or claim injection.
- [Phase 31.2]: Migration ledger Version remains the complete SQL filename; numeric prefixes order rows and filename breaks historical prefix ties. — This exactly matches the identity persisted by live migrations.Apply without changing migration semantics.
- [Phase 31.2]: Runtime ledger equality covers only exact ordered MigrationIdentity version/checksum rows; filename and disposition use a separate static digest. — Ledger evidence cannot observe static metadata and must not imply non-monolith replay proof.
- [Phase 31.2]: The ledger producer accepts PostgreSQL credentials only through --database-url-env NAME and sanitizes all database failures. — This keeps literal DSNs and raw driver errors out of argv, reports, and command output.
- [Phase 31.2]: Migrate only the three router-owned operations in Plan 02; Plans 13/14 own route-helper migration and Plan 15 owns complete parity. — Keeps this foundation non-vacuous without claiming the incomplete 17-owner runtime inventory.
- [Phase 31.2]: Keep route-surface-manifest.json outside runtime registration construction and load it only as a post-registration comparison input. — Preserves OpenAPI as schema authority and Go declarations as handler/middleware runtime authority.
- [Phase 31.2]: Require an explicit registrar capability allowlist and fail snapshot discovery when mount/descriptor parity is internally invalid. — Unknown capability and split handler/descriptor state cannot become passing runtime evidence.
- [Phase 31.2]: Construct http-runtime reports from typed HTTPRuntimeObservation values and trusted resolvers with no caller-supplied identity, outcome, writer, or path. — Keeps mismatch evidence repository-local and prevents identity, skip, or claim injection.
- [Phase 31.2]: Generated operation metadata is the only frontend transport identity authority — Paths and TypeScript generics remain diagnostic and cannot select contracts.
- [Phase 31.2]: The source-compatible implicit HttpClient contract remains JSON envelope — Migrated production callers pass explicit operation transports while later plans close the remaining owner set.
- [Phase 31.2]: Markdown export uses Response.text() for text/markdown — It never enters JSON envelope decoding and keeps structured errors on the single shared non-OK path.
- [Phase 31.2]: Shared stream, upload, and SWR wrappers validate caller-supplied exact contracts — Runtime method/path lookup and generic-only identity cannot authorize dispatch.
- [Phase 31.2]: Migration replay counts derive only from exact before, after-first, and after-second ledger identity transitions. — Human migrate output is diagnostic only and cannot authorize the replay verdict.
- [Phase 31.2]: Unavailable replay emits migration_replay_unavailable while preserving a nonzero producer exit. — Failure evidence cannot become an accepted skip or successful release signal.
- [Phase 31.2]: Docker replay uses container-internal psql and external replay requires host psql through named environment indirection. — Connection material stays out of argv, reports, and logs.
- [Phase 31.2]: Passing replay evidence is written only after resource removal and cleanup verification. — cleanupResult records verified post-session state rather than cleanup intent.
- [Phase 31.2]: Keep providers.tsx as an explicit compiler-proven composition-only non-caller instead of fabricating a transport owner. — Preserves the fixed nine-file inventory while proving the file makes zero transport calls.
- [Phase 31.2]: Preserve AgentToolDefinition.capabilityId only on normalized server responses and retain explicit business-field mutation serialization. — Prevents read-only capability identity from becoming caller-controlled mutation authority.
- [Phase 31.2]: Treat runtime durable Knowledge ingestion as authoritative: multipart upload returns 202 KnowledgeDocumentIngestionJob and canonical/legacy status routes return job arrays. — Reconciles stale OpenAPI and generated consumers with the already-shipped durable runtime behavior.
- [Phase 31.2]: Keep existing API behavior assertions by stripping the new transport argument only inside test client adapters; compiler-symbol verification owns contract-identity proof. — Separates public business behavior compatibility from exact metadata identity verification.
- [Phase 31.2]: Use concrete generated operation constants and Plan 04 codec factories across every Group B HttpClient call, including optional-body actions. — Prevents method, path, media, schema, or status identity from being reconstructed by callers.
- [Phase 31.2]: Project MCP add-server and execute-tool mutations to their explicit business fields. — Caller-supplied capabilityId and unknown authority fields cannot cross the frontend mutation boundary.
- [Phase 31.2]: Route Admin dashboard data through the shared typed SWR fetcher with getAdminStatsOperationContract. — The SWR owner now carries exact request and response identity into the actual transport.
- [Phase 31.2]: Preserve legacy business-argument assertions by stripping only the new transport argument inside test client adapters. — Separates public behavior compatibility from compiler-owned exact metadata proof.
- [Phase 31.2]: Production Group A composition calls only explicit registrar entrypoints; legacy mux helpers remain test-only delegating adapters. — Keeps all 58 Group A dispatch and descriptor identities on one runtime registration value.
- [Phase 31.2]: Routing fallbacks apply auth only and remain outside capability, guard, mount, and descriptor parity; HEAD stays implicit through Go ServeMux GET behavior. — Preserves structured 404/405 behavior without manufacturing release authority or conflicting patterns.
- [Phase 31.2]: Same-status responses are unique by status plus normalized media/schema identity. — Preserves the canonical Gateway JSON and SSE status-200 response contracts while still rejecting exact duplicates.
- [Phase 31.2]: Runtime projectionDigest and generated releaseProjectionDigest remain separate digest classes — The frontend recomputes the authenticated runtime digest and independently exact-joins generated capability IDs and dispositions

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 33] Target object storage and Sandbox capacity/deployment model are not selected.
- [Phases 34, 36, 37, 39] Provider, payment/payout, observability, cluster and signing details require target credentials and fresh external evidence.

### Quick Tasks Completed

| # | Description | Date | Commit | Status | Directory |
|---|-------------|------|--------|--------|-----------|
| 260717-up4 | 审计当前未提交内容，完善 .gitignore，删除确认无关的生成内容，并按逻辑边界逐步提交 | 2026-07-17 | 5d43da8 | Verified | [260717-up4-gitignore](./quick/260717-up4-gitignore/) |
| Phase 31.1 P06 | 40 min | 3 tasks | 9 files |
| Phase 31.1 P07 | 36 min | 3 tasks | 8 files |
| Phase 31.1 P10 | 70 min | 2 tasks | 10 files |
| Phase 31.1 P02 | 40 min | 3 tasks | 11 files |
| Phase 31.1 P09 | 20 min | 2 tasks | 9 files |
| Phase 31.1 P03 | 55 min | 3 tasks | 12 files |
| Phase 31.1 P11 | 30 min | 2 tasks + 4 review fixes | 2 files |
| Phase 31.1-readiness-fail-closed P13 | 25 min | 2 tasks | 6 files |
| Phase 31.1-readiness-fail-closed P14 | 21 min | 2 tasks | 2 files |
| Phase 31.1 P15 | 42 min | 2 tasks | 9 files |
| Phase 31.1 P16 | 3 min | 2 tasks | 2 files |
| Phase 31.1 P17 | 50 min | 2 tasks | 8 files |
| Phase 31.1 P18 | 2h 5m | 3 tasks | 3 files |
| Phase 31.1 P21 | 5 min | 2 tasks | 2 files |
| Phase 31.1 P19 | 39 min | 3 tasks | 8 files |
| Phase 31.1 P22 | 16 min | 2 tasks | 3 files |
| Phase 31.2 P01 | 27 min | 3 tasks | 9 files |
| Phase 31.2 P06 | 42 min | 3 tasks | 10 files |
| Phase 31.2 P07 | 32 min | 3 tasks | 9 files |
| Phase 31.2 P02 | 55 min | 3 tasks | 6 files |
| Phase 31.2 P04 | 39 min | 3 tasks | 14 files |
| Phase 31.2 P08 | 45 min | 2 tasks | 7 files |
| Phase 31.2 P11 | 46m | 3 tasks | 19 files |
| Phase 31.2 P12 | 22 min | 3 tasks | 16 files |
| Phase 31.2 P13 | 40 min | 2 tasks | 12 files |
| Phase 31.2 P10 | 1h 12m | 3 tasks | 13 files |

### Roadmap Evolution

- Phase 31.1 inserted after Phase 31: 动态 Readiness 与持续 Fail-Closed (URGENT)
- Phase 31.2 inserted after Phase 31.1: 契约表面一致性与聚合门禁 (URGENT)

## Deferred Items

Items under `REQUIREMENTS.md` v2 remain outside the current Roadmap until the commercial launch baseline is proven.

## Session Continuity

Last session: 2026-07-22T05:43:57.409Z
Stopped at: Completed 31.2-10-PLAN.md
Resume file: None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 31-release-contract-current-baseline P01 | 47 min | 3 tasks | 8 files |
| Phase 31 P02 | 9 min | 3 tasks | 9 files |
| Phase 31 P03 | 19 min | 3 tasks | 9 files |
| Phase 31 P04 | 50 min | 3 tasks | 7 files |
| Phase 31 P05 | 31 min | 3 tasks | 9 files |
| Phase 31 P06 | 64 min | 3 tasks | 7 files |
| Phase 31 P07 | 29 min | 3 tasks | 13 files |
| Phase 31.1-readiness-fail-closed P01 | 37 min | 3 tasks | 10 files |
| Phase 31.1-readiness-fail-closed | 04 | 20 min | 3 tasks, 11 files |
| Phase 31.1-readiness-fail-closed | 05 | 31 min | 3 tasks, 10 files |
| Phase 31.1-readiness-fail-closed | 12 | 12 min | 2 tasks, 2 files |
| Phase 31.1-readiness-fail-closed | 16 | 3 min | 2 tasks, 2 files |
| Phase 31.1-readiness-fail-closed | 17 | 50 min | 2 tasks, 8 files |

# Codebase Concerns

**Analysis Date:** 2026-07-13

## Summary

Oblivious has a broad commercial SaaS surface with strong repository-local verification, but the highest-risk areas are release-evidence boundaries, verification-script drift, very large workflow/agent/frontend modules, and tests that rely on large mock/fixture surfaces. Future work should preserve the Relay-centered invariant, keep target/live evidence separate from fixture evidence, and update the relevant verification profiles whenever adding commercial behavior.

## Critical Release Boundaries

### Target commercial readiness is external-evidence gated

**Concern:** The repository contains strict final-readiness gates, but final commercial readiness still depends on a target environment evidence bundle outside git. Local fixtures, generated templates, or docs checks must not be promoted to target proof.

**Evidence paths:**
- `scripts/run-target-release-evidence.sh` rejects placeholder target env files and disallows `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true`.
- `scripts/verify-commercial-completion.sh` treats `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` as local partial-evidence mode only and rejects `OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true` for strict final readiness.
- `scripts/verify-target-release-evidence.sh` and `scripts/verify_target_release_evidence.py` validate target manifests and contain placeholder template output that must be filled with real target artifacts before use.
- `docs/release/rc-checklist.md` documents the required outside-git evidence bundle, artifact hashes, provider live rails, Kubernetes proof, gRPC smoke, secret audit, workflow telemetry, and request-log evidence.

**Prescriptive guidance:**
- Use `scripts/run-target-release-evidence.sh` as the target evidence entrypoint for final release proof.
- Keep raw target proof files under an external workdir created by `scripts/init-target-release-evidence-workdir.sh`; do not commit target proof JSON, secrets, or downloaded artifacts.
- Treat `scripts/*-fixtures.sh` runs as verifier-regression proof only, not production readiness proof.
- When reporting status, separate “repository-local checks pass” from “target commercial release proven.”

### Commit-mismatch and env-skip knobs are dangerous if reused casually

**Concern:** The verifier has explicit escape hatches for non-final local evidence. These are useful for development, but any automated release wrapper or CI job that leaks them into final runs invalidates readiness claims.

**Evidence paths:**
- `scripts/verify-target-release-evidence.sh` reads `OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH`.
- `scripts/verify-commercial-completion.sh` reads `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS` and fails strict final readiness when commit mismatch is enabled.
- `scripts/verify-commercial-preflight.mjs` rejects true values for both env-skip and commit-mismatch variables.
- `scripts/verify-target-release-evidence-fixtures.sh` includes invalid cases for quoted and ANSI-C quoted skip/mismatch command forms.

**Prescriptive guidance:**
- Do not add new release commands that default these variables to `true`.
- Always include a negative fixture when expanding target evidence parsing around shell command/env handling.
- Keep final-release command examples free of local-only skip/mismatch values.

## Verification And Script Drift

### Verification script surface is large and paired with many fixtures

**Concern:** The repository has a large shell/Python verification layer. There are 107 files under `scripts/`, including 23 fixture-oriented scripts. Every new evidence family needs updates in multiple places, so drift is a material risk.

**Evidence paths:**
- `scripts/verify-quality-gates.sh` is a central gate and is large enough to become a coupling point.
- `scripts/verify_target_release_evidence.py`, `scripts/assemble_target_release_evidence.py`, and `scripts/target_release_fixture_mutations.py` jointly define target evidence validation, assembly, and negative fixtures.
- `scripts/assemble-target-release-evidence.sh` and `scripts/assemble-target-release-evidence-fixtures.sh` pair runtime assembly with fixture validation.
- `scripts/collect-*-evidence.sh` and `scripts/collect_*.py` patterns exist for provider live rails, request-log observability, RAG, Relay, marketplace, Kubernetes, deployment, gRPC smoke, and microservice database proof.

**Prescriptive guidance:**
- When adding a new release evidence family, update the collector, assembler, verifier, fixture mutations, fixture shell script, quality gate, and release docs together.
- Prefer one canonical implementation per validation rule; keep shell wrappers thin over Python validators where possible.
- Add fixture failures for every new required manifest field, artifact digest, URI, timestamp, and environment class rule.
- Run `bash scripts/verify-quality-gates.sh` and the narrow fixture script for the changed evidence family before claiming release-gate changes are complete.

### Fixture-backed browser evidence can hide backend contract drift

**Concern:** The web test suite uses broad mocks and Playwright route fixtures. This is useful for UI coverage but can drift from backend handlers unless API serialization and contract tests are kept in sync.

**Evidence paths:**
- `src/web/src/app/router.test.tsx` contains a very large router-level mock matrix.
- `src/web/e2e/fixtures/*` files use `page.route('**/api/v1/**', ...)` to simulate API responses.
- `src/web/src/routes/workspace/WorkflowsPage.test.tsx` and `src/web/src/routes/workspace/ChatPage.behavior.test.tsx` contain extensive mocked frontend behavior coverage.
- Backend API contracts live in `docs/api/openapi.yaml`, `docs/api/route-surface-manifest.json`, and route tests such as `src/server/internal/http/route_surface_test.go`.

**Prescriptive guidance:**
- For every new browser fixture route, add or update a frontend API serialization test and a backend route/handler test.
- Use fixture mismatch failures like `fixture_contract_mismatch` for unexpected query strings and payload shapes.
- Do not use Playwright fixture pass results as proof that backend persistence, tenant isolation, provider rails, or billing side effects work.

## Complexity Hotspots

### Workflow, Agent, and page modules are very large

**Concern:** Several implementation and test files are multi-thousand-line hotspots. Small changes in these areas can have wide behavioral impact and should be split or guarded with narrow tests.

**Evidence paths:**
- `src/web/src/routes/workspace/WorkflowsPage.tsx` is one of the largest frontend route components.
- `src/web/src/routes/workspace/WorkflowsPage.test.tsx` mirrors that complexity with a large route-level test suite.
- `src/server/internal/workflow/service.go` and `src/server/internal/workflow/service_test.go` are central workflow engine hotspots.
- `src/server/internal/agent/service.go`, `src/server/internal/agent/service_test.go`, `src/server/internal/agent/runner.go`, and `src/server/internal/agent/store.go` concentrate agent execution, persistence, and test behavior.
- `src/server/internal/http/router.go` and `src/server/internal/http/server_test.go` concentrate many route registration and cross-cutting HTTP behaviors.

**Prescriptive guidance:**
- Prefer adding focused service helpers under existing subpackages before extending very large route/page files.
- Add regression tests at the smallest layer that owns the behavior: store tests for persistence, service tests for orchestration, handler tests for HTTP contracts, and page tests only for UX state.
- When editing `WorkflowsPage.tsx`, keep API calls in `src/web/src/features/workflows/` and keep visual-only state out of backend contract tests.
- When editing agent or workflow runtime code, run the narrow package tests first before broader commercial gates.

### Generated proto defaults are easy false positives

**Concern:** Generated gRPC files include unimplemented server defaults. These are normal generated code, but they become a risk if production registration accidentally wires generated unimplemented servers instead of real adapters.

**Evidence paths:**
- `api/proto/agent_grpc.pb.go` contains generated `UnimplementedAgentServiceServer` methods.
- `api/proto/rag_grpc.pb.go` contains generated `UnimplementedRAGServiceServer` methods.
- Runtime adapters and tests live under `src/server/internal/grpc/agentv1/`, `src/server/internal/grpc/ragv1/`, `src/server/internal/grpc/workflowv1/`, `src/server/internal/grpc/taskv1/`, and `src/server/pkg/agent/`.

**Prescriptive guidance:**
- Do not flag generated `api/proto/*_grpc.pb.go` unimplemented methods as product stubs by themselves.
- When adding a gRPC service, verify registration points use the real adapter package and add smoke coverage for the generated client path.
- Keep generated files updated from `api/proto/*.proto` in the same change as adapter updates.

## Integration And Environment Risks

### Local integration tests can be explicit skips

**Concern:** Some integration coverage is environment-gated. That is acceptable when recorded explicitly, but it must not be counted as strict commercial evidence unless the required services are actually present.

**Evidence paths:**
- `src/server/test/integration/grpc_kafka_test.go` explicitly skips Kafka testcontainers setup.
- `docs/architecture/current-system-contracts.md` records `TEST_DATABASE_URL` as required for CI DB-backed HTTP integration tests and absent for local optional skips.
- `scripts/test.sh` and `scripts/check.sh` are the preferred entrypoints for local/server/web checks.

**Prescriptive guidance:**
- Keep local skip messages explicit and searchable.
- For CI or release profiles, prefer fail-closed required env behavior over optional local skips.
- Do not close persistence, Kafka, ClickHouse, Kubernetes, or provider-live requirements from unit tests alone.

### Mixed JavaScript lockfiles require deliberate maintenance

**Concern:** The repository keeps both pnpm and npm lockfiles. This supports security scanning across lockfile families, but dependency updates can drift if only one lockfile is refreshed.

**Evidence paths:**
- `pnpm-lock.yaml` exists at the repository root.
- `package-lock.json` exists at the repository root.
- `src/web/package-lock.json` exists in the frontend workspace.
- `package.json`, `pnpm-workspace.yaml`, and `src/web/package.json` define the Node workspace surface.
- `scripts/verify-dependency-security.sh` and `scripts/verify-quality-gates.sh` run dependency security checks.

**Prescriptive guidance:**
- When changing frontend/root dependencies, refresh the pnpm lockfile and any tracked npm lockfile that security gates inspect.
- Run both the workspace package-manager command and the repository security gate before claiming dependency changes are clean.
- Avoid ad hoc package-manager usage that updates only one lockfile family.

### Reference repositories are useful context, not product implementation

**Concern:** The `reference/` tree contains many nested upstream/reference repositories with their own `.git`, `.planning`, source, docs, and tests. Broad scans can accidentally treat reference code as Oblivious product code.

**Evidence paths:**
- `reference/ai-gateway/`, `reference/anything-llm/`, `reference/bifrost/`, `reference/claude-code-api/`, `reference/CLIProxyAPI/`, and other nested repositories exist.
- `docs/audit/reference-capability-evidence-v3.json` intentionally records reference capability evidence and marks some upstream TODO/stub signals.
- Product source lives under `src/server/`, `src/web/`, `api/proto/`, `deploy/`, `scripts/`, and product documentation under `docs/`.

**Prescriptive guidance:**
- Exclude `reference/` when auditing current product code, unless the task explicitly asks for reference comparison.
- Normalize all paths returned by agents before inserting them into product audit reports.
- Do not copy upstream TODOs, fake providers, or scaffold comments into Oblivious completion status unless a matching product path exists.

## Security-Sensitive Areas

### Outbound and Relay paths are high blast-radius

**Concern:** Relay is the product invariant for AI calls, billing, rate limiting, audit, and monitoring. Any bypass in outbound HTTP, web search, provider routing, or agent tools can break commercial controls.

**Evidence paths:**
- `src/server/internal/relay/` contains routing, handler, cache, rate-limit, health, channel, and type layers.
- `src/server/internal/outboundpolicy/` centralizes outbound policy behavior.
- `src/server/internal/mcp/websearch/` is a provider-dependent tool surface.
- `src/server/internal/agent/tools/` and `src/server/internal/agent/runtime/` are tool execution surfaces.
- `scripts/verify-relay-security.sh` and `scripts/verify-commercial-completion.sh` include security/release verification paths.

**Prescriptive guidance:**
- Route new AI/provider/tool execution through Relay or a documented policy gate; do not add direct provider calls in page handlers or feature APIs.
- Add SSRF/egress-policy tests when introducing any URL-fetching or web-search behavior.
- Include tenant, quota, request-log, and billing assertions for any new provider or tool execution path.

### Secrets and release artifacts must stay outside git

**Concern:** The release workflow depends on external secret files, target manifests, and downloaded artifacts. Storing these in the repository would create secret leakage and false-proof risk.

**Evidence paths:**
- `config/.env.example` is the safe placeholder reference for environment setup.
- `deploy/kubernetes/secret.example.yaml` is an example only and must not be used as the filled secret file.
- `scripts/verify-commercial-preflight.mjs` rejects checked-in Kubernetes secret files, placeholder values, and repository-internal target evidence.
- `docs/release/release-rollback-runbook.md` instructs operators not to commit provider keys, Stripe secrets, database passwords, session secrets, kubeconfig material, or backup dumps.

**Prescriptive guidance:**
- Keep `.env`, kube secrets, target evidence manifests, raw proof files, and downloaded artifacts outside git.
- Use placeholders only in committed examples.
- Add secret-audit proof to target release evidence instead of relying on manual review.

## Small But Actionable Cleanup

### Service template still contains a TODO

**Concern:** `scripts/migrate-service-template.sh` intentionally contains a service-specific TODO. It is acceptable as a template, but copied migrations must replace it before use.

**Evidence paths:**
- `scripts/migrate-service-template.sh` contains `# TODO: 每个服务自定义数据迁移逻辑`.
- Service-specific migration scripts already exist for chat, agent, relay, RAG, marketplace, gateway, channel, billing, workflow, task, and observability.

**Prescriptive guidance:**
- Keep the TODO only in the template.
- New `scripts/migrate-*.sh` files should have concrete migration behavior or an explicit no-op rationale.
- Add shell syntax checks for new migration scripts.

## False-Positive Filters

Use these filters before opening new bug/cleanup work:

- Do not count generated `api/proto/*_grpc.pb.go` unimplemented methods as runtime stubs without checking real adapter registration.
- Do not count `scripts/verify_target_release_evidence.py --print-template` TODO values as committed fake proof; they are verifier templates and should be rejected unless filled externally.
- Do not count `reference/` upstream TODOs as Oblivious product issues unless the product code copied the same pattern.
- Do not count fixture scripts as target/live evidence; they are regression tests for verifiers and collectors.

---

*Concern analysis: 2026-07-13*

# Stack Research: Commercial Target Closure

**Domain:** Brownfield multi-tenant AI SaaS release completion
**Project:** Oblivious
**Researched:** 2026-07-14
**Overall confidence:** MEDIUM

The repository baseline is directly observed from the current checkout. External version claims were checked against current official project pages, repositories, registries, or OpenAI documentation, but the required confidence classifier rates the available `webfetch` and OpenAI documentation provider paths as LOW. External version rows are therefore tagged **[LOW-external]** even when two official endpoints agree. Re-check patch releases and image digests when the release commit is cut.

## Decision Summary

| Area | Decision | Target for this milestone | Reason |
|------|----------|---------------------------|--------|
| Backend | **Upgrade in place** | Go toolchain 1.26.5; retain current Go package/module structure | Go 1.25 is still supported, but only until two newer majors exist. Standardizing the already-used security toolchain version avoids an imminent support cliff without a rewrite. |
| Frontend | **Retain architecture, upgrade runtime** | Node 24.18.0 LTS, pnpm 10.6.0, Vite 8.1.4; retain React 18.3.1 | Node 20 reached EOL. React 19 is a behavior-changing migration and is not required to close commercial runtime gaps. |
| Primary persistence | **Retain and raise floor** | PostgreSQL 16.14 plus pgvector 0.8.5 | The deployed manifests already use PostgreSQL 16. PostgreSQL 14 support ends 2026-11-12, too close to the target release. |
| Cache and durable jobs | **Retain, pin patches** | Redis 7.4.9, go-redis 9.14.1, Asynq 0.25.1 | Existing rate-limit, cache, session, and billing-worker contracts already use these APIs. Redis 7.4.9 contains current security fixes. |
| Vector service | **Conditional upgrade** | pgvector is the baseline; Qdrant 1.18.2 only for a declared Qdrant deployment profile | Do not require two vector databases for every install. Qdrant 1.12.1 is stale and must be snapshot/restore tested before a declared production profile uses it. |
| Analytics | **Conditional staged upgrade** | ClickHouse 26.3.17.4 LTS, reached through a 25.8 LTS intermediate from 24.12 | ClickHouse documents a one-year/two-LTS mixed-version window. The current 24.12 to 26.3 jump exceeds it. |
| Event broker | **Conditional upgrade** | Kafka 4.3.1 and kafka-go 0.4.51 only when the microservices profile remains a release claim | Kafka is an `infra-extras`/microservices dependency, not a reason to impose distributed topology on the monolith profile. |
| Object storage | **Replace production dependency, retain protocol** | A maintained target-owned S3-compatible service; legacy MinIO image is dev/test only | The MinIO community repository is officially unmaintained and source-only; legacy binaries receive no fixes. The product contract is S3 compatibility, not MinIO ownership. |
| Containers | **Upgrade and make immutable** | Go 1.26.5, Node 24.18.0, Nginx 1.30.3 stable, patched Alpine 3.23.x; all release images by digest | The checkout mixes Go 1.22/1.25, Node 20, Alpine 3.19/3.21, broad tags, and local tags. One canonical release Dockerfile path removes evidence ambiguity. |
| Kubernetes | **Retain as one declared deployment mode** | Validate Kubernetes 1.36.2 and one adjacent supported minor; pin application images by digest | Kubernetes supports the latest three minor branches. Raw stateful manifests should not silently become the required production topology. |
| Observability | **Complete deployment, retain instrumentation** | OTel Go 1.44.0, Collector 0.156.0, Prometheus 3.13.1, Grafana 12.4.5, ClickHouse datasource 4.19.0 | The Go instrumentation is current; the missing work is deployed collection, dashboards, alerts, and target evidence. Grafana 13 is deferred pending datasource validation. |
| Supply chain | **Add release evidence** | Syft 1.46.0, cosign 3.1.1, Trivy 0.72.0, GitHub attest-build-provenance 4.1.1 | Existing workflows verify evidence bundles but do not yet build, scan, attest, and sign immutable artifacts. |
| Verification | **Broaden existing gates** | Go race/vet/lint/vuln, Vitest 4.1.10, Playwright 1.61.1, kubeconform 0.8.0, k6 2.1.0 | Current audits identify missing race, stable lint, fuzz, load, and soak coverage. Keep the existing commercial verifier as the final authority. |

## Repository-Observed Baseline

These are checkout facts, not external recommendations.

| Surface | Observed state | Release consequence |
|---------|----------------|---------------------|
| Go | `go 1.25.0`; most release Dockerfiles use `golang:1.25`, while `src/server/Dockerfile` still uses 1.22; security checks already default to Go 1.26.5 | Align module, CI, security scan, and canonical image toolchains. Do not ship artifacts built by ambiguous Go versions. |
| Web | React 18.3.1, Vite 8.0.16, TypeScript 5.6.3, Playwright 1.52.0, Vitest 4.1.8; CI uses Node 20.19.0 | Node is the urgent change. Framework majors are not. Upgrade test/browser tooling in a dedicated verified slice. |
| PostgreSQL/vector | Compose and Kubernetes use `pgvector/pgvector:pg16`; README still says PostgreSQL 14+; migrations create vector(1536) HNSW indexes | Make PostgreSQL 16 the supported production floor and test migrations, HNSW recall, backup, restore, and tenant filters there. |
| Qdrant | Image 1.12.1; application has organization-scoped Qdrant paths and pgvector paths | Treat as a selectable backend. A configured target must prove snapshot/restore and cross-tenant denial; an unconfigured target should not be failed merely for omitting Qdrant. |
| ClickHouse | Image 24.12; request-log and usage analytics code, migrations, dashboards, and evidence collectors exist | Upgrade only with data backup, migration replay, ingest/query smoke, and request-to-usage/billing joins. |
| Redis/queue | Redis broad tag `7`; go-redis 9.14.1; Asynq 0.25.1; Kafka 3.7.1 with kafka-go 0.4.47 | Pin Redis to a security patch. Kafka is tied to split Workflow/Task/Agent event paths and should remain conditional. |
| Object storage | MinIO legacy 2024 image is under `infra-extras`; application exposes an S3-compatible archive contract implemented over HTTP SigV4 | Preserve the S3 contract. Do not make MinIO a product invariant or add a second object-storage abstraction. |
| Provider integration | Relay has OpenAI, Claude, Gemini, Vertex, and Bedrock adapters and OpenAI-compatible `/v1/*` handlers; there is no global vendor SDK dependency | Keep adapter boundaries and live contract tests. Vendor SDK adoption is not a release prerequisite. |
| Kubernetes | App Deployments, Services, HPA, probes, NetworkPolicy, and raw stateful dependency manifests exist | Validate only deployment modes that will be advertised. Prefer managed stateful dependencies for the commercial target rather than claiming raw single-cluster manifests as universal production HA. |
| Observability | Prometheus metrics, OTel Go packages, alert rules, Grafana dashboard, ClickHouse datasource references, and target evidence tooling exist; no repository-owned external collector/server deployment proof exists | Deployment and target proof are the gap, not a new instrumentation framework. |
| CI/supply chain | Actions use mutable major tags; CI has build/test/security jobs and target-evidence verification, but no SBOM, image scan, signature, or provenance generation | Add an artifact build lane that emits and verifies digest-bound evidence from the same release commit. |

## Recommended Stack

### Core Technologies

| Technology | Version / policy | Purpose | Recommendation |
|------------|------------------|---------|----------------|
| Go | **1.26.5 toolchain**; set the module directive to 1.26 after full compatibility gates | Backend, Relay, workers, services, CLIs | **Upgrade.** Keep Gin/net/http, `database/sql`, gRPC, and protobuf contracts. Use the same patch version in CI and all canonical builders. **[LOW-external]** |
| Gin | 1.10.1 | HTTP routing/middleware | **Retain.** A router migration has no commercial-closure payoff. |
| gRPC / protobuf | grpc-go 1.79.3; protobuf 1.36.10 | Internal service contracts | **Retain.** Regenerate and contract-test only when `.proto` changes; preserve current identity metadata. |
| Node.js | **24.18.0 LTS** | Frontend build/test runtime | **Upgrade.** Node 20 is EOL; Node 24 is supported by Vite 8 and has an LTS window through 2028-04-30. **[LOW-external]** |
| pnpm | 10.6.0, exact `packageManager` and frozen lock | JavaScript dependency manager | **Retain for the target release.** Upgrade only through an explicit lockfile PR; do not regenerate npm and pnpm graphs opportunistically. |
| Vite | **8.1.4** | Frontend build/dev server | **Patch/minor upgrade.** Keep Vite 8; validate build output and proxy behavior on Node 24. **[LOW-external]** |
| React / React DOM | **18.3.1** | Product UI | **Retain.** Defer React 19.2.x until after target closure, with its own compatibility and browser regression plan. |
| React Router | 6.30.4 | Route contracts | **Retain.** Do not combine a Router 7 migration with release hardening. |
| TypeScript | 5.6.3 for closure; exact lock | Static type gate | **Retain initially.** A compiler upgrade can follow after Node/Vite stabilization and must not block runtime closure. |

### Persistence, Vector, Analytics, Cache, Queue, and Storage

| Technology | Version / policy | Role | Recommendation and compatibility notes |
|------------|------------------|------|----------------------------------------|
| PostgreSQL | **16.14**; image tag plus immutable digest | System of record, ledgers, durable execution, tenant state | **Retain and raise the production floor.** PostgreSQL 16 is already the manifest baseline and is supported through 2028-11-09. Stop claiming 14+ for production. **[LOW-external]** |
| pgvector | **0.8.5** on PostgreSQL 16 | Baseline vector search and semantic cache | **Upgrade/pin.** Use `0.8.5-pg16-bookworm` or an equivalent verified image digest. Benchmark HNSW with organization filters; iterative scans introduced in 0.8 help filtered recall. **[LOW-external]** |
| lib/pq | 1.10.9 | Existing `database/sql` driver and array helpers | **Retain for closure.** A pgx v5 migration touches connection semantics and many stores; defer it to a separate migration with dual-driver tests. |
| PgBouncer | **1.25.2**, optional target dependency | Connection pooling | **Upgrade only where used.** Validate transaction/session pooling against prepared statements, migrations, and long-running workflows. **[LOW-external]** |
| Qdrant | **1.18.2**, conditional profile | Dedicated vector service | **Upgrade conditionally.** Take snapshots, restore into staging, verify collection schema/dimension and tenant filters, then roll forward. pgvector remains sufficient for the baseline profile. **[LOW-external]** |
| ClickHouse | **25.8 LTS intermediate, then 26.3.17.4 LTS** | High-volume request logs and analytics | **Staged upgrade.** The current 24.12 is more than a year behind 26.3. Do not direct-jump a live dataset without the documented intermediate/downtime choice and rollback proof. **[LOW-external]** |
| Redis | **7.4.9**, immutable image digest | Distributed rate limit, cache, sessions, Asynq | **Retain and pin.** Do not adopt Redis 8.8 features during closure; schedule Redis 8 after client and persistence/HA tests. **[LOW-external]** |
| go-redis | 9.14.1 | Redis client | **Retain.** Test timeouts, TLS/auth, outage behavior, and tenant-safe key prefixes in the target environment. |
| Asynq | 0.25.1 | Relay billing/background work | **Retain.** Prove enqueue, retry, lease, dead-letter/recovery behavior rather than replacing the queue library. |
| Apache Kafka | **4.3.1**, only for a declared split-service profile | Workflow/Task/Agent events | **Conditional upgrade.** Keep Kafka out of the monolith release dependency set. For split mode, test the official 3.7-to-4.3 KRaft upgrade path, metadata version, consumer groups, replay, idempotency, and rollback. **[LOW-external]** |
| kafka-go | **0.4.51** | Go Kafka producer/consumer | **Upgrade with the broker slice.** Do not update the client independently of live protocol/consumer tests. **[LOW-external]** |
| S3-compatible object storage | Target provider's supported version; immutable service identity | Channel archives and future shared blobs | **Retain the protocol, replace the production implementation.** Use a maintained managed S3-compatible service or supported appliance. Keep the legacy MinIO image only for local compatibility tests. **[LOW-external]** |

### Relay and Provider Integration

| Component | Version / policy | Recommendation |
|-----------|------------------|----------------|
| Relay HTTP adapter layer | Current repository contracts and OpenAPI | **Retain as the only billable AI path.** No Chat, Agent, Workflow, Knowledge, MCP, or `/v1/*` handler may call a provider around Relay. |
| OpenAI-compatible Chat Completions | Preserve current schema and SSE contract | **Retain.** Official OpenAI docs still support Chat Completions; do not break current clients to force a Responses migration. **[LOW-external]** |
| Responses API | Current repository handler; capability-gated by provider | **Complete incrementally.** Keep typed event streaming separate from Chat chunks, compare latency/errors/usage, and enable only after lifecycle and billing evidence passes. **[LOW-external]** |
| Request identity | Internal `request_id`/`trace_id`; upstream `X-Client-Request-Id`; capture provider `x-request-id` | **Upgrade behavior without changing topology.** Persist IDs through execution, usage, billing, audit, and support logs. **[LOW-external]** |
| Streaming usage | Provider-specific final-usage event plus cancellation/reconciliation record | **Do not bill from a hoped-for final SSE chunk alone.** OpenAI documents that interrupted streams may omit the final usage chunk. Record cancellation/partial state and reconcile by provider capability. **[LOW-external]** |
| Provider SDKs | None required globally | **Do not introduce a universal vendor SDK layer.** Keep HTTP adapters and add SDKs only where a verified lifecycle feature cannot be implemented safely with existing contracts. |
| Provider versions/models | Capability registry and live contract probes, not hard-coded marketing names | **Version by behavior.** Store supported endpoint/features per channel and fail closed when streaming, usage, pricing, or lifecycle capability is absent. |

### Containers and Kubernetes

| Technology | Version / policy | Purpose | Recommendation |
|------------|------------------|---------|----------------|
| Docker BuildKit/buildx | Current GitHub-hosted supported release, action pinned by full SHA | Reproducible multi-stage images | **Retain.** Select one canonical server and web Dockerfile path, emit the image digest, and rebuild from the release commit. |
| Go builder image | `golang:1.26.5-bookworm` plus digest | Backend build | **Upgrade and unify.** Retire Go 1.22/1.25 release-builder drift. |
| Node builder image | `node:24.18.0-bookworm-slim` plus digest | Web build | **Upgrade.** Validate native/transitive packages from the frozen lock. |
| Web runtime | `nginx:1.30.3-alpine` plus digest | Static UI serving | **Upgrade to current stable, not mainline.** Validate CSP, cache headers, health endpoint, and non-root filesystem behavior. **[LOW-external]** |
| Go runtime base | Patched Alpine 3.23.x plus digest, or the existing Debian family if runtime behavior requires it | Minimal runtime | **Unify rather than switch families casually.** Ensure CA certificates, timezone data, non-root UID, and read-only filesystem work before release. |
| Kubernetes | **1.36.2 target; 1.35 compatibility smoke** | Declared orchestrated deployment | **Retain conditionally.** Kubernetes maintains three recent minors; `kubectl` stays within one minor. Record cluster/server versions in target evidence. **[LOW-external]** |
| Stateful services in Kubernetes | Managed service or vendor-supported operator/version, per target | PostgreSQL, Redis, Qdrant, ClickHouse, Kafka, object storage | **Do not treat raw in-repo manifests as universal production HA.** They remain local/reference assets unless the exact self-hosted profile receives failover, backup/restore, and upgrade evidence. |
| Service mesh / new ingress stack | None for closure | Networking | **Defer.** Existing Service/Ingress/NetworkPolicy and application identity propagation must be proven before adding mesh complexity. |

### Observability

| Technology | Version / policy | Purpose | Recommendation |
|------------|------------------|---------|----------------|
| OpenTelemetry Go | **1.44.0** | Application traces/metrics context | **Retain.** It is current in `go.mod`; add consistent resource attributes and identity propagation, not a second instrumentation API. **[LOW-external]** |
| OTel Collector Contrib | **0.156.0**, exact image digest | Export fan-out, batching, redaction | **Add to the declared target or use an equivalent managed collector.** Pin exact because Collector 0.x components can evolve quickly. **[LOW-external]** |
| Prometheus | **3.13.1**, exact image digest or managed equivalent | Metrics and alerts | **Deploy/verify.** Validate existing alert rules and SLO queries against live data. **[LOW-external]** |
| Grafana | **12.4.5** with ClickHouse datasource **4.19.0** | Dashboards over Prometheus/ClickHouse | **Deploy/verify this compatible pair.** Defer Grafana 13 until the datasource declares/tests compatibility. **[LOW-external]** |
| Error tracking | OTel exporter to the target-owned backend | Exceptions/incidents | **Keep vendor-neutral.** Do not add a product-wide vendor SDK if the collector can export the required data; production must not silently fall back to fake/local telemetry. |

### Supply-Chain and Development Tools

| Tool | Version / policy | Purpose | Required use |
|------|------------------|---------|--------------|
| Syft | **1.46.0** | SPDX JSON SBOM | Generate one SBOM per final image digest and include it in external target evidence. **[LOW-external]** |
| cosign | **3.1.1** | Keyless/container signature and attestation verification | Sign the immutable digest from trusted CI; verify before deployment and in the final release gate. **[LOW-external]** |
| GitHub artifact attestations | `actions/attest-build-provenance` **4.1.1**, pinned to a full commit SHA | Build provenance | Bind repository, workflow, commit, and image digest. Verify with `gh attestation verify` or cosign in the release gate. **[LOW-external]** |
| Trivy | **0.72.0** | Image, filesystem, config, and secret scanning | Block release on unfixed critical/high findings according to an explicit exception policy; retain the separate target secret audit. **[LOW-external]** |
| GitHub Actions | Every action pinned to a full commit SHA with a version comment | CI integrity | Replace mutable `@v4`/`@v5` references in release workflows. A tag alone is not immutable. **[LOW-external]** |
| Renovate or Dependabot | One configured dependency-update path | Patch visibility | Use one bot/policy for Go, npm/pnpm, Actions, and container digests; do not auto-merge stateful-service majors. |

### Verification Tools

| Layer | Tool/version | Release policy |
|-------|--------------|----------------|
| Go correctness | Go 1.26.5 `go test ./... -count=1`, targeted DB integration tests | No package-level skip for release-critical paths. Preserve `TEST_DATABASE_URL` and enforce `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` in CI. |
| Go concurrency | Go 1.26.5 `go test -race -count=1 ./...` | Required on Linux CI or a documented package partition if total runtime is excessive. |
| Go static/security | `go vet`; golangci-lint **2.12.2**; govulncheck **1.6.0** | Pin versions and configuration. Upgrade the current govulncheck 1.3.0 pin. Findings require resolution or an expiring, reviewed exception. **[LOW-external]** |
| Go fuzz | Native Go fuzzing, fixed CI budget plus scheduled longer run | Focus parsers, webhook/signature handling, outbound URL policy, Relay streaming, and evidence JSON. |
| Frontend unit/component | Vitest **4.1.10**, Testing Library 16.x, jsdom 29.1.1 | Patch upgrade from 4.1.8; keep tests deterministic and typecheck separately. **[LOW-external]** |
| Browser journeys | Playwright **1.61.1** with its matching Chromium bundle | Upgrade package and browser together. Critical commercial journeys use the real Go backend, real PostgreSQL, and applicable external services; fixtures do not satisfy target proof. **[LOW-external]** |
| API/contracts | Existing OpenAPI verifier, protobuf generation/diff, provider adapter contract tests | Preserve `/v1/*`, product API, SSE, error, usage, cancellation, and identity behavior. Add live provider rails without placing credentials in artifacts. |
| Database/migrations | PostgreSQL 16.14 + pgvector 0.8.5 integration, migration replay, backup/restore | Test empty install, upgrade from supported prior schema, retry, rollback boundary, HNSW index creation, and cross-tenant denial. |
| Containers/manifests | kubeconform **0.8.0**, ShellCheck **0.11.0**, Hadolint **2.14.0**, Trivy 0.72.0 | Validate rendered manifests, not only source YAML; verify probes, non-root/read-only settings, network policy, PVCs, and immutable images. **[LOW-external]** |
| Performance | k6 **2.1.0** plus Go benchmarks | Gate the approved API/Relay/RAG/Workflow SLOs with a reproducible short release profile; run soak separately and attach results to target evidence. **[LOW-external]** |
| Final commercial proof | Existing `scripts/verify-commercial-completion.sh` and target-evidence collectors/verifier | Remains the final authority. It must run against the same commit/artifact digests with no environment skip for promised journeys. |

## Upgrade Order for the Roadmap

1. **Toolchain and immutable-build baseline:** Node 24.18.0, Go 1.26.5, canonical Dockerfiles, frozen locks, full-SHA Actions, and digest capture.
2. **Primary state floor:** PostgreSQL 16.14/pgvector 0.8.5, Redis 7.4.9, migration replay, backup/restore, and no-skip DB tests.
3. **Production object storage:** keep the S3 contract, remove the legacy MinIO image from production claims, and prove archive/read/recovery behavior on the chosen target.
4. **Relay/provider contract closure:** request identity, cancellation, partial-stream usage handling, pricing/usage joins, and live provider contract evidence.
5. **Observability and supply-chain proof:** collector/backend deployment, dashboards/alerts, SBOM, scan, signature, provenance, and verifier integration.
6. **Conditional topology closure:** only if advertised, stage Qdrant, ClickHouse, Kafka, PgBouncer, and split-service upgrades with rollback and target smoke. Otherwise remove those variants from the release claim instead of making them permanent goals.

## Alternatives Considered

| Recommended | Alternative | When the alternative is justified |
|-------------|-------------|-----------------------------------|
| Go 1.26 in-place | Rewrite services in another framework/language | Not justified for this milestone; only reconsider after measured constraints the Go runtime cannot meet. |
| React 18 + Node 24 | React 19/Router 7 rewrite now | After commercial closure, in an isolated migration with accessibility, streaming, state, and browser regression coverage. |
| PostgreSQL 16 + pgvector baseline | PostgreSQL 18 immediately | A later database lifecycle phase with extension, backup/restore, query-plan, and rollback validation. PostgreSQL 16 has adequate support runway. |
| pgvector baseline | Qdrant required everywhere | Only for targets whose measured vector scale/latency or operational model warrants a dedicated service. |
| Redis/Asynq baseline jobs | Kafka for every queue | Only for the declared split-service topology needing replayable cross-service events. |
| S3-compatible contract | Permanent self-hosted MinIO community dependency | No longer appropriate for commercial production because the legacy community binary line is unmaintained. |
| Raw app manifests plus target-owned stateful services | Add Helm, an operator set, and a service mesh at once | Only when multiple maintained deployment variants create enough repetition to justify the added lifecycle burden. |
| OTel vendor-neutral export | Multiple vendor SDKs in every service | Only where the selected backend requires a capability unavailable through OTLP. |

## What NOT to Introduce

| Avoid | Why | Use instead |
|-------|-----|-------------|
| Greenfield Next.js, microservice, or persistence rewrite | Breaks live contracts and consumes the milestone without closing target journeys | Incremental fixes inside the current Go/React boundaries |
| React 19, Router 7, Tailwind 4, and TypeScript major upgrades in one release slice | Creates a large, unrelated browser regression surface | Node/Vite runtime correction first; framework migrations later |
| PostgreSQL 14 as the production minimum | Official support ends in November 2026 | PostgreSQL 16.14 + pgvector 0.8.5 |
| Floating `latest`, `redis:7`, `pg16`, broad Go/Node, or mutable application tags in release manifests | The same commit can deploy different bytes | Exact patch tags plus recorded OCI digests |
| Legacy `minio/minio:RELEASE.2024-12-18...` for commercial production | Officially unmaintained legacy binary with no future fixes | Maintained S3-compatible target service; legacy image for local compatibility only |
| Requiring pgvector, Qdrant, ClickHouse, Kafka, MinIO replacement, and every split service in every topology | Turns optional infrastructure into permanent product scope | A small set of declared, evidence-backed deployment profiles |
| Direct provider SDK calls outside Relay | Fragments quota, pricing, usage, audit, cancellation, and tenant controls | Relay channel adapters and capability contracts |
| A second metrics/tracing API beside OpenTelemetry | Duplicates instrumentation and fragments trace identity | OTel Go + Collector/exporters |
| Documentation-only or fixture-only readiness | Does not prove target runtime, dependencies, or artifact identity | No-skip browser/DB/provider/deployment evidence bound to release digests |

## Stack Patterns by Deployment Variant

**Baseline commercial application profile:**
- Go server + React static web + PostgreSQL 16/pgvector + Redis 7.4 + maintained S3-compatible storage.
- ClickHouse is required only if this profile advertises high-volume request-log analytics; otherwise use the supported repository path and state the reduced claim explicitly.
- No Kafka or Qdrant requirement unless configured features depend on them.

**Declared split-service profile:**
- Preserve the same HTTP/gRPC/protobuf, tenant identity, Relay, migration, and evidence contracts.
- Add Kafka 4.3 only after consumer/replay proof and provision separate service databases only where the current ownership contract requires them.
- Every advertised service needs real health/readiness, capability parity, migrations, identity propagation, and target smoke; health-only shells are not a deployment mode.

**Managed-infrastructure target:**
- Prefer target-owned PostgreSQL, Redis, Kafka-compatible, ClickHouse, Qdrant, S3-compatible, Prometheus/Grafana, and OTel services when their supported versions satisfy this matrix.
- Record provider/service versions, TLS/auth posture, backup/restore, and evidence identity without storing credentials or raw secret-bearing manifests in git.

**Self-hosted stateful target:**
- Use vendor-supported operators or documented HA patterns per selected service, not the raw local manifest as implied production HA.
- Require failover, backup/restore, upgrade, rollback, capacity, and alert evidence for each stateful dependency actually claimed.

## Version Compatibility Matrix

| Package/service | Compatible target | Notes |
|-----------------|-------------------|-------|
| Go module 1.26 | Go toolchain 1.26.5 | Run full unit, integration, race, vet, lint, and vulnerability gates before changing the module directive. |
| Vite 8.1.4 | Node 24.18.0 | Vite declares Node `^20.19.0 || >=22.12.0`; Node 24 is inside the supported range. |
| React 18.3.1 | Vite 8.1.4 / plugin-react 6.0.2 | Preserve current render and test behavior; React 19 is deferred. |
| PostgreSQL 16.14 | pgvector 0.8.5 | pgvector supports PostgreSQL 13+ and publishes PostgreSQL 16 image variants. |
| pgvector HNSW | Existing vector(1536) migrations | Changing embedding dimensions requires new columns/indexes or versioned collections; it is not an in-place model-name edit. |
| lib/pq 1.10.9 | PostgreSQL 16.14 | Retain current driver semantics; pgx migration is separate. |
| Qdrant 1.18.2 | Existing HTTP adapter after staged snapshot restore | Verify collection schema, payload filters, API behavior, and rollback with target data. |
| ClickHouse 24.12 -> 25.8 -> 26.3.17.4 | clickhouse-go 2.47.0 | Use intermediate LTS or planned downtime because the start-to-target span exceeds the documented compatibility window. |
| Redis 7.4.9 | go-redis 9.14.1 / Asynq 0.25.1 | Keep Redis 8 features disabled until a later compatibility phase. |
| Kafka 4.3.1 | kafka-go 0.4.51 | Conditional split-mode pairing; validate KRaft metadata upgrades and consumer-group semantics. |
| OTel Go 1.44.0 | OTel Collector 0.156.0 over OTLP | Pin Collector components/exporters and test attribute redaction/cardinality. |
| Grafana 12.4.5 | ClickHouse datasource 4.19.0 | The plugin declares Grafana package compatibility around the 12.4.5 line; defer Grafana 13. |
| Playwright 1.61.1 | Its matching Chromium download, Node 24 | Package and browser binaries must move together. |
| Vitest 4.1.10 | Vite 8.1.4, Node 24 | Vitest declares Vite 8 and Node 24 compatibility. |

## Installation and Pinning Policy

Do not run a bulk dependency upgrade. Each stateful service or framework change gets its own lockfile/image-digest diff, migration/rollback notes, and focused verification.

```bash
# Existing dependency graph remains reproducible
pnpm install --frozen-lockfile

# Proposed Go release toolchain gate
(cd src/server && GOTOOLCHAIN=go1.26.5 go test ./... -count=1)
(cd src/server && GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1)

# Existing project entrypoints remain the integration authority
bash scripts/check.sh all
bash scripts/test.sh all
bash scripts/verify-quality-gates.sh
bash scripts/verify-target-release-evidence.sh
```

Release manifests must record exact application and dependency image digests. Patch automation may open updates, but stateful-service majors, React/Router majors, Go majors, and provider lifecycle changes require explicit compatibility and rollback review.

## Sources

### Repository-observed primary sources

- `.planning/PROJECT.md` - brownfield constraints, Relay/tenant/evidence invariants, declared-mode boundary.
- `.planning/codebase/STACK.md` - current mapped stack and commands.
- `src/server/go.mod` - exact backend dependency baseline.
- `src/web/package.json`, root `package.json`, and `pnpm-lock.yaml` - frontend/runtime/test baseline.
- `Dockerfile*`, `docker-compose.yml`, `deploy/docker/`, and `deploy/kubernetes/` - current base images and deployment variants.
- `.github/workflows/ci.yml`, `.github/workflows/release-evidence.yml`, and `scripts/verify-dependency-security.sh` - current CI/security/evidence coverage.

### Current official external sources

- Go release policy and current release: https://go.dev/doc/devel/release and https://go.dev/VERSION?m=text **[LOW-external; verified 2026-07-14]**
- Node release schedule and Vite package metadata: https://github.com/nodejs/Release/blob/main/schedule.json and https://registry.npmjs.org/vite/latest **[LOW-external]**
- PostgreSQL support table: https://www.postgresql.org/support/versioning/ **[LOW-external]**
- pgvector releases/README: https://github.com/pgvector/pgvector/releases and https://github.com/pgvector/pgvector/blob/master/README.md **[LOW-external]**
- Qdrant releases: https://github.com/qdrant/qdrant/releases **[LOW-external]**
- ClickHouse releases and self-managed upgrade guidance: https://github.com/ClickHouse/ClickHouse/releases and https://clickhouse.com/docs/operations/update **[LOW-external]**
- Redis release notes and Apache Kafka releases/upgrade docs: https://github.com/redis/redis/blob/7.4/00-RELEASENOTES and https://kafka.apache.org/43/getting-started/upgrade/ **[LOW-external]**
- MinIO maintenance/source-only notice: https://github.com/minio/minio/blob/master/README.md **[LOW-external]**
- Kubernetes stable release and skew policy: https://dl.k8s.io/release/stable.txt and https://kubernetes.io/releases/version-skew-policy/ **[LOW-external]**
- OpenTelemetry, Prometheus, Grafana, and ClickHouse datasource releases: https://github.com/open-telemetry/opentelemetry-go/releases, https://github.com/open-telemetry/opentelemetry-collector-releases/releases, https://github.com/prometheus/prometheus/releases, https://github.com/grafana/grafana/releases, https://github.com/grafana/clickhouse-datasource/releases **[LOW-external]**
- GitHub artifact attestations/security guidance: https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations and https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions **[LOW-external]**
- OpenAI request IDs, Chat streaming usage, and incremental Responses migration: https://developers.openai.com/api/reference/overview#debugging-requests, https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events#chat.completion.chunk, https://developers.openai.com/api/docs/guides/migrate-to-responses#incremental-rollout-checklist **[LOW-external]**

### Research-provider limitation

The GSD research-plan seam selected Context7 and web search. Context7 CLI/MCP was unavailable in this agent session, and the Brave-backed web-search fallback reported `BRAVE_API_KEY not set`. Direct official pages and registries were used once as the fallback; no repeated provider retries were performed. Patch versions and image digests must be refreshed at release-cut time.

---
*Stack research for the Oblivious subsequent commercial-completion milestone.*
*No greenfield rewrite and no optional topology promoted to a permanent product goal.*

# External Integrations

**Analysis Date:** 2026-07-13

## APIs And External Services

**AI/provider access:**
- Relay is the required integration boundary for AI calls. Product docs state that Chat, Agent workflows, Knowledge RAG, and supported `/v1/*` endpoints must route AI calls through Relay.
- Relay code lives under `src/server/internal/relay/`, with route/security verification through `scripts/verify-relay-security.sh`.
- Provider runtime/config proof is collected through `scripts/collect-provider-runtime-config-evidence.sh` and `scripts/collect_provider_runtime_config_evidence.py`.

**Payments and marketplace rails:**
- Stripe integration uses `github.com/stripe/stripe-go/v82`, with lifecycle code under `src/server/internal/stripe/`.
- Billing, quota, and marketplace settlement code lives under `src/server/internal/billing/`, `src/server/internal/quota/`, `src/server/internal/payment/`, and `src/server/internal/marketplace/`.
- Target live rails include Stripe, Alipay, and WeChat Pay in `docs/release/rc-checklist.md` and target release evidence scripts.

**gRPC APIs:**
- Public service definitions live under `api/proto/agent.proto`, `api/proto/billing.proto`, `api/proto/rag.proto`, `api/proto/relay.proto`, `api/proto/task.proto`, `api/proto/workflow.proto`, and `api/proto/events.proto`.
- Runtime adapters live under `src/server/internal/grpc/` and `src/server/pkg/`.
- Target gRPC proof is collected through `scripts/target-grpc-smoke.sh` and included in target release evidence.

## Data Storage

**Databases:**
- PostgreSQL is the primary product database per `README.md`; pgvector is required for Knowledge/RAG evidence.
- SQL migrations live under `src/server/migrations/`, including `src/server/migrations/microservices/` and `src/server/migrations/clickhouse/`.
- DB-backed commercial evidence uses `scripts/verify-commercial-db-evidence.sh` and `TEST_DATABASE_URL`.
- ClickHouse is used for request-log / usage analytics surfaces through collectors such as `scripts/collect-request-log-observability-evidence.sh`.

**Cache / queue / event systems:**
- Redis and Asynq dependencies in `src/server/go.mod` support queue/runtime paths.
- Kafka dependency and deployment assets under `deploy/kubernetes/kafka.yaml` support event/microservice paths.
- Qdrant is present in `docker-compose.yml` and `deploy/kubernetes/qdrant.yaml`.

**File/artifact storage:**
- Target release artifacts are expected outside git and validated by `scripts/collect-target-release-artifacts.sh`, `scripts/compute-target-release-digests.sh`, and `scripts/verify-target-release-evidence.sh`.
- Do not commit target manifests, downloaded artifacts, backup dumps, provider logs, or secret audit output.

## Authentication And Identity

- Custom auth/session behavior is implemented under `src/server/internal/auth/` and tested through `src/server/internal/http/auth_middleware_test.go`, `src/server/internal/http/auth_response_test.go`, and related route tests.
- Tenant/organization membership behavior is implemented under `src/server/internal/tenant/` and verified through commercial DB evidence profiles in `scripts/verify-commercial-db-evidence.sh`.
- Frontend auth/marketing routes live under `src/web/src/routes/marketing/`.

## Monitoring And Observability

- Prometheus and OpenTelemetry dependencies are present in `src/server/go.mod`.
- Observability code lives under `src/server/internal/observability/`, `src/server/internal/metrics/`, and `src/server/pkg/metrics/`.
- Grafana and Prometheus assets live under `deploy/observability/grafana-dashboard.json` and `deploy/observability/prometheus-alerts.yaml`.
- Operator docs include `docs/release/observability-slos.md`, `docs/release/incident-response-runbook.md`, and `docs/release/disaster-recovery-runbook.md`.

## CI/CD And Deployment

- CI workflow uses Node 20.19.0 and DB-backed server integration behavior in `.github/workflows/ci.yml`.
- Docker deployment files include root `Dockerfile.server`, `Dockerfile.web`, `Dockerfile.postgres-pgvector`, and service-specific Dockerfiles under `deploy/docker/`.
- Kubernetes resources live under `deploy/kubernetes/`, including service deployments, ingress, HPA, network policy, Postgres, Redis, Kafka, Qdrant, ClickHouse, MinIO, and PgBouncer manifests.
- Deployment validation scripts include `scripts/deploy-validate.sh`, `scripts/deploy-smoke.sh`, `scripts/k8s-validate.sh`, and `scripts/verify-infra-manifests.sh`.

## Environment Configuration

- Required/common variables include `DATABASE_URL`, `TEST_DATABASE_URL`, `SESSION_SECRET`, `VITE_API_PROXY_TARGET`, `COREPACK_HOME`, `GOCACHE`, `GOMODCACHE`, and target-release `OBLIVIOUS_TARGET_*` values.
- Committed files should contain placeholders only. Filled `.env`, Kubernetes secrets, provider keys, target manifests, downloaded artifacts, backup dumps, and kubeconfig material belong outside git.

## Webhooks And Callbacks

- Stripe and domestic payment webhook behavior is tested under `src/server/internal/http/stripe_handler_test.go`, `src/server/internal/http/domestic_payment_webhook_handler_test.go`, and `src/server/internal/http/marketplace_payout_webhook_handler_test.go`.
- Marketplace paid-install and payout behaviors live under `src/server/internal/marketplace/` and HTTP route tests under `src/server/internal/http/`.
- Alert/notification paths live under `src/server/internal/notification/` and `src/server/internal/observability/`.

## Integration Guardrails

- New AI calls must pass through Relay or a documented outbound policy boundary.
- New URL-fetching or web-search integrations must use `src/server/internal/outboundpolicy/` and include SSRF/egress-policy tests.
- New payment/provider behavior must include route tests, lifecycle tests, release evidence profile updates, and target/live evidence guidance.
- New target evidence fields must update collectors, assembler, verifier, fixture mutations, fixture scripts, and release docs together.

---

*Integration audit: 2026-07-13*

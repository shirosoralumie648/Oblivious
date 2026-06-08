# RC Checklist

This checklist is the minimum release-candidate gate for the current Oblivious mainline. It covers only `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, and release docs. `lobehub/` and `new-api/` remain reference trees.

Commercial readiness is a stricter gate than RC readiness. Use [`commercial-gates.md`](commercial-gates.md) before claiming commercial readiness or commercial completeness.

No live provider keys required for docs checks, web tests, server tests, or Admin/Marketplace E2E. Use placeholder environment values unless a deployment smoke target explicitly needs real infrastructure.

## Automated Gates

| Gate | Command | Required env | Evidence |
| --- | --- | --- | --- |
| Docs and release assets | `bash scripts/check.sh docs` | none | Command output or CI `release-gates` URL |
| Web production build | `bash scripts/check.sh web` | `COREPACK_HOME=.tmp/corepack` optional | Command output or CI `web` URL |
| Server release checks | `bash scripts/check.sh server` | `GOCACHE=.tmp/go-build` and `GOMODCACHE=.tmp/go-mod` optional | Compile-check command output or CI `server` URL |
| Web Vitest suite | `bash scripts/test.sh web` | `COREPACK_HOME=.tmp/corepack` optional | Command output or CI `web` URL |
| Server unit and integration tests | `bash scripts/test.sh server` | CI requires `TEST_DATABASE_URL` and `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`; local runs may omit `TEST_DATABASE_URL` only with an explicit skip note; DB-backed CI runs are serialized with `go test -p 1` | Command output, CI `server` URL, or explicit local skip note |
| Admin/Marketplace browser E2E | `bash scripts/test.sh e2e` | Playwright Chromium installed with `pnpm --dir src/web exec playwright install --with-deps chromium` | Command output or CI `e2e` URL |
| Full local gate | `bash scripts/check.sh all && bash scripts/test.sh all` | Same as component gates | Combined terminal log |
| Docker compose smoke | `bash scripts/deploy-validate.sh` | Docker daemon access, registry/proxy access for base image pulls, compose, pgvector-capable PostgreSQL image, placeholder `DATABASE_URL`/`SESSION_SECRET` values from compose | Compose logs plus migration and smoke output |
| PostgreSQL backup/restore smoke | `bash scripts/backup-restore-smoke.sh` | Docker or disposable PostgreSQL URLs; pgvector-capable image; no production secrets | Backup manifest path, restore log, `schema_migrations` checksum result, and fixture verification output |
| Observability artifacts | `rg -n "RelayOutage|QuotaSettlementFailure|StripeWebhookFailure|MigrationFailure|HighProviderErrorRate|TenantIsolationIncident|WorkflowExecutionFailureRate|WorkflowExecutionStuck|WorkflowQueueBacklog|workflow_execution_active|workflow_execution_active_age_seconds|workflow-execution|OPS-04|OPS-05|model usage|feature usage|user cost|time trend|cross-dimension usage/cost analytics" deploy/observability docs/release/observability-slos.md docs/release/commercial-gates.md docs/release/rc-checklist.md docs/product/operator-guide.md docs/release/incident-response-runbook.md && node scripts/verify-observability-dashboard.mjs` | none | Alert-rule, dashboard, SLO, workflow active execution health, and usage/cost analytics references plus explicit no-final-readiness boundary |
| Operations runbooks and v07 evidence | `rg -n "release-rollback-runbook|incident-response-runbook|disaster-recovery-runbook|v07-operations-evidence|OPS-06|DOC-06|no-final-readiness" docs/release scripts/verify-quality-gates.sh` | none | Release/rollback, incident, DR, and v07 evidence references |
| Kubernetes smoke | `kubectl apply -f deploy/kubernetes/namespace.yaml && kubectl apply -f deploy/kubernetes/` | Filled secret derived from `deploy/kubernetes/secret.example.yaml` outside git | Cluster rollout status and `/healthz` smoke output |

## Integration-Test Skip Semantics

`TEST_DATABASE_URL` is optional only for local server gates. When it is unset locally, `bash scripts/test.sh server` must print `Skipping server integration tests: TEST_DATABASE_URL not set.` and the release evidence must record that DB-backed integration tests were skipped intentionally. When it is set, the server gate runs all packages serially to avoid shared test schema resets racing across Go packages.

CI server gates set `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`. With that flag, missing `TEST_DATABASE_URL` is a failure and must print `TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true.`

When `TEST_DATABASE_URL` is set, the evidence must include the exact value class, not the secret value. Acceptable examples:

- `TEST_DATABASE_URL` pointed at disposable local Postgres.
- `TEST_DATABASE_URL` pointed at CI service Postgres.
- `TEST_DATABASE_URL` omitted in a local run; integration tests skipped by explicit rule.

## Documentation Evidence

- `docs/API.md` is the canonical API index for the current routed surface.
- `docs/architecture/current-system-contracts.md` is the current behavior contract for HTTP envelope, session cookie, routes, environment variables, and release commands.
- `config/.env.example` is the environment variable source for local and deployment examples.
- `.planning/phases/04-quality-release/*-SUMMARY.md` files are accepted as phase-level evidence once their verification commands are recorded.

## Deployment Preconditions

- Use only environment variable names and placeholders in committed config.
- If Docker daemon access, Docker registry/proxy access, or Kubernetes tooling is unavailable, follow [`deployment-runtime-remediation.md`](deployment-runtime-remediation.md) before recording DEPLOY-01 evidence.
- Copy `deploy/kubernetes/secret.example.yaml` to an untracked secret manifest before real cluster use.
- For local Docker smoke, run `bash scripts/deploy-validate.sh`. The script runs `docker compose config`, `docker compose build`, bounded `docker compose up -d` calls, `/usr/local/bin/oblivious-migrate`, then `BASE_URL=http://127.0.0.1:8080 bash scripts/deploy-smoke.sh`. Set `KEEP_STACK=true` to leave the compose stack running after validation, and tune `DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS` only when a slow mirror is still making progress.
- If Docker Hub or `proxy.golang.org` is unreliable, use the validated restricted-network form: `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`.
- If pgvector image pulls are blocked, build the local fallback image with `docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .`, then run `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/deploy-validate.sh`.
- For PostgreSQL recovery smoke, run `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh`. See [`backup-restore-runbook.md`](backup-restore-runbook.md) for backup, restore, retention, encryption, and evidence rules.
- For OPS-04/OPS-05 observability evidence, review [`observability-slos.md`](observability-slos.md), `deploy/observability/prometheus-alerts.yaml`, and `deploy/observability/grafana-dashboard.json`, then run `node scripts/verify-observability-dashboard.mjs`. The Grafana dashboard artifact should include usage analytics dashboard coverage for model usage, feature usage, user cost, time trend, cross-dimension usage/cost analytics, and workflow active execution health. These artifacts prove repository-local metrics, alert, dashboard, and SLO coverage; external Prometheus/Grafana/OTel/error-tracking deployment and live dashboard rendering remain environment-specific evidence.
- For OPS-06/DOC-06 operations closeout evidence, review [`release-rollback-runbook.md`](release-rollback-runbook.md), [`incident-response-runbook.md`](incident-response-runbook.md), [`disaster-recovery-runbook.md`](disaster-recovery-runbook.md), and [`v07-operations-evidence.md`](v07-operations-evidence.md). Phase 24 verification records exact commands, skipped checks, residual v08 work, and the no-final-readiness boundary.
- For Kubernetes smoke, build and load/push `oblivious-server:local` and `oblivious-web:local`, fill the secret from `deploy/kubernetes/secret.example.yaml`, then run `kubectl apply -f deploy/kubernetes/namespace.yaml` and `kubectl apply -f deploy/kubernetes/`.
- Do not commit provider keys, Stripe secrets, database passwords, session secrets, or kubeconfig material.
- The deployment smoke target must expose `/healthz` before the release can be called healthy.

## Known accepted debt

- Phase 01 has a missing `SUMMARY.md` artifact tracked as backlog 999.1.
- Legacy `src/web/src/routes/workspace/MarketplacePage.tsx` cleanup was resolved in Phase 999.2; `/marketplace` routes through the active Marketplace route tree.
- Future milestone close policy is recorded in Phase 999.2: `.planning/REQUIREMENTS.md` stays as cross-phase context while milestone snapshots are archived separately.

## Manual Release Review

- [ ] No P0/P1 defects open.
- [ ] Release notes summarize scope, verification evidence, skip reasons, and known accepted debt.
- [ ] Environment variables match [`config/.env.example`](../../config/.env.example).
- [ ] API contract changes are reflected in [`docs/API.md`](../API.md) and [`docs/architecture/current-system-contracts.md`](../architecture/current-system-contracts.md).
- [ ] CI URLs or local command logs are attached for every automated gate above.

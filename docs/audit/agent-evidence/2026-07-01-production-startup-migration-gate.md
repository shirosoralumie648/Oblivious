# Production Startup and Migration Gate Evidence - 2026-07-01

## Scope

- Prevent production server pods from serving before PostgreSQL migrations are applied.
- Make migration execution safe under concurrent server rollout or manual migrator runs.
- Split liveness and readiness probes so Kubernetes does not route traffic to an unready pod.
- Add bounded database startup behavior.

## Implementation

- `src/server/internal/migrations/migrations.go`
  - Extracted the migrator into a reusable internal package.
  - Wraps each migration run in a PostgreSQL advisory lock held on one connection for the full batch.
  - Re-checks the migration ledger while holding the lock, so concurrent pods do not race DDL.

- `src/server/cmd/migrate/main.go`
  - Uses the shared migrator package.
  - Keeps wrapper functions for existing `cmd/migrate` tests.

- `src/server/cmd/server/main.go`
  - Runs migrations before constructing and listening with the HTTP server.
  - Fails startup if migrations fail.

- `src/server/internal/http/router.go`
  - Keeps `/healthz` as a compatibility liveness response.
  - Adds `/livez` for process liveness.
  - Adds `/readyz` that pings PostgreSQL and, outside `APP_ENV=test`, requires `schema_migrations` to exist.

- `src/server/internal/db/db.go`
  - Adds `PingContext` with a 10 second startup timeout.
  - Adds bounded default PostgreSQL connection pool settings.

- `deploy/kubernetes/*.yaml` and `Dockerfile.server`
  - Liveness now points to `/livez`.
  - Readiness/startup/container health checks now point to `/readyz`.
  - Kubernetes ConfigMap includes the production-required request log backend settings.

## Verification

- Passed:
  - `git diff --check -- Dockerfile.server deploy/kubernetes/app-deployment.yaml deploy/kubernetes/configmap.yaml deploy/kubernetes/server.yaml src/server/cmd/migrate/main.go src/server/cmd/server/main.go src/server/internal/db/db.go src/server/internal/http/router.go src/server/internal/http/server_test.go src/server/internal/migrations/migrations.go src/server/internal/quota/service.go src/server/internal/quota/service_test.go docs/audit/agent-evidence/2026-07-01-quota-atomic-preauthorization.md`

- Blocked by local toolchain:
  - `gofmt -w ...`
    - Failed because `gofmt` is not on PATH.
  - `go test ./cmd/migrate ./cmd/server ./internal/db ./internal/http ./internal/quota -run ...`
    - Failed because `go` is not on PATH.

## Residual Risk

- ClickHouse request logging is configured for production manifests, but the repository still needs final proof that the runtime image includes a real ClickHouse SQL driver and that target ClickHouse migrations are applied.
- Background workers are cancelled on shutdown but not explicitly drained with a `WaitGroup`.
- Final release still needs full Go test, Docker build, migration replay, Kubernetes smoke, and provider/live-secret evidence in an environment with the required toolchain.

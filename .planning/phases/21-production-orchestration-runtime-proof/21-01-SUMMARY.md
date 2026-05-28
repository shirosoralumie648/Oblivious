# Phase 21 Summary: Production Orchestration Runtime Proof

## Outcome

Phase 21 closes `OPS-01` with equivalent Docker compose orchestration proof. The stack now builds server/web images, starts pgvector-capable PostgreSQL and Redis, runs `/usr/local/bin/oblivious-migrate`, starts the application stack, and proves `/healthz`, `/metrics`, `/api/v1/auth/me`, and `/v1/chat/completions` without live provider secrets.

`OPS-02` remains partially open for later v07 evidence. Phase 21 records restricted-network proof and missing Kubernetes tooling behavior, but the final release/rollback evidence bundle remains Phase 24.

## Changes

- Added `Dockerfile.postgres-pgvector` as a local fallback for hosts that cannot pull `pgvector/pgvector:pg16`.
- Made compose host ports configurable through `OBLIVIOUS_SERVER_HOST_PORT` and `OBLIVIOUS_WEB_HOST_PORT`; `deploy-validate.sh` derives `BASE_URL` from the configured server host port.
- Fixed Relay runtime mounting by registering concrete Relay handlers inside `relay.NewRelay`; `/v1/chat/completions` now reaches production policy and returns 401 for missing trusted internal identity instead of 404.
- Added a focused Relay test proving the commercial chat route is registered.
- Kept Kubernetes validation strict: missing `kubectl` exits non-zero and is recorded as unavailable proof, not success.

## Verification

- `BASE_URL=http://127.0.0.1:1 DEPLOY_SMOKE_ATTEMPTS=1 DEPLOY_SMOKE_SLEEP_SECONDS=0 bash scripts/deploy-smoke.sh` failed closed as expected.
- `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` failed with missing `kubectl` as expected.
- `DOCKER_BUILDKIT=1 docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .` passed.
- Local pgvector container accepted `CREATE EXTENSION vector` and returned vector distance `1`.
- `cd src/server && go test ./internal/relay -run TestNewRelayRegistersCommercialChatRoute -count=1` passed after the Relay handler registration fix.
- `OBLIVIOUS_SERVER_HOST_PORT=18080 OBLIVIOUS_WEB_HOST_PORT=14173 OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh` passed.

## Residual Work

- Phase 22: backup/restore and migration recovery smoke.
- Phase 23: observability, alerts, dashboards, and SLOs.
- Phase 24: release, rollback, incident, DR runbooks, final v07 evidence, and remaining `OPS-02` release-path proof.
- v08: product completeness and final commercial readiness.

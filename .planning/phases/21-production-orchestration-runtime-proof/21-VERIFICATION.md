# Phase 21 Verification: Production Orchestration Runtime Proof

**Status:** Complete for Phase 21 compose/equivalent orchestration proof. Kubernetes proof remains unavailable in this environment.
**Updated:** 2026-05-28

## Scope

Phase 21 proves migration-aware compose validation, Kubernetes validation entrypoint behavior, and shared app/Relay runtime smoke for `OPS-01` plus the Phase 21 slice of `OPS-02`.

This evidence does not close backup/restore, observability, runbooks, v07 closeout, v08 product completeness, or final commercial readiness.

## Evidence

| Command | Result | Evidence |
| --- | --- | --- |
| `BASE_URL=http://127.0.0.1:1 DEPLOY_SMOKE_ATTEMPTS=1 DEPLOY_SMOKE_SLEEP_SECONDS=0 bash scripts/deploy-smoke.sh` | Expected fail, exit 1 | Smoke fails closed on unreachable `/healthz` with status `000`. |
| `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` | Expected fail, exit 127 | Current environment prints `[k8s-validate] kubectl is required`; this is not Kubernetes proof. |
| Fake Docker RED test for hung `docker compose up` before timeout handling | Expected fail, exit 124 | Previous script behavior was killed by outer `timeout` with only `Terminated`, proving missing project-level diagnostics. |
| Fake Docker GREEN test for hung `docker compose up` after timeout handling | Pass for diagnostic behavior, exit 5 | `deploy-validate.sh` prints bounded-timeout remediation for `postgres redis` when `DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS=1`. |
| `timeout 45 docker manifest inspect docker.m.daocloud.io/pgvector/pgvector:pg16` | Pass | The pgvector tag exists and returns an OCI manifest. |
| `timeout 300 docker pull docker.m.daocloud.io/pgvector/pgvector:pg16` | Fail, exit 124 | Layer download did not complete in the bounded window, so direct mirror pull is not a reliable local proof path. |
| `timeout 300 docker pull docker.1ms.run/pgvector/pgvector:pg16` | Fail, exit 124 | Alternate mirror did not complete in the bounded window. |
| `DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS=45 OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh` | Fail, exit 5 | Server and web images built from cache, then compose timed out while pulling `docker.m.daocloud.io/pgvector/pgvector:pg16`; script printed registry/pre-pull remediation and cleaned up compose resources. |
| `DOCKER_BUILDKIT=1 docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .` before removing the remote Dockerfile frontend directive | Expected fail, exit 1 | BuildKit failed while resolving `docker.io/docker/dockerfile:1`, proving the fallback image path still depended on Docker Hub before the fix. |
| `DOCKER_BUILDKIT=1 docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .` after removing the remote Dockerfile frontend directive | Pass | Local fallback pgvector image built successfully as `oblivious-postgres-pgvector:pg16`. |
| `docker run ... oblivious-postgres-pgvector:pg16` plus `CREATE EXTENSION IF NOT EXISTS vector; SELECT '[1,2,3]'::vector <-> '[1,2,4]'::vector` | Pass | PostgreSQL accepted the pgvector extension and returned vector distance `1`. |
| `OBLIVIOUS_SERVER_HOST_PORT=18080 OBLIVIOUS_WEB_HOST_PORT=14173 docker compose config` | Pass | Compose rendered server port `18080:8080`, web port `14173:80`, and matching CORS origins for port-conflict environments. |
| `cd src/server && go test ./internal/relay -run TestNewRelayRegistersCommercialChatRoute -count=1` before registering Relay handlers | Expected fail | `/v1/chat/completions` returned 404, proving the Relay route was not mounted in `relay.NewRelay`. |
| `cd src/server && go test ./internal/relay -run TestNewRelayRegistersCommercialChatRoute -count=1` after registering Relay handlers | Pass | Production Relay chat route now returns 401 for missing trusted internal identity instead of 404. |
| `OBLIVIOUS_SERVER_HOST_PORT=18080 OBLIVIOUS_WEB_HOST_PORT=14173 OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh` | Pass | Server/web images built, pgvector Postgres and Redis started, `oblivious-migrate` ran with `migrations applied: 0, skipped: 30`, server/web started, and smoke passed for `/healthz`, `/metrics`, `/api/v1/auth/me` with 401, and `/v1/chat/completions` with 401. |

## Finding

`src/server/migrations/0016_pgvector.sql` requires `CREATE EXTENSION vector`, so plain `postgres:16` is not a valid Phase 21 database runtime. Compose, Kubernetes Postgres, and CI point at `pgvector/pgvector:pg16` for normal environments. On this host, mirror pulls for that image timed out, so Phase 21 uses the repository-local `Dockerfile.postgres-pgvector` fallback image as the equivalent compose runtime proof.

The compose proof also found a real Relay mounting regression: `relay.NewRelay` created an empty handler map, so `/v1/chat/completions` returned 404 even when `RELAY_ENABLED=true`. The fix registers the Relay handlers and keeps production requests without trusted internal identity at the policy layer with 401.

## Closed In Phase 21

- `OPS-01`: Complete for equivalent compose orchestration proof. The stack builds, starts, applies migrations, and proves health, metrics, app, and Relay routes without live provider secrets.
- Phase 21 portion of `OPS-02`: Complete for restricted-network evidence and explicit Kubernetes tooling failure evidence.

## Residual Work

- Kubernetes proof remains unavailable until `kubectl` and a reachable cluster plus an untracked filled secret manifest are provided.
- Default normal-network image pulls remain unverified on this host because only mirror-prefixed Go/Node/Nginx base images are currently cached; Phase 24 must record final release-path evidence for the target deployment environment.
- `OPS-02` remains partially open for Phase 24 release/rollback evidence across the final v07 operations bundle.
- `OPS-03` backup/restore smoke remains Phase 22.
- `OPS-04` and `OPS-05` observability, alert, dashboard, and SLO evidence remain Phase 23.
- `OPS-06` and `DOC-06` runbook and v07 closeout evidence remain Phase 24.
- v08 product completeness and final commercial readiness remain open.

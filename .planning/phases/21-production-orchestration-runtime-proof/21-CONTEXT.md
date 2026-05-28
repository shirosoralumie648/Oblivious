# Phase 21 Context: Production Orchestration Runtime Proof

## Milestone

v07 Production Operations.

## Why This Phase Exists

v04 proved the tenant/security/migration foundation, v05 proved Relay authority and billing semantics, and v06 proved commercial money movement plus Marketplace governance. The product still cannot be called commercially operable until a production team can start the stack, run migrations, and smoke the app and Relay surfaces from reproducible deployment commands.

The repository already has Docker compose, Kubernetes manifests, a `/healthz` endpoint, a `/metrics` endpoint, and a compose validation script. The current deployment proof is still release-candidate level: `scripts/deploy-smoke.sh` only checks `/healthz`, `scripts/deploy-validate.sh` does not prove the migration binary ran before declaring success, and there is no executable Kubernetes validation script that waits for rollouts and reuses the same runtime smoke.

## Requirements

- **OPS-01:** Kubernetes or equivalent production orchestration validation starts the actual stack, applies migrations, proves `/healthz`, and proves app and Relay paths without live provider secrets.
- **OPS-02:** Runtime smoke covers both normal network and restricted-network deployment paths, including documented proxy/registry overrides and explicit evidence when cluster tooling is unavailable.

## Current Evidence And Gaps

Existing deployment assets:

- `docker-compose.yml` starts PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`.
- `src/server/migrations/0016_pgvector.sql` requires PostgreSQL with the `vector` extension.
- `Dockerfile.server` builds both `/usr/local/bin/oblivious-server` and `/usr/local/bin/oblivious-migrate`, and copies `src/server/migrations` into the runtime image.
- `scripts/deploy-validate.sh` renders compose config, builds images, starts the compose stack, and calls `scripts/deploy-smoke.sh`.
- `scripts/deploy-smoke.sh` currently verifies only `GET /healthz`.
- `deploy/kubernetes/*.yaml` defines namespace, config, secret example, Postgres, Redis, server, and web manifests with health probes.
- `src/server/internal/http/router.go` exposes `GET /healthz` and `GET /metrics`.
- `docs/release/deployment-runtime-remediation.md` records a previously validated restricted-network compose path and says `kubectl` was not installed at that time.

Current gaps:

- Compose validation does not run `/usr/local/bin/oblivious-migrate` against the compose database before marking the stack healthy.
- Plain `postgres:16` cannot satisfy `CREATE EXTENSION vector`; runtime database images must be pgvector-capable.
- Runtime smoke does not prove `/metrics`, app API routing, or Relay routing.
- There is no `scripts/k8s-validate.sh` for real/local cluster validation.
- Kubernetes validation instructions are prose-only; they do not enforce secret handling, rollout waiting, port-forward smoke, or explicit non-success status when cluster tooling is absent.
- Current environment has Docker installed, but `kubectl`, `kind`, and `minikube` were not found. Phase 21 must record this as missing cluster tooling if it remains true during verification, not as a Kubernetes pass.

## Boundaries

Included:

- Make compose deployment validation migration-aware.
- Expand runtime smoke so deployment proof covers `/healthz`, `/metrics`, an app API route, and a Relay route without live provider credentials.
- Add Kubernetes validation script for the existing manifests with explicit prerequisites, secret-file handling, rollout waits, port-forward smoke, and cleanup behavior.
- Update deployment docs, commercial gates, and planning artifacts to show v07 is active and Phase 21 owns orchestration proof.
- Verify normal and restricted-network compose commands where the local environment permits; record exact failures or skipped cluster checks.

Excluded:

- Backup/restore implementation and runbooks; Phase 22 owns `OPS-03`.
- Structured logging, tracing, alert rules, dashboards, and SLOs; Phase 23 owns `OPS-04` and `OPS-05`.
- Release, rollback, incident, and disaster recovery closeout; Phase 24 owns `OPS-06` and `DOC-06`.
- Live LLM provider calls, live Stripe calls, committed secrets, or external payout execution.
- Claiming v07 completion, v08 product completeness, or final commercial readiness.

## Runtime Smoke Design

Phase 21 should keep one smoke contract shared by compose and Kubernetes:

- `GET /healthz` must return success.
- `GET /metrics` must return Prometheus text and prove the metrics route is mounted.
- An app API route such as `GET /api/v1/auth/me` must return an expected unauthenticated response instead of `404`.
- A Relay route such as `POST /v1/chat/completions` must return an expected auth/policy/client error instead of `404` or an upstream network failure. The smoke must not require live provider keys.

The smoke should fail on route-not-mounted status, connection failure, blank response, or provider-network evidence that implies the request bypassed local auth/policy handling.

## Kubernetes Validation Design

The Kubernetes script should:

1. Require `kubectl` and a reachable context before applying manifests.
2. Require a secret manifest path through an environment variable such as `OBLIVIOUS_K8S_SECRET_FILE`; it must not apply `deploy/kubernetes/secret.example.yaml` as production proof.
3. Apply namespace, secret, configmap, Postgres, Redis, server, and web manifests in deterministic order.
4. Wait for `deployment/oblivious-server` and `deployment/oblivious-web` rollout status in the `oblivious` namespace.
5. Port-forward the server service to a local port and run the shared runtime smoke against that URL.
6. Support cleanup through an environment variable, while defaulting to non-destructive validation behavior.

## Verification Targets

Focused RED/GREEN:

- `bash scripts/deploy-smoke.sh` against no server should fail clearly.
- `bash scripts/deploy-smoke.sh` against a running server should prove health, metrics, app, and Relay route coverage.
- `bash scripts/k8s-validate.sh` without `kubectl` should fail with a clear prerequisite message and non-zero exit.

Broader phase verification:

- `bash scripts/deploy-validate.sh`
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=docker.m.daocloud.io/pgvector/pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`
- `bash scripts/k8s-validate.sh` when `kubectl` plus a local or real cluster are available.
- `bash scripts/check.sh docs`
- `git diff --check`

## Residual Risk After This Phase

Phase 21 should close only the orchestration-runtime portion of v07. The product remains non-commercial-complete until Phase 22 proves backup/restore, Phase 23 proves observability/alerts/SLOs, Phase 24 closes release/rollback/incident/DR evidence, and v08 removes remaining product placeholders and completes customer-facing commercial journeys.

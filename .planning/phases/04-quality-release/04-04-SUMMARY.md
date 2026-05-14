---
phase: 04-quality-release
plan: 04
subsystem: deployment
tags: [docker, kubernetes, release-gate, smoke]

requires:
  - phase: 04-quality-release
    provides: TEST-01, TEST-02, and DOC-01 release gates
provides:
  - Docker image build definitions for the active src/server and src/web mainline
  - Docker compose stack for PostgreSQL, Redis, server, and web
  - Kubernetes namespace, config, secret example, data services, server, and web manifests
  - Deployment smoke script for /healthz validation
affects: [04-quality-release, DEPLOY-01, release-candidate]

tech-stack:
  added: []
  patterns:
    - Deployment examples use env var names and placeholders only
    - Docker/Kubernetes assets target src/server and src/web, not imported reference trees
    - Runtime smoke validation is centralized in scripts/deploy-smoke.sh

key-files:
  created:
    - Dockerfile.server
    - Dockerfile.web
    - .dockerignore
    - docker-compose.yml
    - deploy/kubernetes/namespace.yaml
    - deploy/kubernetes/configmap.yaml
    - deploy/kubernetes/secret.example.yaml
    - deploy/kubernetes/postgres.yaml
    - deploy/kubernetes/redis.yaml
    - deploy/kubernetes/server.yaml
    - deploy/kubernetes/web.yaml
    - scripts/deploy-smoke.sh
    - scripts/deploy-validate.sh
  modified:
    - Dockerfile.server
    - Dockerfile.web
    - docker-compose.yml
    - config/.env.example
    - docs/release/rc-checklist.md
    - docs/release/deployment-runtime-remediation.md
    - scripts/deploy-validate.sh
    - scripts/verify-quality-gates.sh

key-decisions:
  - "Keep Kubernetes namespace manifests valid by asserting metadata.name instead of adding an invalid namespace field."
  - "Keep default Docker Hub and Go module behavior unchanged, but expose build-time registry and Go proxy overrides for restricted networks."
  - "Treat Kubernetes validation as an alternate runtime path; Docker compose smoke is sufficient for DEPLOY-01 once the real stack starts and health-checks."

patterns-established:
  - "RC checklist deployment gates name Docker and Kubernetes commands with explicit secret-copy instructions."
  - "Quality gates protect deploy asset presence, mainline path usage, /healthz probes, and placeholder-only secret examples."
  - "Restricted-network Docker validation can set OBLIVIOUS_IMAGE_REGISTRY_PREFIX, OBLIVIOUS_GOPROXY, and OBLIVIOUS_GOSUMDB without changing committed defaults."

requirements-completed: [DEPLOY-01]
requirements-blocked: []
status: complete

duration: 35min
updated: 2026-05-12
---

# Phase 04 Plan 04: Deployment Configuration And Smoke Path Summary

**The release candidate now has Docker and Kubernetes deployment assets for the active `src/server` + `src/web` stack, plus a repeatable `/healthz` smoke command. Docker compose runtime validation passed on 2026-05-12.**

## Accomplishments

- Added server and web Dockerfiles for the active mainline.
- Added a Docker compose stack for PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`.
- Added Kubernetes manifests for namespace, config, secret example, PostgreSQL, Redis, server, and web.
- Added `scripts/deploy-smoke.sh` to poll `/healthz` and fail with clear diagnostics.
- Added `scripts/deploy-validate.sh` to run the full Docker compose validation path once daemon access is available.
- Updated release quality gates so deployment assets are checked by `bash scripts/check.sh docs`.
- Corrected the namespace quality-gate assertion to validate `metadata.name: oblivious` instead of requiring an invalid `namespace` field on a Namespace resource.
- Added optional restricted-network overrides for Docker base image prefixes and Go module metadata downloads while preserving default Docker Hub behavior.

## Verification

2026-05-12 completion:

Passed:

```bash
docker compose config
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

Observed results:

- Compose rendered successfully with both default settings and restricted-network overrides.
- `oblivious-oblivious-web` built successfully from `Dockerfile.web`.
- `oblivious-oblivious-server` built successfully from `Dockerfile.server`.
- Compose started PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`.
- PostgreSQL, Redis, and `oblivious-server` reached healthy status.
- `scripts/deploy-smoke.sh` passed against `http://127.0.0.1:8080/healthz`.
- `scripts/deploy-validate.sh` cleaned up the compose stack after the smoke passed.

Earlier 2026-05-12 diagnostics:

Passed:

```bash
id
docker info
docker compose config
docker buildx ls
curl -I --proxy http://127.0.0.1:7897 https://registry-1.docker.io/v2/
docker pull docker.m.daocloud.io/library/postgres:16
docker pull docker.m.daocloud.io/library/redis:7
docker pull docker.m.daocloud.io/library/node:20-bookworm-slim
docker pull docker.m.daocloud.io/library/golang:1.25-bookworm
docker pull docker.m.daocloud.io/library/nginx:1.27-alpine
docker pull docker.m.daocloud.io/library/alpine:3.21
```

Observed results:

- User `shirosora` is now in the `docker` group.
- `docker info` passes and shows the default Linux engine is reachable.
- `docker compose config` renders the stack successfully.
- The default buildx builder is running.
- Docker Hub registry is reachable from the shell through the local proxy at `http://127.0.0.1:7897`.
- Docker daemon direct pulls still time out against Docker Hub, but the Daocloud mirror pulled the required base images.

Failed / blocked diagnostics:

```bash
bash scripts/deploy-validate.sh
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.1ms.run/library/ bash scripts/deploy-validate.sh
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://goproxy.cn,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://goproxy.io,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh
DOCKER_CONFIG="$(mktemp -d)" docker pull hello-world:latest
docker pull hello-world:latest
HTTP_PROXY=http://127.0.0.1:7897 HTTPS_PROXY=http://127.0.0.1:7897 NO_PROXY=localhost,127.0.0.1,::1 docker pull hello-world:latest
kubectl version --client
sudo -n true
```

Observed diagnostic failures:

```text
failed to resolve source metadata for docker.io/docker/dockerfile:1
Head "https://registry-1.docker.io/v2/docker/dockerfile/manifests/1": dial tcp ... i/o timeout
docker.1ms.run/library/golang:1.25-bookworm: missing layer from remote: not found
https://goproxy.cn/...: read: connection reset by peer
https://github.com/twitchyliquid64/golang-asm/: Recv failure: Connection reset by peer
docker pull hello-world:latest: dial tcp ... i/o timeout
/bin/bash: line 1: kubectl: command not found
sudo: 需要密码
```

Current interpretation: Docker daemon permission is fixed and DEPLOY-01 is complete through the Docker compose path. Default Docker Hub and default Go module routes remain unreliable in this host environment, so restricted-network validation should use the documented image registry prefix and Aliyun Go proxy overrides. Kubernetes validation remains unavailable because `kubectl` is not installed, but it is now an optional alternate path rather than a DEPLOY-01 blocker.

Historical 2026-05-04 verification:

Passed:

```bash
bash scripts/check.sh docs
docker compose config
bash scripts/check.sh all
bash scripts/test.sh all
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
BASE_URL=http://127.0.0.1:18080 bash scripts/deploy-smoke.sh
bash -n scripts/deploy-validate.sh
```

Observed results:

- `bash scripts/check.sh docs` passed after deployment quality-gate assertions were aligned with valid Kubernetes syntax.
- `docker compose config` rendered PostgreSQL, Redis, server, web, ports, healthchecks, volumes, and placeholder environment values successfully.
- `bash scripts/check.sh all` passed in the approved non-sandbox path: docs gate, web production build, and server `go test ./... -count=1`.
- `bash scripts/test.sh all` passed in the approved non-sandbox path: web Vitest passed 32 files / 110 tests, server `go test ./... -count=1` passed, and DB-backed HTTP integration tests were explicitly skipped because `TEST_DATABASE_URL` was not set.
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` passed 3/3 Admin and Marketplace browser tests.
- `scripts/deploy-smoke.sh` passed against a temporary local `/healthz` stub, proving the script success path.
- `bash -n scripts/deploy-validate.sh` passed shell syntax validation.

Historical environment-limited checks:

```text
docker build -f Dockerfile.server -t oblivious-server:local .
docker build -f Dockerfile.web -t oblivious-web:local .
bash scripts/deploy-validate.sh
```

Both Docker build commands were attempted and failed before build execution because the local Docker daemon socket was inaccessible:

```text
permission denied while trying to connect to the docker API at unix:///var/run/docker.sock
```

`bash scripts/deploy-validate.sh` also fails clearly before mutating the stack when Docker daemon access is unavailable:

```text
[deploy-validate] docker daemon is not reachable for the current user/session
```

Current environment notes:

- Docker daemon access is now available.
- The stale Docker Desktop credential helper reference was removed from the user Docker config.
- Docker image builds pass when restricted-network overrides are set.
- `kubectl` is not installed in this environment, so Kubernetes apply/dry-run validation cannot be executed locally.
- `docker compose config` and `scripts/deploy-validate.sh` passed.

## Deviations from Plan

- Default Docker Hub and default Go proxy paths could not complete in this network; validation used explicit mirror/proxy environment overrides instead.
- Kubernetes runtime validation could not be completed because `kubectl` is not installed. Docker compose validation satisfied DEPLOY-01.

## Restricted-Network Validation Command

Use this command when Docker Hub or `proxy.golang.org` is unreliable:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

For Kubernetes, copy `deploy/kubernetes/secret.example.yaml` to an untracked secret manifest, fill placeholders outside git, then apply the manifests as documented in `docs/release/rc-checklist.md`.

## Self-Check: PASS

- `04-04-SUMMARY.md` exists.
- Requirement `DEPLOY-01` is complete through a real Docker compose startup and health check.
- No real provider keys or secrets were added.
- Docker registry/proxy and `kubectl` limitations are recorded as environment notes and remediation guidance.

## Phase Readiness

Phase 4 / v03.2 Quality and Release is complete. TEST-01, TEST-02, DOC-01, and DEPLOY-01 are closed; the next workflow step is milestone completion.

---
*Phase: 04-quality-release*
*Audited: 2026-05-12*

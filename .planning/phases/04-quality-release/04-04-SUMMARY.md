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
    - config/.env.example
    - docs/release/rc-checklist.md
    - scripts/verify-quality-gates.sh

key-decisions:
  - "Keep Kubernetes namespace manifests valid by asserting metadata.name instead of adding an invalid namespace field."
  - "Treat Docker daemon registry/proxy access and kubectl availability as operator environment prerequisites, not committed config requirements."
  - "Use docker compose config and project release gates as local verification when daemon access is unavailable."

patterns-established:
  - "RC checklist deployment gates name Docker and Kubernetes commands with explicit secret-copy instructions."
  - "Quality gates protect deploy asset presence, mainline path usage, /healthz probes, and placeholder-only secret examples."

requirements-completed: []
requirements-blocked: [DEPLOY-01]
status: blocked

duration: 35min
updated: 2026-05-12
---

# Phase 04 Plan 04: Deployment Configuration And Smoke Path Summary

**The release candidate now has Docker and Kubernetes deployment assets for the active `src/server` + `src/web` stack, plus a repeatable `/healthz` smoke command. Real Docker/Kubernetes runtime validation is still blocked by the current environment.**

## Accomplishments

- Added server and web Dockerfiles for the active mainline.
- Added a Docker compose stack for PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`.
- Added Kubernetes manifests for namespace, config, secret example, PostgreSQL, Redis, server, and web.
- Added `scripts/deploy-smoke.sh` to poll `/healthz` and fail with clear diagnostics.
- Added `scripts/deploy-validate.sh` to run the full Docker compose validation path once daemon access is available.
- Updated release quality gates so deployment assets are checked by `bash scripts/check.sh docs`.
- Corrected the namespace quality-gate assertion to validate `metadata.name: oblivious` instead of requiring an invalid `namespace` field on a Namespace resource.

## Verification

2026-05-12 recheck:

Passed:

```bash
id
docker info
docker compose config
docker buildx ls
curl -I --proxy http://127.0.0.1:7897 https://registry-1.docker.io/v2/
```

Observed results:

- User `shirosora` is now in the `docker` group.
- `docker info` passes and shows the default Linux engine is reachable.
- `docker compose config` still renders the stack successfully.
- The default buildx builder is running.
- Docker Hub registry is reachable from the shell through the local proxy at `http://127.0.0.1:7897`.

Blocked:

```bash
bash scripts/deploy-validate.sh
DOCKER_CONFIG="$(mktemp -d)" docker pull hello-world:latest
kubectl version --client
```

Observed blocker:

```text
failed to resolve source metadata for docker.io/docker/dockerfile:1
Head "https://registry-1.docker.io/v2/docker/dockerfile/manifests/1": dial tcp ... connect: connection refused
docker pull hello-world:latest: dial tcp ... i/o timeout
/bin/bash: line 1: kubectl: command not found
```

Current interpretation: Docker daemon permission is fixed. DEPLOY-01 is still blocked because the daemon cannot reach Docker Hub registry metadata without proxy/registry configuration, and Kubernetes validation is still unavailable because `kubectl` is not installed. The local Docker client config also references `credsStore: desktop` while `docker-credential-desktop` is unavailable, which should be removed or replaced when using the Linux engine.

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
- Docker image builds still fail while pulling Docker Hub metadata.
- `kubectl` is not installed in this environment, so Kubernetes apply/dry-run validation cannot be executed locally.
- `docker compose config` passed.

## Deviations from Plan

- Real Docker image build and real compose startup could not be completed because Docker daemon registry/proxy access is blocked.
- Kubernetes runtime validation could not be completed because `kubectl` is not installed.
- No committed config gap remains for DEPLOY-01; the remaining runtime checks are operator environment prerequisites documented in the RC checklist.

## User Setup Required

Before a real deployment smoke, configure Docker daemon registry/proxy access or provide Kubernetes tooling, then run:

```bash
docker compose build
docker compose up -d
BASE_URL=http://127.0.0.1:8080 bash scripts/deploy-smoke.sh
```

For Kubernetes, copy `deploy/kubernetes/secret.example.yaml` to an untracked secret manifest, fill placeholders outside git, then apply the manifests as documented in `docs/release/rc-checklist.md`.

## Self-Check: BLOCKED

- `04-04-SUMMARY.md` exists.
- Requirement `DEPLOY-01` is not complete until a real Docker compose or Kubernetes startup path is health-checked.
- No real provider keys or secrets were added.
- Docker registry/proxy and kubectl runtime limitations are recorded as environment prerequisites.

## Phase Readiness

Phase 4 / v03.2 Quality and Release remains blocked on DEPLOY-01 runtime validation. TEST-01, TEST-02, and DOC-01 are closed; DEPLOY-01 has configuration and smoke tooling in place but lacks real startup evidence.

---
*Phase: 04-quality-release*
*Audited: 2026-05-04*

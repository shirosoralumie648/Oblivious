---
phase: 07-frontend-e2e-and-deployment-gate-alignment
plan: 03
status: passed
completed: 2026-05-17
requirements: [DEPLOY-02]
---

# 07-03 Summary: CI and Deployment Gate Alignment

## Outcome

Passed. CI, Dockerfiles, compose, env examples, lockfile, and deployment validation are aligned with the Phase 07 web/E2E gates and preserve the v03.2 restricted-network runtime path.

## Files Changed

- `.github/workflows/ci.yml`
- `Dockerfile.server`
- `Dockerfile.web`
- `config/.env.example`
- `docker-compose.yml`
- `package.json`
- `pnpm-lock.yaml`
- `scripts/deploy-validate.sh`

## CI and Deployment Work

- Confirmed CI runs the real repository gates: docs check, web build/check, web tests, Playwright E2E, server checks, and server tests.
- Kept root workspace metadata scoped to `src/web` and excluded reference trees such as `lobehub` and `new-api`.
- Preserved Docker build args for restricted-network hosts: `OBLIVIOUS_IMAGE_REGISTRY_PREFIX`, `OBLIVIOUS_GOPROXY`, and `OBLIVIOUS_GOSUMDB`.
- Kept server and web images on the active mainline entrypoints, with `/healthz` checks for runtime validation.
- Kept `scripts/deploy-validate.sh` on the full config/build/up/smoke/down flow, using `KEEP_STACK=true` only as an explicit operator opt-in.

## Verification

```bash
bash scripts/check.sh docs
```

Passed. Release assets, docs/env consistency, and workspace boundary checks passed.

```bash
COREPACK_HOME=.tmp/corepack pnpm install --frozen-lockfile
```

Passed. Lockfile was up to date.

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test
```

Passed. Vitest ran 32 test files and 110 tests.

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web build
```

Passed. TypeScript and Vite built successfully.

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
```

Passed. Playwright ran 3 Chromium tests.

```bash
docker compose config
```

Passed. Compose rendered successfully.

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh
```

Passed after clearing a local stale port listener. The script built server and web images, started PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`, passed `/healthz` smoke at `http://127.0.0.1:8080/healthz`, and cleaned up the compose stack.

## Environment Recovery

The first deployment validation attempt built both images but failed at compose startup because `0.0.0.0:8080` was already in use. Investigation showed an orphaned local Go binary from `/tmp/go-build/.../api-server`, started on 2026-05-14 with parent PID 1, listening on `*:8080`. It was not a compose container. After stopping that stale process, the documented restricted-network deploy command passed unchanged.

## Acceptance Criteria

- `.github/workflows/ci.yml` contains the docs, web, E2E, and server release-gate commands.
- `Dockerfile.server` contains `ARG GOPROXY` and `ARG GOSUMDB`.
- `Dockerfile.web` contains `pnpm --dir src/web build`.
- `docker-compose.yml` contains `OBLIVIOUS_IMAGE_REGISTRY_PREFIX`, `OBLIVIOUS_GOPROXY`, and `OBLIVIOUS_GOSUMDB`.
- `scripts/deploy-validate.sh` contains `docker compose config`, `docker compose build`, `docker compose up -d`, and `bash scripts/deploy-smoke.sh`.
- `config/.env.example` contains `RELAY_ENABLED=true` and `OPENAI_BASE_URL=https://api.openai.com`.

## Deviations from Plan

None - plan executed exactly as written. The stale local port listener was an environment recovery step before rerunning the required command.

## Self-Check: PASSED

The CI/deployment files satisfy Plan 07-03, and the restricted-network deployment smoke path has real runtime proof.

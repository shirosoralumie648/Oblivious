# RC Checklist

This checklist is the minimum release-candidate gate for the current Oblivious mainline. It covers only `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, and release docs. `lobehub/` and `new-api/` remain reference trees.

No live provider keys required for docs checks, web tests, server tests, or Admin/Marketplace E2E. Use placeholder environment values unless a deployment smoke target explicitly needs real infrastructure.

## Automated Gates

| Gate | Command | Required env | Evidence |
| --- | --- | --- | --- |
| Docs and release assets | `bash scripts/check.sh docs` | none | Command output or CI `release-gates` URL |
| Web production build | `bash scripts/check.sh web` | `COREPACK_HOME=.tmp/corepack` optional | Command output or CI `web` URL |
| Server release checks | `bash scripts/check.sh server` | `GOCACHE=.tmp/go-build` and `GOMODCACHE=.tmp/go-mod` optional | Command output or CI `server` URL |
| Web Vitest suite | `bash scripts/test.sh web` | `COREPACK_HOME=.tmp/corepack` optional | Command output or CI `web` URL |
| Server unit and integration tests | `bash scripts/test.sh server` | `TEST_DATABASE_URL` optional for DB-backed integration tests | Command output, CI `server` URL, or explicit skip note |
| Admin/Marketplace browser E2E | `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` | Playwright Chromium installed with `pnpm --dir src/web exec playwright install chromium` | Command output or CI `e2e` URL |
| Full local gate | `bash scripts/check.sh all && bash scripts/test.sh all` | Same as component gates | Combined terminal log |
| Docker compose smoke | `bash scripts/deploy-validate.sh` | Docker daemon access, registry/proxy access for base image pulls, compose, placeholder `DATABASE_URL`/`SESSION_SECRET` values from compose | Compose logs plus smoke output |
| Kubernetes smoke | `kubectl apply -f deploy/kubernetes/namespace.yaml && kubectl apply -f deploy/kubernetes/` | Filled secret derived from `deploy/kubernetes/secret.example.yaml` outside git | Cluster rollout status and `/healthz` smoke output |

## Integration-Test Skip Semantics

`TEST_DATABASE_URL` is optional for local and CI server gates. When it is unset, `bash scripts/test.sh server` must print `Skipping server integration tests: TEST_DATABASE_URL not set.` and the release evidence must record that DB-backed HTTP integration tests were skipped intentionally.

When `TEST_DATABASE_URL` is set, the evidence must include the exact value class, not the secret value. Acceptable examples:

- `TEST_DATABASE_URL` pointed at disposable local Postgres.
- `TEST_DATABASE_URL` pointed at CI service Postgres.
- `TEST_DATABASE_URL` omitted; integration tests skipped by explicit rule.

## Documentation Evidence

- `docs/API.md` is the canonical API index for the current routed surface.
- `docs/architecture/current-system-contracts.md` is the current behavior contract for HTTP envelope, session cookie, routes, environment variables, and release commands.
- `config/.env.example` is the environment variable source for local and deployment examples.
- `.planning/phases/04-quality-release/*-SUMMARY.md` files are accepted as phase-level evidence once their verification commands are recorded.

## Deployment Preconditions

- Use only environment variable names and placeholders in committed config.
- If Docker daemon access, Docker registry/proxy access, or Kubernetes tooling is unavailable, follow [`deployment-runtime-remediation.md`](deployment-runtime-remediation.md) before recording DEPLOY-01 evidence.
- Copy `deploy/kubernetes/secret.example.yaml` to an untracked secret manifest before real cluster use.
- For local Docker smoke, run `bash scripts/deploy-validate.sh`. The script runs `docker compose config`, `docker compose build`, `docker compose up -d`, then `BASE_URL=http://127.0.0.1:8080 bash scripts/deploy-smoke.sh`. Set `KEEP_STACK=true` to leave the compose stack running after validation.
- For Kubernetes smoke, build and load/push `oblivious-server:local` and `oblivious-web:local`, fill the secret from `deploy/kubernetes/secret.example.yaml`, then run `kubectl apply -f deploy/kubernetes/namespace.yaml` and `kubectl apply -f deploy/kubernetes/`.
- Do not commit provider keys, Stripe secrets, database passwords, session secrets, or kubeconfig material.
- The deployment smoke target must expose `/healthz` before the release can be called healthy.

## Known accepted debt

- Phase 01 has a missing `SUMMARY.md` artifact tracked as backlog 999.1.
- Legacy `src/web/src/routes/workspace/MarketplacePage.tsx` cleanup is tracked as backlog 999.2 because `/marketplace` now routes through the active Marketplace route tree.
- Future milestone close should decide whether `.planning/REQUIREMENTS.md` stays as cross-phase context or resets per milestone.

## Manual Release Review

- [ ] No P0/P1 defects open.
- [ ] Release notes summarize scope, verification evidence, skip reasons, and known accepted debt.
- [ ] Environment variables match [`config/.env.example`](../../config/.env.example).
- [ ] API contract changes are reflected in [`docs/API.md`](../API.md) and [`docs/architecture/current-system-contracts.md`](../architecture/current-system-contracts.md).
- [ ] CI URLs or local command logs are attached for every automated gate above.

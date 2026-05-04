---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Codebase Concerns

## Current Blocking Concern

DEPLOY-01 runtime validation is blocked.

Evidence:

- `bash scripts/deploy-validate.sh` exits before stack mutation because Docker daemon is not reachable.
- `docker info` reports permission denied on `/var/run/docker.sock`.
- `dockerd` exists but requires root privileges.
- Docker Desktop build socket/buildx reports permission errors.
- `kubectl` is not installed.

Impact:

- Dockerfiles, compose config, Kubernetes manifests, and smoke scripts exist.
- `docker compose config` passes.
- The actual service stack has not been built, started, and health-checked in this environment.
- v03.2 must not be archived or marked shipped until a real Docker or Kubernetes runtime path passes.

Mitigation:

- Follow `docs/release/deployment-runtime-remediation.md`.
- Then run `bash scripts/deploy-validate.sh` or a real Kubernetes equivalent.

## Dirty Worktree Risk

The worktree is large and dirty. Current visible changed/untracked areas include:

- `.planning/STATE.md`, roadmap/requirements/project/milestone files
- release docs under `docs/release/`
- scripts: `check.sh`, `test.sh`, `verify-quality-gates.sh`, `deploy-smoke.sh`, `deploy-validate.sh`
- Docker/Kubernetes release assets
- many pre-existing backend/frontend changes outside the codebase map

Impact:

- Mapping reflects current working tree, not a clean commit.
- Git mode churn exists on mounted filesystem.
- Avoid broad cleanup or reset commands.

Mitigation:

- Keep codebase-map refresh scoped to `.planning/codebase/*.md`.
- Verify exact files before commit.

## Accepted Backlog Debt

Tracked in `.planning/STATE.md` and `.planning/ROADMAP.md`:

- Backlog `999.1`: Phase 01 missing `SUMMARY.md` artifact.
- Backlog `999.2`: legacy `src/web/src/routes/workspace/MarketplacePage.tsx` is no longer routed by `/marketplace`.
- Backlog `999.2`: decide future policy for living `.planning/REQUIREMENTS.md` at milestone close.

Impact:

- These are accepted non-blocking debts, but should not be confused with active v03.2 DEPLOY-01 blocker.

## Runtime/Test Environment Concerns

- Server tests using `httptest.NewServer` can fail in sandboxed execution with local socket restrictions.
- Verified server/web gates required approved non-sandbox execution.
- DB-backed HTTP integration tests remain skipped unless `TEST_DATABASE_URL` is set.

Mitigation:

- Preserve explicit skip semantics in `scripts/test.sh`.
- Record whether `TEST_DATABASE_URL` was unset, local disposable Postgres, or CI service Postgres.
- Do not treat unit test success as DB-backed integration success.

## Deployment Security Concerns

- Committed compose uses placeholder credentials such as `SESSION_SECRET=change-me-in-production` and local Postgres password.
- Kubernetes `secret.example.yaml` uses `REPLACE_ME` placeholders.
- Provider key vars are present but empty in compose/config examples.

Mitigation:

- Keep real secrets out of git.
- Copy and fill `deploy/kubernetes/secret.example.yaml` outside the repo for real clusters.
- Check `docs/release/rc-checklist.md` before any release candidate evidence is recorded.

## Relay/LLM Boundary Risk

Core value: all LLM calls must go through Relay.

Risk areas:

- `src/server/internal/http/router.go` creates local Relay URLs using `localhost:<port>/v1`.
- Chat, Agent, and Memory depend on correct Relay wiring when `RELAY_ENABLED` is true.
- Any future direct provider call in Chat/Agent/Memory would bypass quota/billing/monitoring.

Mitigation:

- Keep `chat.NewRelayGateway`, `agent.Service` gateway injection, and `memory.NewRelayEmbedder` under test.
- Include Relay/Quota/Agent/Memory packages in `go test ./... -count=1`.

## API Surface Drift Risk

Route definitions are spread across:

- `src/server/internal/http/router.go`
- `src/server/internal/relay/handler/router.go`
- `src/web/src/app/router.tsx`
- `docs/API.md`
- `docs/architecture/current-system-contracts.md`
- `docs/release/rc-checklist.md`

Mitigation:

- `scripts/verify-quality-gates.sh` asserts key docs/API/checklist anchors.
- Run `bash scripts/check.sh docs` after route or release command changes.

## Migration Risk

- Migrations are applied by reading and executing every `.sql` file in lexical order.
- No explicit migration ledger is visible in `src/server/cmd/migrate/main.go`.
- Old migrations should remain immutable; fixes should be append-only.

Mitigation:

- Review migration idempotency before using `cmd/migrate` against non-empty databases.
- Continue append-only migration practice.

## Imported Tree Confusion

`lobehub/` and `new-api/` are present and large.

Risk:

- Agents or developers may accidentally map/build/test imported reference trees.

Mitigation:

- `pnpm-workspace.yaml` excludes them.
- `.dockerignore` excludes `new-api` and node_modules under imported trees.
- Release docs explicitly scope gates to `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, and release docs.

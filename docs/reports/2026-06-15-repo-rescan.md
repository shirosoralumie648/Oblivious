# Repository Rescan - 2026-06-15

## Current Truth

- Branch: `main`; this refresh scan starts from pushed commit `95f8ef9 test(security): encrypt admin relay channel api keys`.
- Worktree status at refresh scan start: in sync with `origin/main` with the intended dirty files for the Observability alert-provider config at-rest encryption slice: `secretbox`, the SQL alert-provider config store, the focused HTTP test, and release evidence docs.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this implementation scan is **86/100**. Admin Relay channel API-key and Observability alert-provider config at-rest encryption are now covered with repository-local PostgreSQL proof, but target-environment workflow telemetry, Publishing/Workflow at-rest encryption, target secret audits, deployment validation, and final no-skip release readiness remain open.

## What Changed In This Rescan

- Admin Relay channel API keys now use a shared AES-GCM `secretbox` codec before writing `channels.api_key_encrypted`.
- `OBLIVIOUS_SECRET_ENCRYPTION_KEY` is the preferred deployment key, with `SESSION_SECRET` fallback for compatibility; local, Docker, Kubernetes, and architecture env docs now list the variable.
- Admin channel create/update protects API keys at rest, and Admin provider probes decrypt the stored key before calling upstream.
- Relay runtime channel create/update protects API keys at rest, and runtime list/get/pool loading decrypts protected keys before adapters use them.
- Legacy unprefixed plaintext channel rows remain readable so existing deployments do not lose runtime credentials before rotation.
- Observability alert-provider config secrets now use the same shared AES-GCM `secretbox` codec before JSONB persistence, and SQL store reads decrypt before HTTP redaction, marker-preserving updates, provider tests, and alert delivery sink construction.
- This is repository-local PostgreSQL at-rest encryption proof for Admin Relay channel API keys and Observability alert-provider config secrets. Publishing channel configs, Workflow definitions/versions/execution snapshots, and target-environment secret audits remain open.

## Repository Inventory

- First-party tracked file distribution after this slice is intended to be:
  - `src`: 975 files
  - `.planning`: 210 files
  - `docs`: 92 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape remains:
  - `src/server/internal`: 590 tracked files
  - `src/server/migrations`: 106 SQL migration files
  - largest active server domains remain `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape remains:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory after this slice:
  - Go test files: 228
  - Web component/API test files: 67
  - Web Playwright specs: 8 specs
  - Web E2E fixture files: 8 files
- Latest checked-in top-level migration remains `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
- Project-local `AGENTS.md`: none at the main repo root or under first-party source; dependency caches and nested `reference/*` repositories are excluded from this first-party scan.

## Completion Matrix Snapshot

Proven rows remain:

- API gateway and relay
- Knowledge base and RAG
- Multi-channel publishing
- Database schema and migrations

Partial rows remain:

- Workflow engine
- Agent system
- Billing and monetization
- Marketplace ecosystem
- Frontend shell and core pages
- Observability metrics, logs, alerts, recovery
- API contract
- Deployment and operations
- Security and tenant isolation
- Migration strategy and release readiness

This scan does not reclassify any Partial row to Proven. Admin Relay API-key and Observability alert-provider at-rest encryption improve Relay, Security, operations/env, and release evidence, but broader target-environment, remaining at-rest encryption, and final release proof remain open.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -8
git ls-files | awk '...inventory counters...'
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' -o -path './.git/*' \) -prune -o -name AGENTS.md -print
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**'
rg -n "api_key_encrypted|secretbox|ENCRYPTION_KEY|SESSION_SECRET" src/server/internal/{admin,relay,mcp,http,workflow,observability,channel} docs/release docs/reports scripts/verify-commercial-db-evidence.sh -g '*.go' -g '*.md' -g '*.sh'
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/observability -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin ./internal/relay ./internal/http -run 'Test(RelayChannelProbeUsesAPIKeyAndReturnsModels|RelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey|AdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers)$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh secret-response-safety
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```

Result:

- `git status --short --branch` showed `main...origin/main` plus the intended Observability at-rest encryption slice files before this refresh was finalized.
- The intended post-slice inventory counters are: `src=975`, `docs=92`, `scripts=37`, `deploy=42`, `.planning=210`, Go test files `228`, web component/API test files `67`, and Playwright specs `8`.
- `go test ./internal/secretbox -count=1 -v` passed.
- `go test ./internal/observability -count=1` passed.
- The direct Observability HTTP package command compiled; the DB-backed test skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- The package-level Admin/Relay/HTTP command compiled the changed packages; DB-backed tests skipped locally without `TEST_DATABASE_URL`, which is expected for that direct command.
- `scripts/verify-commercial-db-evidence.sh secret-response-safety` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none, including the new runtime at-rest encryption test.
- `scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none.
- `bash scripts/check.sh docs` passed.
- `git diff --check` passed.
- The first-party AGENTS scan found no main-root or first-party source `AGENTS.md`.
- The TODO/stub scan did not reveal a new broad implementation gap. Active first-party matches remain the known release-boundary items from the June 14 scan: disabled future Relay surfaces, generated gRPC `Unimplemented*` boilerplate, test stubs, placeholder-only release docs/config examples, and the service-template migration TODO.
- The secret-storage scan now confirms MCP auth tokens, Admin Relay channel API keys, and Observability alert-provider config secrets have repository-owned reversible at-rest protection. Publishing channel configs and Workflow persisted definitions/snapshots still store raw secrets for runtime use and remain at-rest encryption candidates.

## Notable Scan Findings

- The live worktree before the Observability refresh slice was clean at `95f8ef9`.
- Existing DB-backed tenant-isolation evidence remains stronger than target-environment evidence; this slice adds DB-backed at-rest encryption proof for the Admin Relay channel credential path.
- `src/server/internal/http/admin_channel_secret_response_test.go` now pairs response redaction with direct SQL ciphertext assertions and an upstream probe assertion that the decrypted rotated key is usable.
- `src/server/internal/relay/store_test.go` now proves Relay runtime persistence writes protected channel API keys, hydrates raw keys for runtime callers, and preserves legacy plaintext compatibility.
- `src/server/internal/http/observability_alert_handler_test.go` now pairs Observability response redaction with direct SQL ciphertext assertions and a SQL-backed `/test` assertion that the decrypted webhook URL remains usable.
- Response safety and at-rest encryption remain separate. Admin Relay channel API keys and Observability alert-provider config secrets now have both response safety and at-rest encryption proof; Publishing and Workflow secret stores still have response-safety proof only.

## Recommended Next Slices

1. Continue at-rest encryption work for the remaining secret-bearing stores that still intentionally preserve raw values for runtime use: Publishing channel configs and Workflow definitions/versions/execution snapshots.
2. Expand browser/E2E proof to the next high-value commercial journey not already covered by the current local Playwright paths.
3. Rerun the strict commercial verifier on target infrastructure with deploy and backup/restore enabled before renewing any final readiness claim.
4. Extend Observability/recovery proof from repository-owned panic recovery into target-environment OOM/crash restart, scale, and failover evidence.
5. Keep Deployment and release readiness open until Kubernetes, backup/restore, migration replay, and provider/payment rails have target-environment proof.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until row-specific proof is recorded and rerun in the required environment.

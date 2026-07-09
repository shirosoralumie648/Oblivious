# Workflow Retry And Failure Replay Evidence

Date: 2026-07-05

## Runtime Claim

Repository-local PostgreSQL evidence now proves that a rebuilt Workflow service can continue two persisted recovery paths:

- A paused failure decision can be resolved after service reconstruction, resume through the durable state replay path, mark the failed node as skipped, and seed downstream pending work.
- A due auto-retry node can be resumed after service reconstruction from persisted SQL node-execution context and complete as the next retry attempt.

This is a repository-local PostgreSQL evidence slice. It is not target workflow telemetry, deployed Workflow gRPC smoke, or final target release proof.

## Changed Files

- `src/server/internal/workflow/store_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`
- `docs/audit/agent-evidence/2026-07-05-workflow-retry-failure-replay-evidence.md`
- `docs/release/fusion-spec-evidence-pack.md`
- `docs/release/commercial-completion-audit.md`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/implementation-roadmap.md`

## Contract Changes

- `workflow-sql-isolation` now includes `TestWorkflowSQLStoreResolvesPausedFailureAfterServiceRebuild`.
- `workflow-sql-isolation` now includes `TestWorkflowSQLStoreRunsDueAutoRetryAfterServiceRebuild`.
- `scripts/verify-commercial-db-evidence-profiles.sh` now fails if either rebuilt-service retry/failure replay test is dropped from the no-skip Workflow SQL profile.

## Verification

Focused disposable PostgreSQL verification was run for:

```bash
TEST_DATABASE_URL="postgres://..." \
OBLIVIOUS_REQUIRE_TEST_DATABASE=true \
GOCACHE=/tmp/oblivious-go-cache \
GOMODCACHE=/tmp/oblivious-go-mod-cache \
go test ./internal/workflow -run '^(TestWorkflowSQLStoreResolvesPausedFailureAfterServiceRebuild|TestWorkflowSQLStoreRunsDueAutoRetryAfterServiceRebuild)$' -count=1 -v
```

Result:

- `TestWorkflowSQLStoreResolvesPausedFailureAfterServiceRebuild` passed.
- `TestWorkflowSQLStoreRunsDueAutoRetryAfterServiceRebuild` passed.

Final profile and documentation gates for this slice are tracked by:

```bash
bash scripts/verify-commercial-db-evidence-profiles.sh
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
bash scripts/verify-quality-gates.sh
```

## Deferred

- Target Workflow telemetry.
- Deployed Workflow gRPC smoke.
- Final target release proof.
- Broader non-Workflow commercial completion gaps.

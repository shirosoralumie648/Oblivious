# Fusion Spec Evidence Pack

This pack is the current release-readiness evidence index for the four 2026-06-04 fusion specs. It is not a final completion claim. The source of truth remains `docs/reports/2026-06-07-fusion-spec-completion-matrix.md`; any row marked `Partial` remains open until the row-specific proof is recorded and rerun on the target environment where required.

## Current Boundary

- Current status: partial fusion-spec completion.
- Current matrix: `docs/reports/2026-06-07-fusion-spec-completion-matrix.md`.
- Historical commercial baseline: `docs/release/commercial-completion-audit.md` and `docs/release/commercial-gates.md`.
- This pack only indexes evidence that exists in the repository or must be supplied by a target deployment.
- A new full-project completion claim requires all matrix rows to be proven, then strict final verification with no environment skips.

## Requirement Evidence Index

| Spec area | Current evidence source | Required final proof |
| --- | --- | --- |
| Relay and API gateway | `docs/reports/2026-06-07-relay-requirement-audit.md`, `scripts/verify-relay-security.sh`, Relay/OpenAPI contract gates | Focused Relay tests plus `bash scripts/check.sh relay-security` and `bash scripts/check.sh docs` on the release commit |
| Workflow engine | `docs/reports/2026-06-07-workflow-requirement-audit.md`, `docs/reports/2026-06-08-workflow-success-rate-evidence.md`, `scripts/verify-workflow-success-rate-evidence.sh` | Repository load evidence plus target deployment telemetry for external success-rate claims |
| Knowledge and RAG | `docs/reports/2026-06-07-knowledge-requirement-audit.md`, Knowledge/RAG backend and frontend tests | Focused retrieval, chunking, citation, versioning, and UI tests for changed surfaces |
| Agent system | Agent service/store/runtime tests, Agent route and UI tests, matrix row 24 | ReAct and planning runtime tests, approval/resume tests, memory policy tests, and route/UI tests for changed surfaces |
| Publishing channels | Channel service/adapter/HTTP tests and matrix row 25 | Adapter, worker, retry, fallback, archive, and alert tests for changed surfaces |
| Billing and monetization | Admin/Billing/Relay quota/payment tests, OpenAPI contract gates, `scripts/verify-commercial-db-evidence.sh`, matrix row 26 | DB-free handler proof where valid, DB-backed lifecycle proof under `TEST_DATABASE_URL` or disposable pgvector PostgreSQL, and provider proof for external rails |
| Marketplace ecosystem | Marketplace service/settlement/governance/search tests, `scripts/verify-commercial-db-evidence.sh`, and matrix row 27 | Provider checkout, settlement, payout, review, governance, UI, and DB-backed lifecycle proof for changed surfaces |
| Frontend shell and pages | Real app-router regression tests, page-focused Vitest suites, matrix row 28 | Route-level shell proof plus page-specific workflow tests, `pnpm --dir src/web exec tsc --noEmit`, and targeted Vitest |
| Observability and recovery | `docs/reports/2026-06-07-alert-requirement-audit.md`, `docs/release/recovery-platform-contract.md`, dashboard and deployment contract verifiers | Repository checks plus target platform proof for scale-down and failover claims |
| API contract | `docs/api/openapi.yaml`, `scripts/verify-openapi-contract.sh`, route-surface tests | Contract gate plus focused route tests for every changed request/response family |
| Deployment and operations | `docs/release/v07-operations-evidence.md`, `docs/release/rc-checklist.md`, deployment and backup/restore scripts | `scripts/deploy-validate.sh`, `scripts/k8s-validate.sh`, and `scripts/backup-restore-smoke.sh` on the target installation |
| Security and tenant isolation | Security row 33, auth/CSRF route-surface tests, tenant isolation tests, secret-redaction contracts | DB-free guards where valid, DB-backed tenant-isolation proof where persistence is involved, and dependency scans |
| Migration and release readiness | `docs/reports/2026-06-08-migration-schema-audit.md`, migration verifiers, this evidence pack | Final strict verifier, release evidence logs, deployment validation, backup/restore smoke, unresolved-risk signoff |

## Required Final Commands

Run the narrow tests for the changed slice first, then run the shared release checks below on the release candidate commit.

```bash
bash scripts/check.sh docs
bash scripts/check.sh relay-security
bash scripts/check.sh security
pnpm --dir src/web exec tsc --noEmit
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./... -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh backend-journey
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh app-stateful-routes
TEST_DATABASE_URL="$TEST_DATABASE_URL" GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test -p 1 ./... -count=1
bash scripts/deploy-validate.sh
bash scripts/k8s-validate.sh
bash scripts/backup-restore-smoke.sh
bash scripts/verify-commercial-completion.sh
git diff --check
```

If a command is intentionally skipped because the target environment lacks Docker, Kubernetes, database, browser, or payment-provider access, record the skip as residual risk. A skipped command is not successful proof.

## Unresolved Risk List

| Risk | Evidence still required | Current boundary |
| --- | --- | --- |
| External workflow success-rate proof | Deployed telemetry or load evidence from the target installation | Local deterministic workflow load evidence is repository proof only |
| Billing and Marketplace payment rails | DB-backed lifecycle reruns plus configured provider checkout/refund/payout proof | Fake providers and DB-free handlers prove only local validation/fail-closed behavior |
| Deployment platform recovery | `scripts/k8s-validate.sh` with real cluster access, real secrets, and failover/scale evidence | Static manifests and contract checks do not prove live failover |
| Database-backed tenant isolation | Broader `TEST_DATABASE_URL` integration tests in CI or target release evidence | Focused disposable PostgreSQL app-stateful evidence covers selected tenant/session/ownership paths, but DB-free route guards do not prove all SQL persistence isolation |
| Final release readiness | Strict final verifier with deploy and backup/restore enabled and no environment skips | Historical Phase 30 success does not automatically cover later fusion-spec changes |

## Update Rules

- Add a new matrix evidence line after each completed slice.
- Add exact commands, pass/fail result, skipped checks, and environment class.
- Keep rows marked `Partial` until their own evidence is complete.
- Do not use this pack to claim completion while unresolved risks remain.

# Phase 24 Context: Release Rollback Incident DR and v07 Closeout

## Milestone

v07 Production Operations.

## Why This Phase Exists

Phase 21 proved an equivalent production runtime can build, run migrations, start the stack, and smoke health, metrics, app, and Relay routes. Phase 22 proved PostgreSQL tenant-commercial data can be backed up and restored with migration-ledger integrity. Phase 23 proved repository-local logs, metrics, tracing hooks, error-reporting primitives, alert rules, dashboard artifacts, and SLO definitions.

The v07 Operations Gate still cannot close until operators have verified release, rollback, incident response, and disaster recovery runbooks that tie those previous proofs together. This phase closes the remaining v07 work only: final release-path evidence for `OPS-02`, `OPS-06`, and `DOC-06`. It must not claim v08 Product Completeness or final commercial readiness.

## Requirements

- **OPS-02:** Runtime smoke covers both normal network and restricted-network deployment paths, including documented proxy/registry overrides and explicit evidence when cluster tooling is unavailable. Phase 21 completed the restricted-network compose slice and missing-`kubectl` evidence; Phase 24 owns final release-path evidence.
- **OPS-06:** Release, rollback, incident response, and disaster recovery runbooks are documented and verified against the deployment, restore, alert, and evidence commands.
- **DOC-06:** v07 evidence maps production-operations requirements to scripts, manifests, runbooks, smoke output, skipped checks, residual v08 work, and a no-final-readiness boundary.

## Current Evidence And Gaps

Existing v07 evidence:

- `scripts/deploy-validate.sh` starts the compose stack, runs `/usr/local/bin/oblivious-migrate`, and runs the shared smoke.
- `scripts/deploy-smoke.sh` proves `/healthz`, `/metrics`, `/api/v1/auth/me`, and `/v1/chat/completions` without live provider secrets.
- `scripts/k8s-validate.sh` validates Kubernetes prerequisites, applies manifests, waits for rollouts, port-forwards, and runs the shared smoke when `kubectl`, a cluster, and an untracked filled secret file exist.
- `scripts/backup-postgres.sh`, `scripts/restore-postgres.sh`, and `scripts/backup-restore-smoke.sh` prove PostgreSQL backup/restore and migration-ledger integrity.
- `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, and `docs/release/observability-slos.md` define Phase 23 alert, dashboard, SLO, owner, severity, and Phase 24 runbook targets.
- `docs/release/deployment-runtime-remediation.md`, `docs/release/backup-restore-runbook.md`, `docs/release/rc-checklist.md`, and `docs/release/commercial-gates.md` already contain partial release and operations guidance.

Current gaps:

- There is no dedicated release/rollback runbook that sequences pre-release backup, migration-aware deployment validation, smoke, rollback trigger criteria, database restore boundary, and evidence capture.
- There is no incident response runbook tied to the Phase 23 alert names, owners, dashboard panels, triage commands, mitigation steps, rollback triggers, communication, and evidence capture.
- There is no disaster recovery runbook tying Phase 22 backup/restore, Phase 21 deployment validation, Phase 23 observability artifacts, and post-restore acceptance checks into one executable path.
- `rc-checklist.md` does not yet include the Phase 24 runbook verification gate.
- `scripts/verify-quality-gates.sh` does not yet guard the release/rollback, incident, DR, and v07 evidence artifacts.
- No `24-VERIFICATION.md` or v07 evidence map exists yet, so `OPS-02`, `OPS-06`, and `DOC-06` remain open.
- v07 milestone snapshots do not exist yet; they must be created only after all remaining v07 checks pass.

## Runbook Design

Phase 24 should add or update these operator documents:

- `docs/release/release-rollback-runbook.md`: release prerequisites, normal-network and restricted-network deployment commands, pre-release backup, migration status, smoke expectations, rollback criteria, rollback sequence, database restore boundary, and evidence capture.
- `docs/release/incident-response-runbook.md`: alert-specific triage for `RelayOutage`, `QuotaSettlementFailure`, `StripeWebhookFailure`, `MigrationFailure`, `HighProviderErrorRate`, and `TenantIsolationIncident`, with owners, severity, dashboards, commands, mitigation, escalation, communication, and rollback triggers.
- `docs/release/disaster-recovery-runbook.md`: restore into fresh infrastructure, validate migration ledger, redeploy, run smoke, inspect observability signals, document skipped Kubernetes/vendor checks, and preserve no-final-commercial-readiness boundary.
- `docs/release/v07-operations-evidence.md`: map `OPS-01` through `OPS-06` and `DOC-06` to scripts, manifests, runbooks, verification commands, skipped checks, residual risks, and v08 work.

The runbooks must use placeholders only. They must not include live provider keys, Stripe secrets, database passwords, kubeconfig material, customer data, or dump contents.

## Verification Design

Phase 24 must verify more than prose. The executor should:

- Re-run or record current environment results for `bash scripts/deploy-validate.sh` and the restricted-network/fallback deployment path that is viable on this host.
- Re-run or record the expected non-success result for `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` if `kubectl` remains unavailable.
- Re-run `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` if Docker and the fallback image remain available.
- Check runbook references to Phase 23 alerts, dashboard, SLOs, Phase 22 backup/restore, and Phase 21 deployment smoke.
- Extend `scripts/verify-quality-gates.sh` so the Phase 24 runbook/evidence files cannot disappear without docs checks failing.
- Run `bash scripts/check.sh docs` and `git diff --check`.

If normal-network Docker Hub or default Go module access still fails, record the exact bounded failure and keep restricted-network/fallback evidence as the locally proven path. Missing `kubectl`, Prometheus server, Grafana server, OpenTelemetry collector, or external error-tracking vendor can be recorded as skipped/unavailable only when the verification file states that those are not proven runtime integrations.

## Closeout Boundary

Phase 24 may close the v07 Operations Gate only if `OPS-02`, `OPS-06`, and `DOC-06` pass with current repository evidence. It may archive v07 snapshots after that closeout.

Phase 24 must not claim:

- v08 Product Completeness.
- Customer-facing placeholder removal.
- Knowledge RAG or product-copy alignment.
- Durable Agent workflows with human approval and observable execution state.
- Final public pricing/onboarding/operator guide completeness beyond v07 operations.
- Final commercial readiness or commercial completeness.

The next milestone after v07 is v08 Product Completeness.

---

*Phase: 24-release-rollback-incident-dr-and-v07-closeout*
*Context gathered: 2026-05-28 from current repository evidence*

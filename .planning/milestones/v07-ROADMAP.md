# Roadmap Snapshot: v07 Production Operations

## Milestone Status

**v07 Production Operations — COMPLETE 2026-05-28**

**Goal:** Prove the platform is operable by a production team: orchestration starts the actual stack, migrations run safely, `/healthz` plus app/Relay paths smoke, tenant data can be backed up/restored, observability surfaces exist, and release/rollback/incident/DR runbooks are verified.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 21 | Production Orchestration Runtime Proof | OPS-01, OPS-02 | Complete |
| Phase 22 | Backup Restore and Migration Recovery | OPS-03 | Complete |
| Phase 23 | Observability Alerts Dashboards and SLOs | OPS-04, OPS-05 | Complete |
| Phase 24 | Release Rollback Incident DR and v07 Closeout | OPS-02, OPS-06, DOC-06 | Complete |

## Completion Evidence

- Phase 21 verification: `.planning/phases/21-production-orchestration-runtime-proof/21-VERIFICATION.md`
- Phase 22 verification: `.planning/phases/22-backup-restore-and-migration-recovery/22-VERIFICATION.md`
- Phase 23 verification: `.planning/phases/23-observability-alerts-dashboards-and-slos/23-VERIFICATION.md`
- Phase 24 verification: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-VERIFICATION.md`
- Phase 24 summary: `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-01-SUMMARY.md`
- Operations evidence: `docs/release/v07-operations-evidence.md`
- Commercial gates: `docs/release/commercial-gates.md`

## Verified Commands

- `timeout 900 bash scripts/deploy-validate.sh`
- Restricted/fallback `scripts/deploy-validate.sh` with registry/proxy overrides and `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16`
- `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` expected-fail for missing `kubectl`
- `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh`
- `bash scripts/check.sh docs`
- `git diff --check`

## Boundary

v07 is not final commercial readiness. v08 Product Completeness and final commercial audit remain required.

# Requirements: Oblivious v07 Production Operations

**Defined:** 2026-05-28
**Completed:** 2026-05-28
**Source spec:** `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
**Core Value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

## Production Orchestration

- [x] **OPS-01**: Kubernetes or equivalent production orchestration validation starts the actual stack, applies migrations, proves `/healthz`, and proves app and Relay paths without live provider secrets.
- [x] **OPS-02**: Runtime smoke covers both default-command and restricted-network/fallback deployment paths, including documented proxy/registry overrides and explicit evidence when cluster tooling is unavailable.

## Backup And Recovery

- [x] **OPS-03**: Backup and restore runbooks plus automated smoke prove PostgreSQL tenant data can be backed up and restored into a fresh database with migration ledger integrity.

## Observability

- [x] **OPS-04**: Structured logs, Prometheus metrics, OpenTelemetry tracing hooks, and error-tracking integration points cover HTTP, Relay, billing, jobs, and provider failures.
- [x] **OPS-05**: Alert rules, dashboards, and SLO definitions exist for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, and tenant isolation incidents.

## Runbooks And Evidence

- [x] **OPS-06**: Release, rollback, incident response, and disaster recovery runbooks are documented and verified against deployment, restore, alert, and evidence commands.
- [x] **DOC-06**: v07 evidence maps production-operations requirements to scripts, manifests, runbooks, smoke output, skipped checks, residual v08 work, and a no-final-readiness boundary.

## Future Requirements

- [ ] v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial Admin/Marketplace UX, public docs, onboarding, pricing, operator guides, final end-to-end commercial journeys, and final commercial audit.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| OPS-01 | Phase 21 — Production Orchestration Runtime Proof | Complete |
| OPS-02 | Phase 21 and Phase 24 — Release-path verification | Complete |
| OPS-03 | Phase 22 — Backup Restore and Migration Recovery | Complete |
| OPS-04 | Phase 23 — Observability Alerts Dashboards and SLOs | Complete |
| OPS-05 | Phase 23 — Observability Alerts Dashboards and SLOs | Complete |
| OPS-06 | Phase 24 — Release Rollback Incident DR and v07 Closeout | Complete |
| DOC-06 | Phase 24 — Release Rollback Incident DR and v07 Closeout | Complete |

## Boundary

v07 closes the Operations Gate. It does not close Product Completeness or final commercial readiness.

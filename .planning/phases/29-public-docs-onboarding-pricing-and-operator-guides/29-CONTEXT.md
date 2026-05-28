# Phase 29 Context - Public Docs Onboarding Pricing and Operator Guides

## Scope

Phase 29 closes only `PROD-05`: public docs, onboarding, pricing, and operator guides must match the implemented tenant, Relay, billing, Marketplace, operations, and product behavior.

Phase 30 still owns end-to-end commercial journey proof and `AUDIT-01`. Phase 29 must not claim Product Completeness Gate closure or final commercial readiness.

## Live Evidence

Authoritative state:
- `.planning/STATE.md` routes next work to Phase 29.
- `.planning/REQUIREMENTS.md` marks `PROD-01` through `PROD-04` complete and leaves `PROD-05`, `PROD-06`, and `AUDIT-01` open.
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` defines the commercial complete program and the no-final-readiness evidence model.
- `docs/release/commercial-gates.md` records v04 through v07 completion, Phase 25 through Phase 28 product evidence, and remaining Phase 29/30 requirements.

Observed documentation gaps:
- `README.md` still describes a workspace-oriented release-candidate surface instead of the commercial multi-tenant AI SaaS target.
- `docs/API.md` still names the v03.3 consolidated mainline and says supported Relay routes need settlement/refund proof before v05 closes, even though v05 is complete.
- `docs/architecture/current-system-contracts.md` still says the baseline is v03.3, describes Knowledge retrieval as text matching, and describes SOLO as a runtime MVP.
- No `docs/product/` public overview, onboarding, pricing, or operator guide files exist.
- `scripts/verify-quality-gates.sh` does not yet assert Phase 29 documentation coverage or stale-doc wording.

## Decisions

- **D-01:** Update docs to the current implemented product, not aspirational future behavior. Public docs may describe Phase 30 as the remaining proof step, but they must not say final commercial readiness is already complete.
- **D-02:** Keep pricing docs behavior-based. The repository can document subscriptions, top-ups, quota, invoices, refunds, Marketplace settlement, and plan configuration, but committed docs must not invent production SKU prices or live payment credentials.
- **D-03:** Preserve the Relay invariant everywhere: all Chat, Agent, Knowledge embedding/RAG, and provider-facing AI calls go through Relay for billing, rate limiting, and monitoring.
- **D-04:** Operator guidance should link to existing release, rollback, backup, restore, observability, incident, disaster recovery, and deployment validation runbooks rather than duplicating them.
- **D-05:** Public docs should explain disabled-by-default commercial boundaries for `web_search` and `http_request`, and real default behavior for `calculator`, `datetime`, durable Agent runs, Knowledge RAG, Admin billing, and Marketplace governance.
- **D-06:** The docs quality gate should reject the stale Phase 29 wording classes: v03.3 commercial baseline, text-matching Knowledge, SOLO MVP, pre-v05 Relay settlement language, and release-candidate mainline framing.

## Threats

| Threat | Severity | Mitigation |
| --- | --- | --- |
| Public docs overclaim final commercial readiness before Phase 30 journey proof. | High | Add explicit `Phase 30` and `no-final-readiness` boundary to README, product docs, commercial gates, and Phase 29 evidence. |
| Docs promise behavior no longer aligned with implemented Relay, RAG, billing, or Marketplace code. | High | Tie docs to current files and quality-gate assertions for Relay, subscription, top-up, Marketplace, RAG, and operator guide coverage. |
| Pricing docs invent commercial prices that are not implemented or approved. | Medium | Document pricing model and configuration surfaces, not concrete production price points. |
| Operator docs duplicate runbooks and drift from verified operations evidence. | Medium | Link to existing v07 runbooks and require `bash scripts/check.sh docs` to assert the index. |

## Required Evidence

- `README.md` presents the current commercial platform, Relay invariant, quick start, docs index, and no-final-readiness boundary.
- `docs/product/public-overview.md` explains the customer-facing product surfaces and implemented commercial boundaries.
- `docs/product/onboarding.md` maps user, admin, publisher, and operator onboarding steps to existing product behavior.
- `docs/product/pricing.md` explains subscriptions, top-ups, quota, invoices, refunds, and Marketplace settlement without invented production prices.
- `docs/product/operator-guide.md` points operators to deployment, backup/restore, observability, release/rollback, incident, and disaster recovery procedures.
- `docs/API.md` and `docs/architecture/current-system-contracts.md` no longer carry stale v03.3, text-retrieval, or SOLO MVP claims.
- `scripts/verify-quality-gates.sh` asserts Phase 29 docs and stale-wording checks.
- `29-VERIFICATION.md` and `29-01-SUMMARY.md` record passing docs gates, stale scan, diff hygiene, and the remaining Phase 30 boundary.

## Next

Execute `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-01-PLAN.md`.

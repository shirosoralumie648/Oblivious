# Phase 28 Context — Commercial UX and Journey Hardening

## Scope

Phase 28 closes only `PROD-04`: Chat, Agent/SOLO, Knowledge, Admin, and Marketplace customer journeys must be production-ready with quota enforcement, clear error states, and no enabled placeholder pages or fake commercial behavior.

Phase 29 owns public docs, onboarding, pricing, and operator guides. Phase 30 owns final end-to-end commercial journey proof and `AUDIT-01`. This phase must not claim final commercial readiness.

## Live Evidence

Authoritative state:
- `.planning/STATE.md` routes next work to Phase 28.
- `.planning/REQUIREMENTS.md` marks `PROD-01`, `PROD-02`, and `PROD-03` complete and leaves `PROD-04` planned.
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` defines the Product Completeness Gate.

Routed customer surfaces:
- Workspace: `src/web/src/routes/workspace/ChatPage.tsx`, `src/web/src/routes/workspace/SoloPage.tsx`, `src/web/src/routes/workspace/KnowledgePage.tsx`
- Marketplace: `src/web/src/routes/marketplace/MarketplaceHomePage.tsx`, `MarketplaceAgentDetailPage.tsx`, `MarketplacePublishPage.tsx`, `MarketplaceMyAgentsPage.tsx`
- Admin: `src/web/src/routes/admin/AdminHomePage.tsx`, `AdminChannelsPage.tsx`, `AdminRoutesPage.tsx`, `AdminPlansPage.tsx`, `AdminBillingPage.tsx`, `AdminUsersPage.tsx`, `AdminAuditLogPage.tsx`, `AdminReviewsPage.tsx`
- Shared UI primitives: `src/web/src/components/shared/DataTable.tsx`, `EmptyState.tsx`, status/rating/filter/search components

Observed gaps to plan against:
- `ChatPage.tsx` clears state silently when initial loads fail and does not show retryable loading or send/handoff errors.
- Chat handoff currently renders duplicate `Convert to SOLO task` headings and does not expose Relay/quota failure context when actions fail.
- `SoloPage.tsx` shows execution state but does not present a commercial readiness summary that ties budget, authorization scope, tool boundaries, approval state, and recovery actions together.
- `MarketplaceAgentDetailPage.tsx` installs and reviews without action error handling, disabled loading states, or paid-install settlement copy.
- `MarketplacePublishPage.tsx` does not make paid submission review/settlement boundaries explicit in the form and success state.
- `MarketplaceMyAgentsPage.tsx` handles page-load errors but uninstall action failures can disappear without user feedback.
- Admin pages mostly use shared `DataTable` error states, but Phase 28 needs a cross-module commercial operations journey that proves channels, routes, plans, billing, users, audit, and review queue are not placeholder modules.

## Decisions

- **D-01:** Harden the currently routed pages, not legacy or unrouted workspace views. `KnowledgePageView.tsx` and `SoloPageView.tsx` are not active router imports.
- **D-02:** Use the existing shared primitives before adding new abstractions. Add a small shared commercial alert/status component only if repeated action-level error/success UI becomes duplicated across Chat, SOLO, Marketplace, and Admin.
- **D-03:** Treat generic failure swallowing as a product defect. Customer-facing journeys must show an actionable error, keep the user in context, and expose retry where the action is retryable.
- **D-04:** Do not create fake provider, billing, marketplace, or quota behavior in the frontend. UI copy must reflect existing implemented behavior and boundaries from v05-v07.
- **D-05:** Keep quota and billing copy factual. The UI can surface current budget/quota/settlement state from existing APIs, but Phase 28 must not invent pricing docs or final onboarding flows owned by Phase 29.
- **D-06:** Use focused Vitest coverage for each journey slice and docs/quality gates for anti-placeholder evidence. Broad browser E2E remains Phase 30 unless needed to prove a specific Phase 28 fix.

## Threats

| Threat | Severity | Mitigation |
| --- | --- | --- |
| Customer sees a fake-ready workflow that fails silently under billing, quota, Relay, or Marketplace settlement errors. | High | Add action-level error states and tests for Chat send/handoff, SOLO start/budget/recovery, Marketplace install/review/publish/uninstall, and Admin load/action states. |
| Phase 28 drifts into docs/pricing/end-to-end audit work and falsely implies final readiness. | High | Update planning boundaries and commercial gates to state Phase 28 closes only `PROD-04`. |
| UI copy promises disabled tools, unmanaged paid installs, or provider access outside Relay. | High | Add quality-gate searches and tests that reject placeholder/fake commercial copy in active routes. |
| Admin pages look routed but fail to prove commercial operations coverage. | Medium | Add Admin journey tests that touch dashboard, channels/routes/plans, billing inspection, users, audit, and reviews. |

## Required Evidence

- Focused Chat journey tests for load errors, send errors, SOLO handoff errors, quota/Relay message display, and duplicate heading removal.
- Focused SOLO/Agent journey tests for commercial run summary, budget/authorization/tool boundary display, action errors, and retry/recovery affordances.
- Focused Marketplace tests for paid/free install messaging, install/review action errors, publish review/settlement copy, and uninstall errors.
- Focused Admin journey tests for commercial operations modules and retryable error states.
- Docs and quality gate checks proving active customer routes do not contain enabled placeholder or fake commercial behavior.
- `28-VERIFICATION.md` and `28-01-SUMMARY.md` after implementation; do not create them until execution has evidence.

## Next

Execute `.planning/phases/28-commercial-ux-and-journey-hardening/28-01-PLAN.md`.

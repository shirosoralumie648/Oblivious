# Phase 30 Context - End-to-End Commercial Journey and Final Audit

## Scope

Phase 30 closes `PROD-06` and `AUDIT-01` only after current repository evidence proves the commercial-complete objective end to end:

- signup and session creation,
- organization and tenant scope,
- provider/channel and route configuration,
- subscription and top-up payment lifecycle,
- Chat through Relay,
- Agent workflow durability and approval/retry evidence,
- Knowledge RAG through Relay embeddings and pgvector retrieval,
- Marketplace publish, review, paid install, settlement, payout/refund impact, and governance,
- Admin billing and operations inspection,
- deployment validation,
- backup and restore,
- final commercial gate audit.

This is the final Product Completeness phase. It must not close by copying prior phase summaries; it needs a current Phase 30 evidence harness that re-runs or directly references fresh command output and maps every commercial gate to evidence.

## Live Evidence

Authoritative state:
- `.planning/STATE.md` routes next work to Phase 30.
- `.planning/REQUIREMENTS.md` leaves only `PROD-06` and `AUDIT-01` open.
- `.planning/ROADMAP.md` marks Phase 30 planned.
- `docs/release/commercial-gates.md` says the Product Completeness Gate and Verification Gate remain open until Phase 30.

Available evidence surfaces:
- Backend HTTP integration helpers in `src/server/internal/http/server_test.go` provide `testDatabase`, registration/session/CSRF helpers, organization helpers, and DB-backed route setup.
- Backend tests already cover tenant, Stripe checkout/webhook, top-up, subscriptions, Marketplace settlement/refund/governance, Admin billing, Knowledge RAG, Agent run/tool-run state, Relay billing, and route authority in separate packages.
- Frontend tests cover Chat, SOLO/Agent, Knowledge, Marketplace, Admin, onboarding, settings, console, layout, and API clients.
- Playwright currently has `src/web/e2e/admin-marketplace.spec.ts` and `fixtures/adminMarketplace.ts` for Admin and Marketplace browse/publish/install flows.
- Deployment and recovery scripts exist: `scripts/deploy-validate.sh`, `scripts/deploy-smoke.sh`, `scripts/backup-restore-smoke.sh`, `scripts/backup-postgres.sh`, and `scripts/restore-postgres.sh`.
- Phase 29 added public docs, onboarding, pricing, operator guide, and stale-doc quality gates.

Observed Phase 30 gaps:
- No Phase 30 DB-backed commercial journey test exists that spans signup, tenant scope, provider/channel setup, payment lifecycle, Chat, Agent, Knowledge, Marketplace, and Admin inspection in one named evidence target.
- Playwright only covers Admin and Marketplace. It does not yet exercise a customer journey across onboarding, Chat, Knowledge, SOLO/Agent, Marketplace, and Admin billing context.
- No `scripts/verify-commercial-completion.sh` orchestrates docs, Relay security, targeted backend/frontend journey tests, deployment validation, backup/restore smoke, and final audit asset checks.
- No final `docs/release/commercial-completion-audit.md` maps every commercial gate to files, commands, environment class, pass/fail result, skipped checks, and residual risk.
- Quality gates do not yet require the Phase 30 plan, final audit document, commercial journey test, Playwright journey, or completion verifier script.

## Decisions

- **D-01:** Final readiness requires fresh Phase 30 verification output, not inherited prior phase completion prose.
- **D-02:** Add a DB-backed HTTP journey test under `src/server/internal/http` because it can prove tenant/session/CSRF, billing, Chat, Agent, Knowledge, Marketplace, and Admin inspection against the same test database and router.
- **D-03:** Add a browser journey under `src/web/e2e` to prove the customer/operator route sequence is coherent from the rendered product surface. Use route fixtures for deterministic evidence; do not require live provider or Stripe keys.
- **D-04:** Add `scripts/verify-commercial-completion.sh` as the final local orchestrator. It should run required docs/security/targeted tests and require explicit opt-in environment variables for heavier deploy and backup/restore smoke if the host cannot provide Docker/Postgres.
- **D-05:** The final audit must distinguish proven, skipped, environment-specific, and residual-risk evidence. Skipped deploy or backup checks cannot be counted as final commercial readiness.
- **D-06:** Do not mark the active thread goal complete until the final audit proves every user requirement and commercial gate with current evidence.

## Required Evidence

- Phase 30 context and plan files.
- A backend commercial journey test, tentatively `src/server/internal/http/commercial_journey_test.go`.
- A frontend commercial journey Playwright test, tentatively `src/web/e2e/commercial-journey.spec.ts`, with deterministic fixtures.
- A final verifier script, tentatively `scripts/verify-commercial-completion.sh`.
- A final audit document, `docs/release/commercial-completion-audit.md`, mapping:
  - Tenant And Identity Gate,
  - Relay Authority Gate,
  - Billing And Monetization Gate,
  - Product Completeness Gate,
  - Security Gate,
  - Operations Gate,
  - Verification Gate,
  - every explicit user objective surface: Chat, Agent, Knowledge, Relay, Admin, Marketplace, and direct commercial deployability.
- Updated quality gates requiring Phase 30 assets and rejecting final readiness claims unless `30-VERIFICATION.md` and commercial completion audit exist.
- `30-VERIFICATION.md` with exact command output summary.
- `30-01-SUMMARY.md` after verification.

## Next

Execute `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-01-PLAN.md`.

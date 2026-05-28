# Phase 20 Context: Billing Admin Evidence and v06 Closeout

## Milestone

v06 Billing And Marketplace Operations.

## Why This Phase Exists

Phase 17 mounted Stripe checkout and webhook routes, Phase 18 made verified provider events drive subscription, invoice, top-up, failed-payment, plan-change, and refund lifecycle state, and Phase 19 added paid Marketplace orders, settlement, payout-state modeling, refund impact, takedown, appeal, abuse reports, and governance events.

Those slices created the commercial money-movement records, but administrators still cannot inspect the full billing state from a single Admin surface. The Billing And Monetization Gate requires admins to inspect billing sessions, failed settlements, quota balances, invoices, webhook events, and Marketplace payout state before v06 can close.

## Requirements

- **ADMIN-BILL-01:** Admin can inspect billing sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state.
- **DOC-05:** v06 evidence maps money-movement and Marketplace governance requirements to files, tests, runtime/database proof, and residual v07/v08 work.

## Current Evidence And Gaps

Existing billing and Marketplace assets:

- `payment_intents`, `stripe_webhook_events`, `billing_lifecycle_events`, `billing_invoices`, `billing_refunds`, `subscriptions`, `topup_orders`, `billing_sessions`, `marketplace_orders`, `marketplace_settlements`, and `marketplace_payouts` are present in migrations and HTTP test setup.
- `src/server/internal/http/billing_handler.go` exposes user checkout creation only.
- `src/server/internal/admin` already owns Admin channel, route, plan, user, audit, and review service/store patterns.
- `src/server/internal/http/admin_handler.go` exposes Admin route handlers through `requireAdmin`.
- `src/web/src/features/admin/api.ts`, `src/web/src/types/admin.ts`, `src/web/src/features/layouts/AdminSidebar.tsx`, and `src/web/src/routes/admin/*` define the existing Admin frontend contract.

Current gaps:

- No Admin billing inspection service or store aggregates all money-movement tables.
- No `/api/v1/admin/billing/*` routes expose sessions, webhook events, subscriptions, top-ups, invoices, refunds, Marketplace settlements, and payouts.
- No Admin Billing UI route or sidebar entry exists.
- Existing commercial gate docs still say Phase 20 remains required.
- There is no Phase 20 verification document or v06 milestone snapshot proving all v06 requirements.

## Boundaries

Included:

- Add an Admin billing service/store boundary in `src/server/internal/admin` that reads existing billing and Marketplace tables without mutating money state.
- Add admin-only HTTP routes for a billing summary and list endpoints for sessions, payment intents, webhook events, subscriptions, top-ups, invoices, refunds, Marketplace settlements, and payout state.
- Add a frontend Admin Billing page that can inspect the same surfaces with compact filters and tabular evidence.
- Add backend route tests proving admin-only access and data coverage across the required record types.
- Add frontend API/page/router/sidebar tests for the Admin Billing inspection surface.
- Update docs and `.planning` artifacts to close v06 only after verification, while leaving v07 and v08 open.

Excluded:

- Live Stripe network calls, Stripe Connect payout execution, or external payout provider onboarding.
- Changing money-movement state machines already completed by Phases 17-19.
- Production orchestration, backup/restore smoke, observability dashboards, alerts, and runbooks. Those remain v07.
- Product-completeness work such as durable Agent workflow expansion, Knowledge/RAG behavior changes, MCP tool replacement/disablement, onboarding, pricing copy, and final public docs. Those remain v08.
- Claiming final commercial readiness.

## Admin Billing Inspection Design

Phase 20 should add a read-only Admin billing inspection boundary:

- `BillingInspectionStore` reads from existing tables and returns normalized list types.
- `BillingInspectionFilter` supports `organizationID`, `userID`, `status`, `kind`, `provider`, `limit`, and `offset` where the target table supports those fields.
- `BillingInspectionSummary` aggregates count and amount totals for billing sessions, payment intents, webhook events, subscriptions, top-ups, invoices, refunds, Marketplace settlements, and payouts.
- HTTP handlers live on the existing Admin handler path and require `requireAdmin`.
- Route responses should keep the current envelope style and collection keys used by `createAdminApi`.

Proposed route contract:

- `GET /api/v1/admin/billing/summary`
- `GET /api/v1/admin/billing/sessions`
- `GET /api/v1/admin/billing/payment-intents`
- `GET /api/v1/admin/billing/webhook-events`
- `GET /api/v1/admin/billing/subscriptions`
- `GET /api/v1/admin/billing/topups`
- `GET /api/v1/admin/billing/invoices`
- `GET /api/v1/admin/billing/refunds`
- `GET /api/v1/admin/billing/settlements`
- `GET /api/v1/admin/billing/payouts`

The route set intentionally separates Relay billing sessions from Stripe payment intents because Relay usage settlement and commercial payment checkout are different evidence surfaces.

## Frontend Design

The Admin Billing UI should match the existing operational Admin surfaces:

- Use the existing Admin layout, sidebar, `DataTable`, `SearchBar`, `StatusBadge`, and shadcn primitives.
- Add a `Billing` sidebar item near `Plans`.
- Use compact summary tiles for the aggregate counts and amounts.
- Use tab-like segmented buttons for the inspection surfaces.
- Use a dense table per active surface with predictable filters.
- Avoid marketing copy, decorative cards nested inside cards, or hero-style layout.

## Verification Targets

Focused RED/GREEN:

- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/http -run 'AdminBilling|BillingAdmin' -count=1`
- `cd src/web && npx vitest run src/features/admin/api.test.ts src/routes/admin/AdminBillingPage.test.tsx src/features/layouts/AdminSidebar.test.tsx src/app/router.test.tsx`

Broader phase verification:

- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test -p 1 ./internal/admin ./internal/http ./internal/stripe ./internal/marketplace -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all`
- `git diff --check`

Use `go test -p 1` for DB-backed multi-package tests against the shared PostgreSQL test database to avoid DDL races.

## Residual Risk After This Phase

Phase 20 should close `ADMIN-BILL-01`, `DOC-05`, and v06 only. The product remains non-commercial-complete until v07 proves production operations and v08 removes remaining product placeholders and completes customer-facing commercial journeys.

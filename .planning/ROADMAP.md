# Roadmap: Oblivious

## Milestones

- 🚧 **v06 Billing And Marketplace Operations** — Phase 17 through Phase 20 (active)
- ✅ **v05 Relay Billing Completeness** — Phase 13 through Phase 16 (completed 2026-05-28)
- ✅ **v04 Commercial Foundation** — Phase 9 through Phase 12 (completed 2026-05-28)
- ✅ **v03.3 Mainline Consolidation** — Phase 5 through Phase 8 plus 999.1/999.2 follow-ups (completed 2026-05-27)
- ✅ **v03.2 Quality and Release** — Phase 4 (shipped 2026-05-14; Docker compose runtime validated 2026-05-12)
- ✅ **v03.1 Admin and Marketplace UI** — Phase 03.1 (shipped 2026-05-02)
- ✅ **Foundation through Admin/Marketplace Backend** — Phase 1, Phase 2, Phase 3a (completed 2026-04-27 through 2026-04-29)

## Current Status

Milestone v06 has been initialized from `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`.

**Next workflow step:** execute Phase 19 Marketplace Settlement and Governance Plan 01.

## Current Milestone: v06 Billing And Marketplace Operations — Active

**Goal:** Complete commercial money movement and Marketplace governance: Stripe checkout/webhooks, subscription lifecycle, invoices, refunds, failed-payment states, plan changes, top-ups, billing admin evidence, publisher settlement, platform fees, payout state, refund impact, and moderation workflows.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 17 | Stripe Payment Authority and Webhook Ledger | PAY-01, PAY-02 | Complete |
| Phase 18 | Subscription Invoice Top-up Refund State Machine | PAY-03 | Complete |
| Phase 19 | Marketplace Settlement and Governance | MARKET-03, MARKET-04 | Ready to execute |
| Phase 20 | Billing Admin Evidence and v06 Closeout | ADMIN-BILL-01, DOC-05 | Planned |

### Phase 17: Stripe Payment Authority and Webhook Ledger

**Goal:** Mount Stripe checkout and webhook routes in the running server, make checkout tenant-aware and testable without live Stripe calls, and record webhook events in a dedicated idempotent ledger after raw-body signature verification.

**Requirements:** PAY-01, PAY-02

**Success criteria:**
1. `POST /api/v1/billing/checkout` requires an authenticated tenant session and creates a payment intent/checkout record containing organization ID, user ID, package ID, checkout kind, amount, and provider checkout session ID.
2. Checkout creation uses an interface so route tests can use a fake Stripe client; live Stripe API keys are not required for automated tests.
3. `POST /api/v1/billing/stripe/webhook` is mounted as a public endpoint but rejects missing or invalid Stripe signatures before writing provider state.
4. Signed webhook fixture tests prove Stripe event IDs are recorded exactly once in a provider webhook ledger with event type, processing status, tenant metadata, payload, and error details.
5. Existing quota top-up and subscription mutation behavior is not silently marked paid before a verified payment event; full lifecycle application remains Phase 18.

**Likely verification:**
- `cd src/server && go test ./internal/stripe -run 'Webhook|Checkout|Ledger' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-CONTEXT.md`
- Plan: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-PLAN.md`

**Completion evidence:**
- Summary: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-SUMMARY.md`
- Focused tests: `cd src/server && go test ./internal/stripe ./internal/config -count=1`
- DB-backed route tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`
- Broader package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/config ./internal/quota -count=1`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

### Phase 18: Subscription Invoice Top-up Refund State Machine

**Goal:** Apply verified Stripe events to subscription, invoice, top-up, failed-payment, plan-change, and refund state through an auditable, idempotent lifecycle service.

**Requirements:** PAY-03

**Success criteria:**
1. `checkout.session.completed` for subscription checkout completes the local payment intent, creates or updates an organization-scoped subscription, updates user plan assignment, and records one lifecycle transition.
2. `checkout.session.completed` for top-up checkout marks the top-up paid and credits tenant quota exactly once; no direct paid top-up flow can credit quota without verified payment evidence.
3. `invoice.paid` and `invoice.payment_failed` upsert invoice state and update subscription active/past-due or failed-payment state through append-only transitions.
4. `customer.subscription.updated` and `customer.subscription.deleted` preserve provider subscription IDs, period fields, cancel-at-period-end state, plan changes, and cancellation history.
5. Refund events create refund records, update payment intent refund state, reverse paid top-up effects once where applicable, and leave Marketplace refund impact for Phase 19.

**Likely verification:**
- `cd src/server && go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- `git diff --check`

**Planning evidence:**
- Context: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-CONTEXT.md`
- Plan: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-PLAN.md`

**Completion evidence:**
- Summary: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-SUMMARY.md`
- Focused lifecycle tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`
- DB-backed route tests: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`
- Broader package check: `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1`
- Docs gate: `bash scripts/check.sh docs`
- Broad quality gate: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- Diff hygiene: `git diff --check`

**Boundaries:**
- Marketplace settlement, platform fees, payout state, and refund impact remain Phase 19.
- Admin billing inspection remains Phase 20.
- v07 production operations and v08 product completeness remain required.

### Phase 19: Marketplace Settlement and Governance

**Goal:** Model paid Marketplace order/settlement/payout state and add moderation/abuse governance before paid Marketplace operation is enabled.

**Requirements:** MARKET-03, MARKET-04

**Success criteria:**
1. Paid Marketplace installs create pending orders and checkout/payment intent state without installing the agent before verified payment evidence.
2. Verified `checkout.session.completed` events for Marketplace installs create one install, one paid order, and one settlement with gross amount, platform fee, publisher net, payout state, and append-only lifecycle/governance evidence.
3. Refund events update Marketplace order refund state and adjust/reverse settlement state exactly once.
4. Admin takedown, publisher appeal, admin reinstate, abuse report, and abuse resolution/dismissal workflows are recorded as governance events.
5. Publisher stats expose settlement-backed gross, platform fee, net, refund, pending, available, payout-pending, and paid-out amounts.

**Likely verification:**
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/marketplace -run 'Settlement|Governance|Abuse|Payout|PublisherStats' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Marketplace.*(Paid|Settlement|Refund|Takedown|Appeal|Abuse|PublisherStats)|Stripe.*Marketplace' -count=1`
- `cd src/server && TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/marketplace ./internal/stripe ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`
- `git diff --check`

**Planning evidence:**
- Context: `.planning/phases/19-marketplace-settlement-and-governance/19-CONTEXT.md`
- Plan: `.planning/phases/19-marketplace-settlement-and-governance/19-01-PLAN.md`

**Boundaries:**
- Admin billing inspection remains Phase 20.
- External payout provider execution is not enabled in Phase 19.
- v07 production operations and v08 product completeness remain required.

## Archived Milestone: v05 Relay Billing Completeness — Complete

**Goal:** Make the Relay invariant true for every commercial AI surface: every `/v1/*` endpoint is classified, unsupported production behavior fails closed, supported behavior has auth/rate-limit/billing/audit semantics, and provider-bypass checks prove app services cannot call upstream LLM providers outside Relay.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 13 | Relay Endpoint Authority and Production Fail-Closed | RELAY-08, RELAY-09 | Complete |
| Phase 14 | Relay Provider Bypass and Cost-Abuse Guardrails | RELAY-10, RELAY-11 | Complete |
| Phase 15 | Relay Billing Settlement and Refund Semantics | BILL-01, BILL-02 | Complete |
| Phase 16 | Relay Authority Evidence and v05 Closeout | DOC-04 | Complete |

### Phase 13: Relay Endpoint Authority and Production Fail-Closed

**Goal:** Create the authoritative route policy registry for all registered `/v1/*` endpoints and enforce production fail-closed behavior for disabled or partial endpoints before any provider call.

**Requirements:** RELAY-08, RELAY-09

**Success criteria:**
1. Policy tests prove every route from `src/server/internal/relay/handler/router.go` has exactly one commercial class.
2. Supported routes are explicitly marked as billed commercial surfaces; disabled routes list the reason and owning future work.
3. Production mode rejects disabled or partial routes before invoking native, passthrough, or file-proxy handlers.
4. `docs/API.md` and `docs/release/commercial-gates.md` expose the route table and fail-closed contract.
5. Local non-production behavior remains usable for implementation/testing where existing tests depend on route registration.

**Completion evidence:**
- Implementation commit: `3b9d4dd`
- Summary: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- Route table: `docs/release/relay-route-table.md`
- Focused tests: `cd src/server && go test ./internal/relay/handler -count=1`
- Broader package check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs gate: `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-CONTEXT.md`
- Plan: `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-PLAN.md`

### Phase 14: Relay Provider Bypass and Cost-Abuse Guardrails

**Goal:** Prove production app services can only reach upstream LLM providers through Relay and attach auth, tenant, rate-limit, and audit policy to supported endpoint classes.

**Requirements:** RELAY-10, RELAY-11

**Success criteria:**
1. CI bypass checks fail if non-Relay packages import provider SDKs, instantiate direct provider clients, or hard-code direct provider URLs for AI calls.
2. Relay external/public entry points require an authenticated tenant/API identity appropriate to the endpoint class.
3. Supported endpoint classes have rate-limit policies that prevent cost-abuse before provider calls.
4. Relay audit events capture request identity, organization, endpoint class, policy result, channel, and failure reason.
5. App-internal Chat, Agent, and Knowledge embedding paths keep using Relay metadata and trusted internal headers.

**Likely verification:**
- `bash scripts/check.sh relay-security`
- `cd src/server && go test ./internal/relay ./internal/http -run 'Auth|RateLimit|Audit|Bypass' -count=1`
- Targeted `rg` checks proving direct provider URLs are limited to Relay/channel adapters and docs/examples.

**Completion evidence:**
- Summary: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-SUMMARY.md`
- Relay security gate: `bash scripts/check.sh relay-security`
- Focused policy tests: `cd src/server && go test ./internal/relay/handler -run 'SupportedRoutePolicies|ProductionSupportedRoutesRequireTrustedIdentity|ProductionSupportedRoutesAttachTrustedIdentityAndAudit|FailClosed|RoutePolicy' -count=1`
- App metadata tests: `cd src/server && go test ./internal/chat ./internal/memory -run 'HTTPReplyGenerator|RelayEmbedder|RelayIdentity' -count=1`
- Broader relay/http check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs gate: `bash scripts/check.sh docs`

**Planning evidence:**
- Context: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-CONTEXT.md`
- Plan: `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-01-PLAN.md`

### Phase 15: Relay Billing Settlement and Refund Semantics

**Goal:** Make supported Relay calls charge quota exactly once and refund correctly across success, upstream failure, retry, streaming abort, and async/disabled endpoint behavior.

**Requirements:** BILL-01, BILL-02

**Success criteria:**
1. Billing sessions are scoped by organization, user, endpoint/API type, model, channel, and idempotency key.
2. Successful supported calls pre-authorize and settle exactly once per idempotency key.
3. Upstream errors, client aborts, and unsupported production rejections refund or avoid charge consistently.
4. Streaming/realtime, file, batch, and async flows either have tested settlement semantics or are production-disabled with documented reason.
5. Regression tests cover native, passthrough/file-proxy-disabled, and streaming paths.

**Likely verification:**
- `cd src/server && go test ./internal/relay ./internal/quota -run 'Billing|Settlement|Refund|Idempotency|Streaming' -count=1`
- DB-backed server integration tests for Relay billing and quota ledger behavior.

**Completion evidence:**
- Summary: `.planning/phases/15-relay-billing-settlement-and-refund-semantics/15-01-SUMMARY.md`
- Focused billing lifecycle tests: `cd src/server && go test ./internal/relay -run 'DefaultPricingCovers|RouteWithBilling|BillingHook_Duplicate.*FreshSession' -count=1`
- Provider usage and route policy tests: `cd src/server && go test ./internal/relay/channel -run 'EstimateUsage' -count=1` and `cd src/server && go test ./internal/relay/handler -run 'ProviderResponseFromHTTP|ResponsesStreaming|BillingSettlementPolicy|RoutePoliciesDeclareBilling' -count=1`
- Broader relay/http check: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- Docs and Relay security gates: `bash scripts/check.sh docs` and `bash scripts/check.sh relay-security`

### Phase 16: Relay Authority Evidence and v05 Closeout

**Goal:** Close v05 with reproducible evidence while keeping v06-v08 visible as required future commercial work.

**Requirements:** DOC-04

**Success criteria:**
1. `docs/release/relay-route-table.md` or equivalent documents every `/v1/*` route class, auth policy, rate-limit policy, billing policy, audit behavior, and production status.
2. `docs/release/commercial-gates.md` marks the Relay Authority Gate evidence as complete only after Phase 13-16 verification passes.
3. v05 verification records exact commands, environment class, DB migration status, passed checks, skipped checks, and residual v06-v08 work.
4. `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`, and `.planning/PROJECT.md` close v05 without claiming final commercial readiness.

**Likely verification:**
- `bash scripts/check.sh all`
- `bash scripts/test.sh all` with DB-backed coverage enabled
- Targeted `rg` checks for route table and commercial-gate references.

**Completion evidence:**
- Summary: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-01-SUMMARY.md`
- Verification: `.planning/phases/16-relay-authority-evidence-and-v05-closeout/16-VERIFICATION.md`
- Route table: `docs/release/relay-route-table.md`
- Commercial gate: `docs/release/commercial-gates.md`
- Docs gate: `bash scripts/check.sh docs`
- Relay security gate: `bash scripts/check.sh relay-security`
- Focused Relay/http tests: `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- DB-backed all tests: `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... OBLIVIOUS_REQUIRE_TEST_DATABASE=true bash scripts/test.sh all`
- Broad checks: `GOPROXY=... GOSUMDB=... bash scripts/check.sh all`

## Traceability

| Requirement | Phase | Coverage |
|-------------|-------|----------|
| RELAY-08 | Phase 13 | Complete — route policy registry and complete `/v1/*` classification |
| RELAY-09 | Phase 13 | Complete — production fail-closed behavior for disabled/partial endpoints |
| RELAY-10 | Phase 14 | Complete — provider-bypass CI checks |
| RELAY-11 | Phase 14 | Complete — endpoint auth/rate-limit/audit semantics |
| BILL-01 | Phase 15 | Complete — quota pre-authorization, exactly-once settlement, and refund behavior |
| BILL-02 | Phase 15 | Complete — streaming/realtime/file/batch/async settlement or production disablement |
| DOC-04 | Phase 16 | Complete — v05 route table, evidence, and closeout |

## Archived Milestone Details

<details>
<summary>✅ v04 Commercial Foundation — COMPLETE 2026-05-28</summary>

**Goal:** Establish the SaaS tenant, security, migration, and CI foundation required before commercial Relay billing, Marketplace settlement, production operations, and final product completeness work.

**Requirements:** TENANT-01, TENANT-02, TENANT-03, TENANT-04, TENANT-05, SEC-01, SEC-02, SEC-03, MIGR-01, CI-01, DOC-03.

**Delivered:**
- Phase 9 organization tenant model and append-only migration ledger.
- Phase 10 auditable memberships, roles, invitations, ownership transfer, CSRF, rate limits, password policy, and session rotation.
- Phase 11 tenant scope across Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data with DB-backed cross-tenant denial tests.
- Phase 12 DB-backed CI required mode and commercial gate documentation.

**Archive snapshots:**
- `.planning/milestones/v04-ROADMAP.md`
- `.planning/milestones/v04-REQUIREMENTS.md`
- `.planning/milestones/v04-STATE.md`

</details>

<details>
<summary>✅ v03.3 Mainline Consolidation — COMPLETE 2026-05-27</summary>

**Goal:** Make the current uncommitted mainline work coherent, verified, documented, and ready for clean commits.

**Requirements:** CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02, DOC-02, VERIFY-01.

**Delivered:**
- Phase 5 dirty worktree triage and commit-boundary inventory.
- Phase 6 backend route/service/auth/Relay hardening.
- Phase 7 frontend, E2E, CI, Docker, and deployment gate alignment.
- Phase 8 contract docs and release verification.
- Phase 999.1 missing Phase 01 summary reconstruction.
- Phase 999.2 obsolete workspace MarketplacePage cleanup and living requirements close policy verification.

**Archive snapshots:**
- `.planning/milestones/v03.3-ROADMAP.md`
- `.planning/milestones/v03.3-REQUIREMENTS.md`
- `.planning/milestones/v03.3-STATE.md`

</details>

<details>
<summary>✅ v03.2 Quality and Release — SHIPPED 2026-05-14</summary>

**Goal:** 补齐集成测试、E2E、API 文档和部署发布能力。

**Requirements:** TEST-01, TEST-02, DOC-01, DEPLOY-01.

**Primary verification:**
- `bash scripts/check.sh all`
- `bash scripts/test.sh all`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e`
- `docker compose config`
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`

**Archive:** `.planning/milestones/v03.2-ROADMAP.md`

</details>

<details>
<summary>✅ v03.1 Admin and Marketplace UI — SHIPPED 2026-05-02</summary>

**Goal:** 实现 Admin 管理面板 UI 和 Marketplace 前端页面（8 Admin + 4 Marketplace 页面合同）。

**Requirements:** ADMIN-04, MARKET-02.

**Verification:** Go handler suite, 12 focused Vitest files / 32 tests, and `tsc --noEmit` passed.

**Archive:** `.planning/milestones/v03.1-ROADMAP.md`

</details>

<details>
<summary>✅ Foundation through Admin/Marketplace Backend — COMPLETED 2026-04-27 to 2026-04-29</summary>

- [x] Phase 1 Relay/Chat/Agent/MCP foundation — RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07.
- [x] Phase 2 Agent 与 Memory 增强 — EXEC-01~03, MEM-01~03, QUOTA-01.
- [x] Phase 3a Admin 与 Marketplace 后端 — ADMIN-01~03, MARKET-01.

</details>

## Progress

| Milestone | Scope | Plans | Requirements | Status | Completed |
|-----------|-------|-------|--------------|--------|-----------|
| v06 Billing And Marketplace Operations | Phases 17-20 | 2/4 plans complete, Phase 19 planned | 3/7 requirements complete | Active | — |
| v05 Relay Billing Completeness | Phases 13-16 | 4/4 plans complete | 7/7 requirements complete | Complete | 2026-05-28 |
| v04 Commercial Foundation | Phases 9-12 | 4/4 plans complete | 11/11 requirements complete | Complete | 2026-05-28 |
| v03.3 Mainline Consolidation | Phases 5-8 plus backlog 999.1 and 999.2 | 12/12 steps complete | 7/7 requirements complete | Complete | 2026-05-27 |
| v03.2 Quality and Release | Phase 4 | 4/4 | TEST-01, TEST-02, DOC-01, DEPLOY-01 | Shipped | 2026-05-14 |
| v03.1 Admin and Marketplace UI | Phase 03.1 | 7/7 | ADMIN-04, MARKET-02 | Shipped | 2026-05-02 |
| Foundation through Backend | Phases 1, 2, 3a | Historical | RELAY, CHAT, AGENT, MCP, MEM, EXEC, QUOTA, ADMIN, MARKET | Complete | 2026-04-29 |

---
*Roadmap updated: 2026-05-28 after planning Phase 19 Marketplace Settlement and Governance*

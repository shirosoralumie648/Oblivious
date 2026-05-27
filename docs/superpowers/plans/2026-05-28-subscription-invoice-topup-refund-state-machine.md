# Subscription Invoice Top-up Refund State Machine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply verified Stripe events to subscription, invoice, top-up, failed-payment, plan-change, and refund state exactly once with append-only lifecycle evidence.

**Architecture:** Keep Phase 17's webhook signature and ledger boundary. Add a `LifecycleService` in `src/server/internal/stripe` that runs only after a provider event is newly inserted into `stripe_webhook_events`, writes business state through a SQL lifecycle store, and records append-only `billing_lifecycle_events` transitions. HTTP checkout gains a payment-backed `topup` kind, while the legacy direct quota top-up route stops crediting commercial quota without payment evidence.

**Tech Stack:** Go, `net/http`, PostgreSQL, `github.com/stripe/stripe-go/v83`, existing session/CSRF middleware, existing quota service, DB-backed HTTP tests.

---

### Task 1: Billing Lifecycle Schema

**Files:**
- Create: `src/server/migrations/0029_billing_lifecycle.sql`
- Modify: `src/server/internal/http/server_test.go`

- [ ] **Step 1: Add test database schema surfaces**

Add test schema entries for `billing_lifecycle_events`, `billing_invoices`, `billing_refunds`, provider reconciliation columns on `payment_intents`, provider lifecycle columns on `subscriptions`, and reconciliation/refund columns on `topup_orders`.

- [ ] **Step 2: Create migration**

Create `src/server/migrations/0029_billing_lifecycle.sql` with the same tables, indexes, and check constraints. Required unique keys:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_lifecycle_transition_key ON billing_lifecycle_events(transition_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_invoices_provider_invoice ON billing_invoices(provider, provider_invoice_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_refunds_provider_refund ON billing_refunds(provider, provider_refund_id);
```

- [ ] **Step 3: Run schema compile check**

Run: `cd src/server && go test ./internal/http -run TestNonexistentBillingLifecycleSchema -count=1`

Expected: no test is found yet. Continue to behavior tests.

### Task 2: Checkout Completion Lifecycle RED/GREEN

**Files:**
- Create: `src/server/internal/stripe/lifecycle_test.go`
- Create: `src/server/internal/stripe/lifecycle.go`
- Modify: `src/server/internal/stripe/payment.go`
- Modify: `src/server/internal/quota/service.go`

- [ ] **Step 1: Write failing tests**

Add:

```go
func TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce(t *testing.T)
func TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce(t *testing.T)
```

The subscription test must prove one active organization-scoped subscription, completed payment intent, updated `users.plan_id`, and one transition after duplicate application. The top-up test must prove one paid top-up order, one quota credit, completed payment intent, and one transition after duplicate application.

- [ ] **Step 2: Run RED**

Run: `cd src/server && go test ./internal/stripe -run 'LifecycleApplyCheckoutSessionCompleted' -count=1`

Expected: FAIL because `LifecycleService` does not exist.

- [ ] **Step 3: Implement checkout lifecycle**

Implement `LifecycleService.ApplyStripeEvent`, `SQLLifecycleStore`, checkout metadata parsing, payment intent completion, subscription create/update, top-up paid marking, quota crediting, and transition recording.

- [ ] **Step 4: Run GREEN**

Run: `cd src/server && go test ./internal/stripe -run 'LifecycleApplyCheckoutSessionCompleted' -count=1`

Expected: PASS.

### Task 3: Invoice, Subscription, Refund Lifecycle RED/GREEN

**Files:**
- Modify: `src/server/internal/stripe/lifecycle_test.go`
- Modify: `src/server/internal/stripe/lifecycle.go`
- Modify: `src/server/internal/stripe/payment.go`

- [ ] **Step 1: Write failing tests**

Add:

```go
func TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions(t *testing.T)
func TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions(t *testing.T)
func TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup(t *testing.T)
```

- [ ] **Step 2: Run RED**

Run: `cd src/server && go test ./internal/stripe -run 'LifecycleApply(Invoice|Subscription|Refund)' -count=1`

Expected: FAIL because only checkout completion has been implemented.

- [ ] **Step 3: Implement lifecycle events**

Implement invoice upsert, invoice paid/payment-failed state, subscription update/delete state, scheduled plan-change application, refund upsert, payment intent refund state, top-up quota reversal, and transition records.

- [ ] **Step 4: Run GREEN**

Run: `cd src/server && go test ./internal/stripe -run 'LifecycleApply(Invoice|Subscription|Refund|CheckoutSessionCompleted)' -count=1`

Expected: PASS.

### Task 4: Webhook And Top-up Route Integration

**Files:**
- Modify: `src/server/internal/stripe/webhook.go`
- Modify: `src/server/internal/stripe/webhook_test.go`
- Modify: `src/server/internal/http/billing_handler.go`
- Modify: `src/server/internal/http/quota_handler.go`
- Modify: `src/server/internal/http/router.go`
- Modify: `src/server/internal/http/stripe_handler_test.go`

- [ ] **Step 1: Write failing DB-backed route tests**

Add:

```go
func TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce(t *testing.T)
func TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook(t *testing.T)
func TestQuotaTopupEndpointNoLongerCreditsWithoutPayment(t *testing.T)
```

- [ ] **Step 2: Run RED**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Subscription' -count=1`

Expected: FAIL because webhook lifecycle application and top-up checkout are not wired.

- [ ] **Step 3: Wire lifecycle service**

Add lifecycle applier injection to `WebhookHandler`, construct it in `router.go`, support `kind=topup` in checkout, create pending `topup_orders`, and prevent direct quota top-up from crediting without payment evidence.

- [ ] **Step 4: Run GREEN**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Subscription' -count=1`

Expected: PASS.

### Task 5: Evidence And Closeout

**Files:**
- Modify: `docs/release/commercial-gates.md`
- Modify: `.planning/REQUIREMENTS.md`
- Modify: `.planning/ROADMAP.md`
- Modify: `.planning/STATE.md`
- Modify: `.planning/PROJECT.md`
- Create: `.planning/phases/18-subscription-invoice-topup-refund-state-machine/18-01-SUMMARY.md`

- [ ] **Step 1: Run focused verification**

Run: `cd src/server && go test ./internal/stripe -run 'Lifecycle|CheckoutCompleted|Topup|Invoice|Refund|Subscription' -count=1`

Expected: PASS.

- [ ] **Step 2: Run DB-backed route verification**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook|Topup|Invoice|Refund|Subscription' -count=1`

Expected: PASS with no DB-backed skip.

- [ ] **Step 3: Run broader gates**

Run:

```bash
cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/stripe ./internal/http ./internal/quota -count=1
bash scripts/check.sh docs
GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn bash scripts/check.sh all
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 4: Record PAY-03 completion only**

Mark PAY-03 complete after verification. Keep MARKET-03, MARKET-04, ADMIN-BILL-01, DOC-05, v07, and v08 open.

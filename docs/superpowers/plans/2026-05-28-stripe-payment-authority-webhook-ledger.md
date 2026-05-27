# Stripe Payment Authority and Webhook Ledger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mount Stripe checkout/webhook routes and persist tenant-aware payment/webhook ledger records without relying on live Stripe network calls in tests.

**Architecture:** Add a narrow payment authority layer inside `src/server/internal/stripe`: one store for checkout intents and webhook event idempotency, one checkout client interface, and one webhook handler that verifies raw-body signatures before parsing. HTTP glue in `src/server/internal/http` mounts authenticated checkout and public signature-verified webhook routes.

**Tech Stack:** Go, `net/http`, PostgreSQL, `github.com/stripe/stripe-go/v83`, existing HTTP session/CSRF middleware, existing DB-backed server tests.

---

### Task 1: Payment Authority Schema

**Files:**
- Create: `src/server/migrations/0028_payment_authority.sql`
- Modify: `src/server/internal/http/server_test.go`

- [ ] **Step 1: Write the failing migration-backed test setup**

Add the following test table definitions to `testDatabase` after `topup_orders` so later route tests can query the new tables:

```go
`DROP TABLE IF EXISTS stripe_webhook_events CASCADE`,
`DROP TABLE IF EXISTS payment_intents CASCADE`,
...
`CREATE TABLE payment_intents (id TEXT PRIMARY KEY, provider TEXT NOT NULL, provider_checkout_session_id TEXT UNIQUE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, package_id TEXT REFERENCES packages(id), kind TEXT NOT NULL, amount DECIMAL(15,6) NOT NULL, currency TEXT NOT NULL DEFAULT 'usd', status TEXT NOT NULL DEFAULT 'pending', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
`CREATE TABLE stripe_webhook_events (id TEXT PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', event_id TEXT NOT NULL UNIQUE, event_type TEXT NOT NULL, status TEXT NOT NULL, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, user_id TEXT REFERENCES users(id) ON DELETE SET NULL, payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, payload JSONB NOT NULL, error TEXT, received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), processed_at TIMESTAMPTZ)`,
```

- [ ] **Step 2: Create the migration**

Create `src/server/migrations/0028_payment_authority.sql` with the same tables, indexes, and check constraints used by tests.

- [ ] **Step 3: Run schema-focused compile/test**

Run: `cd src/server && go test ./internal/http -run TestNonexistentStripePaymentAuthoritySchema -count=1`

Expected: no test is found yet, confirming this task only prepares schema surfaces.

### Task 2: Webhook Ledger RED/GREEN

**Files:**
- Create: `src/server/internal/stripe/webhook_test.go`
- Modify: `src/server/internal/stripe/webhook.go`

- [ ] **Step 1: Write failing webhook tests**

Add tests named:

```go
func TestWebhookRejectsInvalidSignature(t *testing.T)
func TestWebhookRecordsSignedEventOnce(t *testing.T)
func TestWebhookDuplicateEventDoesNotInsertTwice(t *testing.T)
```

The signed tests must use `webhook.GenerateTestSignedPayload` from `github.com/stripe/stripe-go/v83/webhook` with a shared secret and a `checkout.session.completed` fixture containing `organization_id`, `user_id`, and `plan_id` metadata.

- [ ] **Step 2: Run RED**

Run: `cd src/server && go test ./internal/stripe -run 'Webhook' -count=1`

Expected: tests fail because the current handler records to `audit_logs`, has no dedicated ledger table, and does not expose the new store behavior.

- [ ] **Step 3: Implement the minimal ledger**

Add `PaymentEventStore`, `SQLPaymentEventStore`, and webhook handler logic that inserts into `stripe_webhook_events` with `status='processed'` or `status='failed'` and treats duplicate `event_id` as already received.

- [ ] **Step 4: Run GREEN**

Run: `cd src/server && go test ./internal/stripe -run 'Webhook' -count=1`

Expected: all webhook tests pass.

### Task 3: Checkout Route RED/GREEN

**Files:**
- Create: `src/server/internal/http/stripe_handler_test.go`
- Create: `src/server/internal/http/billing_handler.go`
- Modify: `src/server/internal/stripe/checkout.go`
- Modify: `src/server/internal/http/router.go`
- Modify: `src/server/internal/config/config.go`
- Modify: `src/server/internal/config/config_test.go`

- [ ] **Step 1: Write failing route tests**

Add tests named:

```go
func TestStripeWebhookRouteRejectsInvalidSignature(t *testing.T)
func TestBillingCheckoutRequiresSession(t *testing.T)
func TestBillingCheckoutPersistsTenantPaymentIntent(t *testing.T)
```

The checkout test should register a user, create/select an organization, insert a package row, send `POST /api/v1/billing/checkout` with CSRF, and assert one `payment_intents` row for the active `organization_id`.

- [ ] **Step 2: Run RED**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`

Expected: route tests fail because routes are not mounted.

- [ ] **Step 3: Implement checkout route glue**

Add `billingHandler` with a checkout creator interface. In tests use a fake creator returning `cs_test_phase17`; in production use the Stripe client from `src/server/internal/stripe/checkout.go`.

- [ ] **Step 4: Run GREEN**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`

Expected: route tests pass with DB-backed payment intent evidence.

### Task 4: Phase Verification and Docs

**Files:**
- Modify: `.planning/REQUIREMENTS.md`
- Modify: `.planning/ROADMAP.md`
- Modify: `.planning/STATE.md`
- Create: `.planning/phases/17-stripe-payment-authority-and-webhook-ledger/17-01-SUMMARY.md`

- [ ] **Step 1: Run focused package verification**

Run: `cd src/server && go test ./internal/stripe ./internal/config -count=1`

Expected: both packages pass.

- [ ] **Step 2: Run DB-backed HTTP verification**

Run: `cd src/server && TEST_DATABASE_URL="$TEST_DATABASE_URL" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run 'Stripe|Billing|Checkout|Webhook' -count=1`

Expected: payment route tests pass and no DB-backed tests are silently skipped.

- [ ] **Step 3: Run docs and diff hygiene**

Run: `bash scripts/check.sh docs && git diff --check`

Expected: both commands exit 0.

- [ ] **Step 4: Record summary**

Mark PAY-01 and PAY-02 complete only if the exact verification commands above pass. Keep PAY-03, MARKET-03, MARKET-04, ADMIN-BILL-01, and DOC-05 open for later v06 phases.

-- Scope provider webhook idempotency by provider and event id.

ALTER TABLE IF EXISTS stripe_webhook_events
  DROP CONSTRAINT IF EXISTS stripe_webhook_events_provider_check;

ALTER TABLE IF EXISTS stripe_webhook_events
  ADD CONSTRAINT stripe_webhook_events_provider_check
  CHECK (provider <> '');

ALTER TABLE IF EXISTS stripe_webhook_events
  DROP CONSTRAINT IF EXISTS stripe_webhook_events_event_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_stripe_webhook_events_provider_event_id
  ON stripe_webhook_events (provider, event_id);

ALTER TABLE IF EXISTS billing_lifecycle_events
  DROP CONSTRAINT IF EXISTS billing_lifecycle_events_provider_check;

ALTER TABLE IF EXISTS billing_lifecycle_events
  ADD CONSTRAINT billing_lifecycle_events_provider_check
  CHECK (provider <> '');

ALTER TABLE IF EXISTS billing_invoices
  DROP CONSTRAINT IF EXISTS billing_invoices_provider_check;

ALTER TABLE IF EXISTS billing_invoices
  ADD CONSTRAINT billing_invoices_provider_check
  CHECK (provider <> '');

ALTER TABLE IF EXISTS billing_refunds
  DROP CONSTRAINT IF EXISTS billing_refunds_provider_check;

ALTER TABLE IF EXISTS billing_refunds
  ADD CONSTRAINT billing_refunds_provider_check
  CHECK (provider <> '');

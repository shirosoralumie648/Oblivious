-- Allow paid marketplace checkout creation failures to fail-close local orders.

ALTER TABLE marketplace_orders DROP CONSTRAINT IF EXISTS marketplace_orders_status_check;
ALTER TABLE marketplace_orders
    ADD CONSTRAINT marketplace_orders_status_check
    CHECK (status IN ('pending_payment', 'paid', 'partially_refunded', 'refunded', 'cancelled', 'failed'));

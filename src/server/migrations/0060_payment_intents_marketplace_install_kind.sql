-- Allow Marketplace paid installs to share the authoritative payment_intents table.
-- Earlier billing migrations only allowed subscription and topup intents.

ALTER TABLE payment_intents DROP CONSTRAINT IF EXISTS payment_intents_kind_check;

ALTER TABLE payment_intents
    ADD CONSTRAINT payment_intents_kind_check
    CHECK (kind IN ('subscription', 'topup', 'marketplace_install'));

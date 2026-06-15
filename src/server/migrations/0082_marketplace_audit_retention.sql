-- Preserve paid Marketplace audit evidence even if a referenced mutable entity is removed.
-- Application code should use takedown/archive paths for published agents with order history.

ALTER TABLE marketplace_orders
    DROP CONSTRAINT IF EXISTS marketplace_orders_agent_id_fkey,
    ADD CONSTRAINT marketplace_orders_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES published_agents(id) ON DELETE RESTRICT;

ALTER TABLE marketplace_orders
    DROP CONSTRAINT IF EXISTS marketplace_orders_payment_intent_id_fkey,
    ADD CONSTRAINT marketplace_orders_payment_intent_id_fkey
        FOREIGN KEY (payment_intent_id) REFERENCES payment_intents(id) ON DELETE RESTRICT;

ALTER TABLE marketplace_settlements
    DROP CONSTRAINT IF EXISTS marketplace_settlements_order_id_fkey,
    ADD CONSTRAINT marketplace_settlements_order_id_fkey
        FOREIGN KEY (order_id) REFERENCES marketplace_orders(id) ON DELETE RESTRICT;

ALTER TABLE marketplace_settlements
    DROP CONSTRAINT IF EXISTS marketplace_settlements_agent_id_fkey,
    ADD CONSTRAINT marketplace_settlements_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES published_agents(id) ON DELETE RESTRICT;

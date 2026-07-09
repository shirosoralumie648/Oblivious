-- Align Marketplace governance action constraints with runtime review events.
-- 0030 created the table before automated review and needs_changes events were
-- written by production code, so deployed databases need a forward migration.

ALTER TABLE marketplace_governance_events
    DROP CONSTRAINT IF EXISTS marketplace_governance_events_action_check;

ALTER TABLE marketplace_governance_events
    ADD CONSTRAINT marketplace_governance_events_action_check
    CHECK (
        action IN (
            'publish',
            'approve',
            'reject',
            'takedown',
            'appeal',
            'appeal_reject',
            'reinstate',
            'review_assign',
            'abuse_report',
            'abuse_resolve',
            'abuse_dismiss',
            'payout_state',
            'automated_review_pass',
            'automated_review_reject',
            'needs_changes'
        )
    );

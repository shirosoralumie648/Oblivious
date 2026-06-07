-- Allow retry workers to claim due outbound channel messages without changing
-- historical retry scheduling metadata.

ALTER TABLE channel_messages
    DROP CONSTRAINT IF EXISTS channel_messages_status_check;

ALTER TABLE channel_messages
    ADD CONSTRAINT channel_messages_status_check
    CHECK (status IN ('recorded', 'retry_pending', 'sending', 'permanent_failure'));

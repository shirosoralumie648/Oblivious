-- User Preferences Extension
-- Adds notification preferences and extended settings

-- Add notification preferences column
ALTER TABLE user_preferences ADD COLUMN IF NOT EXISTS notifications JSONB DEFAULT '{}';

-- Add default_agent_model column
ALTER TABLE user_preferences ADD COLUMN IF NOT EXISTS default_agent_model TEXT DEFAULT 'gpt-4o-mini';

-- Add sidebar_collapsed column
ALTER TABLE user_preferences ADD COLUMN IF NOT EXISTS sidebar_collapsed BOOLEAN DEFAULT false;

-- Notifications table for in-app notifications
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,              -- 'info' | 'warning' | 'error' | 'success'
    category TEXT NOT NULL,          -- 'billing' | 'agent' | 'system' | 'mcp'
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT false,
    action_url TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read) WHERE is_read = false;

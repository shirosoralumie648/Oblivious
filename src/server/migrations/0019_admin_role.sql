-- Admin Role Extension
-- Adds role field to users table for admin functionality

-- Add role column to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

-- Add last_login_at for tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- Add name column if not exists
ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;

-- Create index for admin queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);

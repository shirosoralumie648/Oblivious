-- Membership, roles, and auth security
-- v04 Phase 10: TENANT-02, TENANT-03, SEC-01, SEC-02, SEC-03

CREATE TABLE IF NOT EXISTS organization_memberships (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at TIMESTAMPTZ,
    CHECK (role IN ('owner', 'admin', 'member'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_memberships_active_user
    ON organization_memberships(organization_id, user_id)
    WHERE removed_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_memberships_single_owner
    ON organization_memberships(organization_id)
    WHERE role = 'owner' AND removed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_organization_memberships_org
    ON organization_memberships(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_memberships_user
    ON organization_memberships(user_id);

CREATE TABLE IF NOT EXISTS organization_invitations (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    role TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending',
    invited_by_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    accepted_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (role IN ('admin', 'member')),
    CHECK (status IN ('pending', 'accepted', 'revoked', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_org
    ON organization_invitations(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_invitations_email
    ON organization_invitations(email);
CREATE INDEX IF NOT EXISTS idx_organization_invitations_status
    ON organization_invitations(status);
CREATE INDEX IF NOT EXISTS idx_organization_invitations_token_hash
    ON organization_invitations(token_hash);

CREATE TABLE IF NOT EXISTS auth_rate_limits (
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    blocked_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (scope, key)
);

CREATE INDEX IF NOT EXISTS idx_auth_rate_limits_blocked_until
    ON auth_rate_limits(blocked_until);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user
    ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_hash
    ON password_reset_tokens(token_hash);

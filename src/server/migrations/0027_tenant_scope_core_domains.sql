-- Tenant scope across core domains
-- v04 Phase 11: TENANT-04, TENANT-05

ALTER TABLE IF EXISTS workspaces ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS sessions ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS conversations ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS conversation_configs ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS conversation_knowledge_bindings ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS knowledge_bases ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS knowledge_documents ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS knowledge_document_chunks ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agents ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agent_conversations ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agent_messages ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS memory_documents ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS memory_chunks ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS mcp_servers ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS usage_records ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS quotas ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS billing_sessions ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS subscriptions ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS topup_orders ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS published_agents ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agent_versions ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agent_installs ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS agent_reviews ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS audit_logs ADD COLUMN IF NOT EXISTS organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL;

INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
SELECT
    'org_legacy_' || md5(w.id),
    'legacy-' || substr(md5(w.id), 1, 24),
    COALESCE(NULLIF(w.name, ''), 'Legacy Workspace') || ' Organization',
    'active',
    jsonb_build_object('source', 'workspace_backfill', 'workspaceId', w.id),
    w.user_id
FROM workspaces w
WHERE w.organization_id IS NULL
ON CONFLICT (id) DO NOTHING;

UPDATE workspaces w
SET organization_id = 'org_legacy_' || md5(w.id)
WHERE w.organization_id IS NULL;

INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
SELECT
    'membership_legacy_' || md5(w.organization_id || ':' || w.user_id),
    w.organization_id,
    w.user_id,
    'owner',
    w.user_id
FROM workspaces w
WHERE w.organization_id IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
SELECT
    'org_user_' || md5(u.id),
    'user-' || substr(md5(u.id), 1, 24),
    COALESCE(NULLIF(u.email, ''), u.id) || ' Organization',
    'active',
    jsonb_build_object('source', 'user_backfill', 'userId', u.id),
    u.id
FROM users u
WHERE NOT EXISTS (
    SELECT 1
    FROM organization_memberships m
    WHERE m.user_id = u.id AND m.removed_at IS NULL
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
SELECT
    'membership_user_' || md5(u.id),
    'org_user_' || md5(u.id),
    u.id,
    'owner',
    u.id
FROM users u
WHERE NOT EXISTS (
    SELECT 1
    FROM organization_memberships m
    WHERE m.user_id = u.id AND m.removed_at IS NULL
)
ON CONFLICT DO NOTHING;

UPDATE sessions s
SET organization_id = COALESCE(
    w.organization_id,
    (
        SELECT m.organization_id
        FROM organization_memberships m
        WHERE m.user_id = s.user_id AND m.removed_at IS NULL
        ORDER BY m.created_at ASC
        LIMIT 1
    )
)
FROM workspaces w
WHERE s.workspace_id = w.id
  AND s.organization_id IS NULL;

UPDATE conversations c
SET organization_id = w.organization_id
FROM workspaces w
WHERE c.workspace_id = w.id
  AND c.organization_id IS NULL;

UPDATE messages m
SET organization_id = c.organization_id
FROM conversations c
WHERE m.conversation_id = c.id
  AND m.organization_id IS NULL;

UPDATE conversation_configs cfg
SET organization_id = c.organization_id
FROM conversations c
WHERE cfg.conversation_id = c.id
  AND cfg.organization_id IS NULL;

UPDATE conversation_knowledge_bindings b
SET organization_id = c.organization_id
FROM conversations c
WHERE b.conversation_id = c.id
  AND b.organization_id IS NULL;

UPDATE knowledge_bases kb
SET organization_id = w.organization_id
FROM workspaces w
WHERE kb.workspace_id = w.id
  AND kb.organization_id IS NULL;

UPDATE knowledge_documents kd
SET organization_id = kb.organization_id
FROM knowledge_bases kb
WHERE kd.knowledge_base_id = kb.id
  AND kd.organization_id IS NULL;

UPDATE knowledge_document_chunks kdc
SET organization_id = kb.organization_id
FROM knowledge_documents kd
JOIN knowledge_bases kb ON kb.id = kd.knowledge_base_id
WHERE kdc.document_id = kd.id
  AND kdc.organization_id IS NULL;

UPDATE agents a
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = a.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE a.organization_id IS NULL;

UPDATE agent_conversations ac
SET organization_id = COALESCE(
    a.organization_id,
    (
        SELECT m.organization_id
        FROM organization_memberships m
        WHERE m.user_id = ac.user_id AND m.removed_at IS NULL
        ORDER BY m.created_at ASC
        LIMIT 1
    )
)
FROM agents a
WHERE ac.agent_id = a.id
  AND ac.organization_id IS NULL;

UPDATE agent_messages am
SET organization_id = ac.organization_id
FROM agent_conversations ac
WHERE am.conversation_id = ac.id
  AND am.organization_id IS NULL;

UPDATE memory_documents md
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = md.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE md.organization_id IS NULL;

UPDATE memory_chunks mc
SET organization_id = md.organization_id
FROM memory_documents md
WHERE mc.document_id = md.id
  AND mc.organization_id IS NULL;

UPDATE mcp_servers ms
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = ms.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE ms.organization_id IS NULL;

UPDATE usage_records ur
SET organization_id = COALESCE(
    (
        SELECT c.organization_id
        FROM conversations c
        WHERE c.id = ur.conversation_id
        LIMIT 1
    ),
    w.organization_id,
    (
        SELECT m.organization_id
        FROM organization_memberships m
        WHERE m.user_id = ur.user_id AND m.removed_at IS NULL
        ORDER BY m.created_at ASC
        LIMIT 1
    )
)
FROM workspaces w
WHERE ur.workspace_id = w.id
  AND ur.organization_id IS NULL;

UPDATE quotas q
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = q.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE q.organization_id IS NULL;

WITH quota_totals AS (
    SELECT
        organization_id,
        MIN(id) AS keep_id,
        SUM(balance) AS total_balance,
        SUM(used) AS total_used,
        MAX(updated_at) AS newest_updated_at
    FROM quotas
    WHERE organization_id IS NOT NULL
    GROUP BY organization_id
),
updated AS (
    UPDATE quotas q
    SET
        balance = qt.total_balance,
        used = qt.total_used,
        updated_at = GREATEST(q.updated_at, qt.newest_updated_at)
    FROM quota_totals qt
    WHERE q.id = qt.keep_id
    RETURNING q.id
)
DELETE FROM quotas q
USING quota_totals qt
WHERE q.organization_id = qt.organization_id
  AND q.id <> qt.keep_id;

UPDATE billing_sessions bs
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = bs.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE bs.organization_id IS NULL;

UPDATE subscriptions s
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = s.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE s.organization_id IS NULL;

UPDATE topup_orders t
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = t.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE t.organization_id IS NULL;

UPDATE published_agents pa
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = pa.owner_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE pa.organization_id IS NULL;

UPDATE agent_versions av
SET organization_id = pa.organization_id
FROM published_agents pa
WHERE av.agent_id = pa.id
  AND av.organization_id IS NULL;

UPDATE agent_installs ai
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = ai.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE ai.organization_id IS NULL;

UPDATE agent_reviews ar
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = ar.user_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE ar.organization_id IS NULL;

UPDATE audit_logs al
SET organization_id = al.resource_id
WHERE al.organization_id IS NULL
  AND al.resource_type = 'organization'
  AND EXISTS (SELECT 1 FROM organizations o WHERE o.id = al.resource_id);

UPDATE audit_logs al
SET organization_id = (
    SELECT m.organization_id
    FROM organization_memberships m
    WHERE m.user_id = al.actor_id AND m.removed_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
)
WHERE al.organization_id IS NULL;

ALTER TABLE IF EXISTS workspaces ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS sessions ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS conversations ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS messages ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS conversation_configs ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS conversation_knowledge_bindings ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS knowledge_bases ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS knowledge_documents ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS knowledge_document_chunks ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agents ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agent_conversations ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agent_messages ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS memory_documents ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS memory_chunks ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS mcp_servers ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS usage_records ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS quotas ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS billing_sessions ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS subscriptions ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS topup_orders ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS published_agents ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agent_versions ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agent_installs ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS agent_reviews ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE IF EXISTS audit_logs ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE IF EXISTS quotas DROP CONSTRAINT IF EXISTS quotas_user_id_key;
ALTER TABLE IF EXISTS billing_sessions DROP CONSTRAINT IF EXISTS billing_sessions_idempotency_key_key;

CREATE INDEX IF NOT EXISTS idx_workspaces_organization_id ON workspaces(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_workspaces_organization_user ON workspaces(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_organization_id ON sessions(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_sessions_organization_user ON sessions(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_conversations_organization_id ON conversations(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_conversations_organization_created ON conversations(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_organization_id ON messages(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_organization_created ON messages(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_configs_organization_id ON conversation_configs(organization_id, conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_knowledge_bindings_org ON conversation_knowledge_bindings(organization_id, conversation_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_organization_id ON knowledge_bases(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_organization_created ON knowledge_bases(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_documents_organization_id ON knowledge_documents(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_knowledge_documents_organization_created ON knowledge_documents(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_organization_id ON knowledge_document_chunks(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_agents_organization_id ON agents(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_agents_organization_owner ON agents(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_agent_conversations_organization_id ON agent_conversations(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_agent_conversations_organization_user ON agent_conversations(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_organization_id ON agent_messages(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_memory_documents_organization_id ON memory_documents(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_memory_documents_organization_user ON memory_documents(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_memory_chunks_organization_id ON memory_chunks(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_organization_id ON mcp_servers(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_organization_user ON mcp_servers(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_organization_created ON usage_records(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_records_organization_user ON usage_records(organization_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_quotas_unique_organization ON quotas(organization_id);
CREATE INDEX IF NOT EXISTS idx_quotas_organization_user ON quotas(organization_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_sessions_unique_org_idempotency ON billing_sessions(organization_id, idempotency_key) WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_billing_sessions_organization_user ON billing_sessions(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_billing_sessions_organization_created ON billing_sessions(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscriptions_organization_user ON subscriptions(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_topup_orders_organization_user ON topup_orders(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_published_agents_organization_id ON published_agents(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_published_agents_organization_owner ON published_agents(organization_id, owner_id);
CREATE INDEX IF NOT EXISTS idx_agent_versions_organization_id ON agent_versions(organization_id, id);
CREATE INDEX IF NOT EXISTS idx_agent_installs_organization_user ON agent_installs(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_agent_reviews_organization_user ON agent_reviews(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_organization_created ON audit_logs(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_organization_resource ON audit_logs(organization_id, resource_type, resource_id);

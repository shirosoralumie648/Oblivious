#!/bin/bash
set -euo pipefail

SERVICE="relay"
LEGACY_DB="${LEGACY_DATABASE_URL}"
NEW_DB="${NEW_DATABASE_URL_PREFIX}_${SERVICE}"

echo "Migrating ${SERVICE} service..."

# 1. 创建新库
psql "${NEW_DB}" -f "src/server/migrations/microservices/${SERVICE}.sql"

# 2. 数据迁移（从旧库复制到新库）
psql "${NEW_DB}" <<EOF
INSERT INTO channels
SELECT * FROM dblink('${LEGACY_DB}', 'SELECT * FROM channels')
AS t(id uuid, name text, type text, workspace_id uuid, created_at timestamptz, updated_at timestamptz);
EOF

# 3. 验证
go run src/server/cmd/migrate/validate.go --service="${SERVICE}" --legacy="${LEGACY_DB}" --new="${NEW_DB}"

echo "${SERVICE} migration complete."

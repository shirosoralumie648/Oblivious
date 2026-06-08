#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
migrations_dir="$repo_root/src/server/migrations"
expected_pattern='^[0-9]{4}_[a-z0-9][a-z0-9_]*\.sql$'

declare -A allowed_duplicate_prefixes=(
  [0013]="0013_channels.sql 0013_gateway_tables.sql"
  [0014]="0014_agents.sql 0014_relay_enhanced.sql"
  [0015]="0015_mcp_servers.sql 0015_workflow_enhanced.sql"
  [0016]="0016_knowledge_enhanced.sql 0016_pgvector.sql"
  [0017]="0017_agent_enhanced.sql 0017_quotas.sql"
  [0018]="0018_channel_tables.sql 0018_user_preferences_ext.sql"
  [0019]="0019_admin_role.sql 0019_task_tables.sql"
  [0020]="0020_marketplace_enhanced.sql 0020_memory_hnsw.sql"
  [0021]="0021_billing_enhanced.sql 0021_plan_extensions.sql"
  [0022]="0022_audit_logs.sql 0022_observability_tables.sql"
)

join_sorted() {
  printf '%s\n' "$@" | LC_ALL=C sort | paste -sd' ' -
}

if [[ ! -d "$migrations_dir" ]]; then
  echo "[migration-contract] missing migrations dir: $migrations_dir" >&2
  exit 1
fi

shopt -s nullglob
migration_files=("$migrations_dir"/*.sql)
if (( ${#migration_files[@]} == 0 )); then
  echo "[migration-contract] no PostgreSQL migration files found in src/server/migrations" >&2
  exit 1
fi

declare -A files_by_prefix=()

for path in "${migration_files[@]}"; do
  name=$(basename "$path")
  if [[ ! "$name" =~ $expected_pattern ]]; then
    echo "[migration-contract] invalid migration filename $name: expected NNNN_description.sql" >&2
    exit 1
  fi

  prefix="${name:0:4}"
  files_by_prefix["$prefix"]+="$name "
done

for prefix in $(printf '%s\n' "${!files_by_prefix[@]}" | LC_ALL=C sort); do
  read -r -a current_files <<< "${files_by_prefix[$prefix]}"
  if (( ${#current_files[@]} <= 1 )); then
    continue
  fi

  current=$(join_sorted "${current_files[@]}")
  allowed="${allowed_duplicate_prefixes[$prefix]:-}"
  if [[ -z "$allowed" ]]; then
    echo "[migration-contract] duplicate migration prefix $prefix is not allowed: $current" >&2
    exit 1
  fi

  read -r -a allowed_files <<< "$allowed"
  allowed_sorted=$(join_sorted "${allowed_files[@]}")
  if [[ "$current" != "$allowed_sorted" ]]; then
    echo "[migration-contract] duplicate migration prefix $prefix differs from accepted historical set" >&2
    echo "[migration-contract] current: $current" >&2
    echo "[migration-contract] accepted: $allowed_sorted" >&2
    exit 1
  fi

  echo "[migration-contract] accepted historical duplicate prefix $prefix: $current"
done

echo "[migration-contract] src/server/migrations filenames follow NNNN_description.sql."

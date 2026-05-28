#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
database_url="${BACKUP_DATABASE_URL:-${DATABASE_URL:-}}"
backup_dir="${BACKUP_DIR:-.tmp/backups}"
backup_basename="${BACKUP_BASENAME:-oblivious-$(date -u +%Y%m%d-%H%M%S)}"
client_image="${PG_CLIENT_IMAGE:-${OBLIVIOUS_POSTGRES_IMAGE:-postgres:16}}"
client_network="${PG_CLIENT_DOCKER_NETWORK:-host}"

fail() {
  echo "[backup-postgres] $*" >&2
  exit 2
}

require_client() {
  if command -v pg_dump >/dev/null 2>&1; then
    echo "host"
    return
  fi

  if command -v docker >/dev/null 2>&1; then
    echo "docker"
    return
  fi

  echo "[backup-postgres] pg_dump or docker is required" >&2
  exit 127
}

sanitize_database_url() {
  printf '%s' "$1" | sed -E 's#(postgres(ql)?://[^:/@]+):[^@]*@#\1:***@#'
}

[[ -n "$database_url" ]] || fail "BACKUP_DATABASE_URL or DATABASE_URL is required"

backup_dir_abs="$backup_dir"
if [[ "$backup_dir_abs" != /* ]]; then
  backup_dir_abs="$repo_root/$backup_dir_abs"
fi

mkdir -p "$backup_dir_abs"
chmod 700 "$backup_dir_abs"

dump_name="${backup_basename}.dump"
manifest_name="${backup_basename}.manifest"
dump_path="$backup_dir_abs/$dump_name"
manifest_path="$backup_dir_abs/$manifest_name"

client_mode=$(require_client)
if [[ "$client_mode" == "host" ]]; then
  pg_dump --format=custom --no-owner --no-privileges --file "$dump_path" "$database_url"
else
  docker run --rm \
    --network "$client_network" \
    -v "$backup_dir_abs:/backup" \
    "$client_image" \
    pg_dump --format=custom --no-owner --no-privileges --file "/backup/$dump_name" "$database_url"
fi

dump_bytes=$(wc -c <"$dump_path" | tr -d ' ')
dump_sha256=$(sha256sum "$dump_path" | awk '{print $1}')
timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
sanitized_url=$(sanitize_database_url "$database_url")

cat >"$manifest_path" <<EOF
created_at_utc=$timestamp
dump_file=$dump_name
dump_bytes=$dump_bytes
dump_sha256=$dump_sha256
database_url=$sanitized_url
client_mode=$client_mode
client_image=$client_image
EOF
chmod 600 "$manifest_path"

echo "[backup-postgres] backup written: $dump_path"
echo "[backup-postgres] manifest written: $manifest_path"

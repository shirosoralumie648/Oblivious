#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
database_url="${RESTORE_DATABASE_URL:-${DATABASE_URL:-}}"
backup_file="${BACKUP_FILE:-}"
client_image="${PG_CLIENT_IMAGE:-${OBLIVIOUS_POSTGRES_IMAGE:-postgres:16}}"
client_network="${PG_CLIENT_DOCKER_NETWORK:-host}"
migrations_dir="${MIGRATIONS_DIR:-$repo_root/src/server/migrations}"

fail() {
  echo "[restore-postgres] $*" >&2
  exit 2
}

require_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "[restore-postgres] $tool is required" >&2
    exit 127
  fi
}

resolve_client_mode() {
  if command -v pg_restore >/dev/null 2>&1 && command -v psql >/dev/null 2>&1; then
    echo "host"
    return
  fi

  if command -v docker >/dev/null 2>&1; then
    echo "docker"
    return
  fi

  echo "[restore-postgres] pg_restore/psql or docker is required" >&2
  exit 127
}

run_psql() {
  local database="$1"
  shift

  if [[ "$client_mode" == "host" ]]; then
    psql -X -v ON_ERROR_STOP=1 "$database" "$@"
    return
  fi

  docker run --rm -i \
    --network "$client_network" \
    "$client_image" \
    psql -X -v ON_ERROR_STOP=1 "$database" "$@"
}

run_pg_restore() {
  local database="$1"
  local backup="$2"
  local backup_dir
  local backup_name

  if [[ "$client_mode" == "host" ]]; then
    pg_restore --clean --if-exists --no-owner --no-privileges --dbname "$database" "$backup"
    return
  fi

  backup_dir=$(cd "$(dirname "$backup")" && pwd -P)
  backup_name=$(basename "$backup")
  docker run --rm \
    --network "$client_network" \
    -v "$backup_dir:/backup:ro" \
    "$client_image" \
    pg_restore --clean --if-exists --no-owner --no-privileges --dbname "$database" "/backup/$backup_name"
}

verify_migration_ledger() {
  local expected_count=0
  local actual_count
  local migration_path
  local version
  local version_sql
  local expected_checksum
  local actual_checksum

  shopt -s nullglob
  for migration_path in "$migrations_dir"/*.sql; do
    version=$(basename "$migration_path")
    version_sql=${version//\'/\'\'}
    expected_checksum=$(sha256sum "$migration_path" | awk '{print $1}')
    actual_checksum=$(run_psql "$database_url" -Atc "SELECT checksum FROM schema_migrations WHERE version = '$version_sql';")

    if [[ "$actual_checksum" != "$expected_checksum" ]]; then
      echo "[restore-postgres] checksum mismatch for $version: expected $expected_checksum got ${actual_checksum:-missing}" >&2
      exit 3
    fi

    expected_count=$((expected_count + 1))
  done
  shopt -u nullglob

  actual_count=$(run_psql "$database_url" -Atc "SELECT COUNT(*) FROM schema_migrations;")
  if [[ "$actual_count" != "$expected_count" ]]; then
    echo "[restore-postgres] schema_migrations count mismatch: expected $expected_count got $actual_count" >&2
    exit 3
  fi

  echo "[restore-postgres] migration ledger verified: $actual_count migrations"
}

[[ -n "$database_url" ]] || fail "RESTORE_DATABASE_URL or DATABASE_URL is required"
[[ -n "$backup_file" ]] || fail "BACKUP_FILE is required"
[[ -f "$backup_file" ]] || fail "BACKUP_FILE does not exist: $backup_file"
[[ -d "$migrations_dir" ]] || fail "migrations directory does not exist: $migrations_dir"
require_tool sha256sum

client_mode=$(resolve_client_mode)
run_pg_restore "$database_url" "$backup_file"
verify_migration_ledger

echo "[restore-postgres] restore completed: $backup_file"

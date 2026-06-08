#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
migrations_dir="$server_dir/migrations"
database_url="${MIGRATION_REPLAY_DATABASE_URL:-${TEST_DATABASE_URL:-}}"
postgres_image="${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
container_name=""

fail() {
  echo "[migration-replay] $*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$container_name" ]]; then
    docker rm -f "$container_name" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

expected_migrations=$(find "$migrations_dir" -maxdepth 1 -type f -name '*.sql' | wc -l | tr -d ' ')
if [[ "$expected_migrations" == "0" ]]; then
  fail "no PostgreSQL migrations found in src/server/migrations"
fi

if [[ -z "$database_url" ]]; then
  command -v docker >/dev/null 2>&1 || fail "docker is required when MIGRATION_REPLAY_DATABASE_URL/TEST_DATABASE_URL is not set"
  docker info >/dev/null 2>&1 || fail "docker daemon is not reachable"

  container_name="oblivious-migration-replay-$$"
  docker run -d --rm \
    --name "$container_name" \
    -e POSTGRES_DB=oblivious \
    -e POSTGRES_USER=oblivious \
    -e POSTGRES_PASSWORD=oblivious \
    -p 127.0.0.1::5432 \
    "$postgres_image" >/dev/null

  port=$(docker port "$container_name" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/')
  [[ -n "$port" ]] || fail "could not resolve mapped PostgreSQL port"
  database_url="postgres://oblivious:oblivious@127.0.0.1:${port}/oblivious?sslmode=disable"

  for attempt in $(seq 1 60); do
    if docker exec "$container_name" pg_isready -U oblivious -d oblivious >/dev/null 2>&1; then
      break
    fi
    if [[ "$attempt" == "60" ]]; then
      fail "temporary PostgreSQL did not become ready"
    fi
    sleep 1
  done
fi

corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
mkdir -p "$corepack_home" "$go_cache" "$go_mod_cache"

run_migrate() {
  (
    cd "$server_dir"
    COREPACK_HOME="$corepack_home" \
      GOCACHE="$go_cache" \
      GOMODCACHE="$go_mod_cache" \
      DATABASE_URL="$database_url" \
      SESSION_SECRET=migration-replay-smoke-secret \
      go run ./cmd/migrate
  )
}

echo "[migration-replay] applying migrations to fresh database"
run_migrate

echo "[migration-replay] replaying migrations to verify ledger skips"
second_output=$(run_migrate)
printf '%s\n' "$second_output"
if ! grep -Fq "migrations applied: 0, skipped: $expected_migrations" <<< "$second_output"; then
  fail "second migration run did not skip all $expected_migrations migrations"
fi

if command -v psql >/dev/null 2>&1; then
  actual_migrations=$(psql -X -v ON_ERROR_STOP=1 "$database_url" -Atc "SELECT COUNT(*) FROM schema_migrations;")
else
  [[ -n "$container_name" ]] || fail "psql is required to verify an external migration replay database"
  actual_migrations=$(docker exec "$container_name" psql -U oblivious -d oblivious -Atc "SELECT COUNT(*) FROM schema_migrations;")
fi

if [[ "$actual_migrations" != "$expected_migrations" ]]; then
  fail "schema_migrations count mismatch: expected $expected_migrations got $actual_migrations"
fi

echo "[migration-replay] migration replay ok: $actual_migrations migrations recorded"

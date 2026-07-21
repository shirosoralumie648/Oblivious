#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode=""
output_dir=""
database_env=""
source_root="$repo_root"
postgres_image="${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
work_root=""
container_name=""
network_name=""
database_name=""
database_user="migration_replay"
database_password=""
database_url=""
replay_mode="docker-ephemeral"
cleanup_result="succeeded"
resources_started=false
static_invocations=0
ledger_invocations=0
replay_invocations=0
surface_binary=""
migrate_binary=""
contract_binary=""
static_report=""
ledger_report=""
replay_report=""

usage() {
  cat <<'EOF'
Usage:
  bash scripts/verify-migration-replay.sh --stage-a --output-dir DIR
  bash scripts/verify-migration-replay.sh session --output-dir DIR [--database-url-env NAME]

Stage A creates a clean disposable Git checkout and always uses a unique
ephemeral pgvector PostgreSQL container. Session mode accepts only a named
environment-variable reference for an explicitly isolated external database;
otherwise it uses the same Docker path.
EOF
}

emit_error() {
  printf '{"error":{"code":"%s"}}\n' "$1" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --stage-a)
      [[ -z "$mode" ]] || { emit_error invalid_arguments; exit 2; }
      mode="stage-a"
      shift
      ;;
    session)
      [[ -z "$mode" ]] || { emit_error invalid_arguments; exit 2; }
      mode="session"
      shift
      ;;
    --output-dir)
      [[ $# -ge 2 && -z "$output_dir" ]] || { emit_error invalid_arguments; exit 2; }
      output_dir="$2"
      shift 2
      ;;
    --database-url-env)
      [[ $# -ge 2 && -z "$database_env" ]] || { emit_error invalid_arguments; exit 2; }
      database_env="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      emit_error invalid_arguments
      exit 2
      ;;
  esac
done

[[ -n "$mode" && -n "$output_dir" ]] || { emit_error invalid_arguments; exit 2; }
[[ "$database_env" =~ ^[A-Z_][A-Z0-9_]*$ || -z "$database_env" ]] || { emit_error invalid_arguments; exit 2; }
[[ "$mode" != "stage-a" || -z "$database_env" ]] || { emit_error invalid_arguments; exit 2; }

cleanup_resources() {
  local cleanup_failed=false
  if [[ -n "$container_name" ]]; then
    if ! docker rm -f "$container_name" >/dev/null 2>&1; then
      if docker inspect "$container_name" >/dev/null 2>&1; then
        cleanup_failed=true
      fi
    fi
    if docker inspect "$container_name" >/dev/null 2>&1; then
      cleanup_failed=true
    else
      container_name=""
    fi
  fi
  if [[ -n "$network_name" ]]; then
    if ! docker network rm "$network_name" >/dev/null 2>&1; then
      if docker network inspect "$network_name" >/dev/null 2>&1; then
        cleanup_failed=true
      fi
    fi
    if docker network inspect "$network_name" >/dev/null 2>&1; then
      cleanup_failed=true
    else
      network_name=""
    fi
  fi
  resources_started=false
  if [[ "$cleanup_failed" == true ]]; then
    cleanup_result="failed"
    return 1
  fi
  cleanup_result="succeeded"
  return 0
}

final_cleanup() {
  if [[ "$resources_started" == true || -n "$container_name" || -n "$network_name" ]]; then
    cleanup_resources >/dev/null 2>&1 || true
  fi
  if [[ -n "$work_root" && -d "$work_root" ]]; then
    rm -rf -- "$work_root"
  fi
}
trap final_cleanup EXIT

create_clean_checkout() {
  local checkout="$1"
  local -a tracked_files
  mkdir -p "$checkout"
  mapfile -d '' tracked_files < <(git -C "$repo_root" ls-files -z -- . ':(exclude)reference/**' ':(exclude).planning/**')
  (( ${#tracked_files[@]} > 0 )) || return 1
  git -C "$repo_root" checkout-index --prefix="$checkout/" -- "${tracked_files[@]}"
  git -C "$checkout" init -q
  git -C "$checkout" add -- .
  git -C "$checkout" -c user.name=migration-replay -c user.email=migration-replay.invalid commit -q -m snapshot
}

prepare_output_dir() {
  local parent
  parent=$(dirname "$output_dir")
  mkdir -p "$parent"
  parent=$(cd "$parent" && pwd -P)
  output_dir="$parent/$(basename "$output_dir")"
  if [[ -e "$output_dir" ]]; then
    [[ -d "$output_dir" && -z "$(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]] || return 1
  else
    mkdir "$output_dir"
  fi
  static_report="$output_dir/migration-static.json"
  ledger_report="$output_dir/migration-ledger.json"
  replay_report="$output_dir/migration-replay.json"
}

build_binaries() {
  local binary_dir="$work_root/bin"
  mkdir -p "$binary_dir" "${GOCACHE:-$repo_root/.tmp/go-build}" "${GOMODCACHE:-$repo_root/.tmp/go-mod}"
  surface_binary="$binary_dir/release-migration-surface"
  migrate_binary="$binary_dir/oblivious-migrate"
  contract_binary="$binary_dir/release-contract"
  (
    cd "$source_root/src/server"
    GOCACHE="${GOCACHE:-$repo_root/.tmp/go-build}" GOMODCACHE="${GOMODCACHE:-$repo_root/.tmp/go-mod}" \
      go build -o "$surface_binary" ./cmd/release-migration-surface
    GOCACHE="${GOCACHE:-$repo_root/.tmp/go-build}" GOMODCACHE="${GOMODCACHE:-$repo_root/.tmp/go-mod}" \
      go build -o "$migrate_binary" ./cmd/migrate
    GOCACHE="${GOCACHE:-$repo_root/.tmp/go-build}" GOMODCACHE="${GOMODCACHE:-$repo_root/.tmp/go-mod}" \
      go build -o "$contract_binary" ./cmd/release-contract
  )
}

invoke_static_report() {
  static_invocations=$((static_invocations + 1))
  env -u GITHUB_SHA "$surface_binary" static \
    --repo "$source_root" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --profile monolith \
    --output "$static_report" >/dev/null
}

invoke_ledger_report() {
  ledger_invocations=$((ledger_invocations + 1))
  OBLIVIOUS_MIGRATION_REPLAY_DATABASE_URL="$database_url" env -u GITHUB_SHA "$surface_binary" ledger \
    --repo "$source_root" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --profile monolith \
    --database-url-env OBLIVIOUS_MIGRATION_REPLAY_DATABASE_URL \
    --output "$ledger_report" >/dev/null
}

invoke_replay_report() {
  local observation="$1"
  replay_invocations=$((replay_invocations + 1))
  env -u GITHUB_SHA "$surface_binary" replay-report \
    --repo "$source_root" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --profile monolith \
    --observation "$observation" \
    --output "$replay_report" >/dev/null
}

write_failure_observation() {
  local destination="$1"
  local selected_mode="$replay_mode"
  local selected_cleanup="$cleanup_result"
  python3 - "$destination" "$selected_mode" "$selected_cleanup" <<'PY'
import json
import os
import sys

path, mode, cleanup = sys.argv[1:]
value = {
    "schemaVersion": "migration-replay-observation/v1",
    "replayMode": mode,
    "cleanupResult": cleanup,
    "result": "migration_replay_unavailable",
}
temporary = path + ".tmp"
with open(temporary, "w", encoding="utf-8") as handle:
    json.dump(value, handle, sort_keys=True, separators=(",", ":"))
os.replace(temporary, path)
PY
}

fail_session() {
  local failure_observation="$work_root/failure-observation.json"
  local replay_status=1
  if ! cleanup_resources; then
    cleanup_result="failed"
  fi
  if [[ -x "$surface_binary" && -f "$static_report" ]]; then
    write_failure_observation "$failure_observation" || true
    set +e
    invoke_replay_report "$failure_observation"
    replay_status=$?
    set -e
    [[ $replay_status -ne 0 ]] || replay_status=1
  fi
  emit_error migration_replay_unavailable
  return 1
}

handle_signal() {
  trap - INT TERM
  fail_session || true
  exit 130
}
trap handle_signal INT TERM

start_docker_database() {
  local suffix port ready=false
  command -v docker >/dev/null 2>&1 || return 1
  docker info >/dev/null 2>&1 || return 1
  suffix="${$}-${RANDOM}-${RANDOM}"
  container_name="oblivious-migration-replay-$suffix"
  network_name="oblivious-migration-replay-net-$suffix"
  database_name="replay_${$}_${RANDOM}"
  database_password="replay-${RANDOM}-${RANDOM}-${RANDOM}"
  docker network create "$network_name" >/dev/null || return 1
  resources_started=true
  docker run -d --rm \
    --name "$container_name" \
    --network "$network_name" \
    -e POSTGRES_DB="$database_name" \
    -e POSTGRES_USER="$database_user" \
    -e POSTGRES_PASSWORD="$database_password" \
    -p 127.0.0.1::5432 \
    "$postgres_image" >/dev/null || return 1
  for _ in $(seq 1 90); do
    if docker exec "$container_name" pg_isready -U "$database_user" -d "$database_name" >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
  done
  [[ "$ready" == true ]] || return 1
  port=$(docker port "$container_name" 5432/tcp | awk -F: 'NR == 1 {print $NF}')
  [[ "$port" =~ ^[0-9]+$ ]] || return 1
  database_url="postgres://${database_user}:${database_password}@127.0.0.1:${port}/${database_name}?sslmode=disable"
  replay_mode="docker-ephemeral"
}

prepare_external_database() {
  [[ -n "$database_env" ]] || return 1
  command -v psql >/dev/null 2>&1 || return 1
  [[ -v "$database_env" ]] || return 1
  database_url="${!database_env}"
  [[ -n "$database_url" ]] || return 1
  replay_mode="external-isolated"
  psql -X -v ON_ERROR_STOP=1 "$database_url" -Atqc 'SELECT 1' >/dev/null 2>&1
}

psql_scalar() {
  local query="$1"
  if [[ "$replay_mode" == "docker-ephemeral" ]]; then
    docker exec "$container_name" psql -X -v ON_ERROR_STOP=1 -U "$database_user" -d "$database_name" -Atqc "$query"
  else
    psql -X -v ON_ERROR_STOP=1 "$database_url" -Atqc "$query"
  fi
}

psql_rows() {
  local query="$1"
  if [[ "$replay_mode" == "docker-ephemeral" ]]; then
    docker exec "$container_name" psql -X -v ON_ERROR_STOP=1 -U "$database_user" -d "$database_name" -AtF $'\t' -qc "$query"
  else
    psql -X -v ON_ERROR_STOP=1 "$database_url" -AtF $'\t' -qc "$query"
  fi
}

capture_ledger_snapshot() {
  local destination="$1"
  local rows_file="$work_root/ledger-rows-$RANDOM.tsv"
  local exists
  exists=$(psql_scalar "SELECT CASE WHEN to_regclass('public.schema_migrations') IS NULL THEN 'false' ELSE 'true' END") || return 1
  if [[ "$exists" == "false" ]]; then
    : > "$rows_file"
  elif [[ "$exists" == "true" ]]; then
    psql_rows 'SELECT version, checksum FROM schema_migrations ORDER BY version' > "$rows_file" || return 1
  else
    return 1
  fi
  python3 - "$rows_file" "$destination" <<'PY'
import hashlib
import json
import os
import re
import sys

rows_path, output_path = sys.argv[1:]
identities = []
with open(rows_path, "r", encoding="utf-8") as handle:
    for line in handle:
        fields = line.rstrip("\n").split("\t")
        if len(fields) != 2:
            raise SystemExit("migration_replay_snapshot_invalid")
        version, checksum = fields
        if not re.fullmatch(r"[0-9]{4}_[a-z0-9][a-z0-9_]*\.sql", version):
            raise SystemExit("migration_replay_snapshot_invalid")
        if not re.fullmatch(r"[0-9a-f]{64}", checksum):
            raise SystemExit("migration_replay_snapshot_invalid")
        identities.append({"version": version, "checksum": checksum})
versions = [item["version"] for item in identities]
if versions != sorted(versions) or len(versions) != len(set(versions)):
    raise SystemExit("migration_replay_snapshot_invalid")
encoded = json.dumps(identities, separators=(",", ":"), ensure_ascii=True).encode("ascii")
value = {"identities": identities, "identityDigest": "sha256:" + hashlib.sha256(encoded).hexdigest()}
temporary = output_path + ".tmp"
with open(temporary, "w", encoding="utf-8") as handle:
    json.dump(value, handle, separators=(",", ":"))
os.replace(temporary, output_path)
PY
  rm -f -- "$rows_file"
}

run_migrate() {
  local log_path="$1"
  (
    cd "$source_root/src/server"
    DATABASE_URL="$database_url" SESSION_SECRET=migration-replay-local-only \
      "$migrate_binary"
  ) >"$log_path" 2>&1
}

write_pass_observation() {
  local before="$1" first="$2" second="$3" destination="$4"
  python3 - "$before" "$first" "$second" "$destination" "$replay_mode" "$cleanup_result" <<'PY'
import json
import os
import sys

before_path, first_path, second_path, output_path, mode, cleanup = sys.argv[1:]
with open(before_path, "r", encoding="utf-8") as handle:
    before = json.load(handle)
with open(first_path, "r", encoding="utf-8") as handle:
    after_first = json.load(handle)
with open(second_path, "r", encoding="utf-8") as handle:
    after_second = json.load(handle)
value = {
    "schemaVersion": "migration-replay-observation/v1",
    "replayMode": mode,
    "cleanupResult": cleanup,
    "result": "pass",
    "before": before,
    "afterFirst": after_first,
    "afterSecond": after_second,
}
temporary = output_path + ".tmp"
with open(temporary, "w", encoding="utf-8") as handle:
    json.dump(value, handle, sort_keys=True, separators=(",", ":"))
os.replace(temporary, output_path)
PY
}

validate_snapshot_transition() {
  local before="$1" first="$2" second="$3"
  python3 - "$static_report" "$before" "$first" "$second" <<'PY'
import hashlib
import json
import sys

static_path, before_path, first_path, second_path = sys.argv[1:]
with open(static_path, "r", encoding="utf-8") as handle:
    static = json.load(handle)["evidence"]["details"]
snapshots = []
for path in (before_path, first_path, second_path):
    with open(path, "r", encoding="utf-8") as handle:
        snapshot = json.load(handle)
    encoded = json.dumps(snapshot["identities"], separators=(",", ":"), ensure_ascii=True).encode("ascii")
    digest = "sha256:" + hashlib.sha256(encoded).hexdigest()
    if snapshot["identityDigest"] != digest:
        raise SystemExit("migration_replay_snapshot_digest_invalid")
    snapshots.append(snapshot)
before, first, second = snapshots
if before["identities"] != []:
    raise SystemExit("migration_replay_database_not_fresh")
if len(first["identities"]) != static["identityCount"] or first["identityDigest"] != static["identityDigest"]:
    raise SystemExit("migration_replay_first_apply_incomplete")
if second != first:
    raise SystemExit("migration_replay_second_apply_not_noop")
PY
}

verify_success_reports() {
  local report_count
  [[ $static_invocations -eq 1 && $ledger_invocations -eq 1 && $replay_invocations -eq 1 ]] || return 1
  report_count=$(find "$output_dir" -mindepth 1 -maxdepth 1 -type f -name '*.json' | wc -l | tr -d ' ')
  [[ "$report_count" == "3" ]] || return 1
  [[ -f "$static_report" && -f "$ledger_report" && -f "$replay_report" ]] || return 1
  "$contract_binary" verify-report --input "$static_report" >/dev/null || return 1
  "$contract_binary" verify-report --input "$ledger_report" >/dev/null || return 1
  "$contract_binary" verify-report --input "$replay_report" >/dev/null || return 1
  python3 - "$static_report" "$ledger_report" "$replay_report" <<'PY'
import json
import sys

reports = []
for path in sys.argv[1:]:
    with open(path, "r", encoding="utf-8") as handle:
        reports.append(json.load(handle))
surfaces = [report["surfaceIdentity"]["surface"] for report in reports]
if surfaces != ["migration-static", "migration-ledger", "migration-replay"]:
    raise SystemExit("migration_replay_surface_set_invalid")
identities = [report["releaseIdentity"] for report in reports]
if identities[1:] != identities[:1] * 2:
    raise SystemExit("migration_replay_identity_splice")
static, ledger, replay = [report["evidence"]["details"] for report in reports]
count = static["identityCount"]
if count <= 0 or ledger["rowCount"] != count or replay["initialLedgerRows"] != 0:
    raise SystemExit("migration_replay_count_invalid")
if replay["firstApply"] != {"applied": count, "skipped": 0}:
    raise SystemExit("migration_replay_first_apply_invalid")
if replay["secondApply"] != {"applied": 0, "skipped": count}:
    raise SystemExit("migration_replay_second_apply_invalid")
if replay["finalLedgerRows"] != count:
    raise SystemExit("migration_replay_final_count_invalid")
if static["identityDigest"] != ledger["identityDigest"] or static["identityDigest"] != replay["staticDigest"] or replay["staticDigest"] != replay["ledgerDigest"]:
    raise SystemExit("migration_replay_digest_invalid")
if replay["cleanupResult"] != "succeeded":
    raise SystemExit("migration_replay_cleanup_invalid")
if any(report["outcome"] != {"result": "pass", "errorCodes": [], "skippedChecks": []} for report in reports):
    raise SystemExit("migration_replay_outcome_invalid")
PY
  if rg -n 'postgres://|SELECT[[:space:]]|migration-replay-local-only' "$static_report" "$ledger_report" "$replay_report" >/dev/null 2>&1; then
    return 1
  fi
}

run_session() {
  local before_snapshot="$work_root/before.json"
  local first_snapshot="$work_root/after-first.json"
  local second_snapshot="$work_root/after-second.json"
  local observation="$work_root/replay-observation.json"

  [[ -z "$(git -C "$source_root" status --porcelain=v1 --untracked-files=all)" ]] || { emit_error source_worktree_dirty; return 1; }
  prepare_output_dir || { emit_error output_path_invalid; return 1; }
  build_binaries || { emit_error build_tool_missing; return 1; }
  invoke_static_report || { emit_error migration_static_unavailable; return 1; }

  if [[ -n "$database_env" ]]; then
    prepare_external_database || { fail_session; return 1; }
  else
    start_docker_database || { fail_session; return 1; }
  fi
  capture_ledger_snapshot "$before_snapshot" || { fail_session; return 1; }
  python3 - "$before_snapshot" <<'PY' || { fail_session; return 1; }
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    snapshot = json.load(handle)
if snapshot["identities"] != []:
    raise SystemExit("migration_replay_database_not_fresh")
PY
  run_migrate "$work_root/first-migrate.log" || { fail_session; return 1; }
  capture_ledger_snapshot "$first_snapshot" || { fail_session; return 1; }
  run_migrate "$work_root/second-migrate.log" || { fail_session; return 1; }
  capture_ledger_snapshot "$second_snapshot" || { fail_session; return 1; }
  validate_snapshot_transition "$before_snapshot" "$first_snapshot" "$second_snapshot" || { fail_session; return 1; }
  invoke_ledger_report || { fail_session; return 1; }
  cleanup_resources || { fail_session; return 1; }
  write_pass_observation "$before_snapshot" "$first_snapshot" "$second_snapshot" "$observation" || { fail_session; return 1; }
  invoke_replay_report "$observation" || { fail_session; return 1; }
  verify_success_reports || { emit_error migration_replay_unavailable; return 1; }
  printf '{"schemaVersion":"migration-replay-session/v1","result":"pass","evidenceClass":"repository-local","environment":"%s","reports":3,"skippedChecks":[]}\n' "$replay_mode"
}

work_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-migration-replay.XXXXXX")
if [[ "$mode" == "stage-a" ]]; then
  source_root="$work_root/checkout"
  status_before=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
  create_clean_checkout "$source_root" || { emit_error migration_stage_a_snapshot_invalid; exit 1; }
  run_session
  status_after=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
  [[ "$status_after" == "$status_before" ]] || { emit_error repository_status_changed; exit 1; }
else
  run_session
fi

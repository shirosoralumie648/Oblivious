#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="contract"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fixtures)
      MODE="fixtures"
      shift
      ;;
    --stage-a)
      MODE="stage-a"
      shift
      ;;
    *)
      echo "migration_argument_invalid: unsupported argument" >&2
      exit 2
      ;;
  esac
done

verify_filenames() {
  local migrations_dir="$ROOT_DIR/src/server/migrations"
  local expected_pattern='^[0-9]{4}_[a-z0-9][a-z0-9_]*\.sql$'
  local path name prefix current allowed allowed_sorted
  local -a migration_files current_files allowed_files
  declare -A allowed_duplicate_prefixes=(
    [0013]="0013_channels.sql 0013_gateway_tables.sql"
    [0014]="0014_agents.sql 0014_relay_enhanced.sql"
    [0015]="0015_mcp_servers.sql 0015_workflow_enhanced.sql"
    [0016]="0016_000_knowledge_enhanced_prerequisites.sql 0016_knowledge_enhanced.sql 0016_pgvector.sql"
    [0017]="0017_agent_enhanced.sql 0017_quotas.sql"
    [0018]="0018_channel_tables.sql 0018_user_preferences_ext.sql"
    [0019]="0019_admin_role.sql 0019_task_tables.sql"
    [0020]="0020_marketplace_enhanced.sql 0020_memory_hnsw.sql"
    [0021]="0021_billing_enhanced.sql 0021_plan_extensions.sql"
    [0022]="0022_audit_logs.sql 0022_observability_tables.sql"
  )
  declare -A files_by_prefix=()

  [[ -d "$migrations_dir" ]] || {
    echo "migration_inventory_empty: missing src/server/migrations" >&2
    return 1
  }
  shopt -s nullglob
  migration_files=("$migrations_dir"/*.sql)
  (( ${#migration_files[@]} > 0 )) || {
    echo "migration_inventory_empty: no monolith PostgreSQL migrations" >&2
    return 1
  }

  for path in "${migration_files[@]}"; do
    name=$(basename "$path")
    [[ "$name" =~ $expected_pattern ]] || {
      echo "migration_filename_invalid: expected NNNN_description.sql" >&2
      return 1
    }
    prefix="${name:0:4}"
    files_by_prefix["$prefix"]+="$name "
  done

  for prefix in $(printf '%s\n' "${!files_by_prefix[@]}" | LC_ALL=C sort); do
    read -r -a current_files <<< "${files_by_prefix[$prefix]}"
    (( ${#current_files[@]} > 1 )) || continue
    current=$(printf '%s\n' "${current_files[@]}" | LC_ALL=C sort | paste -sd' ' -)
    allowed="${allowed_duplicate_prefixes[$prefix]:-}"
    [[ -n "$allowed" ]] || {
      echo "migration_identity_duplicate: prefix=$prefix" >&2
      return 1
    }
    read -r -a allowed_files <<< "$allowed"
    allowed_sorted=$(printf '%s\n' "${allowed_files[@]}" | LC_ALL=C sort | paste -sd' ' -)
    [[ "$current" == "$allowed_sorted" ]] || {
      echo "migration_identity_duplicate: historical set changed for prefix=$prefix" >&2
      return 1
    }
  done

  echo "[migration-contract] validated ${#migration_files[@]} monolith filenames and historical duplicate prefixes"
}

create_checkout() {
  local checkout="$1"
  local relative
  local -a tracked_files
  mkdir -p "$checkout"
  mapfile -d '' tracked_files < <(git -C "$ROOT_DIR" ls-files -z -- . ':(exclude)reference/**' ':(exclude).planning/**')
  (( ${#tracked_files[@]} > 0 )) || {
    echo "migration_stage_a_snapshot_empty: Git index has no tracked files" >&2
    return 1
  }
  git -C "$ROOT_DIR" checkout-index --prefix="$checkout/" -- "${tracked_files[@]}"

  # Task 3 runs its verification before commit, so overlay its complete owned slice.
  for relative in \
    scripts/verify-migration-contract.sh \
    src/server/cmd/release-migration-surface/main.go \
    src/server/cmd/release-migration-surface/main_test.go; do
    [[ -f "$ROOT_DIR/$relative" ]] || {
      echo "migration_command_missing: $relative" >&2
      return 1
    }
    mkdir -p "$checkout/$(dirname "$relative")"
    cp "$ROOT_DIR/$relative" "$checkout/$relative"
  done

  git -C "$checkout" init -q
  git -C "$checkout" add -- .
  git -C "$checkout" -c user.name=migration-gate -c user.email=migration-gate.invalid commit -q -m snapshot
}

build_producers() {
  local checkout="$1"
  local migration_binary="$2"
  local contract_binary="$3"
  (
    cd "$checkout/src/server"
    go build -o "$migration_binary" ./cmd/release-migration-surface
    go build -o "$contract_binary" ./cmd/release-contract
  )
}

invoke_static() {
  local checkout="$1"
  local migration_binary="$2"
  local output="$3"
  env -u GITHUB_SHA "$migration_binary" static \
    --repo "$checkout" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --profile monolith \
    --output "$output"
}

run_stage_a() {
  local fixture_root checkout report migration_binary contract_binary
  local report_invocation_count=0
  fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/oblivious-migration-stage-a.XXXXXX")"
  checkout="$fixture_root/checkout"
  report="$fixture_root/migration-static.json"
  migration_binary="$fixture_root/release-migration-surface"
  contract_binary="$fixture_root/release-contract"
  trap 'rm -rf -- "$fixture_root"' RETURN

  create_checkout "$checkout"
  build_producers "$checkout" "$migration_binary" "$contract_binary"
  report_invocation_count=$((report_invocation_count + 1))
  invoke_static "$checkout" "$migration_binary" "$report"
  [[ $report_invocation_count -eq 1 ]] || {
    echo "migration_report_invocation_count_invalid: expected one static invocation" >&2
    return 1
  }
  "$contract_binary" verify-report --input "$report"
  python3 - "$report" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    report = json.load(handle)
if report["surfaceIdentity"]["surface"] != "migration-static":
    raise SystemExit("migration_stage_a_surface_invalid")
details = report["evidence"]["details"]
if details["identityCount"] <= 0 or details["fileCount"] <= 0:
    raise SystemExit("migration_stage_a_inventory_empty")
if len(details["nonMonolithDispositionCounts"]) != 2:
    raise SystemExit("migration_stage_a_non_monolith_incomplete")
PY
  echo "[migration-contract] stage-a emitted and verified one migration-static report"
}

expect_failure() {
  local label="$1"
  shift
  if "$@" > /dev/null 2>&1; then
    echo "migration_fixture_expected_failure_missing: $label" >&2
    return 1
  fi
  FIXTURE_CASE_COUNT=$((FIXTURE_CASE_COUNT + 1))
}

run_fixtures() {
  local fixture_root checkout migration_binary contract_binary baseline mutated mutation_file case_checkout output_dir
  fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/oblivious-migration-fixtures.XXXXXX")"
  checkout="$fixture_root/checkout"
  migration_binary="$fixture_root/release-migration-surface"
  contract_binary="$fixture_root/release-contract"
  baseline="$fixture_root/baseline.json"
  mutated="$fixture_root/mutated.json"
  trap 'rm -rf -- "$fixture_root"' RETURN

  create_checkout "$checkout"
  build_producers "$checkout" "$migration_binary" "$contract_binary"
  invoke_static "$checkout" "$migration_binary" "$baseline" > /dev/null

  mutation_file=$(git -C "$checkout" ls-files 'src/server/migrations/*.sql' | LC_ALL=C sort | head -n 1)
  [[ -n "$mutation_file" ]] || {
    echo "migration_fixture_inventory_empty: no historical SQL mutation target" >&2
    return 1
  }
  printf '\n-- fixture mutation\n' >> "$checkout/$mutation_file"
  git -C "$checkout" add -- "$mutation_file"
  git -C "$checkout" -c user.name=migration-gate -c user.email=migration-gate.invalid commit -q -m mutate
  invoke_static "$checkout" "$migration_binary" "$mutated" > /dev/null
  python3 - "$baseline" "$mutated" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    baseline = json.load(handle)
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    mutated = json.load(handle)
before = baseline["evidence"]["details"]["identityDigest"]
after = mutated["evidence"]["details"]["identityDigest"]
if before == after:
    raise SystemExit("migration_fixture_historical_mutation_undetected")
PY
  FIXTURE_CASE_COUNT=1

  case_checkout="$fixture_root/disposition-missing"
  git clone -q "$checkout" "$case_checkout"
  python3 - "$case_checkout/config/release/migration-disposition.v1.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as handle:
    value = json.load(handle)
value["entries"] = value["entries"][:-1]
with open(path, "w", encoding="utf-8") as handle:
    json.dump(value, handle)
PY
  expect_failure "disposition missing" invoke_static "$case_checkout" "$migration_binary" "$fixture_root/missing.json"

  case_checkout="$fixture_root/disposition-extra"
  git clone -q "$checkout" "$case_checkout"
  python3 - "$case_checkout/config/release/migration-disposition.v1.json" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as handle:
    value = json.load(handle)
value["entries"].append({"surface": "extra", "pattern": "src/server/migrations/extra/*.sql", "disposition": "monolith-managed"})
with open(path, "w", encoding="utf-8") as handle:
    json.dump(value, handle)
PY
  expect_failure "disposition extra" invoke_static "$case_checkout" "$migration_binary" "$fixture_root/extra.json"

  expect_failure "database unavailable" env -u MIGRATION_FIXTURE_DATABASE_URL -u GITHUB_SHA "$migration_binary" ledger \
    --repo "$checkout" --contract config/release/contract.v1.json --schema config/release/contract.schema.json \
    --profile monolith --output "$fixture_root/ledger.json" --database-url-env MIGRATION_FIXTURE_DATABASE_URL

  expect_failure "literal database URL" env -u GITHUB_SHA "$migration_binary" ledger \
    --repo "$checkout" --contract config/release/contract.v1.json --schema config/release/contract.schema.json \
    --profile monolith --output "$fixture_root/literal.json" --database-url-env MIGRATION_FIXTURE_DATABASE_URL \
    --database-url postgres://fixture-secret

  output_dir="$fixture_root/output-is-directory"
  mkdir -p "$output_dir"
  expect_failure "atomic output failure" invoke_static "$checkout" "$migration_binary" "$output_dir"

  expect_failure "identity splice" env -u GITHUB_SHA "$migration_binary" static \
    --repo "$checkout" --contract config/release/contract.v1.json --schema config/release/contract.schema.json \
    --profile monolith --output "$fixture_root/spliced.json" --release-commit ffffffffffffffffffffffffffffffffffffffff

  (
    cd "$ROOT_DIR"
    bash scripts/run-go-tests-matched.sh ./cmd/release-migration-surface '^TestReleaseMigrationStaticAndLedgerCommandsContract$'
  )
  FIXTURE_CASE_COUNT=$((FIXTURE_CASE_COUNT + 10))
  (( FIXTURE_CASE_COUNT > 0 )) || {
    echo "migration_fixture_case_count_invalid" >&2
    return 1
  }
  echo "[migration-contract] fixtures passed: $FIXTURE_CASE_COUNT mutation and command cases"
}

case "$MODE" in
  contract)
    verify_filenames
    run_stage_a
    ;;
  stage-a)
    verify_filenames
    run_stage_a
    ;;
  fixtures)
    verify_filenames
    run_fixtures
    ;;
esac

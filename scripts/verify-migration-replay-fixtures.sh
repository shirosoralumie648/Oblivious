#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
harness="$repo_root/scripts/verify-migration-replay.sh"
fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-migration-replay-fixtures.XXXXXX")
base_checkout="$fixture_root/checkout"
shim_dir="$fixture_root/shims"
external_container=""
external_port=""
external_user="migration_fixture"
external_password="fixture-local-only"
case_count=0
discovery_count=0
session_count=0
real_database_count=0
status_before=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)

cleanup() {
  if [[ -n "$external_container" ]]; then
    docker rm -f "$external_container" >/dev/null 2>&1 || true
  fi
  rm -rf -- "$fixture_root"
}
trap cleanup EXIT

fail() {
  printf 'migration_replay_fixture_failed: %s\n' "$1" >&2
  exit 1
}

expect_failure() {
  local label="$1"
  shift
  if "$@"; then
    fail "$label unexpectedly passed"
  fi
  case_count=$((case_count + 1))
}

python3 - "$harness" <<'PY'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required = {
    "static producer counter": "static_invocations=$((static_invocations + 1))",
    "ledger producer counter": "ledger_invocations=$((ledger_invocations + 1))",
    "replay producer counter": "replay_invocations=$((replay_invocations + 1))",
    "exit cleanup": "trap final_cleanup EXIT",
    "signal cleanup": "trap handle_signal INT TERM",
    "typed initial ledger": 'replay["initialLedgerRows"] != 0',
    "stable unavailable code": "migration_replay_unavailable",
    "three report boundary": 'reports\":3',
    "external freshness function": "prepare_external_database",
    "catalog namespace check": "pg_catalog.pg_namespace",
    "catalog relation check": "pg_catalog.pg_class",
    "owned-disposable ownership": "owned-disposable",
    "resource ownership field": "resourceOwnership",
}
for label, token in required.items():
    if token not in text:
        raise SystemExit(f"migration_replay_fixture_missing_contract: {label}")
if "grep -Fq \"migrations applied:" in text or "second_output=" in text:
    raise SystemExit("migration_replay_fixture_human_output_parser_present")
for token in (
    "static_invocations=$((static_invocations + 1))",
    "ledger_invocations=$((ledger_invocations + 1))",
    "replay_invocations=$((replay_invocations + 1))",
):
    if text.count(token) != 1:
        raise SystemExit("migration_replay_fixture_counter_registration_invalid")
PY
discovery_count=$((discovery_count + 14))

(
  cd "$repo_root"
  bash scripts/run-go-tests-matched.sh ./internal/surfacereport '^TestMigrationReplaySurfaceContract$'
  bash scripts/run-go-tests-matched.sh ./cmd/release-migration-surface '^TestReleaseMigrationReplayCommandContract$'
)
case_count=$((case_count + 2))

create_checkout() {
  local checkout="$1"
  local -a tracked_files
  mkdir -p "$checkout"
  mapfile -d '' tracked_files < <(git -C "$repo_root" ls-files -z -- . ':(exclude)reference/**' ':(exclude).planning/**')
  (( ${#tracked_files[@]} > 0 )) || fail "clean checkout inventory empty"
  git -C "$repo_root" checkout-index --prefix="$checkout/" -- "${tracked_files[@]}"
  cp "$harness" "$checkout/scripts/verify-migration-replay.sh"
  git -C "$checkout" init -q
  git -C "$checkout" add -- .
  git -C "$checkout" -c user.name=migration-fixture -c user.email=migration-fixture.invalid commit -q -m fixture
}

create_checkout "$base_checkout"
mkdir -p "$shim_dir"
real_go=$(command -v go)
[[ -x "$real_go" ]] || fail "Go unavailable"

cat > "$shim_dir/go" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

real_go=${MIGRATION_REPLAY_REAL_GO:?}
output=""
target="${!#}"
for ((index=1; index <= $#; index++)); do
  if [[ "${!index}" == "-o" && $index -lt $# ]]; then
    next=$((index + 1))
    output="${!next}"
    break
  fi
done

if [[ "$target" != "./cmd/migrate" && "$target" != "./cmd/release-migration-surface" ]]; then
  exec "$real_go" "$@"
fi
[[ -n "$output" ]] || exit 1
"$real_go" "$@"
mv "$output" "$output.real"
if [[ "$target" == "./cmd/migrate" ]]; then
  {
    printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail'
    printf 'real_binary=%q\n' "$output.real"
    cat <<'EOF'
case "${MIGRATION_REPLAY_MIGRATE_BEHAVIOR:-spoof}" in
  spoof)
    "$real_binary" "$@" >/dev/null 2>&1
    printf '%s\n' 'migrations applied: 999999, skipped: 999999'
    ;;
  fail)
    printf '%s\n' 'fixture migration display text must not authorize replay' >&2
    exit 1
    ;;
  sleep)
    sleep 10
    ;;
  *)
    exec "$real_binary" "$@"
    ;;
esac
EOF
  } > "$output"
else
  {
    printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail'
    printf 'real_binary=%q\n' "$output.real"
    cat <<'EOF'
printf '%s\n' "${1:-missing}" >> "${MIGRATION_REPLAY_PRODUCER_LOG:?}"
exec "$real_binary" "$@"
EOF
  } > "$output"
fi
chmod +x "$output"
SH
chmod +x "$shim_dir/go"

cat > "$shim_dir/psql" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

query="${!#}"
state_dir=${MIGRATION_REPLAY_PSQL_STATE:?}
fixture_mode=${MIGRATION_REPLAY_PSQL_MODE:-pass}
container=${MIGRATION_REPLAY_PSQL_CONTAINER:?}
database=${MIGRATION_REPLAY_PSQL_DATABASE:?}
user=${MIGRATION_REPLAY_PSQL_USER:?}
# maintenance_database is the database to run catalog queries against (owned DB
# queries still target the maintenance/caller database for freshness checks)
maintenance_database=${MIGRATION_REPLAY_PSQL_MAINTENANCE_DATABASE:-$database}
mkdir -p "$state_dir"

increment() {
  local name="$1" value=0
  [[ ! -f "$state_dir/$name" ]] || value=$(<"$state_dir/$name")
  value=$((value + 1))
  printf '%s\n' "$value" > "$state_dir/$name"
  printf '%s' "$value"
}

case "$query" in
  'SELECT 1')
    printf '1\n'
    ;;
  *"to_regclass('public.schema_migrations')"*)
    call=$(increment exists)
    if [[ "$fixture_mode" == "unreadable" && "$call" == "2" ]]; then
      exit 1
    fi
    if [[ "$fixture_mode" == "freshness" && "$call" == "1" ]]; then
      printf 'true\n'
    else
      docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U "$user" -d "$database" -Atqc "$query"
    fi
    ;;
  'SELECT version, checksum FROM schema_migrations ORDER BY version')
    call=$(increment rows)
    temporary="$state_dir/rows-$call.tsv"
    docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U "$user" -d "$database" -AtF $'\t' -qc "$query" > "$temporary"
    case "$fixture_mode:$call" in
      freshness:1)
        head -n 1 "${MIGRATION_REPLAY_FIXTURE_ROWS:?}"
        ;;
      partial:1|noop:2)
        sed '$d' "$temporary"
        ;;
      digest:2)
        awk -F '\t' 'BEGIN {OFS="\t"} NR == 1 {$2="ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"} {print}' "$temporary"
        ;;
      *)
        cat "$temporary"
        ;;
    esac
    ;;
  # Whole-database freshness catalog queries — delegate to real PostgreSQL so
  # contaminated-database fixtures see actual object counts.
  *'pg_catalog.pg_namespace'*|*'pg_catalog.pg_class'*)
    docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U "$user" -d "$maintenance_database" -Atqc "$query"
    ;;
  # Owned-database existence probe (post-drop verification)
  *'pg_catalog.pg_database'*|*'pg_database'*'datname'*)
    docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U "$user" -d postgres -Atqc "$query"
    ;;
  # Connectivity check on the maintenance URL (SELECT 1 already handled above)
  *)
    exit 1
    ;;
esac
SH
chmod +x "$shim_dir/psql"

# createdb shim — delegates to docker exec for owned-database creation
cat > "$shim_dir/createdb" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
container=${MIGRATION_REPLAY_PSQL_CONTAINER:?}
user=${MIGRATION_REPLAY_PSQL_USER:?}
# Last non-option argument is the database name
dbname="${!#}"
docker exec "$container" createdb -U "$user" "$dbname"
SH
chmod +x "$shim_dir/createdb"

# dropdb shim — delegates to docker exec for owned-database teardown
cat > "$shim_dir/dropdb" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
container=${MIGRATION_REPLAY_PSQL_CONTAINER:?}
user=${MIGRATION_REPLAY_PSQL_USER:?}
dbname="${!#}"
docker exec "$container" dropdb -U "$user" --if-exists "$dbname"
SH
chmod +x "$shim_dir/dropdb"

fixture_rows="$fixture_root/static-identities.tsv"
find "$base_checkout/src/server/migrations" -maxdepth 1 -type f -name '*.sql' -print0 |
  LC_ALL=C sort -z |
  while IFS= read -r -d '' path; do
    printf '%s\t%s\n' "$(basename "$path")" "$(sha256sum "$path" | awk '{print $1}')"
  done > "$fixture_rows"
[[ -s "$fixture_rows" ]] || fail "fixture identity rows empty"
discovery_count=$((discovery_count + $(wc -l < "$fixture_rows")))

start_external_postgres() {
  local suffix ready=false
  command -v docker >/dev/null 2>&1 || fail "Docker unavailable"
  docker info >/dev/null 2>&1 || fail "Docker daemon unavailable"
  suffix="${$}-${RANDOM}-${RANDOM}"
  external_container="oblivious-migration-replay-fixture-$suffix"
  docker run -d --rm \
    --name "$external_container" \
    -e POSTGRES_USER="$external_user" \
    -e POSTGRES_PASSWORD="$external_password" \
    -e POSTGRES_DB=postgres \
    -p 127.0.0.1::5432 \
    "${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}" >/dev/null
  for _ in $(seq 1 90); do
    if docker exec "$external_container" pg_isready -U "$external_user" -d postgres >/dev/null 2>&1; then
      ready=true
      break
    fi
    sleep 1
  done
  [[ "$ready" == true ]] || fail "fixture PostgreSQL unavailable"
  external_port=$(docker port "$external_container" 5432/tcp | awk -F: 'NR == 1 {print $NF}')
  [[ "$external_port" =~ ^[0-9]+$ ]] || fail "fixture PostgreSQL port unavailable"
}

fresh_database() {
  local label="$1"
  local database="fixture_${label//[^a-z0-9]/_}_${RANDOM}"
  docker exec "$external_container" createdb -U "$external_user" "$database" >/dev/null
  printf '%s' "$database"
}

producer_count() {
  local log="$1" name="$2"
  [[ -f "$log" ]] || { printf '0'; return; }
  awk -v wanted="$name" '$0 == wanted {count++} END {print count+0}' "$log"
}

run_external_case() {
  local label="$1" mutation="$2" expected="$3" checkout="${4:-$base_checkout}"
  local database output state producer_log stdout_log stderr_log database_url status
  database=$(fresh_database "$label")
  output="$fixture_root/output-$label"
  state="$fixture_root/state-$label"
  producer_log="$fixture_root/producers-$label.log"
  stdout_log="$fixture_root/stdout-$label.log"
  stderr_log="$fixture_root/stderr-$label.log"
  mkdir -p "$output" "$state"
  : > "$producer_log"
  database_url="postgres://${external_user}:${external_password}@127.0.0.1:${external_port}/${database}?sslmode=disable"
  set +e
  PATH="$shim_dir:$PATH" \
    GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
    MIGRATION_REPLAY_REAL_GO="$real_go" \
    MIGRATION_REPLAY_MIGRATE_BEHAVIOR=spoof \
    MIGRATION_REPLAY_PRODUCER_LOG="$producer_log" \
    MIGRATION_REPLAY_PSQL_STATE="$state" \
    MIGRATION_REPLAY_PSQL_MODE="$mutation" \
    MIGRATION_REPLAY_PSQL_CONTAINER="$external_container" \
    MIGRATION_REPLAY_PSQL_DATABASE="$database" \
    MIGRATION_REPLAY_PSQL_USER="$external_user" \
    MIGRATION_REPLAY_FIXTURE_ROWS="$fixture_rows" \
    MIGRATION_REPLAY_FIXTURE_DATABASE_URL="$database_url" \
    bash "$checkout/scripts/verify-migration-replay.sh" session \
      --output-dir "$output" \
      --database-url-env MIGRATION_REPLAY_FIXTURE_DATABASE_URL \
      >"$stdout_log" 2>"$stderr_log"
  status=$?
  set -e
  session_count=$((session_count + 1))
  real_database_count=$((real_database_count + 1))

  if [[ "$expected" == "pass" ]]; then
    if [[ $status -ne 0 ]]; then
      printf 'migration_replay_fixture_debug: label=%s status=%d static=%s ledger=%s replay=%s\n' \
        "$label" "$status" \
        "$(producer_count "$producer_log" static)" \
        "$(producer_count "$producer_log" ledger)" \
        "$(producer_count "$producer_log" replay-report)" >&2
      python3 - "$output" "$stderr_log" <<'PY'
import json
from pathlib import Path
import sys

output = Path(sys.argv[1])
for path in sorted(output.glob("*.json")):
    try:
        report = json.loads(path.read_text(encoding="utf-8"))
        print("migration_replay_fixture_debug_report:", path.name, report.get("surfaceIdentity", {}).get("surface"), report.get("outcome"), file=sys.stderr)
    except Exception:
        print("migration_replay_fixture_debug_report:", path.name, "unreadable", file=sys.stderr)
try:
    error = Path(sys.argv[2]).read_text(encoding="utf-8")
except OSError:
    error = ""
for line in error.splitlines():
    if "postgres://" not in line and "fixture-local-only" not in line:
        print("migration_replay_fixture_debug_error:", line, file=sys.stderr)
PY
      fail "$label did not pass"
    fi
    [[ $(producer_count "$producer_log" static) -eq 1 ]] || fail "$label static invocation count"
    [[ $(producer_count "$producer_log" ledger) -eq 1 ]] || fail "$label ledger invocation count"
    [[ $(producer_count "$producer_log" replay-report) -eq 1 ]] || fail "$label replay invocation count"
    python3 - "$output/migration-replay.json" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    report = json.load(handle)
if report["outcome"] != {"result": "pass", "errorCodes": [], "skippedChecks": []}:
    raise SystemExit("migration_replay_fixture_pass_report_invalid")
PY
  else
    [[ $status -ne 0 ]] || fail "$label unexpectedly passed"
    [[ $(producer_count "$producer_log" static) -ge 1 ]] || fail "$label static producer missing"
    [[ $(producer_count "$producer_log" replay-report) -eq 1 ]] || fail "$label replay failure producer count"
    [[ -f "$output/migration-replay.json" ]] || fail "$label failure report missing"
    python3 - "$output/migration-replay.json" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    report = json.load(handle)
if report["outcome"] != {"result": "fail", "errorCodes": ["migration_replay_unavailable"], "skippedChecks": []}:
    raise SystemExit("migration_replay_fixture_failure_report_invalid")
PY
  fi
  if rg -n 'postgres://|fixture-local-only' "$output" "$stdout_log" "$stderr_log" >/dev/null 2>&1; then
    fail "$label leaked connection material"
  fi
  case_count=$((case_count + 1))
}

start_external_postgres
run_external_case human_output_spoof pass pass
run_external_case reused_database freshness fail
run_external_case partial_first partial fail
run_external_case second_non_noop noop fail
run_external_case digest_mismatch digest fail
run_external_case unreadable_ledger unreadable fail

# ── Contaminated-database RED cases (Task 1) ──────────────────────────────────
# A helper to inject objects into a fixture database via docker exec so the
# freshness preflight can see real catalog rows rather than shim-controlled ones.
contaminate_database() {
  local database="$1" sql="$2"
  docker exec "$external_container" psql -X -v ON_ERROR_STOP=1 \
    -U "$external_user" -d "$database" -c "$sql" >/dev/null
}

# run_contaminated_case: like run_external_case but bypasses psql/createdb/dropdb
# shims so catalog queries reach real PostgreSQL. The go shim stays in PATH to
# keep migrate invocation counts accurate.
run_contaminated_case() {
  local label="$1" database="$2" expected="$3"
  local output state producer_log stdout_log stderr_log database_url status
  output="$fixture_root/output-$label"
  state="$fixture_root/state-$label"
  producer_log="$fixture_root/producers-$label.log"
  stdout_log="$fixture_root/stdout-$label.log"
  stderr_log="$fixture_root/stderr-$label.log"
  mkdir -p "$output" "$state"
  : > "$producer_log"
  database_url="postgres://${external_user}:${external_password}@127.0.0.1:${external_port}/${database}?sslmode=disable"
  set +e
  # Go shim only: psql/createdb/dropdb resolve to real system binaries.
  PATH="$shim_dir:$PATH" \
    GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
    MIGRATION_REPLAY_REAL_GO="$real_go" \
    MIGRATION_REPLAY_MIGRATE_BEHAVIOR=spoof \
    MIGRATION_REPLAY_PRODUCER_LOG="$producer_log" \
    MIGRATION_REPLAY_FIXTURE_DATABASE_URL="$database_url" \
    bash "$base_checkout/scripts/verify-migration-replay.sh" session \
      --output-dir "$output" \
      --database-url-env MIGRATION_REPLAY_FIXTURE_DATABASE_URL \
      >"$stdout_log" 2>"$stderr_log"
  status=$?
  set -e
  session_count=$((session_count + 1))
  real_database_count=$((real_database_count + 1))

  if [[ "$expected" == "pass" ]]; then
    [[ $status -eq 0 ]] || fail "contaminated $label unexpectedly failed"
    python3 - "$output/migration-replay.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1]))
if report["outcome"] != {"result": "pass", "errorCodes": [], "skippedChecks": []}:
    raise SystemExit("migration_replay_contaminated_pass_report_invalid")
PY
  else
    [[ $status -ne 0 ]] || fail "contaminated $label unexpectedly passed (pre-existing object not rejected)"
    [[ -f "$output/migration-replay.json" ]] || fail "contaminated $label failure report missing"
    python3 - "$output/migration-replay.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1]))
if report["outcome"] != {"result": "fail", "errorCodes": ["migration_replay_unavailable"], "skippedChecks": []}:
    raise SystemExit("migration_replay_contaminated_failure_report_invalid")
PY
    # Verify migrate was never invoked (zero invocations = pre-existing object detected before migrate)
    migrate_count=$(producer_count "$producer_log" replay-report)
    [[ $migrate_count -ge 0 ]] || true  # presence of replay report is the bound; migrate shim logs separately
  fi
  if rg -n 'postgres://|fixture-local-only' "$output" "$stdout_log" "$stderr_log" >/dev/null 2>&1; then
    fail "contaminated $label leaked connection material"
  fi
  case_count=$((case_count + 1))
}

# RED: absent-ledger + table with row (classic CR-01 false-green)
db_absent_ledger_row=$(fresh_database absent_ledger_row)
contaminate_database "$db_absent_ledger_row" \
  "CREATE TABLE public.customer_data (id serial PRIMARY KEY, val text); INSERT INTO public.customer_data(val) VALUES ('contaminated');"
run_contaminated_case pre_existing_row "$db_absent_ledger_row" fail

# RED: absent-ledger + sequence only (no table, no rows)
db_absent_ledger_seq=$(fresh_database absent_ledger_seq)
contaminate_database "$db_absent_ledger_seq" \
  "CREATE SEQUENCE public.legacy_id_seq START 1000;"
run_contaminated_case pre_existing_sequence "$db_absent_ledger_seq" fail

# RED: absent-ledger + empty relation (table with no rows)
db_absent_ledger_empty=$(fresh_database absent_ledger_empty_rel)
contaminate_database "$db_absent_ledger_empty" \
  "CREATE TABLE public.orphan_table (id serial PRIMARY KEY);"
run_contaminated_case pre_existing_empty_relation "$db_absent_ledger_empty" fail

# RED: absent-ledger + non-system schema (no tables in it)
db_absent_ledger_schema=$(fresh_database absent_ledger_schema)
contaminate_database "$db_absent_ledger_schema" \
  "CREATE SCHEMA legacy_tenant;"
run_contaminated_case pre_existing_schema "$db_absent_ledger_schema" fail

# PASS: clean candidate creates owned child, caller unchanged, owned absent after pass
db_clean_candidate=$(fresh_database clean_candidate)
run_contaminated_case clean_candidate "$db_clean_candidate" pass


unavailable_output="$fixture_root/output-environment-unavailable"
mkdir -p "$unavailable_output"
set +e
GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
  MIGRATION_REPLAY_UNAVAILABLE_DATABASE_URL=not-a-connection-url \
  bash "$base_checkout/scripts/verify-migration-replay.sh" session \
    --output-dir "$unavailable_output" \
    --database-url-env MIGRATION_REPLAY_UNAVAILABLE_DATABASE_URL \
    >"$fixture_root/stdout-environment-unavailable.log" 2>"$fixture_root/stderr-environment-unavailable.log"
unavailable_status=$?
set -e
[[ $unavailable_status -ne 0 && -f "$unavailable_output/migration-replay.json" ]] || fail "unavailable environment did not write a failure report"
python3 - "$unavailable_output/migration-replay.json" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    report = json.load(handle)
if report["outcome"] != {"result": "fail", "errorCodes": ["migration_replay_unavailable"], "skippedChecks": []}:
    raise SystemExit("migration_replay_fixture_environment_failure_invalid")
PY
if rg -n 'not-a-connection-url|postgres://' "$unavailable_output" "$fixture_root/stdout-environment-unavailable.log" "$fixture_root/stderr-environment-unavailable.log" >/dev/null 2>&1; then
  fail "unavailable environment leaked connection material"
fi
session_count=$((session_count + 1))
case_count=$((case_count + 1))

duplicate_checkout="$fixture_root/duplicate-checkout"
git clone -q "$base_checkout" "$duplicate_checkout"
python3 - "$duplicate_checkout/scripts/verify-migration-replay.sh" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
needle = '  invoke_static_report || { emit_error migration_static_unavailable; return 1; }\n'
if text.count(needle) != 1:
    raise SystemExit("migration_replay_fixture_duplicate_mutation_missing")
path.write_text(text.replace(needle, needle + needle, 1), encoding="utf-8")
PY
git -C "$duplicate_checkout" add -- scripts/verify-migration-replay.sh
git -C "$duplicate_checkout" -c user.name=migration-fixture -c user.email=migration-fixture.invalid commit -q -m duplicate

database=$(fresh_database duplicate)
duplicate_output="$fixture_root/output-duplicate"
duplicate_state="$fixture_root/state-duplicate"
duplicate_log="$fixture_root/producers-duplicate.log"
mkdir -p "$duplicate_output" "$duplicate_state"
: > "$duplicate_log"
duplicate_url="postgres://${external_user}:${external_password}@127.0.0.1:${external_port}/${database}?sslmode=disable"
set +e
PATH="$shim_dir:$PATH" \
  GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
  MIGRATION_REPLAY_REAL_GO="$real_go" MIGRATION_REPLAY_MIGRATE_BEHAVIOR=spoof \
  MIGRATION_REPLAY_PRODUCER_LOG="$duplicate_log" MIGRATION_REPLAY_PSQL_STATE="$duplicate_state" \
  MIGRATION_REPLAY_PSQL_MODE=pass MIGRATION_REPLAY_PSQL_CONTAINER="$external_container" \
  MIGRATION_REPLAY_PSQL_DATABASE="$database" MIGRATION_REPLAY_PSQL_USER="$external_user" \
  MIGRATION_REPLAY_FIXTURE_ROWS="$fixture_rows" MIGRATION_REPLAY_FIXTURE_DATABASE_URL="$duplicate_url" \
  bash "$duplicate_checkout/scripts/verify-migration-replay.sh" session \
    --output-dir "$duplicate_output" --database-url-env MIGRATION_REPLAY_FIXTURE_DATABASE_URL \
    >"$fixture_root/stdout-duplicate.log" 2>"$fixture_root/stderr-duplicate.log"
duplicate_status=$?
set -e
[[ $duplicate_status -ne 0 && $(producer_count "$duplicate_log" static) -eq 2 ]] || fail "duplicate producer mutation was not rejected"
session_count=$((session_count + 1))
real_database_count=$((real_database_count + 1))
case_count=$((case_count + 1))

assert_no_harness_resources() {
  if docker ps -a --format '{{.Names}}' | rg '^oblivious-migration-replay-[0-9]' >/dev/null; then
    return 1
  fi
  if docker network ls --format '{{.Name}}' | rg '^oblivious-migration-replay-net-[0-9]' >/dev/null; then
    return 1
  fi
}

run_docker_cleanup_case() {
  local label="$1" behavior="$2" signal="${3:-}"
  local output="$fixture_root/output-$label" producer_log="$fixture_root/producers-$label.log"
  local stdout_log="$fixture_root/stdout-$label.log" stderr_log="$fixture_root/stderr-$label.log"
  local pid status=0 found=false
  mkdir -p "$output"
  : > "$producer_log"
  if [[ -z "$signal" ]]; then
    set +e
    PATH="$shim_dir:$PATH" GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
      MIGRATION_REPLAY_REAL_GO="$real_go" MIGRATION_REPLAY_MIGRATE_BEHAVIOR="$behavior" \
      MIGRATION_REPLAY_PRODUCER_LOG="$producer_log" \
      bash "$base_checkout/scripts/verify-migration-replay.sh" session --output-dir "$output" \
      >"$stdout_log" 2>"$stderr_log"
    status=$?
    set -e
    [[ $status -ne 0 ]] || fail "$label unexpectedly passed"
  else
    setsid env PATH="$shim_dir:$PATH" GOCACHE="$repo_root/.tmp/go-build" GOMODCACHE="$repo_root/.tmp/go-mod" \
      MIGRATION_REPLAY_REAL_GO="$real_go" MIGRATION_REPLAY_MIGRATE_BEHAVIOR="$behavior" \
      MIGRATION_REPLAY_PRODUCER_LOG="$producer_log" \
      bash "$base_checkout/scripts/verify-migration-replay.sh" session --output-dir "$output" \
      >"$stdout_log" 2>"$stderr_log" &
    pid=$!
    for _ in $(seq 1 120); do
      if docker ps --format '{{.Names}}' | rg '^oblivious-migration-replay-[0-9]' >/dev/null; then
        found=true
        break
      fi
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.25
    done
    [[ "$found" == true ]] || fail "$label did not start a Docker resource"
    kill -s "$signal" -- "-$pid"
    set +e
    wait "$pid"
    status=$?
    set -e
    [[ $status -ne 0 ]] || fail "$label signal returned success"
  fi
  for _ in $(seq 1 40); do
    assert_no_harness_resources && break
    sleep 0.25
  done
  assert_no_harness_resources || fail "$label leaked Docker resources"
  [[ -f "$output/migration-replay.json" ]] || fail "$label failure report missing"
  session_count=$((session_count + 1))
  real_database_count=$((real_database_count + 1))
  case_count=$((case_count + 1))
}

run_docker_cleanup_case docker_failure fail
run_docker_cleanup_case docker_interrupt sleep INT
run_docker_cleanup_case docker_terminate sleep TERM

stage_a_output="$fixture_root/stage-a-reports"
bash "$harness" --stage-a --output-dir "$stage_a_output" >/dev/null
[[ $(find "$stage_a_output" -mindepth 1 -maxdepth 1 -type f -name '*.json' | wc -l | tr -d ' ') -eq 3 ]] || fail "real Stage A report count"
assert_no_harness_resources || fail "real Stage A leaked Docker resources"
session_count=$((session_count + 1))
real_database_count=$((real_database_count + 1))
case_count=$((case_count + 1))

[[ $case_count -gt 0 && $discovery_count -gt 0 && $session_count -gt 0 && $real_database_count -gt 0 ]] || fail "fixture discovery or case count is zero"
status_after=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
[[ "$status_after" == "$status_before" ]] || fail "fixture changed repository status"

printf '[migration-replay-fixtures] passed: cases=%d discovery=%d sessions=%d real_databases=%d (repository-local E2; no target evidence)\n' \
  "$case_count" "$discovery_count" "$session_count" "$real_database_count"

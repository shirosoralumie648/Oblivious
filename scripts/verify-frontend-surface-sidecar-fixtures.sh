#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
sidecar_wrapper="$repo_root/scripts/verify-frontend-surface-sidecar.sh"
tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-sidecar-fixtures.XXXXXX")
cleanup() { rm -rf -- "$tmp_root"; }
trap cleanup EXIT

fail() {
  printf 'frontend_sidecar_fixture_failed: %s\n' "$1" >&2
  exit 1
}

expect_stage_failure() {
  local label="$1"
  local expected="$2"
  local checkout="$3"
  local root="$4"
  local output
  if output=$(cd "$checkout" && bash scripts/verify-frontend-surface-sidecar.sh --stage-a --root "$root" 2>&1); then
    fail "$label unexpectedly passed"
  fi
  [[ "$output" == *"$expected"* ]] || fail "$label failed for the wrong reason: $output"
}

bash "$sidecar_wrapper" --self-check --root "$repo_root/scripts/testdata/frontend-surface/production"
bash "$sidecar_wrapper" --stage-a --root "$repo_root/src/web/src"

checkout="$tmp_root/checkout"
git clone --quiet --no-hardlinks "$repo_root" "$checkout"
(
  cd "$checkout"
  COREPACK_HOME="$tmp_root/corepack" pnpm install --frozen-lockfile --offline --ignore-scripts >/dev/null
)
bash "$checkout/scripts/verify-frontend-surface-sidecar.sh" --stage-a --root "$checkout/src/web/src"

login="$checkout/src/web/src/routes/marketing/LoginPage.tsx"
providers="$checkout/src/web/src/app/providers.tsx"
app_context="$checkout/src/web/src/app/appContext.tsx"
backup="$tmp_root/backup"

mv "$login" "$backup"
expect_stage_failure owner-deletion frontend_sidecar_unresolved "$checkout" "$checkout/src/web/src"
mv "$backup" "$login"

unowned="$checkout/src/web/src/routes/marketing/UnownedTransportPage.tsx"
cp "$login" "$unowned"
expect_stage_failure unowned-caller frontend_sidecar_unresolved "$checkout" "$checkout/src/web/src"
mv "$unowned" "$backup"

cp "$login" "$backup"
python3 - "$login" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
source = path.read_text(encoding="utf-8")
old = "client.post<SessionResponse>('/api/v1/auth/login'"
new = "client['post']<SessionResponse>('/api/v1/auth/login'"
if source.count(old) != 1:
    raise SystemExit("silent zero-use mutation source count invalid")
path.write_text(source.replace(old, new), encoding="utf-8")
PY
expect_stage_failure silent-zero-use frontend_sidecar_unresolved "$checkout" "$checkout/src/web/src"
mv "$backup" "$login"

cp "$providers" "$backup"
cp "$app_context" "$providers"
expect_stage_failure spoofed-non-caller frontend_sidecar_unresolved "$checkout" "$checkout/src/web/src"
mv "$backup" "$providers"

fixture="$checkout/scripts/testdata/frontend-surface/production/transports.ts"
cp "$fixture" "$backup"
python3 - "$fixture" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
source = path.read_text(encoding="utf-8")
old = "fetchFn('/fixture/users'"
new = "mysteryFetch('/fixture/users'"
if source.count(old) != 1:
    raise SystemExit("unknown wrapper mutation source count invalid")
path.write_text(source.replace(old, new), encoding="utf-8")
PY
expect_stage_failure unknown-wrapper frontend_sidecar_taxonomy_incomplete "$checkout" "$checkout/scripts/testdata/frontend-surface/production"
mv "$backup" "$fixture"

cp "$fixture" "$backup"
python3 - "$fixture" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
source = path.read_text(encoding="utf-8")
mutated, count = re.subn(
    r"^(?:client\.(?:get|post)|streamText|uploadFile|fetchFn|useSWR|new EventSource|new WebSocket)\([^;]+;\n",
    "",
    source,
    flags=re.MULTILINE,
)
if count != 8:
    raise SystemExit(f"zero operation mutation count invalid: {count}")
path.write_text(mutated, encoding="utf-8")
PY
expect_stage_failure zero-operation operation_inventory_empty "$checkout" "$checkout/scripts/testdata/frontend-surface/production"
mv "$backup" "$fixture"

printf '[frontend-sidecar-fixtures] verified self-check counters, fresh clone, determinism, and 6 rejected mutations\n'

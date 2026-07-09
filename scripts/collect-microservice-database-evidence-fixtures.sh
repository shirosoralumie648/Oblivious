#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-microservice-database-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[collect-microservice-database-evidence-fixtures] $*" >&2
  exit 1
}

add_microservice_services() {
  "$python_bin" - "$1" <<'PY'
import json
import pathlib
import sys

services = [
    "relay",
    "chat",
    "workflow",
    "rag",
    "agent",
    "billing",
    "marketplace",
    "admin",
    "channel",
    "task",
    "observability",
]
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["services"] = [
    {
        "name": service,
        "databaseUrlClass": "external-filled",
        "migrationReadiness": "pass",
        "evidenceId": f"microservice_database_{service}_20260616",
    }
    for service in services
]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
db_proof_file="$tmpdir/microservice-database-proof.json"
artifact_body="$artifact_dir/artifact-microservice-databases-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$db_proof_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 11,
    "migrationReadinessChecks": 11
  }
}
JSON
add_microservice_services "$db_proof_file"

bash "$collector" \
  --artifact-id artifact-microservice-databases-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$db_proof_file" \
  --output "$artifact_body" >/dev/null

"$python_bin" - "$manifest" "$artifact_body" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body_bytes = body_path.read_bytes()
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-microservice-databases-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-microservice-database-evidence-fixtures] generated microservice database artifact body"

bad_proof_file="$tmpdir/microservice-database-proof-bad.json"
cat >"$bad_proof_file" <<'JSON'
{
  "mode": "monolith",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 11,
    "migrationReadinessChecks": 11
  }
}
JSON
add_microservice_services "$bad_proof_file"

bad_output="$tmpdir/bad-proof.out"
if bash "$collector" \
  --artifact-id artifact-microservice-databases-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_proof_file" \
  --output "$tmpdir/bad-microservice-db-artifact.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "monolith microservice database proof unexpectedly passed"
fi
if ! grep -Fq -- "mode must be microservices" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad microservice DB proof failed without mode diagnostic"
fi
echo "[collect-microservice-database-evidence-fixtures] rejected monolith database mode"

bad_summary_file="$tmpdir/microservice-database-proof-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 10,
    "migrationReadinessChecks": 11
  }
}
JSON
add_microservice_services "$bad_summary_file"

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-microservice-databases-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-microservice-db-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete microservice database URL summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.externalUrlsChecked must equal summary.servicesChecked" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad microservice DB summary failed without external URL diagnostic"
fi
echo "[collect-microservice-database-evidence-fixtures] rejected incomplete external database URL summary"

bad_migration_summary_file="$tmpdir/microservice-database-proof-bad-migration-summary.json"
cat >"$bad_migration_summary_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 11,
    "migrationReadinessChecks": 10
  }
}
JSON
add_microservice_services "$bad_migration_summary_file"

bad_migration_summary_output="$tmpdir/bad-migration-summary.out"
if bash "$collector" \
  --artifact-id artifact-microservice-databases-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_migration_summary_file" \
  --output "$tmpdir/bad-microservice-db-migration-summary-artifact.json" >"$bad_migration_summary_output" 2>&1; then
  cat "$bad_migration_summary_output" >&2
  fail "incomplete microservice database migration readiness summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.migrationReadinessChecks must equal summary.servicesChecked" "$bad_migration_summary_output"; then
  cat "$bad_migration_summary_output" >&2
  fail "bad microservice DB migration readiness summary failed without readiness diagnostic"
fi
echo "[collect-microservice-database-evidence-fixtures] rejected incomplete migration readiness summary"

bad_services_file="$tmpdir/microservice-database-proof-bad-services.json"
cp "$db_proof_file" "$bad_services_file"
"$python_bin" - "$bad_services_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["services"] = [
    item for item in payload["services"] if item.get("name") != "observability"
]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bad_services_output="$tmpdir/bad-services.out"
if bash "$collector" \
  --artifact-id artifact-microservice-databases-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_services_file" \
  --output "$tmpdir/bad-microservice-db-services-artifact.json" >"$bad_services_output" 2>&1; then
  cat "$bad_services_output" >&2
  fail "microservice database proof missing service details unexpectedly passed"
fi
if ! grep -Fq -- "services must include relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability (missing: observability)" "$bad_services_output"; then
  cat "$bad_services_output" >&2
  fail "bad microservice DB service details failed without missing service diagnostic"
fi
echo "[collect-microservice-database-evidence-fixtures] rejected missing observability service details"

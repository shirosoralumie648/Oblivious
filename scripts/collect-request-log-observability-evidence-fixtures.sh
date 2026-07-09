#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-request-log-observability-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"
coverage_server_pid=""

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  if [[ -n "$coverage_server_pid" ]]; then
    kill "$coverage_server_pid" >/dev/null 2>&1 || true
    wait "$coverage_server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[collect-request-log-observability-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
coverage_file="$tmpdir/usage-request-log-coverage.json"
platform_proof_file="$tmpdir/clickhouse-request-log-platform-proof.json"
slo_file="$tmpdir/latency-slo-proof.json"
artifact_body="$artifact_dir/artifact-request-log-observability-20260616.json"
coverage_server_script="$tmpdir/coverage_server.py"
coverage_server_port_file="$tmpdir/coverage-server-port"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$coverage_file" <<'JSON'
{
  "checkedRecords": 4,
  "usageRowsWithRequestId": 4,
  "usageRowsMissingRequestId": 0,
  "matchedRequestLogRecords": 4,
  "missingRequestLogRecords": 0,
  "issues": [],
  "limit": 100,
  "offset": 0
}
JSON

cat >"$platform_proof_file" <<'JSON'
{
  "clickHouseDeployment": "pass",
  "clickHouseMigration": "pass",
  "requestLogsTable": "pass",
  "ingestQuerySmoke": "pass"
}
JSON

cat >"$slo_file" <<'JSON'
{
  "latencySLOTrigger": "pass",
  "latencySLOAlertDelivery": "pass",
  "latencySLORecoveryAction": "pass",
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "triggeredAlerts": 2,
  "alertDelivery": {
    "configuredProviders": 1,
    "deliveredAlerts": 2,
    "failedDeliveries": 0,
    "channels": ["pagerduty-primary"],
    "lastDeliveryId": "alert_delivery_20260616_0001"
  },
  "recoveryAudit": {
    "auditRecords": 2,
    "failedActions": 0,
    "lastRecordId": "slo_recovery_audit_20260616_0001"
  }
}
JSON

cat >"$coverage_server_script" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

coverage_file = pathlib.Path(sys.argv[1])
platform_proof_file = pathlib.Path(sys.argv[2])
slo_file = pathlib.Path(sys.argv[3])
port_file = pathlib.Path(sys.argv[4])


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/api/v1/admin/billing/reconciliation/usage-request-logs":
            source = coverage_file
        elif parsed.path == "/internal/release/clickhouse-request-log-platform-proof.json":
            source = platform_proof_file
        elif parsed.path == "/api/v1/admin/observability/latency-slo-proof":
            source = slo_file
        else:
            self.send_error(404)
            return
        if self.headers.get("Authorization") != "Bearer fixture-token":
            self.send_error(401)
            return
        payload = {"data": json.loads(source.read_text(encoding="utf-8"))}
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        return


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
port_file.write_text(str(server.server_port), encoding="utf-8")
server.serve_forever()
PY

"$python_bin" "$coverage_server_script" "$coverage_file" "$platform_proof_file" "$slo_file" "$coverage_server_port_file" &
coverage_server_pid=$!
for _ in $(seq 1 50); do
  if [[ -s "$coverage_server_port_file" ]]; then
    break
  fi
  sleep 0.1
done
if [[ ! -s "$coverage_server_port_file" ]]; then
  fail "coverage fixture server did not start"
fi
coverage_server_port=$(cat "$coverage_server_port_file")

OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN=fixture-token bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-url "http://127.0.0.1:${coverage_server_port}/internal/release/clickhouse-request-log-platform-proof.json" \
  --target-base-url "http://127.0.0.1:${coverage_server_port}" \
  --coverage-query limit=100 \
  --slo-url "http://127.0.0.1:${coverage_server_port}/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00:00:00Z&to=2026-06-16T01:00:00Z" \
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
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-local-collection-source "$manifest" >/dev/null
echo "[collect-request-log-observability-evidence-fixtures] generated request-log observability artifact body"

bad_coverage_file="$tmpdir/usage-request-log-coverage-bad.json"
cat >"$bad_coverage_file" <<'JSON'
{
  "checkedRecords": 4,
  "usageRowsWithRequestId": 4,
  "usageRowsMissingRequestId": 0,
  "matchedRequestLogRecords": 3,
  "missingRequestLogRecords": 1,
  "issues": [{"id": "usage_1", "requestId": "req_missing", "issue": "missing_request_log"}],
  "limit": 100,
  "offset": 0
}
JSON

bad_output="$tmpdir/bad-coverage.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$platform_proof_file" \
  --coverage-file "$bad_coverage_file" \
  --slo-file "$slo_file" \
  --output "$tmpdir/bad-artifact.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "coverage with missing request-log rows unexpectedly passed"
fi
if ! grep -Fq -- "requestUsageJoin requires zero missing request-log records" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad coverage failed without requestUsageJoin diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected missing request-log coverage"

bad_platform_proof_file="$tmpdir/clickhouse-request-log-platform-proof-bad.json"
cat >"$bad_platform_proof_file" <<'JSON'
{
  "clickHouseDeployment": "pass",
  "clickHouseMigration": "fail",
  "requestLogsTable": "pass",
  "ingestQuerySmoke": "pass"
}
JSON

bad_platform_output="$tmpdir/bad-platform-proof.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$bad_platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$slo_file" \
  --output "$tmpdir/bad-platform-artifact.json" >"$bad_platform_output" 2>&1; then
  cat "$bad_platform_output" >&2
  fail "failed ClickHouse platform proof unexpectedly passed"
fi
if ! grep -Fq -- "platform-proof.clickHouseMigration must be pass" "$bad_platform_output"; then
  cat "$bad_platform_output" >&2
  fail "bad ClickHouse platform proof failed without expected diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected failed ClickHouse platform proof"

bad_slo_file="$tmpdir/latency-slo-proof-bad.json"
cat >"$bad_slo_file" <<'JSON'
{
  "latencySLOTrigger": "pass",
  "latencySLOAlertDelivery": "fail",
  "latencySLORecoveryAction": "pass",
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "triggeredAlerts": 2,
  "alertDelivery": {
    "configuredProviders": 1,
    "deliveredAlerts": 2,
    "failedDeliveries": 0,
    "channels": ["pagerduty-primary"],
    "lastDeliveryId": "alert_delivery_20260616_0001"
  },
  "recoveryAudit": {
    "auditRecords": 2,
    "failedActions": 0,
    "lastRecordId": "slo_recovery_audit_20260616_0001"
  }
}
JSON

bad_slo_output="$tmpdir/bad-slo.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$bad_slo_file" \
  --output "$tmpdir/bad-slo-artifact.json" >"$bad_slo_output" 2>&1; then
  cat "$bad_slo_output" >&2
  fail "failed latency SLO proof unexpectedly passed"
fi
if ! grep -Fq -- "latencySLOAlertDelivery must be pass" "$bad_slo_output"; then
  cat "$bad_slo_output" >&2
  fail "bad latency SLO proof failed without expected diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected failed latency SLO proof"

bad_slo_delivery_file="$tmpdir/latency-slo-proof-bad-delivery.json"
cat >"$bad_slo_delivery_file" <<'JSON'
{
  "latencySLOTrigger": "pass",
  "latencySLOAlertDelivery": "pass",
  "latencySLORecoveryAction": "pass",
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "triggeredAlerts": 2,
  "alertDelivery": {
    "configuredProviders": 1,
    "deliveredAlerts": 2,
    "failedDeliveries": 1,
    "channels": ["pagerduty-primary"],
    "lastDeliveryId": "alert_delivery_20260616_0001"
  },
  "recoveryAudit": {
    "auditRecords": 2,
    "failedActions": 0,
    "lastRecordId": "slo_recovery_audit_20260616_0001"
  }
}
JSON

bad_slo_delivery_output="$tmpdir/bad-slo-delivery.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$bad_slo_delivery_file" \
  --output "$tmpdir/bad-slo-delivery-artifact.json" >"$bad_slo_delivery_output" 2>&1; then
  cat "$bad_slo_delivery_output" >&2
  fail "latency SLO proof with failed alert deliveries unexpectedly passed"
fi
if ! grep -Fq -- "slo.alertDelivery.failedDeliveries must be zero" "$bad_slo_delivery_output"; then
  cat "$bad_slo_delivery_output" >&2
  fail "bad latency SLO delivery proof failed without expected diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected failed latency SLO alert delivery details"

bad_slo_sample_id_file="$tmpdir/latency-slo-proof-sample-id.json"
cp "$slo_file" "$bad_slo_sample_id_file"
"$python_bin" - "$bad_slo_sample_id_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["alertDelivery"]["lastDeliveryId"] = "sample-alert-id"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_slo_sample_id_output="$tmpdir/bad-slo-sample-id.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$bad_slo_sample_id_file" \
  --output "$tmpdir/bad-slo-sample-id-artifact.json" >"$bad_slo_sample_id_output" 2>&1; then
  cat "$bad_slo_sample_id_output" >&2
  fail "latency SLO proof with sample delivery id unexpectedly passed"
fi
if ! grep -Fq -- "slo.alertDelivery.lastDeliveryId must be concrete" "$bad_slo_sample_id_output"; then
  cat "$bad_slo_sample_id_output" >&2
  fail "bad latency SLO sample delivery id failed without expected diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected sample latency SLO delivery id"

bad_url_output="$tmpdir/bad-url.out"
if bash "$collector" \
  --artifact-id artifact-request-log-observability-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --platform-proof-file "$platform_proof_file" \
  --target-base-url "http://127.0.0.1:${coverage_server_port}" \
  --coverage-query token=secret \
  --slo-file "$slo_file" \
  --output "$tmpdir/bad-url-artifact.json" >"$bad_url_output" 2>&1; then
  cat "$bad_url_output" >&2
  fail "coverage URL with secret query unexpectedly passed"
fi
if ! grep -Fq -- "coverage-query must not include secret-like query parameters" "$bad_url_output"; then
  cat "$bad_url_output" >&2
  fail "bad coverage URL failed without expected diagnostic"
fi
echo "[collect-request-log-observability-evidence-fixtures] rejected secret-like coverage query"

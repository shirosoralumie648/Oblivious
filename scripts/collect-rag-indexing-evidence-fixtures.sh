#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-rag-indexing-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"
proof_server_pid=""

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  if [[ -n "$proof_server_pid" ]]; then
    kill "$proof_server_pid" >/dev/null 2>&1 || true
    wait "$proof_server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[collect-rag-indexing-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
rag_proof_file="$tmpdir/rag-indexing-proof.json"
artifact_body="$artifact_dir/artifact-rag-indexing-20260616.json"
proof_server_script="$tmpdir/rag_proof_server.py"
proof_server_port_file="$tmpdir/rag-proof-server-port"
cookie_file="$tmpdir/release-cookie.txt"
release_window_query="from=2026-06-16T00:00:00Z&to=2026-06-16T01:00:00Z"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$rag_proof_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 3,
    "workerCompletedJobs": 3,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 2,
    "staleVectorRowsFiltered": 1
  }
}
JSON

cat >"$proof_server_script" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

proof_file = pathlib.Path(sys.argv[1])
port_file = pathlib.Path(sys.argv[2])


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path != "/api/v1/admin/release-evidence/rag-indexing":
            self.send_error(404)
            return
        bearer_ok = self.headers.get("Authorization") == "Bearer fixture-token"
        cookie_ok = self.headers.get("Cookie") == "release-session=fixture-session"
        if not bearer_ok and not cookie_ok:
            self.send_error(401)
            return
        payload = {"data": json.loads(proof_file.read_text(encoding="utf-8"))}
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

"$python_bin" "$proof_server_script" "$rag_proof_file" "$proof_server_port_file" &
proof_server_pid=$!
for _ in $(seq 1 50); do
  if [[ -s "$proof_server_port_file" ]]; then
    break
  fi
  sleep 0.1
done
if [[ ! -s "$proof_server_port_file" ]]; then
  fail "RAG proof fixture server did not start"
fi
proof_server_port=$(cat "$proof_server_port_file")

OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN=fixture-token bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-url "http://127.0.0.1:${proof_server_port}/api/v1/admin/release-evidence/rag-indexing?${release_window_query}" \
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
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-local-collection-source "$manifest" >/dev/null
echo "[collect-rag-indexing-evidence-fixtures] generated RAG indexing artifact body"

printf '%s\n' 'release-session=fixture-session' >"$cookie_file"
bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-url "http://127.0.0.1:${proof_server_port}/api/v1/admin/release-evidence/rag-indexing?${release_window_query}" \
  --cookie-file "$cookie_file" \
  --output "$tmpdir/cookie-rag-artifact.json" >/dev/null
echo "[collect-rag-indexing-evidence-fixtures] fetched RAG proof with cookie auth"

bad_url_output="$tmpdir/bad-url.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-url "http://127.0.0.1:${proof_server_port}/api/v1/admin/release-evidence/rag-indexing?${release_window_query}&token=secret" \
  --output "$tmpdir/bad-url-rag-artifact.json" >"$bad_url_output" 2>&1; then
  cat "$bad_url_output" >&2
  fail "RAG proof URL with secret query unexpectedly passed"
fi
if ! grep -Fq -- "proof-url must not carry secret-like query parameters" "$bad_url_output"; then
  cat "$bad_url_output" >&2
  fail "bad RAG proof URL failed without expected diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected secret-like RAG proof URL"

bad_proof_file="$tmpdir/rag-indexing-proof-bad.json"
cat >"$bad_proof_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "fail",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass"
}
JSON

bad_output="$tmpdir/bad-proof.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_proof_file" \
  --output "$tmpdir/bad-rag-artifact.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "failed raw parser replay proof unexpectedly passed"
fi
if ! grep -Fq -- "rawParserReplay must be pass" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad RAG proof failed without rawParserReplay diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected failed raw parser replay proof"

missing_summary_file="$tmpdir/rag-indexing-proof-missing-summary.json"
cat >"$missing_summary_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 2,
    "workerCompletedJobs": 3,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 1,
    "staleVectorRowsFiltered": 1
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$missing_summary_file" \
  --output "$tmpdir/bad-rag-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete RAG indexing summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.drainedJobs must equal summary.queuedJobs" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad RAG summary failed without drain diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected incomplete RAG drain summary"

worker_mismatch_file="$tmpdir/rag-indexing-proof-worker-mismatch.json"
cat >"$worker_mismatch_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 3,
    "workerCompletedJobs": 2,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 1,
    "staleVectorRowsFiltered": 1
  }
}
JSON

worker_mismatch_output="$tmpdir/worker-mismatch.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$worker_mismatch_file" \
  --output "$tmpdir/worker-mismatch-artifact.json" >"$worker_mismatch_output" 2>&1; then
  cat "$worker_mismatch_output" >&2
  fail "RAG worker mismatch summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.workerCompletedJobs must equal summary.drainedJobs" "$worker_mismatch_output"; then
  cat "$worker_mismatch_output" >&2
  fail "RAG worker mismatch summary failed without worker/drain diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected RAG worker mismatch summary"

zero_worker_file="$tmpdir/rag-indexing-proof-zero-worker-completed.json"
cat >"$zero_worker_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 2,
    "drainedJobs": 2,
    "workerCompletedJobs": 0,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 1,
    "staleVectorRowsFiltered": 1
  }
}
JSON

zero_worker_output="$tmpdir/zero-worker-completed.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$zero_worker_file" \
  --output "$tmpdir/zero-worker-completed-artifact.json" >"$zero_worker_output" 2>&1; then
  cat "$zero_worker_output" >&2
  fail "zero worker-completed summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.workerCompletedJobs must be greater than zero" "$zero_worker_output"; then
  cat "$zero_worker_output" >&2
  fail "zero worker-completed summary failed without worker diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected zero worker-completed summary"

zero_raw_parser_count_file="$tmpdir/rag-indexing-proof-zero-raw-parser-count.json"
cat >"$zero_raw_parser_count_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 2,
    "drainedJobs": 2,
    "workerCompletedJobs": 2,
    "rawParserReplayCount": 0,
    "retrievalProbeCount": 1,
    "staleVectorRowsFiltered": 1
  }
}
JSON

zero_raw_parser_count_output="$tmpdir/zero-raw-parser-count.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$zero_raw_parser_count_file" \
  --output "$tmpdir/zero-raw-parser-count-artifact.json" >"$zero_raw_parser_count_output" 2>&1; then
  cat "$zero_raw_parser_count_output" >&2
  fail "zero raw-parser replay count summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.rawParserReplayCount must be greater than zero" "$zero_raw_parser_count_output"; then
  cat "$zero_raw_parser_count_output" >&2
  fail "zero raw-parser replay count summary failed without raw parser diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected zero raw-parser replay count summary"

zero_stale_vector_file="$tmpdir/rag-indexing-proof-zero-stale-vector.json"
cat >"$zero_stale_vector_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 2,
    "drainedJobs": 2,
    "workerCompletedJobs": 2,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 1,
    "staleVectorRowsFiltered": 0
  }
}
JSON

zero_stale_output="$tmpdir/zero-stale-vector.out"
if bash "$collector" \
  --artifact-id artifact-rag-indexing-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$zero_stale_vector_file" \
  --output "$tmpdir/zero-stale-vector-artifact.json" >"$zero_stale_output" 2>&1; then
  cat "$zero_stale_output" >&2
  fail "zero stale-vector filter summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.staleVectorRowsFiltered must be greater than zero" "$zero_stale_output"; then
  cat "$zero_stale_output" >&2
  fail "zero stale-vector summary failed without stale vector diagnostic"
fi
echo "[collect-rag-indexing-evidence-fixtures] rejected zero stale-vector filter summary"

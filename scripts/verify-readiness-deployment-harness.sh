#!/usr/bin/env bash
set -euo pipefail

mode=""
repo_root=""
output_dir=""
artifact_bundle=""
image_tag=""
image_digest=""
python_bin=${PYTHON:-python3}
nonce="${$}-${RANDOM}"
prefix="oblivious-readiness-${nonce}"
network_name="${prefix}-network"
postgres_name="${prefix}-postgres"
redis_name="${prefix}-redis"
qdrant_name="${prefix}-qdrant"
clickhouse_name="${prefix}-clickhouse"
kafka_name="${prefix}-kafka"
server_name="${prefix}-server"
audit_volume="${prefix}-audit"
control_pid=""
control_dir=""
build_output=""
resources_started=false

fail() {
  local code="$1"
  if [[ -n "${output_dir:-}" && -d "${output_dir:-}" ]]; then
    docker logs "$postgres_name" >"$output_dir/postgres-failure.log" 2>&1 || true
    docker logs "$redis_name" >"$output_dir/redis-failure.log" 2>&1 || true
    docker logs "$qdrant_name" >"$output_dir/qdrant-failure.log" 2>&1 || true
    docker logs "$clickhouse_name" >"$output_dir/clickhouse-failure.log" 2>&1 || true
    docker logs "$kafka_name" >"$output_dir/kafka-failure.log" 2>&1 || true
    docker logs "$server_name" >"$output_dir/server-failure.log" 2>&1 || true
  fi
  printf '{"error":{"code":"%s"}}\n' "$code" >&2
  exit 1
}

cleanup() {
  local status=$?
  if [[ -n "$control_pid" ]]; then
    kill "$control_pid" >/dev/null 2>&1 || true
    wait "$control_pid" >/dev/null 2>&1 || true
  fi
  docker rm -f "$server_name" "$postgres_name" "$redis_name" "$qdrant_name" "$clickhouse_name" "$kafka_name" >/dev/null 2>&1 || true
  docker network rm "$network_name" >/dev/null 2>&1 || true
  docker volume rm -f "$audit_volume" >/dev/null 2>&1 || true
  if [[ -n "$control_dir" && -d "$control_dir" ]]; then
    rm -rf "$control_dir"
  fi
  if [[ "$resources_started" == true ]]; then
    if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -Fxq "$server_name"; then status=1; fi
    for resource_name in "$postgres_name" "$redis_name" "$qdrant_name" "$clickhouse_name" "$kafka_name"; do
      if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -Fxq "$resource_name"; then status=1; fi
    done
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) [[ $# -ge 2 ]] || fail invalid_arguments; mode="$2"; shift 2 ;;
    --repo-root) [[ $# -ge 2 ]] || fail invalid_arguments; repo_root="$2"; shift 2 ;;
    --output-dir) [[ $# -ge 2 ]] || fail invalid_arguments; output_dir="$2"; shift 2 ;;
    --artifact-bundle) [[ $# -ge 2 ]] || fail invalid_arguments; artifact_bundle="$2"; shift 2 ;;
    --image-tag) [[ $# -ge 2 ]] || fail invalid_arguments; image_tag="$2"; shift 2 ;;
    --image-digest) [[ $# -ge 2 ]] || fail invalid_arguments; image_digest="$2"; shift 2 ;;
    *) fail invalid_arguments ;;
  esac
done

[[ "$mode" == "standalone-build" || "$mode" == "aggregate-consume" ]] || fail invalid_mode
[[ -n "$repo_root" && -n "$output_dir" ]] || fail invalid_arguments
if [[ "$mode" == "aggregate-consume" ]]; then
  [[ -n "$artifact_bundle" && -n "$image_tag" && -n "$image_digest" ]] || fail artifact_bundle_required
  [[ -f "$artifact_bundle" ]] || fail artifact_bundle_missing
  [[ "$image_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || fail image_digest_mismatch
fi
for command_name in git go docker jq curl sha256sum "$python_bin"; do
  command -v "$command_name" >/dev/null 2>&1 || fail harness_tool_missing
done
docker info >/dev/null 2>&1 || fail docker_unavailable
git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail repo_invalid
repo_root=$(cd "$repo_root" && pwd -P)
if [[ -n "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]]; then
  fail source_worktree_dirty
fi
if [[ -e "$output_dir" && -n "$(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
  fail output_not_empty
fi
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd -P)

contract_path="config/release/contract.v1.json"
schema_path="config/release/contract.schema.json"
identity_json=$(cd "$repo_root/src/server" && go run ./cmd/release-contract identity --repo "$repo_root" --contract "$contract_path" --schema "$schema_path") || fail build_identity_mismatch
release_commit=$(jq -er '.releaseCommit' <<<"$identity_json") || fail build_identity_mismatch
source_tree=$(jq -er '.sourceTree' <<<"$identity_json") || fail build_identity_mismatch
contract_digest=$(jq -er '.contractDigest' <<<"$identity_json") || fail build_identity_mismatch
jq -e '.schemaVersion == "build-identity/v1" and .dirty == false and .evidenceClass == "repository-local"' <<<"$identity_json" >/dev/null || fail build_identity_mismatch

build_count=0
if [[ "$mode" == "standalone-build" ]]; then
  [[ -z "$artifact_bundle" && -z "$image_digest" ]] || fail invalid_arguments
  if [[ -z "$image_tag" ]]; then image_tag="oblivious-readiness:${release_commit}"; fi
  build_output="$repo_root/.tmp/readiness-build-${nonce}"
  mkdir -p "$build_output"
  env -u RELEASE_COMMIT -u SOURCE_TREE -u CONTRACT_DIGEST -u BUILD_DIRTY -u EVIDENCE_CLASS \
    bash "$repo_root/scripts/build-release-image.sh" \
      --image-tag "$image_tag" --contract "$contract_path" --schema "$schema_path" --output-dir "$build_output" \
      >"$output_dir/build-result.json" || fail image_build_failed
  build_count=1
  image_digest=$(docker image inspect --format '{{.Id}}' "$image_tag") || fail image_missing
else
  jq -e \
    --arg commit "$release_commit" --arg tree "$source_tree" --arg contract "$contract_digest" \
    --arg tag "$image_tag" --arg digest "$image_digest" \
    '.schemaVersion == "release-artifact-bundle/v1" and
     .releaseIdentity.releaseCommit == $commit and .releaseIdentity.sourceTree == $tree and
     .releaseIdentity.contractDigest == $contract and .releaseIdentity.deploymentProfile == "monolith" and
     .releaseIdentity.dirty == false and .releaseIdentity.evidenceClass == "repository-local" and
     .image.tag == $tag and .image.digest == $digest' "$artifact_bundle" >/dev/null || fail artifact_bundle_mismatch
  resolved_digest=$(docker image inspect --format '{{.Id}}' "$image_tag" 2>/dev/null) || fail image_missing
  [[ "$resolved_digest" == "$image_digest" ]] || fail image_digest_mismatch
fi

[[ "$build_count" == "0" || "$mode" == "standalone-build" ]] || fail aggregate_rebuild_attempted
printf '%s\n' "$build_count" >"$output_dir/build-invocation-count.txt"
resolved_digest=$(docker image inspect --format '{{.Id}}' "$image_tag" 2>/dev/null) || fail image_missing
[[ "$resolved_digest" == "$image_digest" ]] || fail image_digest_mismatch
labels=$(docker image inspect --format '{{json .Config.Labels}}' "$image_tag") || fail image_identity_mismatch
jq -e --arg commit "$release_commit" --arg tree "$source_tree" --arg contract "$contract_digest" \
  '."org.opencontainers.image.revision" == $commit and ."io.oblivious.source-tree" == $tree and
   ."io.oblivious.release-contract-digest" == $contract and ."io.oblivious.build-identity-schema" == "build-identity/v1" and
   ."io.oblivious.evidence-class" == "repository-local" and ."io.oblivious.deployment-profile" == "monolith"' \
  <<<"$labels" >/dev/null || fail image_identity_mismatch
docker run --rm --entrypoint /usr/local/bin/oblivious-release-contract "$image_tag" inspect \
  --repo /app --contract "$contract_path" --schema "$schema_path" >"$output_dir/image-inspection.json" || fail image_inspection_failed
jq -e --arg commit "$release_commit" --arg tree "$source_tree" --arg contract "$contract_digest" \
  '.releaseCommit == $commit and .sourceTree == $tree and .contractDigest == $contract and .dirty == false' \
  "$output_dir/image-inspection.json" >/dev/null || fail image_identity_mismatch

control_dir="$output_dir/.fault-probe-${nonce}"
mkdir -p "$control_dir"
printf '{"mode":"healthy"}\n' >"$control_dir/control.json.tmp"
mv "$control_dir/control.json.tmp" "$control_dir/control.json"
: >"$control_dir/requests.jsonl"
cat >"$control_dir/server.py" <<'PY'
import http.server
import json
import os
import pathlib
import threading
import time
import urllib.error
import urllib.request

root = pathlib.Path(os.environ["FAULT_PROBE_ROOT"])
target = os.environ["QDRANT_TARGET_URL"].rstrip("/")
lock = threading.Lock()

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        control = json.loads((root / "control.json").read_text(encoding="utf-8"))
        mode = control["mode"]
        record = {"at": time.time_ns(), "mode": mode, "path": self.path}
        with lock:
            with (root / "requests.jsonl").open("a", encoding="utf-8") as handle:
                handle.write(json.dumps(record, sort_keys=True, separators=(",", ":")) + "\n")
        if mode == "blocked":
            self.send_response(503)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"selected dependency unavailable: should-never-escape")
            return
        try:
            with urllib.request.urlopen(target + self.path, timeout=5) as response:
                body = response.read(65536)
                self.send_response(response.status)
                self.send_header("Content-Type", response.headers.get("Content-Type", "text/plain"))
        except urllib.error.HTTPError as error:
            body = error.read(65536)
            self.send_response(error.code)
            self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        return

server = http.server.ThreadingHTTPServer(("0.0.0.0", 0), Handler)
(root / "port.tmp").write_text(str(server.server_port), encoding="utf-8")
os.replace(root / "port.tmp", root / "port")
server.serve_forever()
PY
docker network create "$network_name" >/dev/null || fail docker_network_failed
docker volume create "$audit_volume" >/dev/null || fail docker_volume_failed
resources_started=true
postgres_image=${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}
docker run -d --name "$postgres_name" --network "$network_name" \
  -e POSTGRES_USER=oblivious -e POSTGRES_PASSWORD=oblivious -e POSTGRES_DB=oblivious \
  "$postgres_image" >/dev/null || fail database_start_failed
for _ in $(seq 1 90); do
  if docker exec "$postgres_name" psql -U oblivious -d oblivious -Atqc 'SELECT 1' 2>/dev/null | grep -Fqx 1; then break; fi
  sleep 1
done
docker exec "$postgres_name" psql -U oblivious -d oblivious -Atqc 'SELECT 1' 2>/dev/null | grep -Fqx 1 || fail database_unavailable
docker run -d --name "$redis_name" --network "$network_name" redis:7-alpine >/dev/null || fail redis_start_failed
docker run -d --name "$qdrant_name" --network "$network_name" -p 127.0.0.1::6333 qdrant/qdrant:v1.12.1 >/dev/null || fail qdrant_start_failed
docker run -d --name "$clickhouse_name" --network "$network_name" \
  -e CLICKHOUSE_DB=oblivious -e CLICKHOUSE_USER=oblivious -e CLICKHOUSE_PASSWORD=oblivious \
  clickhouse/clickhouse-server:24.12-alpine >/dev/null || fail clickhouse_start_failed
docker run -d --name "$kafka_name" --network "$network_name" \
  -e CLUSTER_ID=MkU3OEVBNTcwNTJENDM2Qk -e KAFKA_NODE_ID=0 \
  -e KAFKA_PROCESS_ROLES=controller,broker -e KAFKA_CONTROLLER_QUORUM_VOTERS="0@${kafka_name}:9093" \
  -e KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
  -e KAFKA_ADVERTISED_LISTENERS="PLAINTEXT://${kafka_name}:9092" \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER -e KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 -e KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0 \
  apache/kafka:3.7.1 >/dev/null || fail kafka_start_failed
for _ in $(seq 1 90); do docker exec "$redis_name" redis-cli ping 2>/dev/null | grep -Fqx PONG && break; sleep 1; done
docker exec "$redis_name" redis-cli ping 2>/dev/null | grep -Fqx PONG || fail redis_unavailable
qdrant_host_port=$(docker port "$qdrant_name" 6333/tcp | awk -F: 'NR==1 {print $NF}')
for _ in $(seq 1 90); do curl -fsS "http://127.0.0.1:${qdrant_host_port}/healthz" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS "http://127.0.0.1:${qdrant_host_port}/healthz" >/dev/null 2>&1 || fail qdrant_unavailable
for _ in $(seq 1 90); do docker exec "$clickhouse_name" clickhouse-client --user oblivious --password oblivious --query 'SELECT 1' >/dev/null 2>&1 && break; sleep 1; done
docker exec "$clickhouse_name" clickhouse-client --user oblivious --password oblivious --query 'SELECT 1' >/dev/null 2>&1 || fail clickhouse_unavailable
for _ in $(seq 1 120); do docker exec "$kafka_name" /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list >/dev/null 2>&1 && break; sleep 1; done
docker exec "$kafka_name" /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list >/dev/null 2>&1 || fail kafka_unavailable

FAULT_PROBE_ROOT="$control_dir" QDRANT_TARGET_URL="http://127.0.0.1:${qdrant_host_port}" "$python_bin" "$control_dir/server.py" >"$output_dir/fault-probe.log" 2>&1 &
control_pid=$!
for _ in $(seq 1 100); do [[ -s "$control_dir/port" ]] && break; kill -0 "$control_pid" 2>/dev/null || fail fault_probe_failed; sleep 0.1; done
[[ -s "$control_dir/port" ]] || fail fault_probe_failed
control_port=$(<"$control_dir/port")

database_url="postgres://oblivious:oblivious@${postgres_name}:5432/oblivious?sslmode=disable"
common_env=(
  -e APP_ENV=development -e DATABASE_URL="$database_url" -e SESSION_SECRET=readiness-harness-session-secret
  -e OBLIVIOUS_DEPLOYMENT_PROFILE=monolith -e OBLIVIOUS_READINESS_AUDIT_PATH=/var/lib/oblivious/audit/readiness.json
  -e REDIS_URL="redis://${redis_name}:6379/0" -e RELAY_RATE_LIMIT_BACKEND=memory
  -e QDRANT_URL="http://host.docker.internal:${control_port}"
  -e OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse -e CLICKHOUSE_DRIVER=clickhouse
  -e CLICKHOUSE_DSN="tcp://oblivious:oblivious@${clickhouse_name}:9000?database=oblivious"
  -e KAFKA_BROKERS="${kafka_name}:9092"
  -e RELAY_ENABLED=false -e SCHEDULE_WORKER_ENABLED=false -e RAG_INDEX_WORKER_ENABLED=false -e RAG_INGESTION_WORKER_ENABLED=false
)
docker run --rm --network "$network_name" --add-host host.docker.internal:host-gateway "${common_env[@]}" --entrypoint /usr/local/bin/oblivious-migrate "$image_tag" \
  >"$output_dir/migration.log" 2>&1 || fail migration_failed
grep -Eq 'migrations applied: [0-9]+, skipped: [0-9]+' "$output_dir/migration.log" || fail migration_unverified
docker run --rm -v "$audit_volume:/audit" --entrypoint /bin/sh alpine:3.21 -ec 'chown 1000:1000 /audit' >/dev/null || fail audit_storage_unwritable

host_port=$($python_bin - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)
docker run -d --name "$server_name" --network "$network_name" --add-host host.docker.internal:host-gateway \
  -p "127.0.0.1:${host_port}:8080" -v "$audit_volume:/var/lib/oblivious/audit" \
  "${common_env[@]}" "$image_tag" >/dev/null || fail server_start_failed
base_url="http://127.0.0.1:${host_port}"

poll_status() {
  local path="$1" expected="$2" attempts="$3"
  local code=""
  for _ in $(seq 1 "$attempts"); do
    code=$(curl -sS -o "$output_dir/http-response.json" -w '%{http_code}' "$base_url$path" 2>/dev/null || true)
    [[ "$code" == "$expected" ]] && return 0
    sleep 1
  done
  return 1
}
copy_audit() {
  local destination="$1"
  docker cp "$server_name:/var/lib/oblivious/audit/readiness.json" "$destination" >/dev/null 2>&1
}
set_control_mode() {
  local next="$1"
  printf '{"mode":"%s"}\n' "$next" >"$control_dir/control.json.tmp"
  mv "$control_dir/control.json.tmp" "$control_dir/control.json"
}
wait_generation_after() {
  local baseline="$1" destination="$2" attempts="$3"
  local generation=""
  for _ in $(seq 1 "$attempts"); do
    if copy_audit "$destination"; then
      generation=$(jq -r '.generation // 0' "$destination" 2>/dev/null || echo 0)
      if [[ "$generation" =~ ^[0-9]+$ ]] && (( generation > baseline )); then return 0; fi
    fi
    sleep 1
  done
  return 1
}

poll_status /livez 200 300 || { docker logs "$server_name" >&2 || true; fail livez_failed; }
poll_status /readyz 200 60 || { docker logs "$server_name" >&2 || true; fail readyz_failed; }
copy_audit "$output_dir/healthy-initial.json" || fail audit_missing
initial_generation=$(jq -er '.generation' "$output_dir/healthy-initial.json") || fail audit_invalid

set_control_mode blocked
wait_generation_after "$initial_generation" "$output_dir/blocked.json" 65 || fail blocked_generation_missing
poll_status /readyz 503 10 || fail blocked_readyz_failed
grep -Eq 'capability_blocked|dependency_unproven' "$output_dir/http-response.json" || fail blocked_reason_missing
blocked_generation=$(jq -er '.generation' "$output_dir/blocked.json") || fail audit_invalid

set_control_mode healthy
poll_status /readyz 200 65 || fail healthy_restore_failed
wait_generation_after "$blocked_generation" "$output_dir/healthy-restored.json" 10 || fail healthy_generation_missing

stale_baseline=$(jq -er '.generation' "$output_dir/healthy-restored.json") || fail audit_invalid
docker kill --signal=STOP "$server_name" >/dev/null || fail stale_pause_failed
stale_pids=()
for attempt in $(seq 1 32); do
  curl -sS --max-time 140 -o "$output_dir/stale-response-${attempt}.json" -w '%{http_code}' "$base_url/readyz" >"$output_dir/stale-status-${attempt}.txt" 2>/dev/null &
  stale_pids+=("$!")
done
sleep 125
docker kill --signal=CONT "$server_name" >/dev/null || fail stale_resume_failed
for stale_pid in "${stale_pids[@]}"; do wait "$stale_pid" || true; done
if ! grep -lF 'readiness_stale' "$output_dir"/stale-response-*.json >"$output_dir/stale-observed-files.txt"; then
  fail readiness_stale_not_observed
fi
poll_status /livez 200 5 || fail stale_livez_changed
poll_status /readyz 200 65 || fail final_restore_failed
wait_generation_after "$stale_baseline" "$output_dir/readiness-snapshot.json" 10 || fail final_generation_missing
cp "$control_dir/requests.jsonl" "$output_dir/fault-requests.jsonl"

readiness_report="$output_dir/readiness-report.json"
deployment_report="$output_dir/deployment-report.json"
deployment_observation="$output_dir/deployment-observation.json"
jq -n '{profile:"monolith",canonicalWorkload:"deploy/kubernetes/app-deployment.yaml",startupEndpoint:"/livez",livenessEndpoint:"/livez",readinessEndpoint:"/readyz",auditStorage:"emptyDir",migrationState:"applied_and_validated",harnessResult:"passed"}' >"$deployment_observation"
(
  cd "$repo_root/src/server"
  go run ./cmd/release-contract report-readiness --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" \
    --profile monolith --snapshot "$output_dir/readiness-snapshot.json" --output "$readiness_report"
) >"$output_dir/readiness-report-command.json" || fail readiness_report_failed
(
  cd "$repo_root/src/server"
  go run ./cmd/release-contract report-deployment --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" \
    --profile monolith --observation "$deployment_observation" --output "$deployment_report"
) >"$output_dir/deployment-report-command.json" || fail deployment_report_failed

readiness_identity=$(jq -cS '.releaseIdentity' "$readiness_report") || fail report_invalid
deployment_identity=$(jq -cS '.releaseIdentity' "$deployment_report") || fail report_invalid
[[ "$readiness_identity" == "$deployment_identity" ]] || fail report_identity_splice
jq -e '.surfaceIdentity.surface == "readiness" and .outcome.result == "pass" and (.outcome.skippedChecks | length) == 0 and .releaseIdentity.evidenceClass == "repository-local"' "$readiness_report" >/dev/null || fail report_invalid
jq -e '.surfaceIdentity.surface == "deployment" and .outcome.result == "pass" and (.outcome.skippedChecks | length) == 0 and .releaseIdentity.evidenceClass == "repository-local"' "$deployment_report" >/dev/null || fail report_invalid

jq -n \
  --arg mode "$mode" --arg commit "$release_commit" --arg tree "$source_tree" --arg contract "$contract_digest" \
  --arg imageTag "$image_tag" --arg imageDigest "$image_digest" --argjson buildCount "$build_count" \
  '{schemaVersion:"readiness-deployment-harness/v1",result:"pass",evidenceClass:"repository-local",environment:"disposable-docker",
    mode:$mode,releaseCommit:$commit,sourceTree:$tree,contractDigest:$contract,deploymentProfile:"monolith",
    imageTag:$imageTag,imageDigest:$imageDigest,buildInvocationCount:$buildCount,migrationState:"applied_and_validated",
    reports:["readiness-report.json","deployment-report.json"],skippedChecks:[],
    residualRisks:["external target not inspected","Kubernetes workload not applied","profile parity deferred"],
    claim:"repository-local E2 only; no E3/E4, target, or commercial release claim"}' >"$output_dir/harness-result.json"

echo "[readiness-deployment-harness] readiness deployment harness passed; repository-local E2 mode=$mode image=$image_digest"

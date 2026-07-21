#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
python_bin=${PYTHON:-python3}
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

fail() {
  echo "[k8s-readiness-runner-fixtures] $*" >&2
  exit 1
}

copy_contract_fixture() {
  local destination="$1"
  mkdir -p "$destination/deploy/kubernetes" "$destination/scripts"
  cp "$repo_root/Dockerfile.server" "$destination/Dockerfile.server"
  cp "$repo_root/docker-compose.yml" "$destination/docker-compose.yml"
  cp "$repo_root/deploy/kubernetes/app-deployment.yaml" "$destination/deploy/kubernetes/app-deployment.yaml"
  cp "$repo_root/deploy/kubernetes/configmap.yaml" "$destination/deploy/kubernetes/configmap.yaml"
  cp "$repo_root/deploy/kubernetes/kafka.yaml" "$destination/deploy/kubernetes/kafka.yaml"
  cp "$repo_root/scripts/k8s-validate.sh" "$destination/scripts/k8s-validate.sh"
  cp "$repo_root/scripts/verify-readiness-deployment-contract.sh" "$destination/scripts/verify-readiness-deployment-contract.sh"
  cp "$repo_root/scripts/verify_deployment_operations_contract.py" "$destination/scripts/verify_deployment_operations_contract.py"
}

replace_once() {
  local path="$1"
  local old="$2"
  local new="$3"
  "$python_bin" - "$path" "$old" "$new" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
old = sys.argv[2]
new = sys.argv[3]
content = path.read_text(encoding="utf-8")
if content.count(old) != 1:
    raise SystemExit(f"mutation source count must be one: {old!r}")
path.write_text(content.replace(old, new, 1), encoding="utf-8")
PY
}

expect_contract_failure() {
  local label="$1"
  local expected="$2"
  local fixture="$tmpdir/mutation-$label"
  local output
  shift 2

  copy_contract_fixture "$fixture"
  "$@" "$fixture"
  if output=$(OBLIVIOUS_READINESS_DEPLOYMENT_ASSETS_ONLY=true \
    bash "$fixture/scripts/verify-readiness-deployment-contract.sh" 2>&1); then
    fail "mutation unexpectedly passed: $label"
  fi
  if [[ "$output" != *"$expected"* ]]; then
    printf '%s\n' "$output" >&2
    fail "mutation failed for the wrong reason: $label (expected: $expected)"
  fi
  echo "[k8s-readiness-runner-fixtures] rejected $label"
}

require_contract_success() {
  local fixture="$1"
  local output
  output=$(OBLIVIOUS_READINESS_DEPLOYMENT_ASSETS_ONLY=true \
    bash "$fixture/scripts/verify-readiness-deployment-contract.sh" 2>&1) || {
    printf '%s\n' "$output" >&2
    return 1
  }
  if [[ "$output" != *"readiness deployment assets passed"* ]]; then
    echo "deployment verifier completion marker is missing; verifier did not execute" >&2
    return 1
  fi
}

mutate_missing_kafka_apply() {
  replace_once "$1/scripts/k8s-validate.sh" \
    'kubectl apply -f deploy/kubernetes/kafka.yaml' ''
}

mutate_missing_kafka_wait() {
  replace_once "$1/scripts/k8s-validate.sh" \
    'kubectl -n "$namespace" rollout status statefulset/kafka --timeout="${OBLIVIOUS_K8S_KAFKA_TIMEOUT:-300s}"' ''
}

mutate_duplicate_kafka_apply() {
  replace_once "$1/scripts/k8s-validate.sh" \
    'kubectl apply -f deploy/kubernetes/kafka.yaml' \
    $'kubectl apply -f deploy/kubernetes/kafka.yaml\nkubectl apply -f deploy/kubernetes/kafka.yaml'
}

mutate_wrong_service() {
  replace_once "$1/deploy/kubernetes/configmap.yaml" \
    'oblivious-kafka.oblivious.svc.cluster.local:9092' \
    'missing-kafka.oblivious.svc.cluster.local:9092'
}

mutate_wrong_port() {
  replace_once "$1/deploy/kubernetes/configmap.yaml" \
    'oblivious-kafka.oblivious.svc.cluster.local:9092' \
    'oblivious-kafka.oblivious.svc.cluster.local:9094'
}

mutate_wrong_statefulset() {
  replace_once "$1/scripts/k8s-validate.sh" 'statefulset/kafka' 'statefulset/missing-kafka'
}

mutate_wait_after_app() {
  "$python_bin" - "$1/scripts/k8s-validate.sh" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
content = path.read_text(encoding="utf-8")
wait = 'kubectl -n "$namespace" rollout status statefulset/kafka --timeout="${OBLIVIOUS_K8S_KAFKA_TIMEOUT:-300s}"\n'
app = 'kubectl apply -f "$render_dir/app-deployment.yaml"\n'
if content.count(wait) != 1 or content.count(app) != 1:
    raise SystemExit("wait-after-app mutation source must be unique")
content = content.replace(wait, "", 1).replace(app, app + wait, 1)
path.write_text(content, encoding="utf-8")
PY
}

mutate_healthz_only() {
  replace_once "$1/scripts/k8s-validate.sh" \
    'curl -fsS "http://127.0.0.1:$port/readyz"' \
    'curl -fsS "http://127.0.0.1:$port/healthz"'
}

mutate_verifier_not_executed() {
  printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$1/scripts/verify-readiness-deployment-contract.sh"
}

write_runner_fakes() {
  local fixture="$1"
  mkdir -p "$fixture/bin"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'printf "kubectl" >>"$OBLIVIOUS_K8S_TRACE_FILE"' \
    'printf " %s" "$@" >>"$OBLIVIOUS_K8S_TRACE_FILE"' \
    'printf "\\n" >>"$OBLIVIOUS_K8S_TRACE_FILE"' \
    'if [[ " $* " == *" port-forward "* ]]; then' \
    '  trap "exit 0" TERM INT' \
    '  while :; do /bin/sleep 1; done' \
    'fi' \
    'exit 0' >"$fixture/bin/kubectl"
  chmod +x "$fixture/bin/kubectl"

  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'output_file=""' \
    'url=""' \
    'write_status=false' \
    'args=("$@")' \
    'for ((index=0; index < ${#args[@]}; index++)); do' \
    '  case "${args[$index]}" in' \
    '    -o) output_file="${args[$((index + 1))]}" ;;' \
    '    -w) write_status=true ;;' \
    '    http://*|https://*) url="${args[$index]}" ;;' \
    '  esac' \
    'done' \
    'printf "curl %s\\n" "$url" >>"$OBLIVIOUS_K8S_TRACE_FILE"' \
    'status=200' \
    'body="ok"' \
    'case "$url" in' \
    '  */metrics) body="# HELP fake_metric fixture metric" ;;' \
    '  */api/v1/auth/me) status=401 ;;' \
    '  */v1/chat/completions) status=400 ;;' \
    'esac' \
    'if [[ -n "$output_file" ]]; then printf "%s\\n" "$body" >"$output_file"; fi' \
    'if [[ "$write_status" == true ]]; then printf "%s" "$status"; fi' \
    'exit 0' >"$fixture/bin/curl"
  chmod +x "$fixture/bin/curl"
}

assert_trace_contract() {
  local trace="$1"
  "$python_bin" - "$trace" <<'PY'
import pathlib
import sys

trace = pathlib.Path(sys.argv[1])
if not trace.is_file() or not trace.read_text(encoding="utf-8").strip():
    raise SystemExit("runner trace is empty; the real runner did not execute")
lines = trace.read_text(encoding="utf-8").splitlines()

checks = [
    ("Kafka apply", lambda line: line == "kubectl apply -f deploy/kubernetes/kafka.yaml"),
    ("Kafka wait", lambda line: line == "kubectl -n oblivious rollout status statefulset/kafka --timeout=300s"),
    ("app apply", lambda line: line.startswith("kubectl apply -f /tmp/oblivious-k8s-validate.") and line.endswith("/app-deployment.yaml")),
    ("server rollout", lambda line: line == "kubectl -n oblivious rollout status deployment/oblivious-server"),
    ("runner readyz", lambda line: line == "curl http://127.0.0.1:18080/readyz"),
]
positions = []
for label, predicate in checks:
    matches = [index for index, line in enumerate(lines) if predicate(line)]
    if len(matches) != 1:
        raise SystemExit(f"runner trace requires exactly one {label}; found {len(matches)}")
    positions.append(matches[0])
if positions != sorted(positions) or len(set(positions)) != len(positions):
    raise SystemExit("runner trace must apply/wait Kafka before app rollout and /readyz")

healthz = [index for index, line in enumerate(lines) if line == "curl http://127.0.0.1:18080/healthz"]
metrics = [index for index, line in enumerate(lines) if line == "curl http://127.0.0.1:18080/metrics"]
if len(healthz) != 2 or len(metrics) != 1:
    raise SystemExit("runner trace must include port-forward healthz and the real deploy smoke")
if not (healthz[0] < positions[-1] < healthz[1] < metrics[0]):
    raise SystemExit("runner /readyz must follow port-forward healthz and precede deploy smoke")
PY
}

run_positive_trace() {
  local fixture="$tmpdir/positive-runner"
  local trace="$fixture/trace.log"
  copy_contract_fixture "$fixture"
  cp "$repo_root/scripts/deploy-smoke.sh" "$fixture/scripts/deploy-smoke.sh"
  write_runner_fakes "$fixture"
  printf '%s\n' \
    'apiVersion: v1' \
    'kind: Secret' \
    'metadata:' \
    '  name: oblivious-secrets' \
    '  namespace: oblivious' \
    'stringData:' \
    '  SESSION_SECRET: fixture-session-secret-31-1-22' >"$fixture/secret.yaml"
  : >"$trace"

  (
    cd "$fixture"
    PATH="$fixture/bin:$PATH" \
      OBLIVIOUS_K8S_TRACE_FILE="$trace" \
      OBLIVIOUS_K8S_SECRET_FILE="$fixture/secret.yaml" \
      OBLIVIOUS_K8S_PORT_FORWARD_ATTEMPTS=1 \
      OBLIVIOUS_K8S_PORT_FORWARD_SLEEP_SECONDS=0 \
      DEPLOY_SMOKE_ATTEMPTS=1 \
      DEPLOY_SMOKE_SLEEP_SECONDS=0 \
      bash scripts/k8s-validate.sh >/dev/null
  )
  assert_trace_contract "$trace"
  echo "[k8s-readiness-runner-fixtures] positive real-runner trace passed"
}

OBLIVIOUS_READINESS_DEPLOYMENT_ASSETS_ONLY=true \
  bash "$repo_root/scripts/verify-readiness-deployment-contract.sh" >/dev/null
run_positive_trace

expect_contract_failure address-without-deployment \
  'ordered command: kubectl apply -f deploy/kubernetes/kafka.yaml' \
  mutate_missing_kafka_apply
expect_contract_failure missing-kafka-wait \
  'ordered command: kubectl -n "$namespace" rollout status statefulset/kafka' \
  mutate_missing_kafka_wait
expect_contract_failure duplicate-kafka-apply \
  'ordered command: kubectl apply -f deploy/kubernetes/kafka.yaml' \
  mutate_duplicate_kafka_apply
expect_contract_failure wrong-kafka-service \
  'Kubernetes KAFKA_BROKERS must resolve to the canonical Kafka Service' \
  mutate_wrong_service
expect_contract_failure wrong-kafka-port \
  'Kubernetes KAFKA_BROKERS port must match the Kafka client Service port' \
  mutate_wrong_port
expect_contract_failure wrong-kafka-statefulset \
  'ordered command: kubectl -n "$namespace" rollout status statefulset/kafka' \
  mutate_wrong_statefulset
expect_contract_failure kafka-wait-after-app \
  'Kubernetes runner must apply/wait Kafka before app rollout and /readyz' \
  mutate_wait_after_app
expect_contract_failure healthz-only \
  'ordered command: curl -fsS "http://127.0.0.1:$port/readyz"' \
  mutate_healthz_only

fixture="$tmpdir/mutation-verifier-not-executed"
copy_contract_fixture "$fixture"
mutate_verifier_not_executed "$fixture"
if output=$(require_contract_success "$fixture" 2>&1); then
  fail "verifier-not-executed mutation unexpectedly passed"
fi
if [[ "$output" != *"deployment verifier completion marker is missing; verifier did not execute"* ]]; then
  printf '%s\n' "$output" >&2
  fail "verifier-not-executed mutation failed for the wrong reason"
fi
echo "[k8s-readiness-runner-fixtures] rejected verifier-not-executed"

empty_trace="$tmpdir/runner-not-executed.log"
: >"$empty_trace"
if output=$(assert_trace_contract "$empty_trace" 2>&1); then
  fail "runner-not-executed mutation unexpectedly passed"
fi
if [[ "$output" != *"runner trace is empty; the real runner did not execute"* ]]; then
  printf '%s\n' "$output" >&2
  fail "runner-not-executed mutation failed for the wrong reason"
fi
echo "[k8s-readiness-runner-fixtures] rejected runner-not-executed"

echo "[k8s-readiness-runner-fixtures] k8s readiness runner fixtures passed (10 mutations)"

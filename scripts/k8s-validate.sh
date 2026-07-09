#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
namespace="${OBLIVIOUS_K8S_NAMESPACE:-oblivious}"
port="${OBLIVIOUS_K8S_PORT:-18080}"
cleanup_namespace="${OBLIVIOUS_K8S_CLEANUP:-false}"
secret_file="${OBLIVIOUS_K8S_SECRET_FILE:-}"
port_forward_pid=""
render_dir=""

server_image="${OBLIVIOUS_K8S_SERVER_IMAGE:-}"
server_pull_policy="${OBLIVIOUS_K8S_IMAGE_PULL_POLICY:-}"
postgres_sslmode="${OBLIVIOUS_K8S_POSTGRES_SSLMODE:-disable}"
if [[ -z "$server_image" ]]; then
  if [[ -n "${OBLIVIOUS_IMAGE_TAG:-}" ]]; then
    server_image="ghcr.io/oblivious/server:${OBLIVIOUS_IMAGE_TAG}"
    server_pull_policy="${server_pull_policy:-Always}"
  else
    server_image="oblivious-server:local"
    server_pull_policy="${server_pull_policy:-IfNotPresent}"
  fi
fi
server_pull_policy="${server_pull_policy:-Always}"

cd "$repo_root"

cleanup() {
  if [[ -n "$port_forward_pid" ]] && kill -0 "$port_forward_pid" >/dev/null 2>&1; then
    kill "$port_forward_pid" >/dev/null 2>&1 || true
    wait "$port_forward_pid" >/dev/null 2>&1 || true
  fi

  if [[ "$cleanup_namespace" == "true" ]]; then
    echo "[k8s-validate] OBLIVIOUS_K8S_CLEANUP=true; deleting namespace $namespace"
    kubectl delete namespace "$namespace" --ignore-not-found
  fi

  if [[ -n "$render_dir" && "$render_dir" == /tmp/oblivious-k8s-validate.* ]]; then
    rm -rf "$render_dir"
  fi
}
trap cleanup EXIT

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\/&|]/\\&/g'
}

file_matches() {
  local pattern="$1"
  local path="$2"

  if command -v rg >/dev/null 2>&1; then
    rg -q "$pattern" "$path"
    return
  fi

  grep -Eq "$pattern" "$path"
}

render_manifest() {
  local source="$1"
  local target="$2"
  local escaped_server_image
  local escaped_pull_policy
  local escaped_sslmode

  escaped_server_image=$(escape_sed_replacement "$server_image")
  escaped_pull_policy=$(escape_sed_replacement "$server_pull_policy")
  escaped_sslmode=$(escape_sed_replacement "$postgres_sslmode")

  sed \
    -e "s|ghcr.io/oblivious/server:\${OBLIVIOUS_IMAGE_TAG}|$escaped_server_image|g" \
    -e "s|imagePullPolicy: Always|imagePullPolicy: $escaped_pull_policy|g" \
    -e "s|POSTGRES_SSLMODE: \"require\"|POSTGRES_SSLMODE: \"$escaped_sslmode\"|g" \
    -e "s|sslmode=require|sslmode=$escaped_sslmode|g" \
    "$source" >"$target"
}

if ! command -v kubectl >/dev/null 2>&1; then
  echo "[k8s-validate] kubectl is required" >&2
  exit 127
fi

if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "[k8s-validate] kubernetes cluster is not reachable for the current kubectl context" >&2
  exit 2
fi

if [[ -z "$secret_file" ]]; then
  echo "[k8s-validate] OBLIVIOUS_K8S_SECRET_FILE is required" >&2
  echo "[k8s-validate] copy deploy/kubernetes/secret.example.yaml to an untracked path, fill real placeholders, and rerun with OBLIVIOUS_K8S_SECRET_FILE=/path/to/secret.yaml" >&2
  exit 2
fi

if [[ ! -f "$secret_file" ]]; then
  echo "[k8s-validate] secret file does not exist: $secret_file" >&2
  exit 2
fi

secret_realpath=$(cd "$(dirname "$secret_file")" && pwd -P)/$(basename "$secret_file")
example_realpath=$(cd deploy/kubernetes && pwd -P)/secret.example.yaml
if [[ "$secret_realpath" == "$example_realpath" ]]; then
  echo "[k8s-validate] refusing to use deploy/kubernetes/secret.example.yaml as runtime proof" >&2
  echo "[k8s-validate] copy it to an untracked path and replace every placeholder first" >&2
  exit 2
fi

if file_matches "REPLACE_ME|CHANGE_ME|change-me-in-production" "$secret_file"; then
  echo "[k8s-validate] secret file still contains placeholder values" >&2
  exit 2
fi

render_dir=$(mktemp -d /tmp/oblivious-k8s-validate.XXXXXX)
render_manifest deploy/kubernetes/configmap.yaml "$render_dir/configmap.yaml"
render_manifest "$secret_file" "$render_dir/secret.yaml"
render_manifest deploy/kubernetes/app-deployment.yaml "$render_dir/app-deployment.yaml"

echo "[k8s-validate] server image: $server_image"
echo "[k8s-validate] image pull policy: $server_pull_policy"
echo "[k8s-validate] postgres sslmode: $postgres_sslmode"

echo "[k8s-validate] applying namespace"
kubectl apply -f deploy/kubernetes/namespace.yaml

echo "[k8s-validate] applying network policy"
kubectl apply -f deploy/kubernetes/network-policy.yaml

echo "[k8s-validate] applying secret"
kubectl apply -f "$render_dir/secret.yaml"

echo "[k8s-validate] applying config and data services"
kubectl apply -f "$render_dir/configmap.yaml"
kubectl apply -f deploy/kubernetes/postgres.yaml
kubectl apply -f deploy/kubernetes/redis.yaml
kubectl apply -f deploy/kubernetes/qdrant.yaml
kubectl apply -f deploy/kubernetes/clickhouse.yaml

echo "[k8s-validate] waiting for data service rollouts"
kubectl -n "$namespace" rollout status deployment/oblivious-qdrant
kubectl -n "$namespace" rollout status deployment/oblivious-clickhouse
kubectl -n "$namespace" wait --for=condition=complete job/oblivious-clickhouse-migrate --timeout="${OBLIVIOUS_K8S_CLICKHOUSE_MIGRATION_TIMEOUT:-300s}"

echo "[k8s-validate] applying application workloads"
kubectl apply -f "$render_dir/app-deployment.yaml"
kubectl apply -f deploy/kubernetes/app-service.yaml
kubectl apply -f deploy/kubernetes/hpa.yaml
kubectl apply -f deploy/kubernetes/ingress.yaml

echo "[k8s-validate] waiting for rollouts"
kubectl -n "$namespace" rollout status deployment/oblivious-server

echo "[k8s-validate] port-forwarding oblivious-server on localhost:$port"
kubectl -n "$namespace" port-forward service/oblivious-server "$port:http" >/tmp/oblivious-k8s-port-forward.log 2>&1 &
port_forward_pid=$!

for attempt in $(seq 1 "${OBLIVIOUS_K8S_PORT_FORWARD_ATTEMPTS:-20}"); do
  if kill -0 "$port_forward_pid" >/dev/null 2>&1; then
    if command -v curl >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
      break
    fi
  else
    echo "[k8s-validate] port-forward exited unexpectedly" >&2
    sed 's/^/[k8s-validate] port-forward: /' /tmp/oblivious-k8s-port-forward.log >&2 || true
    exit 3
  fi

  if [[ "$attempt" -eq "${OBLIVIOUS_K8S_PORT_FORWARD_ATTEMPTS:-20}" ]]; then
    echo "[k8s-validate] port-forward did not expose /healthz in time" >&2
    sed 's/^/[k8s-validate] port-forward: /' /tmp/oblivious-k8s-port-forward.log >&2 || true
    exit 3
  fi

  sleep "${OBLIVIOUS_K8S_PORT_FORWARD_SLEEP_SECONDS:-1}"
done

echo "[k8s-validate] running smoke against Kubernetes service"
BASE_URL="http://127.0.0.1:$port" bash scripts/deploy-smoke.sh

echo "[k8s-validate] kubernetes validation ok"

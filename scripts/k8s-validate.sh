#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
namespace="${OBLIVIOUS_K8S_NAMESPACE:-oblivious}"
port="${OBLIVIOUS_K8S_PORT:-18080}"
cleanup_namespace="${OBLIVIOUS_K8S_CLEANUP:-false}"
secret_file="${OBLIVIOUS_K8S_SECRET_FILE:-}"
port_forward_pid=""

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
}
trap cleanup EXIT

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

if rg -q "REPLACE_ME|change-me-in-production" "$secret_file"; then
  echo "[k8s-validate] secret file still contains placeholder values" >&2
  exit 2
fi

echo "[k8s-validate] applying namespace"
kubectl apply -f deploy/kubernetes/namespace.yaml

echo "[k8s-validate] applying secret"
kubectl apply -f "$secret_file"

echo "[k8s-validate] applying config and data services"
kubectl apply -f deploy/kubernetes/configmap.yaml
kubectl apply -f deploy/kubernetes/postgres.yaml
kubectl apply -f deploy/kubernetes/redis.yaml

echo "[k8s-validate] applying application workloads"
kubectl apply -f deploy/kubernetes/server.yaml
kubectl apply -f deploy/kubernetes/web.yaml

echo "[k8s-validate] waiting for rollouts"
kubectl -n "$namespace" rollout status deployment/oblivious-server
kubectl -n "$namespace" rollout status deployment/oblivious-web

echo "[k8s-validate] port-forwarding oblivious-server on localhost:$port"
kubectl -n "$namespace" port-forward service/oblivious-server "$port:8080" >/tmp/oblivious-k8s-port-forward.log 2>&1 &
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

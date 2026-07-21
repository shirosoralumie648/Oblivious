#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
python_bin=${PYTHON:-python3}
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

fail() {
  echo "[readiness-deployment-contract] $*" >&2
  exit 1
}

check_assets() {
  local root="$1"
  "$python_bin" - "$root" <<'PY'
import pathlib
import sys
import yaml

root = pathlib.Path(sys.argv[1])
dockerfile = (root / "Dockerfile.server").read_text(encoding="utf-8")
required_docker = [
    "oblivious-release-contract", "contract.v1.json", "contract.schema.json",
    "io.oblivious.release-contract-digest", "io.oblivious.deployment-profile=\"monolith\"",
    "io.oblivious.readiness-audit-path", "OBLIVIOUS_DEPLOYMENT_PROFILE=monolith",
    "OBLIVIOUS_READINESS_AUDIT_PATH=/var/lib/oblivious/audit/readiness.json", "USER oblivious",
]
for needle in required_docker:
    if needle not in dockerfile:
        raise SystemExit(f"Dockerfile.server missing {needle}")

compose = yaml.safe_load((root / "docker-compose.yml").read_text(encoding="utf-8"))
service = (compose.get("services") or {}).get("oblivious-server") or {}
environment = service.get("environment") or []
if "OBLIVIOUS_DEPLOYMENT_PROFILE=monolith" not in environment:
    raise SystemExit("Compose must explicitly select monolith")
if "OBLIVIOUS_READINESS_AUDIT_PATH=/var/lib/oblivious/audit/readiness.json" not in environment:
    raise SystemExit("Compose must set the output-only readiness audit path")
mounts = service.get("volumes") or []
if len(mounts) != 1 or "/var/lib/oblivious/audit" not in str(mounts[0]):
    raise SystemExit("Compose must mount exactly one output-only audit volume")
if any(any(token in str(mount).lower() for token in ("snapshot", "report.json", "readiness.json:")) for mount in mounts):
    raise SystemExit("Compose must not mount readiness snapshots or reports as input")

docs = list(yaml.safe_load_all((root / "deploy/kubernetes/app-deployment.yaml").read_text(encoding="utf-8")))
deployment = next(doc for doc in docs if doc.get("kind") == "Deployment")
container = (deployment.get("spec", {}).get("template", {}).get("spec", {}).get("containers") or [])[0]
env = {entry.get("name"): entry.get("value") for entry in container.get("env") or []}
if env.get("OBLIVIOUS_DEPLOYMENT_PROFILE") != "monolith":
    raise SystemExit("Kubernetes must explicitly select monolith")
if env.get("OBLIVIOUS_READINESS_AUDIT_PATH") != "/var/lib/oblivious/audit/readiness.json":
    raise SystemExit("Kubernetes must set the output-only readiness audit path")
if container.get("startupProbe", {}).get("httpGet", {}).get("path") != "/livez":
    raise SystemExit("Kubernetes startup probe must use /livez")
if container.get("livenessProbe", {}).get("httpGet", {}).get("path") != "/livez":
    raise SystemExit("Kubernetes liveness probe must use /livez")
if container.get("readinessProbe", {}).get("httpGet", {}).get("path") != "/readyz":
    raise SystemExit("Kubernetes readiness probe must use /readyz")
mounts = container.get("volumeMounts") or []
if len(mounts) != 1 or mounts[0].get("mountPath") != "/var/lib/oblivious/audit":
    raise SystemExit("Kubernetes must mount exactly one audit output path")
volumes = deployment.get("spec", {}).get("template", {}).get("spec", {}).get("volumes") or []
audit = next((volume for volume in volumes if volume.get("name") == "readiness-audit"), None)
if not audit or "emptyDir" not in audit:
    raise SystemExit("Kubernetes audit storage must be emptyDir")

configmap = yaml.safe_load((root / "deploy/kubernetes/configmap.yaml").read_text(encoding="utf-8"))
broker = (configmap.get("data") or {}).get("KAFKA_BROKERS", "")
try:
    broker_host, broker_port_text = broker.rsplit(":", 1)
    broker_port = int(broker_port_text)
except (TypeError, ValueError):
    raise SystemExit("Kubernetes KAFKA_BROKERS must be one host:port address")

kafka_docs = [
    doc for doc in yaml.safe_load_all(
        (root / "deploy/kubernetes/kafka.yaml").read_text(encoding="utf-8")
    )
    if doc
]
broker_parts = broker_host.split(".")
if len(broker_parts) != 5 or broker_parts[2:] != ["svc", "cluster", "local"]:
    raise SystemExit("Kubernetes Kafka broker must use exact in-cluster Service DNS")
service_name, service_namespace = broker_parts[:2]
service = next(
    (
        doc for doc in kafka_docs
        if doc.get("kind") == "Service"
        and doc.get("metadata", {}).get("name") == service_name
        and doc.get("metadata", {}).get("namespace") == service_namespace
    ),
    None,
)
if service is None:
    raise SystemExit("Kubernetes KAFKA_BROKERS must resolve to the canonical Kafka Service")
client_port = next(
    (port for port in service.get("spec", {}).get("ports") or [] if port.get("name") == "client"),
    None,
)
if client_port is None or client_port.get("port") != broker_port:
    raise SystemExit("Kubernetes KAFKA_BROKERS port must match the Kafka client Service port")

kafka = next((doc for doc in kafka_docs if doc.get("kind") == "StatefulSet"), None)
if kafka is None or kafka.get("metadata", {}).get("name") != "kafka":
    raise SystemExit("canonical Kafka StatefulSet must be named kafka")
if kafka.get("metadata", {}).get("namespace") != service_namespace:
    raise SystemExit("Kafka Service and StatefulSet namespaces must match")
kafka_spec = kafka.get("spec", {})
selector = kafka_spec.get("selector", {}).get("matchLabels") or {}
pod_labels = kafka_spec.get("template", {}).get("metadata", {}).get("labels") or {}
if not selector or service.get("spec", {}).get("selector") != selector or pod_labels != selector:
    raise SystemExit("Kafka Service, StatefulSet selector, and pod labels must exact-match")
if kafka_spec.get("replicas") != 3:
    raise SystemExit("canonical Kafka StatefulSet must retain three brokers")
containers = kafka_spec.get("template", {}).get("spec", {}).get("containers") or []
kafka_container = next((item for item in containers if item.get("name") == "kafka"), None)
container_port = next(
    (
        port for port in (kafka_container or {}).get("ports") or []
        if port.get("name") == client_port.get("targetPort")
    ),
    None,
)
if container_port is None or container_port.get("containerPort") != broker_port:
    raise SystemExit("Kafka Service targetPort must match the StatefulSet client container port")

runner = (root / "scripts/k8s-validate.sh").read_text(encoding="utf-8")
ordered_runner_fragments = [
    "kubectl apply -f deploy/kubernetes/kafka.yaml",
    'kubectl -n "$namespace" rollout status statefulset/kafka --timeout="${OBLIVIOUS_K8S_KAFKA_TIMEOUT:-300s}"',
    'kubectl apply -f "$render_dir/app-deployment.yaml"',
    'kubectl -n "$namespace" rollout status deployment/oblivious-server',
    'curl -fsS "http://127.0.0.1:$port/readyz"',
    'BASE_URL="http://127.0.0.1:$port" bash scripts/deploy-smoke.sh',
]
positions = []
for fragment in ordered_runner_fragments:
    if runner.count(fragment) != 1:
        raise SystemExit(f"Kubernetes runner must contain exactly one ordered command: {fragment}")
    positions.append(runner.index(fragment))
if positions != sorted(positions) or len(set(positions)) != len(positions):
    raise SystemExit("Kubernetes runner must apply/wait Kafka before app rollout and /readyz")

inventory = (root / "scripts/verify_deployment_operations_contract.py").read_text(encoding="utf-8")
if '        "deploy/kubernetes/server.yaml",' in inventory:
    raise SystemExit("server.yaml must not enter release validation inventory")
if '"deploy/kubernetes/app-deployment.yaml"' not in inventory:
    raise SystemExit("app-deployment.yaml must remain the canonical workload")
PY
}

check_assets "$repo_root"
if [[ "${OBLIVIOUS_READINESS_DEPLOYMENT_ASSETS_ONLY:-false}" == "true" ]]; then
  echo "[readiness-deployment-contract] readiness deployment assets passed"
  exit 0
fi
python_output=$($python_bin "$repo_root/scripts/verify_deployment_operations_contract.py" "$repo_root" 2>&1) || {
  printf '%s\n' "$python_output" >&2
  fail "existing deployment operations contract failed"
}

copy_fixture() {
  local destination="$1"
  mkdir -p "$destination/deploy/kubernetes" "$destination/scripts"
  cp "$repo_root/Dockerfile.server" "$destination/Dockerfile.server"
  cp "$repo_root/docker-compose.yml" "$destination/docker-compose.yml"
  cp "$repo_root/deploy/kubernetes/configmap.yaml" "$destination/deploy/kubernetes/configmap.yaml"
  cp "$repo_root/deploy/kubernetes/kafka.yaml" "$destination/deploy/kubernetes/kafka.yaml"
  cp "$repo_root/deploy/kubernetes/app-deployment.yaml" "$destination/deploy/kubernetes/app-deployment.yaml"
  cp "$repo_root/scripts/k8s-validate.sh" "$destination/scripts/k8s-validate.sh"
  cp "$repo_root/scripts/verify_deployment_operations_contract.py" "$destination/scripts/verify_deployment_operations_contract.py"
}

expect_mutation_failure() {
  local label="$1"; shift
  local fixture="$tmpdir/$label"
  copy_fixture "$fixture"
  "$@" "$fixture"
  if check_assets "$fixture" >/dev/null 2>&1; then
    fail "mutation unexpectedly passed: $label"
  fi
  echo "[readiness-deployment-contract] rejected $label"
}

mutate_text() {
  local path="$1"; local from="$2"; local to="$3"
  "$python_bin" - "$path" "$from" "$to" <<'PY'
import pathlib
import sys
path = pathlib.Path(sys.argv[1])
old = sys.argv[2]
new = sys.argv[3]
content = path.read_text(encoding="utf-8")
if old not in content:
    raise SystemExit(f"mutation source not found: {old}")
path.write_text(content.replace(old, new, 1), encoding="utf-8")
PY
}

fixture="$tmpdir/docker-missing-inspector"
rm -rf "$fixture"
copy_fixture "$fixture"
mutate_text "$fixture/Dockerfile.server" "io.oblivious.release-contract-digest" "io.oblivious.missing-digest"
if check_assets "$fixture" >/dev/null 2>&1; then fail "docker inspector mutation unexpectedly passed"; fi
echo "[readiness-deployment-contract] rejected docker-missing-inspector"

fixture="$tmpdir/compose-profile"
rm -rf "$fixture"
copy_fixture "$fixture"
mutate_text "$fixture/docker-compose.yml" "OBLIVIOUS_DEPLOYMENT_PROFILE=monolith" "OBLIVIOUS_DEPLOYMENT_PROFILE=microservices"
if check_assets "$fixture" >/dev/null 2>&1; then fail "compose profile mutation unexpectedly passed"; fi
echo "[readiness-deployment-contract] rejected compose-profile"

fixture="$tmpdir/kubernetes-startup-probe"
rm -rf "$fixture"
copy_fixture "$fixture"
mutate_text "$fixture/deploy/kubernetes/app-deployment.yaml" "path: /livez" "path: /readyz"
if check_assets "$fixture" >/dev/null 2>&1; then fail "startup probe mutation unexpectedly passed"; fi
echo "[readiness-deployment-contract] rejected kubernetes-startup-probe"

echo "[readiness-deployment-contract] readiness deployment contract passed"

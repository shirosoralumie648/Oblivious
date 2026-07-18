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

inventory = (root / "scripts/verify_deployment_operations_contract.py").read_text(encoding="utf-8")
if '        "deploy/kubernetes/server.yaml",' in inventory:
    raise SystemExit("server.yaml must not enter release validation inventory")
if '"deploy/kubernetes/app-deployment.yaml"' not in inventory:
    raise SystemExit("app-deployment.yaml must remain the canonical workload")
PY
}

check_assets "$repo_root"
python_output=$($python_bin "$repo_root/scripts/verify_deployment_operations_contract.py" "$repo_root" 2>&1) || {
  printf '%s\n' "$python_output" >&2
  fail "existing deployment operations contract failed"
}

copy_fixture() {
  local destination="$1"
  mkdir -p "$destination/deploy/kubernetes" "$destination/scripts"
  cp "$repo_root/Dockerfile.server" "$destination/Dockerfile.server"
  cp "$repo_root/docker-compose.yml" "$destination/docker-compose.yml"
  cp "$repo_root/deploy/kubernetes/app-deployment.yaml" "$destination/deploy/kubernetes/app-deployment.yaml"
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

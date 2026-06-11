#!/usr/bin/env bash
set -euo pipefail

# Static validation for the infra-extras reference manifests
# (deploy/kubernetes/pgbouncer.yaml, minio.yaml, kafka.yaml) against the
# fusion spec part3 §9.1/§9.3 shapes.

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

fail() {
  echo "[verify-infra-manifests] FAIL: $1" >&2
  exit 1
}

command -v python3 >/dev/null 2>&1 || fail "python3 is required"

python3 - <<'PY' || exit 1
import sys
import yaml

def load(path):
    with open(path) as fh:
        docs = [d for d in yaml.safe_load_all(fh) if d]
    if not docs:
        raise SystemExit(f"FAIL: {path} contains no documents")
    return docs

def by_kind(docs, kind):
    return [d for d in docs if d.get("kind") == kind]

problems = []

# --- pgbouncer.yaml -------------------------------------------------------
docs = load("deploy/kubernetes/pgbouncer.yaml")
kinds = sorted(d["kind"] for d in docs)
if kinds != ["ConfigMap", "Deployment", "Service"]:
    problems.append(f"pgbouncer.yaml kinds = {kinds}, want ConfigMap+Deployment+Service")
cm = by_kind(docs, "ConfigMap")[0]
if cm["data"].get("MAX_CLIENT_CONN") != "500":
    problems.append("pgbouncer MAX_CLIENT_CONN != 500")
if cm["data"].get("DB_HOST") != "oblivious-postgres":
    problems.append("pgbouncer DB_HOST does not point at oblivious-postgres")
dep = by_kind(docs, "Deployment")[0]
if dep["spec"].get("replicas") != 1:
    problems.append("pgbouncer replicas != 1")

# --- minio.yaml -----------------------------------------------------------
docs = load("deploy/kubernetes/minio.yaml")
sts = by_kind(docs, "StatefulSet")
if len(sts) != 1:
    problems.append("minio.yaml must contain exactly one StatefulSet")
else:
    spec = sts[0]["spec"]
    if spec.get("replicas") != 4:
        problems.append(f"minio replicas = {spec.get('replicas')}, want 4 (distributed)")
    args = " ".join(spec["template"]["spec"]["containers"][0].get("args", []))
    if "minio-{0...3}" not in args:
        problems.append("minio args missing distributed pool notation minio-{0...3}")
    if not spec.get("volumeClaimTemplates"):
        problems.append("minio StatefulSet missing volumeClaimTemplates")
services = by_kind(docs, "Service")
if not any(s["spec"].get("clusterIP") == "None" for s in services):
    problems.append("minio.yaml missing headless Service")

# --- kafka.yaml -----------------------------------------------------------
docs = load("deploy/kubernetes/kafka.yaml")
sts = by_kind(docs, "StatefulSet")
if len(sts) != 1:
    problems.append("kafka.yaml must contain exactly one StatefulSet")
else:
    spec = sts[0]["spec"]
    if spec.get("replicas") != 3:
        problems.append(f"kafka replicas = {spec.get('replicas')}, want 3 brokers")
    env = {e["name"]: e.get("value") for e in spec["template"]["spec"]["containers"][0].get("env", [])}
    if env.get("KAFKA_CFG_DEFAULT_REPLICATION_FACTOR") != "3":
        problems.append("kafka default replication factor != 3")
    if env.get("KAFKA_CFG_OFFSETS_TOPIC_REPLICATION_FACTOR") != "3":
        problems.append("kafka offsets topic replication factor != 3")
    voters = env.get("KAFKA_CFG_CONTROLLER_QUORUM_VOTERS", "")
    if voters.count("@") != 3:
        problems.append("kafka controller quorum must list 3 voters")
services = by_kind(docs, "Service")
if not any(s["spec"].get("clusterIP") == "None" for s in services):
    problems.append("kafka.yaml missing headless Service")

if problems:
    for p in problems:
        print(f"FAIL: {p}", file=sys.stderr)
    raise SystemExit(1)

print("manifest content checks passed")
PY

echo "[verify-infra-manifests] validating docker compose syntax (default profile)"
docker compose config -q || fail "docker compose config failed for default profile"

echo "[verify-infra-manifests] validating docker compose syntax (infra-extras profile)"
docker compose --profile infra-extras config -q || fail "docker compose config failed for infra-extras profile"

if ! docker compose config --services | grep -q "^pgbouncer$"; then
  echo "[verify-infra-manifests] default profile excludes infra extras: OK"
else
  fail "pgbouncer must not be part of the default compose profile"
fi

docker compose --profile infra-extras config --services | grep -q "^pgbouncer$" || fail "pgbouncer missing from infra-extras profile"
docker compose --profile infra-extras config --services | grep -q "^minio$" || fail "minio missing from infra-extras profile"
docker compose --profile infra-extras config --services | grep -q "^kafka$" || fail "kafka missing from infra-extras profile"

echo "[verify-infra-manifests] PASS"

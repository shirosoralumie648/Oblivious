#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
runner="$repo_root/scripts/run-target-release-evidence.sh"
initializer="$repo_root/scripts/init-target-release-evidence-workdir.sh"
tmpdir=$(mktemp -d)
workdir="$tmpdir/release"
inside_repo="$repo_root/.tmp/target-release-runner-inside"
sha=$(printf '%064d' 0 | tr '0' 'a')

cleanup() {
  rm -rf "$tmpdir" "$inside_repo"
}
trap cleanup EXIT

fail() {
  echo "[run-target-release-evidence-fixtures] $*" >&2
  exit 1
}

bash "$initializer" --workdir "$workdir" >/dev/null

cat >"$workdir/.env" <<EOF
export OBLIVIOUS_TARGET_RELEASE_WORKDIR="$workdir"
export OBLIVIOUS_TARGET_EVIDENCE_RUN_ID="release-20260710-120000"
export OBLIVIOUS_TARGET_ENVIRONMENT_NAME="production-us-east"
export OBLIVIOUS_TARGET_ENVIRONMENT_CLASS="production"
export OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL="https://prod.oblivious.internal"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT="2026-07-10T12:00:00Z"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT="2026-07-10T12:30:00Z"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE="0.999"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW="2026-07-10T11:00:00Z/2026-07-10T12:00:00Z"
export OBLIVIOUS_TARGET_EVIDENCE_FROM="2026-07-10T11:00:00Z"
export OBLIVIOUS_TARGET_EVIDENCE_TO="2026-07-10T12:00:00Z"
export OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN="fixture-release-bearer"
export OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_URI="https://prod.oblivious.internal/internal/release/clickhouse-request-log-platform-proof.json"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_URI="https://prod.oblivious.internal/internal/release/strict-verifier.json"
export OBLIVIOUS_TARGET_DEPLOYMENT_URI="https://prod.oblivious.internal/internal/release/deployment-proof.json"
export OBLIVIOUS_TARGET_KUBERNETES_URI="https://prod.oblivious.internal/internal/release/kubernetes-proof.json"
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI="https://prod.oblivious.internal/internal/release/stripe-provider-live-rail.json"
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI="https://prod.oblivious.internal/internal/release/alipay-provider-live-rail.json"
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI="https://prod.oblivious.internal/internal/release/wechatpay-provider-live-rail.json"
export OBLIVIOUS_TARGET_GRPC_SMOKE_URI="https://prod.oblivious.internal/internal/release/grpc-smoke.json"
export OBLIVIOUS_TARGET_SECRET_AUDIT_URI="https://prod.oblivious.internal/internal/release/secret-audit.json"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI="https://prod.oblivious.internal/internal/release/workflow-telemetry.json"
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI="https://prod.oblivious.internal/internal/release/request-log-observability.json"
export OBLIVIOUS_TARGET_RAG_INDEXING_URI="https://prod.oblivious.internal/internal/release/rag-indexing-proof.json"
export OBLIVIOUS_TARGET_RELAY_REALTIME_URI="https://prod.oblivious.internal/internal/release/relay-realtime-proof.json"
export OBLIVIOUS_TARGET_RELAY_BATCH_URI="https://prod.oblivious.internal/internal/release/relay-batch-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI="https://prod.oblivious.internal/internal/release/marketplace-payout-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI="https://prod.oblivious.internal/internal/release/marketplace-governance-proof.json"
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI="https://prod.oblivious.internal/internal/release/provider-runtime-config-proof.json"
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI="https://prod.oblivious.internal/internal/release/microservice-database-proof.json"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256="$sha"
export OBLIVIOUS_TARGET_DEPLOYMENT_SHA256="$sha"
export OBLIVIOUS_TARGET_KUBERNETES_SHA256="$sha"
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256="$sha"
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_SHA256="$sha"
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_SHA256="$sha"
export OBLIVIOUS_TARGET_GRPC_SMOKE_SHA256="$sha"
export OBLIVIOUS_TARGET_SECRET_AUDIT_SHA256="$sha"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SHA256="$sha"
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_SHA256="$sha"
export OBLIVIOUS_TARGET_RAG_INDEXING_SHA256="$sha"
export OBLIVIOUS_TARGET_RELAY_REALTIME_SHA256="$sha"
export OBLIVIOUS_TARGET_RELAY_BATCH_SHA256="$sha"
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_SHA256="$sha"
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_SHA256="$sha"
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_SHA256="$sha"
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_SHA256="$sha"
export OBLIVIOUS_TARGET_EVIDENCE_FILE="$workdir/target-release-evidence.json"
export OBLIVIOUS_TARGET_ARTIFACT_DIR="$workdir/artifacts"
export TEST_DATABASE_URL="postgres://oblivious:fixture@prod-db.oblivious.internal:5432/oblivious?sslmode=require"
export OBLIVIOUS_K8S_SECRET_FILE="$workdir/secret.yaml"
EOF

cat >"$workdir/secret.yaml" <<'YAML'
apiVersion: v1
kind: Secret
metadata:
  name: oblivious-runtime
stringData:
  SESSION_SECRET: fixture-session-secret-not-for-production
YAML

raw_files=(
  strict-verifier.json
  deployment-proof.json
  kubernetes-proof.json
  stripe-provider-live-rail.json
  alipay-provider-live-rail.json
  wechatpay-provider-live-rail.json
  grpc-smoke.json
  secret-audit.json
  workflow-telemetry.json
  clickhouse-request-log-platform-proof.json
  usage-request-log-coverage.json
  latency-slo-proof.json
  rag-indexing-proof.json
  relay-realtime-proof.json
  relay-batch-proof.json
  marketplace-payout-proof.json
  marketplace-governance-proof.json
  provider-runtime-config-proof.json
  microservice-database-proof.json
)
for name in "${raw_files[@]}"; do
  printf '{}\n' >"$workdir/raw/$name"
done

prepare_output="$tmpdir/prepare-dry-run.out"
bash "$runner" --workdir "$workdir" --prepare-only --dry-run >"$prepare_output"
for expected in \
  "DRY-RUN assemble target evidence manifest" \
  "DRY-RUN collect target artifact bodies" \
  "DRY-RUN refresh target release digests" \
  "DRY-RUN verify target evidence bundle" \
  "DRY-RUN target evidence preflight" \
  "prepare-only completed; final commercial verifier was not run"; do
  grep -Fq -- "$expected" "$prepare_output" || {
    cat "$prepare_output" >&2
    fail "prepare-only dry run did not include: $expected"
  }
done
if grep -Fq -- "DRY-RUN final no-skip commercial verifier" "$prepare_output"; then
  cat "$prepare_output" >&2
  fail "prepare-only dry run unexpectedly included final verifier"
fi
echo "[run-target-release-evidence-fixtures] prepare-only dry run is ordered"

full_output="$tmpdir/full-dry-run.out"
bash "$runner" --workdir "$workdir" --dry-run >"$full_output"
grep -Fq -- "DRY-RUN final no-skip commercial verifier" "$full_output" || {
  cat "$full_output" >&2
  fail "full dry run did not include final no-skip verifier"
}
grep -Fq -- "final target release evidence workflow passed" "$full_output" || {
  cat "$full_output" >&2
  fail "full dry run did not reach workflow completion"
}
echo "[run-target-release-evidence-fixtures] full dry run includes final verifier"

rm "$workdir/raw/rag-indexing-proof.json"
if bash "$runner" --workdir "$workdir" --prepare-only --dry-run >"$tmpdir/missing-raw.out" 2>&1; then
  cat "$tmpdir/missing-raw.out" >&2
  fail "runner unexpectedly accepted missing raw proof"
fi
grep -Fq -- "required raw proof file is missing" "$tmpdir/missing-raw.out" || {
  cat "$tmpdir/missing-raw.out" >&2
  fail "missing raw proof did not fail with expected message"
}
printf '{}\n' >"$workdir/raw/rag-indexing-proof.json"
echo "[run-target-release-evidence-fixtures] missing raw proof is rejected"

cp "$workdir/.env" "$workdir/.env.valid"
sed 's#https://prod\.oblivious\.internal#https://target.example.com#g' "$workdir/.env.valid" >"$workdir/.env"
if bash "$runner" --workdir "$workdir" --prepare-only --dry-run >"$tmpdir/placeholder.out" 2>&1; then
  cat "$tmpdir/placeholder.out" >&2
  fail "runner unexpectedly accepted placeholder env"
fi
grep -Fq -- "env file still contains placeholder values" "$tmpdir/placeholder.out" || {
  cat "$tmpdir/placeholder.out" >&2
  fail "placeholder env did not fail with expected message"
}
mv "$workdir/.env.valid" "$workdir/.env"
echo "[run-target-release-evidence-fixtures] placeholder env is rejected"

mkdir -p "$inside_repo"
if bash "$runner" --workdir "$inside_repo" --prepare-only --dry-run >"$tmpdir/inside-repo.out" 2>&1; then
  cat "$tmpdir/inside-repo.out" >&2
  fail "runner unexpectedly accepted repository-internal workdir"
fi
grep -Fq -- "workdir must be outside the repository" "$tmpdir/inside-repo.out" || {
  cat "$tmpdir/inside-repo.out" >&2
  fail "repository-internal workdir did not fail with expected message"
}
echo "[run-target-release-evidence-fixtures] repository-internal workdir is rejected"

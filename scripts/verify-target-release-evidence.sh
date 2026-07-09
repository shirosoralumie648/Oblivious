#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/verify_target_release_evidence.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: OBLIVIOUS_TARGET_EVIDENCE_FILE=/path/outside/git/target-release-evidence.json bash scripts/verify-target-release-evidence.sh
       bash scripts/verify-target-release-evidence.sh /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --print-template > /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --manifest-only /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --allow-file-collection-source /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --allow-local-collection-source /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --allow-non-production-target /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --allow-disabled-commercial-lifecycle /path/outside/git/target-release-evidence.json

Validates target/live release evidence that cannot be proven by repository-local tests.
The evidence file must be JSON, must not contain secrets, and must refer to the current git HEAD.

Required strict verifier command:
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh

Required target evidence sections:
  requestLogObservability, ragIndexing, relayRealtime, relayBatch, marketplacePayouts, marketplaceGovernance, providerRuntimeConfig, microserviceDatabases

Optional:
  OBLIVIOUS_TARGET_ARTIFACT_DIR=/path/outside/git/downloaded-artifacts
    Validate locally downloaded artifact bodies named <artifact-id>.json against the manifest SHA-256,
    lineage metadata, required artifact body proof fields, and canonical target release digest fields.
    This is required for default final target evidence validation. Use --manifest-only only for
    non-final standalone manifest linting.
    Run bash scripts/compute-target-release-digests.sh --manifest ... --artifact-dir ... --write
    after artifact collection to refresh targetEvidenceSha256 and artifactBundleSha256 before validation.
  --manifest-only
    Validate only manifest-level semantics without downloaded artifact bodies. This is never final
    commercial readiness proof.
  OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true
    Allow validating an evidence file for a different commit. This is never final readiness proof.
  --allow-file-collection-source
    Allow artifact bodies collected from local proof files. This is only for collector fixture/manual fallback
    validation and is never final commercial readiness proof.
  --allow-local-collection-source
    Allow artifact body collectionSource URLs to point at localhost/loopback. This is only for local HTTP
    fixture validation and is never final commercial readiness proof.
  --allow-non-production-target
    Allow staging/preproduction target evidence. This is only for RC validation and is never final commercial readiness proof.
  --allow-disabled-commercial-lifecycle
    Allow Relay Realtime/Batch disabled lifecycle proofs. This is only for RC validation when those routes are
    explicitly excluded from the commercial surface and is never full commercial release proof.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

current_commit=$(git -C "$repo_root" rev-parse HEAD)

if [[ "${1:-}" == "--print-template" ]]; then
  "$python_bin" "$impl" --current-commit "$current_commit" --print-template
  exit 0
fi

allow_file_collection_source=()
allow_local_collection_source=()
allow_non_production_target=()
allow_disabled_commercial_lifecycle=()
manifest_only=false
while [[ "${1:-}" == "--manifest-only" || "${1:-}" == "--allow-file-collection-source" || "${1:-}" == "--allow-local-collection-source" || "${1:-}" == "--allow-non-production-target" || "${1:-}" == "--allow-disabled-commercial-lifecycle" ]]; do
  if [[ "${1:-}" == "--allow-file-collection-source" ]]; then
    allow_file_collection_source+=(--allow-file-collection-source)
  elif [[ "${1:-}" == "--allow-local-collection-source" ]]; then
    allow_local_collection_source+=(--allow-local-collection-source)
  elif [[ "${1:-}" == "--allow-non-production-target" ]]; then
    allow_non_production_target+=(--allow-non-production-target)
  elif [[ "${1:-}" == "--allow-disabled-commercial-lifecycle" ]]; then
    allow_disabled_commercial_lifecycle+=(--allow-disabled-commercial-lifecycle)
  elif [[ "${1:-}" == "--manifest-only" ]]; then
    manifest_only=true
  fi
  shift
done

evidence_file="${1:-${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}}"
if [[ -z "$evidence_file" ]]; then
  echo "[target-release-evidence] OBLIVIOUS_TARGET_EVIDENCE_FILE or file argument is required" >&2
  usage >&2
  exit 1
fi
if [[ ! -f "$evidence_file" ]]; then
  echo "[target-release-evidence] evidence file not found: $evidence_file" >&2
  exit 1
fi

allow_commit_mismatch="${OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH:-false}"
allow_flag=()
if [[ "$allow_commit_mismatch" == "true" ]]; then
  allow_flag+=(--allow-commit-mismatch)
fi
artifact_dir="${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}"
artifact_dir_flag=()
if [[ -n "$artifact_dir" ]]; then
  artifact_dir_flag+=(--artifact-dir "$artifact_dir")
elif [[ "$manifest_only" != "true" ]]; then
  echo "[target-release-evidence] OBLIVIOUS_TARGET_ARTIFACT_DIR is required for final target evidence validation; use --manifest-only only for non-final standalone manifest linting" >&2
  exit 1
fi

"$python_bin" "$impl" \
  --current-commit "$current_commit" \
  "${allow_flag[@]}" \
  "${allow_file_collection_source[@]}" \
  "${allow_local_collection_source[@]}" \
  "${allow_non_production_target[@]}" \
  "${allow_disabled_commercial_lifecycle[@]}" \
  "${artifact_dir_flag[@]}" \
  "$evidence_file"

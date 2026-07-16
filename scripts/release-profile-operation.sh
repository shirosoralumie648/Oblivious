#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [[ "$#" -ne 2 ]]; then
  echo "profile_required: usage: release-profile-operation.sh <profileId> <migrate|deploy|rollback>" >&2
  exit 64
fi

profile_id="$1"
operation="$2"

case "$profile_id" in
  monolith) ;;
  microservices | dual | split)
    echo "profile_excluded: deployment profile '$profile_id' is not committed" >&2
    exit 65
    ;;
  *)
    echo "profile_unknown: deployment profile '$profile_id' is not authored" >&2
    exit 66
    ;;
esac

cd "$repo_root"

case "$operation" in
  migrate)
    exec docker compose run --rm --no-deps oblivious-server /usr/local/bin/oblivious-migrate
    ;;
  deploy)
    exec bash "$repo_root/scripts/deploy-validate.sh"
    ;;
  rollback)
    echo "operation_unproven: monolith rollback remains fail-closed until Phase 38 operational proof" >&2
    exit 78
    ;;
  *)
    echo "operation_unknown: expected migrate, deploy, or rollback" >&2
    exit 67
    ;;
esac

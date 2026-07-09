#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
hpa_file="$repo_root/deploy/kubernetes/hpa.yaml"
server_deployment_file="$repo_root/deploy/kubernetes/app-deployment.yaml"

file_matches() {
  local pattern="$1"
  local file="$2"

  if command -v rg >/dev/null 2>&1; then
    rg -q -- "$pattern" "$file"
    return
  fi

  grep -Eq -- "$pattern" "$file"
}

require_pattern() {
  local file="$1"
  local pattern="$2"
  local message="$3"
  if ! file_matches "$pattern" "$file"; then
    echo "[k8s-recovery-policy] $message" >&2
    exit 1
  fi
}

require_probe_threshold() {
  local probe="$1"
  if ! awk -v probe="$probe" '
    $0 ~ "^[[:space:]]*" probe ":" {
      in_probe = 1
      probe_indent = match($0, /[^ ]/) - 1
      next
    }
    in_probe && $0 ~ /[^ ]/ {
      indent = match($0, /[^ ]/) - 1
      if (indent <= probe_indent && $1 ~ /:$/) {
        in_probe = 0
      }
    }
    in_probe && $1 == "failureThreshold:" && $2 == "3" { found = 1 }
    END { exit(found ? 0 : 1) }
  ' "$server_deployment_file"; then
    echo "[k8s-recovery-policy] server deployment must define ${probe} failureThreshold 3" >&2
    exit 1
  fi
}

require_hpa_block_pattern() {
  local block="$1"
  local pattern="$2"
  local message="$3"
  if ! awk -v block="$block" -v pattern="$pattern" '
    $0 ~ "^[[:space:]]*" block ":" {
      in_block = 1
      block_indent = match($0, /[^ ]/) - 1
      next
    }
    in_block && $0 ~ /[^ ]/ {
      indent = match($0, /[^ ]/) - 1
      if (indent <= block_indent && $1 ~ /:$/) {
        in_block = 0
      }
    }
    in_block && $0 ~ pattern { found = 1 }
    END { exit(found ? 0 : 1) }
  ' "$hpa_file"; then
    echo "[k8s-recovery-policy] $message" >&2
    exit 1
  fi
}

require_probe_threshold "livenessProbe"
require_probe_threshold "readinessProbe"

require_pattern "$hpa_file" 'minReplicas: 3' "HPA minReplicas must be 3"
require_pattern "$hpa_file" 'maxReplicas: [1-9][0-9]*' "HPA maxReplicas must be configured"
require_hpa_block_pattern "scaleUp" 'stabilizationWindowSeconds: 300' "HPA scale-up cooldown must be 5 minutes"
require_hpa_block_pattern "scaleUp" 'type: Percent' "HPA scale-up policy must include a percent policy"
require_hpa_block_pattern "scaleUp" 'value: 50' "HPA scale-up policy must increase by 50 percent"
require_hpa_block_pattern "scaleUp" 'type: Pods' "HPA scale-up policy must allow a pod floor"
require_hpa_block_pattern "scaleUp" 'value: 1' "HPA scale-up policy must allow at least one pod"
require_hpa_block_pattern "scaleDown" 'stabilizationWindowSeconds: 900' "HPA scale-down cooldown must be 15 minutes"
require_hpa_block_pattern "scaleDown" 'type: Percent' "HPA scale-down policy must include a percent policy"
require_hpa_block_pattern "scaleDown" 'value: 20' "HPA scale-down policy must reduce by 20 percent"
require_pattern "$hpa_file" 'averageUtilization: 80' "HPA CPU target must be 80 percent"
require_pattern "$hpa_file" 'averageUtilization: 85' "HPA memory target must be 85 percent"
require_pattern "$hpa_file" 'name: workflow_queue_backlog' "HPA must include workflow queue backlog metric"
require_pattern "$hpa_file" 'averageValue: "100"' "HPA queue backlog threshold must be 100"

echo "[k8s-recovery-policy] kubernetes recovery policy validated"

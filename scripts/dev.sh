#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF'
No workspace development command is configured yet.

Action required:
1) Add a "dev" script to one or more workspace packages (for example in lobehub/package.json or new-api/package.json)
2) Update /scripts/dev.sh to run those package dev scripts (for example via pnpm -r --parallel dev)
EOF

exit 1

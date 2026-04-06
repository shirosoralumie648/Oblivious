# Oblivious

Oblivious is a workspace-oriented application with a Go backend, a React frontend, and PostgreSQL as the system of record. The current mainline scope covers chat, knowledge base CRUD plus retrieval MVP, SOLO task runtime MVP, settings/preferences, and console summary pages.

## Prerequisites

- Go 1.22
- Node.js 20+
- pnpm 10.6.0
- PostgreSQL 14+

## Quick Start

1. Install workspace dependencies.

   ```bash
   pnpm install --frozen-lockfile
   ```

2. Export runtime environment variables from [`config/.env.example`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/config/.env.example).

3. Apply database migrations.

   ```bash
   cd src/server
   go run ./cmd/migrate
   ```

4. Start the web app and API.

   ```bash
   bash scripts/dev.sh
   ```

## Quality Gates

Run the same top-level commands used by CI before pushing changes:

```bash
bash scripts/check.sh
bash scripts/test.sh
```

`bash scripts/check.sh` verifies release assets, docs and environment consistency, the web production build, and the server unit/contract packages.

`bash scripts/test.sh` runs the web Vitest suite, the server unit packages, and the HTTP integration package. If `TEST_DATABASE_URL` is not set, the integration step is skipped explicitly.

## Repository Layout

- [`src/server`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/src/server): Go API, migrations, and domain services
- [`src/web`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/src/web): React workspace and console UI
- [`docs/architecture/current-system-contracts.md`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/docs/architecture/current-system-contracts.md): current API and runtime contract baseline
- [`docs/release/rc-checklist.md`](/home/shirosora/code_storage/Oblivious/.worktrees/phase0-task1-contracts/docs/release/rc-checklist.md): RC readiness checklist

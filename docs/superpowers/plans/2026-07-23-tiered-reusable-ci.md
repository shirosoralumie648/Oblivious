# Tiered Reusable CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every branch fast GitHub-hosted feedback while preserving the complete repository-local gate set for pull requests, default branches, and manual runs.

**Architecture:** Keep `.github/workflows/ci.yml` as a read-only event facade that computes one trusted `run_full` boolean and calls `.github/workflows/_ci-reusable.yml`. Put all quick and full jobs in the reusable workflow, use `.github/actions/setup-toolchain/action.yml` for deterministic Go and Node/pnpm setup, and enforce the structure through `scripts/verify-quality-gates.sh`.

**Tech Stack:** GitHub Actions YAML, composite actions, Bash, Docker Compose, Go 1.25, Node 20.19.0, pnpm 10.6.0, PostgreSQL/pgvector, Playwright.

---

### Task 1: Add a failing repository contract for the approved CI structure

**Files:**
- Modify: `scripts/verify-quality-gates.sh`
- Test: `scripts/verify-quality-gates.sh`

- [ ] **Step 1: Declare all CI contract artifacts**

Add these variables next to the existing `workflow_file` declaration:

```bash
reusable_workflow_file="$repo_root/.github/workflows/_ci-reusable.yml"
toolchain_action_file="$repo_root/.github/actions/setup-toolchain/action.yml"
```

- [ ] **Step 2: Require the files and facade/reusable/action contracts**

Add a `verify_ci_contract` function that owns `assert_file_exists` calls for both new files and the focused assertions below. Invoke it for the normal full gate and expose `--ci-contract-only` as an early-exit mode so branch CI can validate workflow structure without running the release clean-head aggregate:

```bash
verify_ci_contract() {
  # focused assertions below
}

if [[ "${1:-}" == "--ci-contract-only" ]]; then
  verify_ci_contract
  echo "[quality-gates] CI contract verified."
  exit 0
fi

if [[ $# -gt 0 ]]; then
  echo "Usage: bash scripts/verify-quality-gates.sh [--ci-contract-only]" >&2
  exit 2
fi

verify_ci_contract
```

Replace the old single-file CI assertions with focused assertions that require:

```bash
assert_file_contains "$workflow_file" "push:"
assert_file_contains "$workflow_file" "pull_request:"
assert_file_contains "$workflow_file" "workflow_dispatch:"
assert_file_contains "$workflow_file" "contents: read"
assert_file_contains "$workflow_file" "cancel-in-progress: true"
assert_file_contains "$workflow_file" "uses: ./.github/workflows/_ci-reusable.yml"
assert_file_contains "$workflow_file" "run_full:"
assert_file_contains "$workflow_file" "refs/heads/main"
assert_file_contains "$workflow_file" "refs/heads/master"
assert_file_not_contains "$workflow_file" "pull_request_target"

assert_file_contains "$reusable_workflow_file" "workflow_call:"
assert_file_contains "$reusable_workflow_file" "type: boolean"
assert_file_contains "$reusable_workflow_file" "quick-server:"
assert_file_contains "$reusable_workflow_file" "quick-web:"
assert_file_contains "$reusable_workflow_file" "quick-compose:"
assert_file_contains "$reusable_workflow_file" "quick-gate:"
assert_file_contains "$reusable_workflow_file" "full-gate:"
assert_file_contains "$reusable_workflow_file" "pgvector/pgvector:pg16"
assert_file_contains "$reusable_workflow_file" "OBLIVIOUS_REQUIRE_TEST_DATABASE"
assert_file_contains "$reusable_workflow_file" "bash scripts/check.sh security"
assert_file_contains "$reusable_workflow_file" "bash scripts/test.sh e2e"
assert_file_contains "$reusable_workflow_file" "actions/upload-artifact@v4"
assert_file_contains "$reusable_workflow_file" "if: failure()"
assert_file_not_contains "$reusable_workflow_file" "pull_request_target"

assert_file_contains "$toolchain_action_file" "actions/setup-go@v5"
assert_file_contains "$toolchain_action_file" "pnpm/action-setup@v4"
assert_file_contains "$toolchain_action_file" "actions/setup-node@v4"
assert_file_contains "$toolchain_action_file" "pnpm install --frozen-lockfile"
```

Keep the existing release, target-fixture, strict database, and command assertions, but point them at `reusable_workflow_file` because the facade deliberately contains no implementation jobs.

- [ ] **Step 3: Run the contract and verify RED**

Run:

```bash
bash scripts/verify-quality-gates.sh
```

Expected: exit 1 with `[quality-gates] missing file:` for `_ci-reusable.yml` or `setup-toolchain/action.yml`. This proves the contract detects the missing architecture.

- [ ] **Step 4: Commit the contract only after the implementation turns it green**

Do not commit the deliberately red intermediate state. Stage it together with Task 2 after all assertions pass.

### Task 2: Implement the event facade, shared toolchain action, and reusable tiers

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/_ci-reusable.yml`
- Create: `.github/actions/setup-toolchain/action.yml`
- Modify: `scripts/verify-quality-gates.sh`
- Test: `scripts/verify-quality-gates.sh`

- [ ] **Step 1: Replace `ci.yml` with the event facade**

Use this exact shape:

```yaml
name: CI

on:
  push:
  pull_request:
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

jobs:
  ci:
    uses: ./.github/workflows/_ci-reusable.yml
    with:
      run_full: ${{ github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch' || (github.event_name == 'push' && (github.ref == 'refs/heads/main' || github.ref == 'refs/heads/master')) }}
```

Do not add `secrets: inherit`, `pull_request_target`, write permissions, or deployment behavior.

- [ ] **Step 2: Add the local toolchain composite action**

Create explicit string boolean inputs named `setup-go`, `setup-node`, and `install-dependencies`, all defaulting to `false`. Its composite steps must:

```yaml
- uses: actions/setup-go@v5
  if: inputs.setup-go == 'true'
  with:
    go-version-file: src/server/go.mod
    cache-dependency-path: src/server/go.sum
- uses: pnpm/action-setup@v4
  if: inputs.setup-node == 'true'
  with:
    version: 10.6.0
    run_install: false
- uses: actions/setup-node@v4
  if: inputs.setup-node == 'true'
  with:
    node-version: 20.19.0
    cache: pnpm
    cache-dependency-path: pnpm-lock.yaml
- if: inputs.install-dependencies == 'true'
  shell: bash
  run: pnpm install --frozen-lockfile
```

- [ ] **Step 3: Add the reusable workflow contract**

Declare `workflow_call.inputs.run_full` as required boolean and `permissions.contents: read`. Add these unconditional quick jobs:

```text
quick-server  -> setup Go -> check.sh server -> test.sh server
quick-web     -> setup Node/pnpm/deps -> check.sh web -> test.sh web
quick-compose -> docker compose config --quiet
                 docker compose --profile microservices config --quiet
                 bash scripts/verify-quality-gates.sh --ci-contract-only
                 bash -n scripts/check.sh scripts/test.sh scripts/verify-quality-gates.sh
quick-gate    -> always(), require all three quick job results == success
```

Use explicit timeouts of 20, 20, 10, and 5 minutes respectively.

- [ ] **Step 4: Add conditional full-tier jobs**

Every full job uses `if: ${{ inputs.run_full }}` and an explicit timeout:

```text
release-gates           -> existing protobuf digest/cache/bootstrap, docs, Relay security
target-release-evidence -> verifier, assembler, artifact collector, digest fixtures
security                -> frozen workspace plus dependency security gate
server-database         -> pgvector/pgvector:pg16, strict TEST_DATABASE_URL, serial tests
e2e                     -> cached Chromium install, scripts/test.sh e2e, failure artifacts
full-gate               -> always() && inputs.run_full, require quick and all full results
```

For Playwright diagnostics, upload `src/web/playwright-report/` and `src/web/test-results/` with `actions/upload-artifact@v4`, `if: failure()`, `if-no-files-found: ignore`, and `retention-days: 7`.

- [ ] **Step 5: Run the focused contract and verify GREEN**

Run:

```bash
bash scripts/verify-quality-gates.sh --ci-contract-only
```

Expected: exit 0 with `[quality-gates] CI contract verified.` and no missing-pattern error.

- [ ] **Step 6: Commit the CI implementation atomically**

```bash
git add .github/workflows/ci.yml \
  .github/workflows/_ci-reusable.yml \
  .github/actions/setup-toolchain/action.yml \
  scripts/verify-quality-gates.sh
git diff --cached --check
git commit -m "ci: add tiered reusable validation"
```

Before committing, confirm `git diff --cached --name-only` contains exactly those four files.

### Task 3: Verify locally, record GSD completion, push, and inspect GitHub

**Files:**
- Modify if baseline drift is confirmed: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
- Modify if baseline drift is confirmed: `src/web/src/services/http/upload.test.ts`
- Modify if baseline drift is confirmed: `src/web/src/routes/admin/AdminAlertsPage.test.tsx`
- Create: `.planning/quick/260723-1rx-implement-approved-tiered-reusable-ci-an/260723-1rx-SUMMARY.md`
- Modify: `.planning/STATE.md`
- Verify: `.github/workflows/ci.yml`
- Verify: `.github/workflows/_ci-reusable.yml`
- Verify: `.github/actions/setup-toolchain/action.yml`
- Verify: `scripts/verify-quality-gates.sh`

- [ ] **Step 0: Close deterministic baseline failures exposed by the quick tier**

If focused reproduction confirms the committed tests disagree with newer authority-bearing contracts, update only the stale expectations and commit them separately. The current known authorities are the conditional `mcp.custom_execution` navigation projection introduced by `a01905b`, the HTTP 202 upload success contract introduced by `62ad448`, and the generated HTTP 201 alert-provider creation contract consumed by `src/web/src/features/admin/api.ts`.

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/layouts/WorkspaceLayout.test.tsx \
  src/services/http/upload.test.ts \
  src/routes/admin/AdminAlertsPage.test.tsx \
  --maxWorkers=1
```

Expected: 3 files and 22 tests pass. Do not alter application behavior or weaken the quick-web job.

- [ ] **Step 1: Parse local configuration surfaces**

Run:

```bash
docker compose config --quiet
docker compose --profile microservices config --quiet
ruby -e 'require "yaml"; ARGV.each { |path| YAML.parse_file(path) }' \
  .github/workflows/ci.yml \
  .github/workflows/_ci-reusable.yml \
  .github/actions/setup-toolchain/action.yml
bash -n scripts/verify-quality-gates.sh
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 2: Run the repository-owned focused gates**

Run:

```bash
bash scripts/verify-quality-gates.sh --ci-contract-only
bash scripts/check.sh server
pnpm --dir src/web build
```

Expected: every command exits 0. After committing the implementation, run the default `bash scripts/verify-quality-gates.sh` from a temporary clean worktree at that commit; the primary checkout is intentionally dirty with protected user work and its clean-head aggregate must remain fail-closed. Do not claim browser, database-backed, or GitHub-hosted full-tier success from these local checks.

- [ ] **Step 3: Review the implementation diff and protected dirty paths**

Run:

```bash
git show --stat --oneline HEAD
git status --short
git diff -- .github/workflows/ci.yml \
  .github/workflows/_ci-reusable.yml \
  .github/actions/setup-toolchain/action.yml \
  scripts/verify-quality-gates.sh
```

Confirm the implementation does not stage or modify `.planning/config.json`, `.planning/intel/`, Phase 31.2 gap plans, or either `effect_registry` file.

- [ ] **Step 4: Write the quick-task summary and state row**

Record exact commands, exit status, implementation commit, evidence ceiling, and any remote-auth or GitHub-run limitation. Commit only the implementation plan, quick-task plan/summary, and `.planning/STATE.md` as:

```bash
git commit -m "docs(quick-260723-1rx): record tiered reusable CI implementation"
```

- [ ] **Step 5: Push and inspect the development-branch run**

Run:

```bash
git push origin commercial-target-release-runner
gh run list --workflow CI --branch commercial-target-release-runner --limit 5
```

If GitHub CLI authentication is unavailable, report that remote run inspection is blocked even when `git push` succeeds. A branch push is expected to run only the quick tier; do not claim the PR/default-branch `full-gate` has executed until a matching hosted run is observed.

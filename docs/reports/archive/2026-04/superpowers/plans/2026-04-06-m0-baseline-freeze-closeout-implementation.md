# M0 Baseline Freeze Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 冻结当前主线的 workspace 边界、根级验证入口、契约/环境变量文档，以及 owner/周报/阻塞升级模板，使后续里程碑基于单一主线事实执行。

**Architecture:** 以 root 脚本和 root CI 为统一门面，把 `lobehub`、`new-api` 明确降级为仓内参考目录，并通过 `scripts/check.sh` + `scripts/verify-quality-gates.sh` 持续约束 workspace、文档和治理资产的一致性。文档层只追平当前代码现实，不回改业务实现；治理层只交付最小角色矩阵和执行模板，不引入组织实名信息。

**Tech Stack:** pnpm workspace, Bash scripts, GitHub Actions, Markdown docs, Go config contract (`src/server/internal/config/config.go`)

---

## File Structure

- Modify: `pnpm-workspace.yaml`
  - 收缩 root workspace 到 `src/web`
- Modify: `.gitignore`
  - 忽略本地 agent 过程产物，避免 `.superpowers/` 干扰主线冻结验收
- Modify: `scripts/check.sh`
  - 增加主线 workspace / 文档 / 环境变量一致性检查
- Modify: `scripts/verify-quality-gates.sh`
  - 增加负向断言 helper，并冻结主线边界、CI 触发和治理文档存在性
- Modify: `.github/workflows/ci.yml`
  - 去掉历史里程碑分支触发，只保留主线 CI 入口
- Verify only: `package.json`
  - 当前 `dev` / `check` / `test` 已指向 root scripts，本里程碑只保留一致性验证，不做语义改写
- Modify: `README.md`
  - 纠正主线范围说明、修复旧 worktree 绝对路径、声明参考目录边界
- Modify: `docs/release/rc-checklist.md`
  - 修复旧 worktree 绝对路径链接
- Modify: `docs/architecture/current-system-contracts.md`
  - 冻结主线边界、workspace 策略、root 验证入口说明
- Modify: `config/.env.example`
  - 按当前真实配置消费字段重排并加注释分组
- Create: `docs/governance/owner-matrix.md`
  - 角色级 owner 映射基线
- Create: `docs/governance/weekly-status-template.md`
  - 周报模板，预留负责人字段
- Create: `docs/governance/blocker-escalation.md`
  - 阻塞升级机制和响应窗口

## Task 1: Freeze Workspace Boundary And Top-Level References

**Files:**
- Modify: `scripts/verify-quality-gates.sh`
- Modify: `pnpm-workspace.yaml`
- Modify: `.gitignore`
- Modify: `README.md`
- Modify: `docs/release/rc-checklist.md`

- [ ] **Step 1: Write the failing quality-gate assertions for workspace scope, local ignore rules, and stale worktree links**

```bash
# scripts/verify-quality-gates.sh
assert_file_not_contains() {
  local path="$1"
  local pattern="$2"
  if rg -q --fixed-strings "$pattern" "$path"; then
    echo "[quality-gates] unexpected pattern '$pattern' in $path" >&2
    exit 1
  fi
}

workspace_file="$repo_root/pnpm-workspace.yaml"
gitignore_file="$repo_root/.gitignore"

assert_file_not_contains "$workspace_file" '"lobehub"'
assert_file_not_contains "$workspace_file" '"new-api"'
assert_file_contains "$gitignore_file" ".superpowers/"
assert_file_not_contains "$readme_file" ".worktrees/phase0-task1-contracts"
assert_file_not_contains "$release_checklist_file" ".worktrees/phase0-task1-contracts"
```

- [ ] **Step 2: Run docs verification to confirm it fails for the current workspace drift**

Run:

```bash
bash scripts/check.sh docs
```

Expected: FAIL because `pnpm-workspace.yaml` still contains `lobehub` / `new-api`, `.gitignore` does not yet ignore `.superpowers/`, and both `README.md` and `docs/release/rc-checklist.md` still contain `.worktrees/phase0-task1-contracts` links.

- [ ] **Step 3: Implement the workspace boundary freeze and top-level doc reference fixes**

```yaml
# pnpm-workspace.yaml
packages:
  - src/web
```

```gitignore
# .gitignore
.worktrees/
.superpowers/
node_modules/
dist/
coverage/
.env
.env.*
!.env.example
bin/
.build/
.tmp/
*.log
```

````md
# README.md
# Oblivious

Oblivious is a workspace-oriented application with a Go backend, a React frontend, and PostgreSQL as the system of record. The current mainline scope covers chat, knowledge base CRUD, SOLO starter task flows, settings/preferences, and console overview pages.

## Mainline Boundary

The current mainline covers:

- `src/server`
- `src/web`
- `config`
- `scripts`
- `.github/workflows`

`lobehub` and `new-api` remain in the repository as reference directories only. They are not part of the root workspace, root CI, or release scope.

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

2. Export runtime environment variables from [`config/.env.example`](config/.env.example).

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

- [`src/server`](src/server): Go API, migrations, and domain services
- [`src/web`](src/web): React workspace and console UI
- [`docs/architecture/current-system-contracts.md`](docs/architecture/current-system-contracts.md): current API and runtime contract baseline
- [`docs/release/rc-checklist.md`](docs/release/rc-checklist.md): RC readiness checklist
- `lobehub/`: repository-local reference code, excluded from mainline workspace and CI
- `new-api/`: repository-local reference code, excluded from mainline workspace and CI
````

```md
# docs/release/rc-checklist.md
## Manual Release Review

- [ ] No P0/P1 defects open
- [ ] Release notes summarize scope and known limitations
- [ ] Environment variables match [`config/.env.example`](../../config/.env.example)
- [ ] API contract changes are reflected in [`docs/architecture/current-system-contracts.md`](../architecture/current-system-contracts.md)
```

- [ ] **Step 4: Re-run docs verification to confirm the boundary freeze passes**

Run:

```bash
bash scripts/check.sh docs
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  pnpm-workspace.yaml \
  .gitignore \
  scripts/verify-quality-gates.sh \
  README.md \
  docs/release/rc-checklist.md
git commit -m "build: freeze mainline workspace boundary"
```

## Task 2: Align Root Checks And CI Scope

**Files:**
- Modify: `scripts/verify-quality-gates.sh`
- Modify: `scripts/check.sh`
- Modify: `.github/workflows/ci.yml`
- Verify only: `package.json`

- [ ] **Step 1: Write the root-entry and CI-boundary assertions**

```bash
# scripts/verify-quality-gates.sh
package_file="$repo_root/package.json"

assert_file_not_contains "$workflow_file" "phase0-task1-contracts"
assert_file_contains "$package_file" '"dev": "bash scripts/dev.sh"'
assert_file_contains "$package_file" '"check": "bash scripts/check.sh"'
assert_file_contains "$package_file" '"test": "bash scripts/test.sh"'
assert_file_contains "$check_script" 'workspace_file="$repo_root/pnpm-workspace.yaml"'
assert_file_contains "$check_script" 'Unexpected workspace member: lobehub'
assert_file_contains "$check_script" 'Unexpected workspace member: new-api'
```

- [ ] **Step 2: Run docs verification to confirm the new CI/root-boundary assertions fail**

Run:

```bash
bash scripts/check.sh docs
```

Expected: FAIL because `.github/workflows/ci.yml` still includes `phase0-task1-contracts`, and `scripts/check.sh` does not yet enforce the root workspace boundary. The `package.json` root entry assertions should already pass and therefore become part of the frozen baseline without requiring a file edit.

- [ ] **Step 3: Implement the root boundary checks and CI trigger cleanup**

```bash
# scripts/check.sh
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
workspace_file="$repo_root/pnpm-workspace.yaml"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
target="${1:-all}"

run_docs_checks() {
  local required_vars

  echo "[check] Verifying release assets."
  bash "$repo_root/scripts/verify-quality-gates.sh"

  echo "[check] Verifying docs and env consistency."
  required_vars=(
    DATABASE_URL
    SESSION_SECRET
    LLM_BASE_URL
    LLM_API_KEY
    MODEL_DEFAULT_NAME
  )

  for var_name in "${required_vars[@]}"; do
    rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
    rg -q --fixed-strings "$var_name" "$repo_root/docs/architecture/current-system-contracts.md"
    rg -q --fixed-strings "$var_name" "$repo_root/src/server/internal/config/config.go"
  done

  echo "[check] Verifying mainline workspace boundary."
  rg -q --fixed-strings "packages:" "$workspace_file"
  rg -q --fixed-strings "  - src/web" "$workspace_file"
  if rg -q --fixed-strings 'lobehub' "$workspace_file"; then
    echo "[check] Unexpected workspace member: lobehub" >&2
    exit 1
  fi
  if rg -q --fixed-strings 'new-api' "$workspace_file"; then
    echo "[check] Unexpected workspace member: new-api" >&2
    exit 1
  fi
}
```

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches:
      - main
      - master
  pull_request:

jobs:
  release-gates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Verify release assets
        run: bash scripts/check.sh docs

  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 10.6.0
          run_install: false
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: pnpm
          cache-dependency-path: pnpm-lock.yaml
      - name: Install workspace dependencies
        run: pnpm install --frozen-lockfile
      - name: Build web app
        run: bash scripts/check.sh web
      - name: Run web tests
        run: bash scripts/test.sh web

  server:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: src/server/go.mod
      - name: Run server checks
        run: bash scripts/check.sh server
      - name: Run server tests
        run: bash scripts/test.sh server
```

- [ ] **Step 4: Re-run docs verification to confirm root checks and CI scope now align**

Run:

```bash
bash scripts/check.sh docs
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  scripts/verify-quality-gates.sh \
  scripts/check.sh \
  .github/workflows/ci.yml
git commit -m "build: align root checks with CI scope"
```

## Task 3: Freeze Runtime Contract And Environment Matrix

**Files:**
- Modify: `scripts/check.sh`
- Modify: `docs/architecture/current-system-contracts.md`
- Modify: `config/.env.example`

- [ ] **Step 1: Write the failing docs/env consistency checks for the full runtime matrix**

```bash
# scripts/check.sh
contracts_file="$repo_root/docs/architecture/current-system-contracts.md"
frontend_vars=(
  WEB_PORT
  WEB_API_BASE_URL
)
backend_vars=(
  SERVER_PORT
  APP_ENV
  CORS_ALLOWED_ORIGINS
  DATABASE_URL
  SESSION_SECRET
  SESSION_COOKIE_NAME
  SESSION_COOKIE_SECURE
  LLM_BASE_URL
  LLM_API_KEY
  LLM_TIMEOUT_MS
  MODEL_DEFAULT_NAME
)

for var_name in "${frontend_vars[@]}"; do
  rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
  rg -q --fixed-strings "$var_name" "$contracts_file"
done

for var_name in "${backend_vars[@]}"; do
  rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
  rg -q --fixed-strings "$var_name" "$contracts_file"
  rg -q --fixed-strings "$var_name" "$repo_root/src/server/internal/config/config.go"
done

rg -q --fixed-strings "bash scripts/check.sh" "$contracts_file"
rg -q --fixed-strings "bash scripts/test.sh" "$contracts_file"
```

- [ ] **Step 2: Run docs verification to confirm the contract document currently fails the stronger checks**

Run:

```bash
bash scripts/check.sh docs
```

Expected: FAIL because `docs/architecture/current-system-contracts.md` does not yet document the root verification entry commands and the fully frozen runtime matrix.

- [ ] **Step 3: Implement the contract and env freeze**

```env
# config/.env.example
# Frontend local development
WEB_PORT=5173
WEB_API_BASE_URL=http://localhost:8080

# Backend runtime
SERVER_PORT=8080
APP_ENV=development
CORS_ALLOWED_ORIGINS=http://localhost:5173
DATABASE_URL=postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable
SESSION_SECRET=change-me
SESSION_COOKIE_NAME=oblivious_session
SESSION_COOKIE_SECURE=false
LLM_BASE_URL=
LLM_API_KEY=
LLM_TIMEOUT_MS=30000
MODEL_DEFAULT_NAME=demo-reply
```

````md
# docs/architecture/current-system-contracts.md
## 2. Mainline Boundaries

```text
Browser
  -> src/web (React + React Router + Vite)
  -> /api/*
  -> src/server (Go net/http + PostgreSQL)
  -> PostgreSQL
```

边界说明：

- `src/web` 是唯一主线前端。
- `src/server` 是唯一主线后端。
- `config`、`scripts` 和 `.github/workflows` 属于主线执行基线。
- `new-api` 与 `lobehub` 当前不属于 root workspace、root CI 或 root 交付链路的一部分。

## 7.4 Root Verification Entry

| Command | Scope | Notes |
| --- | --- | --- |
| `bash scripts/check.sh` | 主线 docs + web build + server unit checks | 作为 CI 与本地共同的静态门面 |
| `bash scripts/test.sh` | 主线 web tests + server unit tests + optional integration tests | 当 `TEST_DATABASE_URL` 缺失时，server integration 会显式 skip |
````

```bash
# scripts/check.sh
contracts_file="$repo_root/docs/architecture/current-system-contracts.md"
frontend_vars=(
  WEB_PORT
  WEB_API_BASE_URL
)
backend_vars=(
  SERVER_PORT
  APP_ENV
  CORS_ALLOWED_ORIGINS
  DATABASE_URL
  SESSION_SECRET
  SESSION_COOKIE_NAME
  SESSION_COOKIE_SECURE
  LLM_BASE_URL
  LLM_API_KEY
  LLM_TIMEOUT_MS
  MODEL_DEFAULT_NAME
)

for var_name in "${frontend_vars[@]}"; do
  rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
  rg -q --fixed-strings "$var_name" "$contracts_file"
done

for var_name in "${backend_vars[@]}"; do
  rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
  rg -q --fixed-strings "$var_name" "$contracts_file"
  rg -q --fixed-strings "$var_name" "$repo_root/src/server/internal/config/config.go"
done

rg -q --fixed-strings "bash scripts/check.sh" "$contracts_file"
rg -q --fixed-strings "bash scripts/test.sh" "$contracts_file"
```

- [ ] **Step 4: Re-run docs verification to confirm contract and env docs are frozen**

Run:

```bash
bash scripts/check.sh docs
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  scripts/check.sh \
  docs/architecture/current-system-contracts.md \
  config/.env.example
git commit -m "docs: freeze mainline runtime contract"
```

## Task 4: Add Governance Templates And Gate Them

**Files:**
- Modify: `scripts/verify-quality-gates.sh`
- Create: `docs/governance/owner-matrix.md`
- Create: `docs/governance/weekly-status-template.md`
- Create: `docs/governance/blocker-escalation.md`

- [ ] **Step 1: Write the failing governance-asset checks**

```bash
# scripts/verify-quality-gates.sh
owner_matrix_file="$repo_root/docs/governance/owner-matrix.md"
weekly_status_file="$repo_root/docs/governance/weekly-status-template.md"
blocker_escalation_file="$repo_root/docs/governance/blocker-escalation.md"

assert_file_exists "$owner_matrix_file"
assert_file_exists "$weekly_status_file"
assert_file_exists "$blocker_escalation_file"

assert_file_contains "$owner_matrix_file" "| TL |"
assert_file_contains "$owner_matrix_file" "| FE |"
assert_file_contains "$owner_matrix_file" "| BE |"
assert_file_contains "$weekly_status_file" "Actual Owner:"
assert_file_contains "$weekly_status_file" "## Risks / Blockers"
assert_file_contains "$blocker_escalation_file" "| Severity | Definition | Response Window | Escalate To |"
```

- [ ] **Step 2: Run docs verification to confirm governance assets are currently missing**

Run:

```bash
bash scripts/check.sh docs
```

Expected: FAIL because the governance documents do not exist yet.

- [ ] **Step 3: Create the governance freeze documents**

```md
# docs/governance/owner-matrix.md
# Mainline Owner Matrix

日期：2026-04-06

本文件只冻结角色级 owner，不写真实姓名。

| Role | Owns | Decision Scope | Default Deliverables |
| --- | --- | --- | --- |
| TL | 范围控制、架构裁决、里程碑验收、阻塞升级 | 是否接受契约变更、是否调整里程碑范围、是否允许跨阶段插入需求 | 里程碑验收结论、范围裁决 |
| FE | `src/web`、前端类型、路由、页面回归 | 前端实现拆分、测试补充、UI 行为一致性 | 前端实现、页面回归说明 |
| BE | `src/server`、数据库、接口稳定性、测试环境 | 后端接口实现、迁移、测试策略 | 接口实现、后端验证说明 |
| QA | 回归用例、缺陷门禁、发布签署 | 是否允许里程碑进入验收、缺陷严重级别判定 | 验收清单、缺陷状态 |
| OPS | CI、运行脚本、环境约束、发布入口 | CI 调整、环境变量发布约束、流水线问题升级 | CI 变更、运行环境说明 |
```

```md
# docs/governance/weekly-status-template.md
# Weekly Status Template

- Reporting Window:
- Prepared By:
- Role:
- Actual Owner:

## Completed This Week

- 

## Planned Next Week

- 

## Risks / Blockers

| Severity | Item | Impact | Needed Decision By | Actual Owner |
| --- | --- | --- | --- | --- |
|  |  |  |  |  |

## Decisions Needed

- 

## Verification Evidence

- `bash scripts/check.sh`
- `bash scripts/test.sh`
```

```md
# docs/governance/blocker-escalation.md
# Blocker Escalation Policy

## Severity Matrix

| Severity | Definition | Response Window | Escalate To |
| --- | --- | --- | --- |
| P0 | 主线 `check/test/build` 无法执行，或主线交付链完全阻断 | 同工作日 | TL |
| P1 | 当前里程碑关键路径阻断，24 小时内无法自行解除 | 1 个工作日内 | TL + 对应 owner |
| P2 | 有绕行方案但会影响计划完成度 | 本周周报中升级 | 对应 owner |

## Trigger Rules

- 同一阻塞持续超过 1 个工作日且没有明确 owner 时，升级到 TL。
- 任一阻塞导致 root `bash scripts/check.sh` 或 `bash scripts/test.sh` 失效时，按 `P0` 处理。
- 任何跨 FE / BE / OPS 的边界问题，先记录在周报，再按严重级别升级。

## Escalation Payload

升级时必须包含以下字段：

- Severity
- Impacted milestone
- Exact failing command or asset
- First observed date
- Current workaround
- Needed decision
```

- [ ] **Step 4: Re-run docs verification to confirm governance assets are now gated**

Run:

```bash
bash scripts/check.sh docs
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  scripts/verify-quality-gates.sh \
  docs/governance/owner-matrix.md \
  docs/governance/weekly-status-template.md \
  docs/governance/blocker-escalation.md
git commit -m "docs: add M0 governance templates"
```

## Task 5: Run Final Mainline Baseline Verification

**Files:**
- Verify: `pnpm-workspace.yaml`
- Verify: `.gitignore`
- Verify: `package.json`
- Verify: `scripts/check.sh`
- Verify: `scripts/test.sh`
- Verify: `scripts/verify-quality-gates.sh`
- Verify: `.github/workflows/ci.yml`
- Verify: `README.md`
- Verify: `docs/release/rc-checklist.md`
- Verify: `docs/architecture/current-system-contracts.md`
- Verify: `config/.env.example`
- Verify: `docs/governance/owner-matrix.md`
- Verify: `docs/governance/weekly-status-template.md`
- Verify: `docs/governance/blocker-escalation.md`

- [ ] **Step 1: Verify the root install surface is stable**

Run:

```bash
pnpm install --frozen-lockfile
```

Expected: PASS without trying to resolve `lobehub` / `new-api` as workspace importers.

- [ ] **Step 2: Verify the frozen root quality gates**

Run:

```bash
bash scripts/check.sh
```

Expected: PASS with release assets, docs/env consistency, web build, and server unit checks all green.

- [ ] **Step 3: Verify the frozen root test entry**

Run:

```bash
bash scripts/test.sh
```

Expected: PASS. If `TEST_DATABASE_URL` is not set, expect the explicit message `Skipping server integration tests: TEST_DATABASE_URL not set.` and treat that as acceptable for this milestone.

- [ ] **Step 4: Verify there are no stale worktree links or unintended staged artifacts**

Run:

```bash
rg -n ".worktrees/phase0-task1-contracts" README.md docs/architecture docs/release docs/governance .github
git diff --check
git status --short
```

Expected:

- `rg` returns no matches
- `git diff --check` returns no whitespace / conflict-marker errors
- `git status --short` does not show `.superpowers/` artifacts because they are ignored at repo root

- [ ] **Step 5: Commit**

```bash
git add \
  pnpm-workspace.yaml \
  .gitignore \
  scripts/check.sh \
  scripts/verify-quality-gates.sh \
  .github/workflows/ci.yml \
  README.md \
  config/.env.example \
  docs/release/rc-checklist.md \
  docs/architecture/current-system-contracts.md \
  docs/governance/owner-matrix.md \
  docs/governance/weekly-status-template.md \
  docs/governance/blocker-escalation.md
git commit -m "build: freeze M0 mainline baseline"
```

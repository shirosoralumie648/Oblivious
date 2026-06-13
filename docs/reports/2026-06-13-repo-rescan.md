# Repository Rescan - 2026-06-13

## Current Truth

- Branch: `main`; the dependency-security rescan started from pushed `HEAD` `884c276` before the local dependency slice.
- The project is **not complete** against the four fusion specs. `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` still has 4 `Proven` rows and 10 `Partial` rows.
- The previous `Phase 2 complete / goal.md complete` statement in `docs/design/FUSION_GAP_CLOSURE_PLAN.md` has been corrected to a recovery/verification status.
- Current progress estimate after this rescan: **70/100**. Most repository-owned feature surfaces exist and have focused evidence; remaining progress is dominated by target-environment proof, DB-backed/commercial reruns, security/tenant-isolation depth, and release readiness.

## Verified In This Rescan Slice

- Frontend type check passed: `COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit`.
- Docs gate passed: `bash scripts/check.sh docs`.
- Diff whitespace gate passed: `git diff --check`.
- Server target build is restored:
  - `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent ./internal/chat ./internal/relay ./internal/workflow ./internal/observability ./internal/task ./cmd/channel ./cmd/observability ./cmd/task ./cmd/workflow -run '^$' -count=1`
- Full server package test passed after restoring Observability proto generation:
  - `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./...`
- Focused Agent/RAG/event checks passed:
  - `go test ./internal/agent -run 'TestExecuteReAct|TestRunnerSkillSelector|TestBuildToolsFromSkills|TestInjectSkillInstructions' -count=1`
  - `go test ./pkg/rag ./pkg/event -count=1`
- `src/server/api/proto/events.proto` was reconstructed and verified to regenerate `src/server/pkg/event/proto/events.pb.go` without diff when invoked with the tracked source path.
- `src/server/api/proto/observability/v1/observability.proto` and its generated Go/gRPC files were restored because `pkg/metrics` imports `oblivious/server/api/proto/observability/v1`.

## Dependency Security Refresh

- `bash scripts/check.sh security` passed after dependency cleanup: `pnpm audit` reported no known npm vulnerabilities, and `govulncheck v1.3.0` reported the code is affected by 0 vulnerabilities.
- The frontend toolchain moved from `vite@6.4.3` / `@vitejs/plugin-react@4.x` to `vite@8.0.16` / `@vitejs/plugin-react@6.0.2`, removing the previously audited `esbuild@0.25.x` path.
- The unused root `wireit` devDependency was removed from `package.json`, `package-lock.json`, and `pnpm-lock.yaml`, clearing the remaining `brace-expansion@4.0.1` advisory path.
- Regression gates passed after the dependency slice:
  - `COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web build`
  - `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/workflows.spec.ts --project=chromium`
  - `bash scripts/check.sh docs`
  - `git diff --check`
- Vite 8 currently emits non-fatal Lightning CSS warnings for Tailwind-style `@theme` / `@utility` rules and a large bundle warning. These are not blockers for this dependency-security slice, but they remain frontend build-quality debt.

## Cleanup Decisions

- Kept as real source: `src/server/api/proto/events.proto` and `src/server/pkg/event/proto/events.pb.go`, because current server code imports `oblivious/server/pkg/event/proto`.
- Kept as real source: `src/server/api/proto/observability/v1/observability.proto`, `observability.pb.go`, and `observability_grpc.pb.go`, because `src/server/pkg/metrics/client.go` depends on that gRPC contract.
- Repointed `src/server/pkg/rag` to the already tracked `oblivious/server/internal/grpc/ragv1` package so it no longer depends on duplicate untracked generated files.
- Moved stale or speculative untracked artifacts to `.tmp/rescan-stale-artifacts/`:
  - root implementation summaries under `src/server/`
  - old task queue example entrypoint
  - duplicate untracked RAG generated proto files
  - unused Agent proto draft
  - unverified architecture docs that claimed active integration paths without current proof

## Remaining Work

- Continue closing the 10 `Partial` matrix rows: Workflow production evidence, Agent dual-engine/runtime edge cases, Billing/operator evidence, Marketplace provider integration, Frontend route/component parity, Observability production recovery evidence, API contract depth, Deployment live validation, security/tenant-isolation depth beyond the cleared dependency audit, and final release readiness.
- The next implementation slice should start from the highest-risk repo-owned gap with local proof, not from the corrected completion claim.

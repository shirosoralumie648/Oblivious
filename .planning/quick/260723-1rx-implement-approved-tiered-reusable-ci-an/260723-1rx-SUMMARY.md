---
quick_id: 260723-1rx
status: complete
completed_at: 2026-07-22T18:18:08Z
implementation_commit: e543a6d
supporting_commit: 0adddf3
---

# Quick Task 260723-1rx Summary

## Outcome

The approved tiered reusable CI architecture is implemented. Every branch push invokes the quick tier, while pull requests, `main`/`master` pushes, and manual dispatches also invoke the full repository-local tier. The facade remains least-privilege and credential-free, and the reusable workflow exposes stable `quick-gate` and `full-gate` aggregate jobs.

The quick tier surfaced deterministic frontend fixture drift against newer release contracts. Commit `0adddf3` updates those tests without weakening runtime capability enforcement or changing production behavior. Commit `e543a6d` contains the CI facade, reusable workflow, shared toolchain action, and structural quality-gate assertions.

## Changes

- Added the repository-owned reusable CI workflow with quick server, web, Compose, and aggregate jobs.
- Added conditional full jobs for release gates, target-evidence fixtures, dependency security, strict PostgreSQL tests, Playwright E2E, and the full aggregate gate.
- Added a shared Go and Node/pnpm setup action with frozen dependency installation.
- Added focused CI contract verification through `scripts/verify-quality-gates.sh --ci-contract-only`.
- Aligned router, workspace layout, upload, and admin alert fixtures with the current release projection and HTTP contracts.

## Verification

- Focused reproduction of the original frontend failures passed: 5 tests.
- The complete router suite passed: 62 tests.
- The complete frontend suite passed: 72 files and 682 tests.
- The frontend production build passed; existing CSS and chunk-size warnings remain non-blocking.
- Fresh quick-tier reruns of `check.sh server`, `check.sh web`, `test.sh server`, and `test.sh web` exited successfully before the documentation commit. The server script explicitly skipped database integration tests because `TEST_DATABASE_URL` was not set; strict PostgreSQL coverage remains owned by the conditional full-tier job.
- The CI structural contract, both Docker Compose profile parses, YAML parsing, shell syntax, and `git diff --check` passed.
- `actionlint` v1.7.7 passed against the facade and reusable workflow. The composite action passed YAML parsing and the repository structural contract; `actionlint` accepts workflow files rather than composite action metadata.
- The default quality gate passed from a temporary clean worktree at exact commit `e543a6dc3514f8a6c4cb6af417f74dfaa956c23b` after frozen dependencies were installed. Its Stage B report recorded `result: pass`, `evidenceClass: repository-local`, `migrationState: fresh-first-apply-second-no-op`, `surfaceCount: 10`, and zero skipped checks.
- Protected `.planning/config.json`, `effect_registry` source/test edits, `.planning/intel/`, and Phase 31.2 gap plans were not included in either implementation commit.

## Evidence Boundary

This task proves repository-local CI structure and local execution at the committed head. It does not prove a GitHub-hosted run, external target state, target Kubernetes deployment, Provider/payment rails, signing, or commercial release readiness. Hosted quick-tier status must be reported separately after push and only when GitHub-side evidence is observable.

## Commits

- `0adddf3 test(web): align fixtures with release contracts`
- `e543a6d ci: add tiered reusable validation`

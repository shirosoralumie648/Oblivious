# Tiered Reusable CI Design

> Status: Approved design
>
> Date: 2026-07-23
>
> Decision: Use a minimal event facade that calls one repository-owned reusable workflow. Run fast checks for every development branch and add the complete repository-local gate set for pull requests, default branches, and manual runs.

## Purpose

The current CI runs only for pushes to `main` or `master` and for pull requests. Development branches can therefore accumulate build or test failures without receiving GitHub-hosted feedback. This design adds fast branch feedback without running the most expensive release, database, browser, and evidence-fixture jobs for every intermediate push.

The design preserves the project's evidence boundary. Ordinary GitHub Actions validate repository-local behavior only. They do not use production credentials, mutate target infrastructure, generate target/live proof, or claim commercial release readiness.

## Architecture

The implementation has three repository-owned components:

| Component | Responsibility |
|---|---|
| `.github/workflows/ci.yml` | Event facade for all pushes, pull requests, and manual runs; owns read-only permissions, concurrency, and the full-tier decision. |
| `.github/workflows/_ci-reusable.yml` | `workflow_call` implementation containing quick jobs, conditional full jobs, timeouts, caches, failure artifacts, and aggregate gates. |
| `.github/actions/setup-toolchain/action.yml` | Local composite action that installs the requested Go and/or Node/pnpm toolchain and optionally installs the frozen workspace dependencies. |

The facade calls the reusable workflow once and passes a required boolean `run_full` input. The value is true for pull requests, `workflow_dispatch`, and pushes to `main` or `master`; it is false for other branch pushes. No caller-provided string, repository variable, or secret may override this classification.

## Trigger And Concurrency Contract

`ci.yml` listens to:

- every `push`, so development branches receive quick feedback;
- every `pull_request`, so merge candidates receive the full tier;
- `workflow_dispatch`, so operators can rerun the full repository-local tier explicitly.

Top-level permissions are `contents: read`. The caller passes no secrets to the reusable workflow. Individual jobs must not request broader permissions.

Concurrency groups use workflow identity plus pull-request number or branch ref. A newer run cancels an older run for the same branch or pull request, but unrelated refs remain independent.

## Quick Tier

The quick tier runs for every invocation:

1. `quick-server` checks that every Go package compiles and runs the existing server suite without requiring a database. Database-dependent coverage remains explicit and cannot silently pass as full-tier evidence.
2. `quick-web` installs the frozen pnpm workspace, runs the production TypeScript/Vite build, and runs frontend unit tests.
3. `quick-compose` parses the default and `microservices` Docker Compose profiles and runs focused workflow/configuration syntax checks that require no credentials or external services.
4. `quick-gate` depends on all quick jobs and succeeds only when every required result is `success`.

The quick tier targets rapid developer feedback. It does not run Playwright browser installation, a PostgreSQL service, the full release-asset aggregate, dependency audits, or target-evidence mutation suites.

## Full Tier

When `run_full` is true, the reusable workflow additionally runs:

- `release-gates`: protobuf bootstrap/cache, release assets, documentation contracts, and Relay security;
- `target-release-evidence`: repository-local verifier, assembler, collector, and digest mutation fixtures;
- `security`: dependency security checks using the frozen lockfiles;
- `server-database`: the existing PostgreSQL/pgvector service with `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` and serial database-backed Go tests;
- `e2e`: Chromium-backed Playwright coverage;
- `full-gate`: an aggregate result that depends on `quick-gate` and every full-tier job and fails unless all required results are `success`.

The existing standalone `release-evidence.yml` remains manual and unchanged. It verifies an externally produced evidence bundle and is not folded into ordinary CI.

## Shared Toolchain Action

The local composite action accepts explicit boolean inputs for Go setup, Node/pnpm setup, and frozen workspace installation. It uses:

- `src/server/go.mod` and `src/server/go.sum` for the Go version and cache dependency;
- Node `20.19.0`, pnpm `10.6.0`, and `pnpm-lock.yaml` for the web toolchain;
- `pnpm install --frozen-lockfile` only when requested.

Each job must check out the repository before invoking the local action. Jobs that only run shell/Python fixtures or Compose parsing do not install unused toolchains.

## Reliability And Diagnostics

Every job has an explicit `timeout-minutes` value sized for its existing command set. The timeout is a failure, never a skip or success.

Go module/build caching, pnpm store caching, and the manifest-digest-keyed protobuf cache remain deterministic. Cache misses must affect duration only, not behavior.

The E2E job uploads Playwright reports and test results when it fails, with short retention. Artifact upload uses `if: failure()` and must not convert a failing test step into a successful job.

Job names and the two aggregate gate names are stable contracts. Branch protection should require `quick-gate` for development-branch policies where desired and `full-gate` for pull requests into protected branches.

## Security And Evidence Boundary

The workflow must:

- use read-only repository permissions;
- avoid `pull_request_target` and untrusted privileged execution;
- avoid production, Provider, payment, Kubernetes, signing, or target credentials;
- preserve strict database-test requirements in the full tier;
- keep every environment-dependent absence visible as a failure or as work owned by the manual target-evidence workflow;
- never label fixture success as E3/E4 or commercial-release proof.

## Repository Contract Updates

`scripts/verify-quality-gates.sh` will be extended to prevent regressions in the CI structure. Its focused assertions will require:

- the facade, reusable workflow, and composite action files;
- `push`, `pull_request`, `workflow_dispatch`, `workflow_call`, and the boolean `run_full` contract;
- read-only permissions and concurrency cancellation;
- stable `quick-gate` and `full-gate` jobs;
- PostgreSQL-backed strict server testing, security checks, E2E, release gates, and target-evidence fixtures;
- Playwright failure-artifact upload;
- no `pull_request_target` trigger.

These structural assertions complement GitHub's workflow parser; they do not replace executing the workflow on GitHub.

## Verification Plan

Before committing implementation:

1. Parse both default and microservices Compose profiles.
2. Run focused CI contract assertions and `git diff --check`.
3. Run the repository docs/quality gate that owns workflow structure.
4. Run the current backend compile and frontend production build commands.
5. Inspect the implementation diff to prove unrelated user changes were not staged.
6. Push the CI commits and inspect the resulting development-branch quick run.
7. Open or update a pull request and inspect the full-tier run before treating `full-gate` as enforceable branch-protection evidence.

Local checks can prove structure and commands. Only the GitHub-hosted runs prove event classification, reusable-workflow invocation, cache behavior, aggregate check names, and uploaded failure artifacts.

## Non-Goals

- Replacing the release-evidence workflow.
- Deploying to Docker, Kubernetes, staging, or production from ordinary CI.
- Adding fake Provider or payment credentials.
- Changing application source code or Phase 31.2 product behavior.
- Claiming that CI success closes target/live commercial readiness.

# Commercial Final Program Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert Oblivious from the current aggregate-server-plus-partial-services state into the approved 17-service commercial release, proven across global SaaS, China SaaS, self-hosted Kubernetes, and air-gapped profiles.

**Architecture:** Freeze governance and shared contracts before service implementation, then execute independent service worktrees against immutable F0/F1 baselines. Infrastructure profiles consume the same image, chart, contract, and migration digests; the release integrator accepts only real target-environment evidence tied to one RC digest.

**Tech Stack:** Go 1.25, PostgreSQL, Kafka, ClickHouse, Qdrant, Redis/Valkey, S3-compatible object storage, OpenAPI, Protobuf/gRPC, React/TypeScript, Playwright, Helm, Kubernetes, OpenTelemetry, GitHub Actions, Cosign, SBOM/provenance tooling.

---

## Source Of Truth

- Approved design: `docs/superpowers/specs/2026-07-10-commercial-final-microservices-design.md`
- Repository audit: `docs/audit/2026-07-10-full-repository-scan.md`
- Module gap matrix: `docs/audit/2026-07-10-module-capability-gap-matrix.md`
- Module reference design: `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md`
- Current release boundary: `docs/release/commercial-completion-audit.md`

The current untracked paths `scripts/run-target-release-evidence.sh`, `scripts/run-target-release-evidence-fixtures.sh`, and `src/server/internal/outboundpolicy/` are outside this planning commit and must not be staged by plan workers.

## Plan Set

1. `docs/superpowers/plans/2026-07-10-commercial-final-architecture-governance-plan.md`
2. `docs/superpowers/plans/2026-07-10-commercial-final-contract-freeze-plan.md`
3. `docs/superpowers/plans/2026-07-10-commercial-final-deployment-profiles-plan.md`
4. `docs/superpowers/plans/2026-07-10-commercial-final-service-delivery-plan.md`

## Specification Coverage Matrix

| Approved design section | Implementing plan and tasks |
|---|---|
| 1. Purpose | Master Tasks 1-6 and the program completion definition |
| 2. Binding Decisions | Governance Tasks 2-6; Contract Tasks 3-7; Profile Tasks 1-10 |
| 3. Scope Control | Service Tasks 1-5 and explicit V1 golden journeys |
| 4. Target Service Architecture | Governance Tasks 2-5; Service Worktree Contract and Tasks 1-4 |
| 5. Communication and Consistency | Contract Tasks 2-5 and 7 |
| 6. Relay and Billing Authority | Contract Task 7; Service Task 2 |
| 7. Product Completion Boundary | Service Tasks 1-5 and Common Service DoD |
| 8. Regional and Commercial Architecture | Profile Tasks 4-6; Service Task 2 and Task 4 |
| 9. Deployment Profiles | Profile Tasks 1-10 |
| 10. Security and Compliance | Governance Tasks 5-6; Profile Task 5; Service Task 6 |
| 11. Reliability and Operations | Profile Tasks 7-9; Service Tasks 5-6 |
| 12. Engineering and Agent Governance | Master shared leases/worktrees; Governance Tasks 3-7; Service Task 7 |
| 13. Program Phases and Gates | Master execution order and Tasks 1-6 |
| 14. Testing and Evidence | Contract Tasks 1-8; Profile Tasks 1 and 7-10; Service Tasks 5-6 |
| 15. Final No-Skip Release Gate | Master Task 6; Profile Task 10; Service Task 8 |
| 16. Definitions of Done | Master completion definition; Service Common DoD and Task 8 |
| 17. Primary Risks | Governance lease/CODEOWNERS controls; Profile drift/recovery controls; Service NFR gates |
| 18. Next Design Artifacts | Governance ADR/ownership artifacts; Contract baselines; Profile chart/manifests; Service golden journeys |

## Execution Order

| Phase | Required plan | Maximum writers | Exit condition |
|---|---|---:|---|
| F0-A | Architecture governance | 3 | ADR-013, 17-service ownership, CODEOWNERS, lease/RFC policy, governance gate |
| F0-B | Shared contract freeze | 3 | One OpenAPI source, one Proto source, event envelope, database ownership, F0 baseline |
| F1 | Service contract freeze | 6 | All 17 provider/consumer contracts and generated clients pass compatibility gates |
| F2 | Foundation and profiles | 6 | Foundation services plus four deployment profiles run from one digest lock |
| F3 | AI runtime and commerce authority | 6 | Relay/Billing authority chain, RAG, MCP, Sandbox reach functional parity |
| F4 | Product runtime | 6 | Chat, Agent, Workflow, Task, Channel, Marketplace, Admin reach parity |
| F5 | NFR and recovery | 6 | Race, load, soak, chaos, security, upgrade, restore, and air-gap evidence pass |
| F6 | RC | 1 writer, 2 reviewers | One RC digest passes the final no-skip commercial verifier |

## Shared Write Leases

The following paths require an active human-approved lease and a single writer:

```text
docs/api/openapi.yaml
api/openapi/common/**
api/proto/oblivious/common/**
api/proto/oblivious/events/**
contracts/**
src/server/migrations/microservices/table-ownership.json
src/server/Makefile
src/web/src/generated/**
pnpm-lock.yaml
.github/CODEOWNERS
.github/workflows/**
deploy/helm/oblivious/values.yaml
deploy/helm/oblivious/values.schema.json
deploy/profiles/**
scripts/verify-commercial-completion.sh
scripts/check.sh
```

Each lease records `scope`, `issue`, `holder`, `worktree`, `baseCommit`, `grantedBy`, `grantedAt`, `expiresAt`, and exact paths. The default duration is four hours; renewal requires a new human approval.

## Worktree Layout

Use `superpowers:using-git-worktrees` before implementation. Worktrees live outside the repository root:

```bash
mkdir -p ../Oblivious-wt
git worktree add ../Oblivious-wt/f0-governance -b program/f0-governance 3531446
git worktree add ../Oblivious-wt/f0-contracts -b program/f0-contracts 3531446
git worktree add ../Oblivious-wt/f2-profiles -b program/f2-profiles 3531446
```

Expected: each worktree starts at `3531446`; `git status --short` shows no plan-worker changes. Existing untracked files in the original checkout remain untouched.

## Task 1: Complete F0 Architecture Governance

**Files:** Follow `docs/superpowers/plans/2026-07-10-commercial-final-architecture-governance-plan.md`.

- [ ] **Step 1: Create the governance worktree**

Run:

```bash
git worktree add ../Oblivious-wt/f0-governance -b program/f0-governance 3531446
```

Expected: branch `program/f0-governance` is created from the approved design commit.

- [ ] **Step 2: Execute every governance-plan checkbox in order**

Expected: the governance verifier reports exactly 17 services, four domain units, eight lease domains, and no duplicate table owner.

- [ ] **Step 3: Merge only after independent review**

Run:

```bash
bash scripts/check.sh docs
git diff --check 3531446..HEAD
```

Expected: both commands exit `0`; the diff contains only governance files listed by the subplan.

## Task 2: Freeze Shared And Service Contracts

**Files:** Follow `docs/superpowers/plans/2026-07-10-commercial-final-contract-freeze-plan.md`.

- [ ] **Step 1: Rebase the contract worktree on the merged governance commit**

```bash
git -C ../Oblivious-wt/f0-contracts rebase program/f0-governance
```

Expected: clean rebase; governance ownership and lease files are present.

- [ ] **Step 2: Complete F0 shared contract tasks**

Expected: OpenAPI bundle, Proto descriptor, event catalog, and database manifest produce deterministic baselines under `contracts/baselines/f0/`.

- [ ] **Step 3: Complete F1 service contract tasks in dependency order**

Order:

```text
Foundation
Relay measured usage -> Billing reservation/settlement
Knowledge RAG -> Tool MCP -> Sandbox
Chat -> Agent -> Workflow -> Task Scheduler -> Channel
Marketplace -> Admin Console
```

Expected: `contracts/baselines/f1/` is created only after all generated clients and consumer contract tests pass.

## Task 3: Build Four Deployment Profiles

**Files:** Follow `docs/superpowers/plans/2026-07-10-commercial-final-deployment-profiles-plan.md`.

- [ ] **Step 1: Start profile work after F0 schemas merge**

Profile workers may create Helm templates while service implementations continue, but they must consume the frozen service catalog and may not change API, Proto, event, or database contracts.

- [ ] **Step 2: Produce one immutable digest lock**

Expected files:

```text
release/manifests/rc/images.lock.yaml
release/manifests/rc/chart.lock.yaml
release/manifests/rc/contracts.lock.json
```

- [ ] **Step 3: Pass all profile conformance checks**

```bash
helm lint deploy/helm/oblivious
bash scripts/verify-profile-conformance.sh --all
bash scripts/verify-airgap-bundle.sh --deny-network
```

Expected: all four profiles resolve the same application digests, chart interface, contract hashes, and migration set.

## Task 4: Deliver Seventeen Services In Batches

**Files:** Follow `docs/superpowers/plans/2026-07-10-commercial-final-service-delivery-plan.md`.

- [ ] **Step 1: Open no more than six service worktrees**

Each worktree owns one service entrypoint, domain implementation, private migrations, tests, image, and service template. Shared files are excluded.

- [ ] **Step 2: Require test-first vertical slices**

Every service change follows:

```text
provider contract test fails
consumer contract test fails
minimal implementation passes
database isolation test passes
service component test passes
golden journey passes
target evidence is captured
```

- [ ] **Step 3: Keep integration continuous**

At least daily, merge the queue onto the integration branch and run:

```bash
bash scripts/check.sh docs
bash scripts/check.sh server
bash scripts/test.sh server
bash scripts/check.sh web
bash scripts/test.sh web
```

Expected: mainline remains deployable; a red integration branch blocks new merges.

## Task 5: Execute NFR And Recovery Gates

**Files:** Created by the deployment and service-delivery subplans under `test/load/`, `test/chaos/`, `test/recovery/`, and `test/install/`.

- [ ] **Step 1: Run race and security gates**

```bash
(cd src/server && go test -race -count=1 ./...)
bash scripts/check.sh security
```

Expected: zero race reports and no unwaived Critical or High findings.

- [ ] **Step 2: Run load and soak gates**

```bash
k6 run test/load/commercial.js
k6 run test/load/soak-8h.js
```

Expected: commercial load thresholds in the service-delivery plan are met without data loss or ledger mismatch.

- [ ] **Step 3: Run chaos and recovery gates**

```bash
bash test/chaos/run.sh
bash test/recovery/j09_upgrade_restore.sh
bash test/install/j10_airgap_install.sh
```

Expected: recovery meets approved RPO/RTO, and the restored system reruns J02, J04, and J08 successfully.

## Task 6: Build And Approve The Release Candidate

**Files:** `release/manifests/rc/`, external target evidence workspace, and existing commercial verifier inputs.

- [ ] **Step 1: Freeze F2**

Only blocker fixes are allowed after the RC commit. Contract, migration, dependency-lock, deployment-interface, and release-gate changes require Release Commander approval.

- [ ] **Step 2: Run the final no-skip command**

```bash
COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
COMMERCIAL_COMPLETION_RUN_K8S=true \
COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=false \
OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=false \
TEST_DATABASE_URL="$RC_TEST_DATABASE_URL" \
OBLIVIOUS_TARGET_EVIDENCE_FILE="$EXTERNAL_EVIDENCE/target-release-evidence.json" \
OBLIVIOUS_TARGET_ARTIFACT_DIR="$EXTERNAL_EVIDENCE/artifacts" \
bash scripts/verify-commercial-completion.sh
```

Expected: exit `0`; no skip, fixture-only, commit mismatch, digest mismatch, missing restore, or missing live-provider evidence.

- [ ] **Step 3: Record human approvals**

Required signers: Product, Engineering, Security, Operations, Finance, Tax/Legal, and Release Commander. AI agents may prepare evidence but may not approve the release.

## Program Completion Definition

The program is complete only when all four subplans are fully checked, all 17 services satisfy Service DoD, all four profiles consume one RC digest set, and the final no-skip command exits `0` against external target evidence.

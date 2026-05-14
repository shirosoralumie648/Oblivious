# Retrospective: Oblivious

## Milestone: v03.1 - Admin and Marketplace UI

**Shipped:** 2026-05-02
**Phases:** 1 | **Plans:** 7

### What Was Built

- Admin and Marketplace API route exposure for UI consumption.
- Shared UI/API foundation for admin and marketplace pages.
- Admin dashboard, channels, routes, plans, users, audit log, and reviews.
- Marketplace browse/search, detail/install/review, publish, and my-agents workflows.
- UAT, security, Nyquist validation, and milestone audit artifacts.

### What Worked

- Phase-local gap closure plans let incomplete UAT items be finished without reopening the whole phase.
- Targeted frontend tests plus `tsc --noEmit` gave fast feedback for each UI slice.
- Backend route tests avoided requiring a live database while still proving handler registration and auth boundaries.
- Security and validation artifacts made the final milestone audit straightforward.

### What Was Inefficient

- `.planning/STATE.md` and `.planning/ROADMAP.md` drifted behind the actual phase artifacts.
- `REQUIREMENTS.md` traceability only covered Phase 1, so current Phase 3 requirements had to be reconciled manually.
- The legacy workspace marketplace page remained as an obsolete surface after `/marketplace` was rewired.

### Patterns Established

- Admin route groups are wrapped by `requireAdmin`; mixed marketplace route groups keep public reads public and wrap authenticated branches with `requireSession`.
- Frontend pages use typed API factories, shared UI primitives, and focused Vitest page tests.
- Nyquist validation maps every plan task to a concrete automated command before milestone close.

### Key Lessons

- In this repo, live phase artifacts are more reliable than the top-level STATE summary.
- When `gsd-sdk` is unavailable, a file-based fallback can still preserve the GSD gates if every generated artifact records concrete evidence.
- Audit debt should be separated from functional blockers; Phase 03.1 was shippable even though metadata cleanup remained.

## Milestone: v03.2 - Quality and Release

**Shipped:** 2026-05-14
**Phases:** 1 | **Plans:** 4

### What Was Built

- Backend release gate covering Admin, Marketplace, Relay, Agent, Memory, and Quota boundaries.
- Playwright browser E2E for Admin and Marketplace release workflows.
- API documentation, current system contracts, README release links, RC checklist, and docs quality-gate assertions.
- Dockerfiles, Docker compose stack, Kubernetes manifests, env examples, deploy smoke script, and deploy validation script.
- Restricted-network Docker compose validation path that built images, started the stack, and passed `/healthz`.

### What Worked

- Keeping DB-backed integration tests explicit behind `TEST_DATABASE_URL` allowed normal release gates to stay deterministic.
- Playwright route fixtures covered browser workflows without live provider, payment, or database dependencies.
- Release docs tied commands to concrete evidence, which made the final DEPLOY-01 closeout auditable.
- Registry and Go proxy overrides let deployment validation succeed without changing committed defaults.

### What Was Inefficient

- Docker daemon access, Docker Hub routing, Go module access, and missing `kubectl` required several rounds of environment diagnosis.
- `gsd-sdk roadmap.analyze` misread the collapsed roadmap/backlog shape during closeout, so milestone archive stats had to be corrected manually.
- The worktree remains broadly dirty, which makes milestone commit/tag creation unsafe without a separate cleanup or staging pass.

### Patterns Established

- Deployment gates should distinguish default-path failures from documented restricted-network success paths.
- DEPLOY-01 can be satisfied by one real runtime path; Kubernetes is an alternate path when Docker compose has already proven build/start/smoke.
- Living `.planning/REQUIREMENTS.md` should not be deleted in this repo until the backlog item about reset policy is resolved.

### Key Lessons

- Do not mark deployment complete from compose parsing or stub smoke; require a real stack startup and health check.
- Keep exact mirror/proxy env names in the runbook because they are part of the validated operator path.
- GSD closeout automation needs a manual sanity check when roadmap parsing returns impossible stats like 0 phases and 0 plans.

## Cross-Milestone Trends

| Theme | Observation |
|-------|-------------|
| State drift | Top-level planning files need explicit refresh after fast phase execution and after environment-blocker resolution. |
| Verification | Targeted commands plus one full release gate give better evidence than either narrow slices or broad scripts alone. |
| Dirty worktree handling | Workflow artifacts should be edited narrowly and unrelated changes left untouched; commit/tag steps need a clean staging plan. |
| Deployment | Runtime claims require real startup evidence; restricted-network overrides are valid only when recorded as exact commands. |

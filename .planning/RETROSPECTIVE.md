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

## Cross-Milestone Trends

| Theme | Observation |
|-------|-------------|
| State drift | Top-level planning files need explicit refresh after fast phase execution. |
| Verification | Small targeted commands are more reliable than broad scripts in this environment. |
| Dirty worktree handling | Workflow artifacts should be committed narrowly and unrelated changes left untouched. |

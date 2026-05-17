# Phase 8: Contract Docs and Release Verification - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md - this log preserves the alternatives considered.

**Date:** 2026-05-17
**Phase:** 08-contract-docs-and-release-verification
**Areas discussed:** Documentation hierarchy, verification evidence, worktree boundaries, wording and release level

---

## Documentation hierarchy

| Option | Description | Selected |
|--------|-------------|----------|
| Canonical docs split | Keep `docs/API.md` as the route index, `current-system-contracts.md` as the behavior contract, and `README.md` as the summary entrypoint. | ✓ |
| README-first flattening | Duplicate the route and release tables into `README.md` for one-stop reading. | |
| Loose split | Keep the current files but do not make their roles explicit. | |

**User's choice:** Auto-selected recommended default.
**Notes:** The docs should have clear roles instead of repeating the same surface in three places.

---

## Verification evidence

| Option | Description | Selected |
|--------|-------------|----------|
| Docs-first proof | Run `bash scripts/check.sh docs`, then targeted route and grep checks, and record any broader gates as confirmation only. | ✓ |
| Full re-prove everything | Rerun `check.sh all`, `test.sh all`, and deployment smoke as if Phase 7 had not already proven them. | |
| Prior-phase only | Cite Phase 7 evidence and skip new verification in Phase 8. | |

**User's choice:** Auto-selected recommended default.
**Notes:** Phase 8 still needs targeted verification, but it should not pretend the already-verified runtime smoke never happened.

---

## Worktree boundaries

| Option | Description | Selected |
|--------|-------------|----------|
| Keep cleanup separate | Leave historical/reference docs and frontend cleanup debt out of Phase 8 unless a later cleanup phase claims them. | ✓ |
| Absorb everything | Fold the root reference docs, theme/token churn, and Marketplace cleanup into the docs phase. | |
| Only touch docs | Ignore the non-docs files but also avoid naming them as deferred work. | |

**User's choice:** Auto-selected recommended default.
**Notes:** The phase should stay focused on contract docs and release verification, not on unrelated cleanup.

---

## Wording and release level

| Option | Description | Selected |
|--------|-------------|----------|
| Current-mainline wording | Describe the live consolidated mainline, while keeping v03.2 wording only for archived evidence and the already-validated deployment path. | ✓ |
| RC-only wording | Keep v03.2 release-candidate language everywhere for consistency with older artifacts. | |
| Rename everything to v03.3 | Rewrite all docs to use v03.3 terminology immediately, including archive references. | |

**User's choice:** Auto-selected recommended default.
**Notes:** The live docs should match the current state, but the archived evidence still matters.

---

## the agent's Discretion

- Exact sentence-level wording inside the docs.

## Deferred Ideas

- Historical/reference docs and frontend cleanup debt remain separate from Phase 8.

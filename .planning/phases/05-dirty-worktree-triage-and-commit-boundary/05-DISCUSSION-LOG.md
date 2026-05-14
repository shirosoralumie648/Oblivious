# Phase 5: Dirty Worktree Triage and Commit Boundary - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-14
**Phase:** 05-dirty-worktree-triage-and-commit-boundary
**Areas discussed:** Commit boundary policy, Source of truth, Work slice expectations, Verification boundary

---

## Commit Boundary Policy

| Option | Description | Selected |
|--------|-------------|----------|
| Planning-only commit first | Keep Phase 5 planning/inventory artifacts separate from source implementation commits. | ✓ |
| Commit all changed files together | Fast but mixes unrelated source, docs, deployment, and historical material. | |
| Defer all staging decisions | Leaves downstream phases without a stable boundary. | |

**User's choice:** Auto-selected planning-only commit first.
**Notes:** The current worktree is dirty and user-owned. The safe default is explicit path staging and no broad cleanup.

---

## Source of Truth

| Option | Description | Selected |
|--------|-------------|----------|
| `.planning` and active mainline are canonical | Trust current GSD planning files and active `src/server` / `src/web` / deployment surfaces. | ✓ |
| Root historical docs override planning | Risky because several root docs are older governance/reference material. | |
| Rebuild project state from all docs | Too broad for Phase 5 and likely to mix reference material into implementation. | |

**User's choice:** Auto-selected `.planning` and active mainline are canonical.
**Notes:** `lobehub/` and `new-api/` remain reference trees. Root `CURRENT_STATUS.md` and `ROADMAP.md` require explicit promotion before they can affect active planning.

---

## Work Slice Expectations

| Option | Description | Selected |
|--------|-------------|----------|
| Split by operational ownership | Backend integration, frontend/E2E, deployment/CI, contract docs, historical/reference docs. | ✓ |
| Split by tracked vs untracked only | Easy to compute but poor for implementation planning. | |
| Split by file extension | Too mechanical and ignores architecture boundaries. | |

**User's choice:** Auto-selected split by operational ownership.
**Notes:** This aligns with the v03.3 roadmap: Phase 6 handles backend, Phase 7 handles frontend/deployment, and Phase 8 handles docs/verification.

---

## Verification Boundary

| Option | Description | Selected |
|--------|-------------|----------|
| Non-destructive inventory checks only | Use status, diff, diff-check, and rg inventory checks in Phase 5. | ✓ |
| Run the full release suite now | Premature before the current work is grouped and planned. | |
| Skip verification until final release | Leaves commit-boundary mistakes undiscovered. | |

**User's choice:** Auto-selected non-destructive inventory checks only.
**Notes:** Full Go, frontend, E2E, and Docker checks belong to later phases after the work slices are planned.

---

## the agent's Discretion

- Choose the exact Phase 5 inventory filename and table format during planning.
- Decide whether to create one inventory artifact or a small set of inventory/checklist artifacts.
- Add lightweight read-only helper checks if they improve repeatability.

## Deferred Ideas

- Backend hardening belongs to Phase 6.
- Frontend/E2E/deployment alignment belongs to Phase 7.
- Contract docs and final verification evidence belong to Phase 8.

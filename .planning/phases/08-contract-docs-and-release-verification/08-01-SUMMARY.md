---
phase: 08-contract-docs-and-release-verification
plan: 01
status: passed
completed: 2026-05-17
requirements: [DOC-02]
---

# 08-01 Summary: Contract Docs Reconciliation

## Outcome

Passed. `README.md`, `docs/API.md`, `docs/architecture/current-system-contracts.md`, `docs/release/rc-checklist.md`, and `docs/release/deployment-runtime-remediation.md` now describe the consolidated v03.3 mainline and preserve the validated restricted-network deployment baseline.

## Files Changed

- `README.md`
- `docs/API.md`
- `docs/architecture/current-system-contracts.md`
- `docs/release/rc-checklist.md`
- `docs/release/deployment-runtime-remediation.md`

## Work Completed

- Made `docs/API.md` the canonical routed HTTP index and kept `GET /v1/models` in the not-routed note.
- Reframed `docs/architecture/current-system-contracts.md` to v03.3 mainline consolidation language and kept the live route/env matrix current.
- Kept `README.md` summary-level and pointed it to `docs/API.md` and `docs/release/rc-checklist.md`.
- Preserved the exact release gate and deployment remediation commands, including restricted-network overrides and `/healthz` smoke.

## Verification

```bash
rg -n "docs/API.md|docs/release/rc-checklist.md|RELAY_ENABLED|RELAY_DEFAULT_MODEL|OPENAI_API_KEY|OPENAI_BASE_URL|bash scripts/deploy-validate.sh|http://127.0.0.1:8080/healthz" README.md docs/API.md docs/architecture/current-system-contracts.md docs/release/rc-checklist.md docs/release/deployment-runtime-remediation.md
```

Passed. The canonical links, environment variables, and deployment commands are present.

```bash
bash scripts/check.sh docs
```

Passed. Release assets, docs/env consistency, and workspace boundary checks passed.

## Deviations from Plan

None.

## Deferred / Outside Scope

- Root `ROADMAP.md`, `CURRENT_STATUS.md`, `ARCHAEOLOGY_REPORT.md`, and `docs/superpowers/*` stayed out of this plan.
- Frontend cleanup debt in `src/web/src/theme/tokens.css`, `src/web/tailwind.config.ts`, and `src/web/src/routes/workspace/MarketplacePage.tsx` stayed outside Phase 8.

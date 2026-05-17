# Phase 8: Contract Docs and Release Verification - Context

**Gathered:** 2026-05-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 8 reconciles the live mainline contract docs with the code and command surface already shipped in Phases 5-7. It covers README, API index, architecture contract docs, release checklist, and deployment remediation notes, then proves the docs match current routes, env vars, and release gates.

This phase does not add product features or reopen the frontend, deployment, or runtime-proof slices from Phase 7. Historical and reference material stays out of scope unless a later cleanup phase explicitly promotes it.

</domain>

<decisions>
## Implementation Decisions

### Documentation hierarchy
- **D-01:** `docs/API.md` is the canonical routed HTTP index; it should remain exhaustive and mirror the live `src/server/internal/http/router.go` and `src/server/internal/relay/handler/router.go` surfaces.
- **D-02:** `docs/architecture/current-system-contracts.md` is the long-form behavior contract, not the route index. It should describe current live behavior, env contracts, and validation commands without drifting into future design.
- **D-03:** `README.md` should stay summary-level. It should point to `docs/API.md` and `docs/release/rc-checklist.md` instead of duplicating route tables.
- **D-04:** `docs/release/rc-checklist.md` remains the release gate ledger, and `docs/release/deployment-runtime-remediation.md` remains the host-specific remediation note. Do not merge them.
- **D-05:** Phase-facing prose should describe the current consolidated mainline. Keep v03.2 wording only where it refers to archived evidence or the already-validated deployment path.

### Verification evidence
- **D-06:** Phase 8 verification should be docs-first: `bash scripts/check.sh docs` plus targeted grep and route checks against the live code and script surfaces.
- **D-07:** If broader gates are rerun, record them as confirmation of already-shipped behavior rather than re-proving Phase 7 runtime smoke.
- **D-08:** `TEST_DATABASE_URL` absence is an intentional skip for `bash scripts/test.sh server`, not a failure.
- **D-09:** The validated restricted-network `scripts/deploy-validate.sh` command is the deployment proof baseline. It should be referenced verbatim in docs rather than replaced with a new ad hoc variant.
- **D-10:** If deployment smoke is mentioned in docs, keep the `/healthz` target and the restricted-network override form explicit.

### Worktree boundaries
- **D-11:** Keep historical/reference files (`CURRENT_STATUS.md`, root `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, `docs/superpowers/*`, and the root reference-report docs) out of the Phase 8 implementation commit unless a later decision explicitly promotes them.
- **D-12:** Keep unrelated frontend cleanup changes (`src/web/src/theme/tokens.css`, `src/web/tailwind.config.ts`, `src/web/src/routes/workspace/MarketplacePage.tsx`) out of this docs/release phase unless a separate cleanup phase claims them.
- **D-13:** The new architecture boundary docs for Knowledge and SOLO can be cited as supporting references, but Phase 8 should not expand them into new product scope.

### Agent's Discretion
- Exact sentence-level wording and ordering inside the docs may be tuned as long as the source-of-truth layering above stays intact.
- Whether Phase 8 verification uses only docs checks or also reruns full `check.sh all` / `test.sh all` is a planning choice, but any rerun must be recorded with skip reasons and evidence.

</decisions>

<specifics>
## Specific Ideas

- `docs/API.md` already enumerates auth, app, console, admin, marketplace, websocket, and relay routes aligned with `src/server/internal/http/router.go` and `src/server/internal/relay/handler/router.go`.
- `docs/architecture/current-system-contracts.md` still carries v03.2 release-candidate framing; it needs to be reconciled with v03.3 mainline consolidation and the live `config.go` and scripts contract.
- `docs/release/deployment-runtime-remediation.md` already records the validated restricted-network `deploy-validate.sh` command and the `/tmp/go-build/.../api-server` port 8080 issue; that note should stay factual and host-specific.
- `docs/architecture/knowledge-evolution-decision.md` and `docs/architecture/solo-runtime-decision.md` lock the Knowledge and SOLO boundaries, so the docs should not imply those capabilities have advanced beyond their current contract.
- The user-owned dirty docs and source edits outside `.planning` stay separate unless a later cleanup phase explicitly claims them.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Active planning
- `.planning/PROJECT.md` - v03.3 scope, target features, and core value.
- `.planning/REQUIREMENTS.md` - DOC-02 and VERIFY-01 definitions and traceability.
- `.planning/ROADMAP.md` - Phase 8 goal, success criteria, and likely verification commands.
- `.planning/STATE.md` - current milestone state, completed phases, and deferred items.

### Public docs to reconcile
- `README.md` - public summary, quality gates, and top-level navigation.
- `docs/API.md` - canonical routed HTTP index for the live surface.
- `docs/architecture/current-system-contracts.md` - long-form behavior and env contract baseline.
- `docs/architecture/knowledge-evolution-decision.md` - Knowledge scope boundary.
- `docs/architecture/solo-runtime-decision.md` - SOLO runtime boundary.
- `docs/release/rc-checklist.md` - release gate ledger and evidence shape.
- `docs/release/deployment-runtime-remediation.md` - validated restricted-network deployment remediation.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `src/server/internal/http/router.go` is the live app route registry for route comparison.
- `src/server/internal/http/route_surface_test.go` is the auth/admin route gate baseline.
- `scripts/check.sh` is the docs/env consistency gate.
- `scripts/test.sh` is the web/server test entry point and documents the intentional integration skip behavior.
- `scripts/deploy-validate.sh` is the validated deployment smoke command and its host-proxy failure handling.
- `src/web/src/app/router.tsx` is the frontend route tree if the README or contract docs need to mention browser entry points.
- `config/.env.example` and `src/server/internal/config/config.go` are the authoritative env var contract pair.
- `Dockerfile.web` and `docker-compose.yml` preserve `/api` and `/v1` proxying plus placeholder-only deploy env behavior.

### Established Patterns
- Docs checks are already enforced by `scripts/check.sh docs`; they read env names and script references with `rg`.
- `docs/API.md` should be the exhaustive route index while `current-system-contracts.md` summarizes contract behavior and env/defaults.
- Release docs should preserve exact command strings and skip reasons instead of paraphrasing them away.
- The validated restricted-network deploy command should stay verbatim so later proof can be copied directly from docs.

### Integration Points
- `docs/API.md` must match `src/server/internal/http/router.go` and `src/server/internal/relay/handler/router.go`.
- `docs/architecture/current-system-contracts.md` must match `config/.env.example`, `src/server/internal/config/config.go`, `scripts/check.sh`, and `scripts/test.sh`.
- `docs/release/rc-checklist.md` must match `scripts/deploy-validate.sh`, `docker-compose.yml`, and `Dockerfile.web`.
- `README.md` should point at the canonical docs rather than duplicating their route tables.
- Frontend route summaries should stay consistent with `src/web/src/app/router.tsx` if the README mentions browser entry points.

</code_context>

<deferred>
## Deferred Ideas

- `CURRENT_STATUS.md`, root `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, and `docs/superpowers/*` are reference material, not Phase 8 implementation scope.
- `src/web/src/theme/tokens.css`, `src/web/tailwind.config.ts`, and `src/web/src/routes/workspace/MarketplacePage.tsx` remain separate cleanup work.
- Future docs or cleanup phases can decide whether to promote the reference-report docs into the mainline docs set.

</deferred>

---

*Phase: 08-contract-docs-and-release-verification*
*Context gathered: 2026-05-17*

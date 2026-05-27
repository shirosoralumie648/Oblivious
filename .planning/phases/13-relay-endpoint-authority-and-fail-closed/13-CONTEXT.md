# Phase 13: Relay Endpoint Authority and Production Fail-Closed - Context

**Gathered:** 2026-05-28
**Status:** Ready for planning
**Source:** Current goal context, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`, live Relay router code

<domain>
## Phase Boundary

Phase 13 starts v05 Relay Billing Completeness. It owns the first enforceable Relay Authority boundary:
- classify every registered `/v1/*` endpoint as commercial-supported and billed, internal/admin-only, or disabled in production;
- enforce production fail-closed behavior for unsupported or partial endpoints before native, passthrough, file-proxy, or upstream-provider code can run;
- document the route table and disabled reasons so later billing, auth, audit, and settlement work has a stable surface.

This phase does not complete Stripe, Marketplace payouts, production orchestration, Agent workflow completeness, or final commercial readiness. It also does not finish all Relay billing semantics; Phase 15 owns settlement/refund depth.
</domain>

<current_state>
## Live Route Facts

Authoritative route registration is `src/server/internal/relay/handler/router.go`.

Current `getOpenAIRoutes()` returns 34 registered routes, although the comment says "35"; Phase 13 should settle this by adding tests against the actual route table and correcting comments/docs if needed.

Registered route groups:
- Chat / Responses: `POST /v1/chat/completions`, `POST /v1/responses`
- Realtime: `GET /v1/realtime`
- Embeddings: `POST /v1/embeddings`
- Images: `POST /v1/images/generations`, `POST /v1/images/edits`, `POST /v1/images/variations`
- Videos: `POST /v1/videos`
- Audio: `POST /v1/audio/speech`, `POST /v1/audio/transcriptions`, `POST /v1/audio/translations`
- Moderations / Legacy: `POST /v1/moderations`, `POST /v1/completions`
- Batch: `POST /v1/batch`, `GET /v1/batches`, `GET /v1/batches/:id`
- Files: `POST /v1/files`, `GET /v1/files`, `GET /v1/files/:id`, `DELETE /v1/files/:id`, `GET /v1/files/:id/content`
- Fine-tuning: `POST /v1/fine_tuning/jobs`, `GET /v1/fine_tuning/jobs`, `GET /v1/fine_tuning/jobs/:id`, `POST /v1/fine_tuning/jobs/:id/cancel`, `GET /v1/fine_tuning/jobs/:id/events`
- Assistants / Threads / Runs: `POST /v1/assistants`, `GET /v1/assistants`, `GET /v1/assistants/:id`, `POST /v1/threads`, `GET /v1/threads/:id`, `POST /v1/threads/:id/runs`, `GET /v1/threads/:id/runs/:rid`, `POST /v1/threads/:id/runs/:rid/submit`

Current docs list these routes in `docs/API.md` under "Relay /v1 Endpoints" but do not classify commercial status, auth/rate-limit/billing policy, audit behavior, or production disabled reasons.
</current_state>

<decisions>
## Phase 13 Decisions

### Initial commercial classes
- `commercial_supported_billed`: endpoints that Phase 13 may leave callable in production because they already have native handler paths that can be brought under v05 billing in Phase 15.
- `disabled_in_production`: endpoints that are passthrough, partial, async, file-mapping-incomplete, or otherwise not safe to expose commercially until later work supplies billing/audit/settlement semantics.
- `internal_admin_only`: reserved for `/v1/*` routes that are operational/admin-only. Current registered routes do not appear to be internal/admin-only, but the enum must exist because the commercial spec requires the class.

### Conservative initial policy
Commercial-supported in Phase 13 should be limited to native synchronous model endpoints that already use Relay routing paths:
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/embeddings`
- `POST /v1/images/generations`
- `POST /v1/images/edits`
- `POST /v1/images/variations`
- `POST /v1/audio/speech`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/translations`
- `POST /v1/moderations`
- `POST /v1/completions`

Disable in production until later policy/settlement work:
- `GET /v1/realtime` until streaming/realtime settlement is defined.
- `POST /v1/videos` until video billing and provider behavior are verified.
- Batch endpoints until async settlement and audit are defined.
- Files endpoints until file mapping persistence and storage/audit semantics are implemented.
- Fine-tuning endpoints until training-token billing and job lifecycle audit are implemented.
- Assistants/Threads/Runs endpoints until run/tool lifecycle and billing boundaries are implemented.

### Production detection
Use the existing `APP_ENV=production` convention. Relay handler tests can set a route policy option directly, but runtime enforcement should derive production behavior from server config or an explicit policy option rather than a hidden package global.

### Non-production compatibility
Development/test environments can continue registering routes for local compatibility, but production must reject disabled routes before handler code reaches provider adapters, passthrough helpers, or file proxy code.

### Documentation
Phase 13 should add or update route table documentation with:
- method/path;
- API type;
- strategy;
- commercial class;
- production status;
- disabled reason if applicable;
- owning future phase if not supported.
</decisions>

<canonical_refs>
## Canonical References

Downstream agents MUST read these before implementing.

### Commercial Objective
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md` - Relay Authority Gate and v05 scope.
- `.planning/PROJECT.md` - Current v05 project state.
- `.planning/REQUIREMENTS.md` - RELAY-08 and RELAY-09 requirements.
- `.planning/ROADMAP.md` - Phase 13 goal, success criteria, and verification expectations.
- `.planning/STATE.md` - Current execution position.

### Relay Code
- `src/server/internal/relay/handler/router.go` - Inbound `/v1/*` registration truth.
- `src/server/internal/relay/handler/common.go` - Shared passthrough helper that must not run for production-disabled routes.
- `src/server/internal/relay/handler/files.go` - File proxy/passthrough behavior; includes non-persistent file mapping placeholder.
- `src/server/internal/relay/handler/batch.go` - Async batch behavior and passthrough routes.
- `src/server/internal/relay/handler/assistants.go` - Assistants/Threads/Runs passthrough behavior.
- `src/server/internal/relay/handler/fine_tuning.go` - Fine-tuning passthrough behavior.
- `src/server/internal/http/server.go` and `src/server/internal/http/router.go` - Relay mounting and `APP_ENV=production` config convention.

### Docs And Gates
- `docs/API.md` - Current route list.
- `docs/release/commercial-gates.md` - Commercial gate contract.
- `scripts/verify-quality-gates.sh` - Existing docs quality assertions.
</canonical_refs>

<verification>
## Expected Verification

- Focused handler tests must fail before implementation when policy coverage is missing.
- Focused handler tests must pass after the route policy registry and production fail-closed wrapper are implemented.
- Docs checks must prove the route classification table exists and is referenced from the commercial gate docs.
- Phase 13 summary must explicitly leave RELAY-10, RELAY-11, BILL-01, BILL-02, and DOC-04 open.
</verification>

---
*Phase: 13-relay-endpoint-authority-and-fail-closed*
*Context gathered: 2026-05-28*

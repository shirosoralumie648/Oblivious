# Phase 14: Relay Provider Bypass and Cost-Abuse Guardrails - Context

**Gathered:** 2026-05-28
**Status:** Ready for execution planning
**Source:** Current goal context, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, Phase 13 summary, live Relay/app service code

<domain>
## Phase Boundary

Phase 14 owns `RELAY-10` and `RELAY-11` for v05 Relay Billing Completeness:
- prove app services cannot call upstream LLM providers outside Relay/channel adapter code;
- require trusted tenant/user identity before production-supported `/v1/*` endpoints can reach handlers/providers;
- attach route-level auth, tenant, rate-limit, and audit policy to every supported Relay endpoint class;
- preserve app-internal Chat, Agent, and Knowledge embedding paths by forwarding trusted Relay metadata.

This phase does not implement final quota settlement/refund (`BILL-01`, `BILL-02`), Stripe/payment operations, Marketplace payouts, Kubernetes proof, or final product completeness.
</domain>

<current_state>
## Live Facts

- Phase 13 added `src/server/internal/relay/handler/policy.go`, classifying all 34 registered `/v1/*` routes and production-disabling partial/passthrough routes.
- `src/server/internal/http/server.go` sends `/v1/*` directly to the Relay Gin engine, bypassing app session middleware. That is acceptable only if Relay performs its own production auth/tenant guard.
- `chat.RelayGateway` sends trusted internal headers when `chat.RelayRequestMetadata` exists in context.
- Chat and Agent HTTP handlers seed `chat.RelayRequestMetadata` before model calls.
- `memory.RelayEmbedder` calls `/embeddings` through Relay but currently does not forward trusted internal headers.
- `chat.HTTPReplyGenerator` still contains a direct provider-style `baseURL + "/chat/completions"` path and `src/server/internal/http/router.go` still wires it from `cfg.LLMBaseURL` for non-Relay or non-production fallback.
- Existing `relay.Router.SelectChannel` uses a token bucket before provider selection; Phase 14 should expose that as route policy evidence while Phase 15 owns billing settlement depth.
</current_state>

<decisions>
## Phase 14 Decisions

### Production Relay identity
Production-supported Relay routes require a trusted internal identity for this phase:
- `X-Oblivious-Internal-Auth` must equal `OBLIVIOUS_INTERNAL_AUTH_TOKEN` or the default shared internal token.
- `X-Oblivious-Internal-User-ID` must be present.
- `X-Oblivious-Internal-Organization-ID` must be present.

External customer API keys are not introduced in Phase 14. Until a tenant API-key model exists, commercial `/v1/*` provider access is app-internal only.

### Direct provider bypass
App services must not instantiate direct provider clients, call provider URLs, or use `LLM_BASE_URL` for AI calls. Direct upstream provider calls stay inside Relay/channel adapter code. Non-Relay chat fallback becomes demo-only rather than provider-backed.

### Audit semantics
Phase 14 audit is a route-policy decision event emitted before handler/provider execution:
- method/path/API type/class;
- user ID and organization ID when accepted;
- request ID when present;
- result `allowed` or `rejected`;
- failure reason for disabled or unauthenticated requests.

Persisted commercial audit storage can be extended later, but Phase 14 must create an injectable audit sink and tests proving the decision is emitted before provider execution.

### Rate-limit semantics
Supported route policies must declare the existing Relay global token-bucket guard as their rate-limit policy. Phase 15 will deepen per-endpoint billing/settlement and may replace or extend the token-bucket dimensions.
</decisions>

<canonical_refs>
## Canonical References

- `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-01-SUMMARY.md`
- `.planning/PROJECT.md`
- `.planning/REQUIREMENTS.md`
- `.planning/ROADMAP.md`
- `.planning/STATE.md`
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/router.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/chat/gateway.go`
- `src/server/internal/chat/relay_gateway.go`
- `src/server/internal/memory/embedder.go`
- `src/server/internal/memory/service.go`
- `scripts/check.sh`
- `scripts/verify-quality-gates.sh`
</canonical_refs>

<verification>
## Expected Verification

- RED tests prove supported routes currently lack enforced production identity/audit semantics.
- RED tests prove Memory Relay embeddings currently do not propagate trusted identity.
- RED tests prove direct provider fallback is still present.
- `bash scripts/check.sh relay-security` fails until direct-provider app-service paths are removed.
- After implementation:
  - `cd src/server && go test ./internal/relay/handler -run 'Policy|ProductionSupported|Identity|Audit' -count=1`
  - `cd src/server && go test ./internal/chat ./internal/memory -run 'HTTPReplyGenerator|RelayEmbedder|RelayIdentity' -count=1`
  - `bash scripts/check.sh relay-security`
  - `bash scripts/check.sh docs`
  - `git diff --check`
</verification>

---
*Phase: 14-relay-provider-bypass-and-cost-abuse-guardrails*
*Context gathered: 2026-05-28*

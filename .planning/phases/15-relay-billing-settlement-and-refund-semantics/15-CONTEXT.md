# Phase 15: Relay Billing Settlement and Refund Semantics - Context

**Gathered:** 2026-05-28
**Status:** Ready for execution planning
**Source:** Current goal context, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, Phase 13-14 artifacts, live Relay billing/router/quota code

<domain>
## Phase Boundary

Phase 15 owns `BILL-01` and `BILL-02` for v05 Relay Billing Completeness.

It must make the Phase 13-14 Relay Authority boundary billable:
- supported Relay calls pre-authorize quota before provider dispatch;
- successful supported calls settle exactly once per organization-scoped idempotency key;
- upstream failures, provider error responses, nil responses, missing required usage, and partial/aborted flows refund or avoid charge consistently;
- streaming/realtime, file, batch, and async endpoints have explicit settlement models or production-disabled evidence.

This phase does not implement Stripe checkout/webhooks, subscription lifecycle, Marketplace payouts, Kubernetes proof, RAG upgrade, Agent workflow expansion, or final commercial readiness. Those remain v06-v08 work.
</domain>

<current_state>
## Live Code Facts

Authoritative billing lifecycle code:
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/billing.go`
- `src/server/internal/quota/service.go`

Current `RouteWithBilling` behavior:
- creates a `BillingSession` with API type, model, idempotency key, trusted user ID, and trusted organization ID;
- passes the caller-provided `channelID`, but supported handlers currently pass `""` because the router selects the channel later;
- does not set `RequestID` on the billing session from trusted context;
- calls `PreBill` before provider routing;
- calls `PostBill` only when `err == nil`, `resp != nil`, and `resp.Usage != nil`;
- refunds only when `err != nil`;
- ignores `PostBill` and `Refund` errors;
- treats provider non-2xx responses without `err` as neither settled nor refunded if usage is absent;
- treats success responses with missing `Usage` as neither settled nor refunded.

Current `BillingHook` behavior:
- uses `seenIdem map[string]bool` for in-memory idempotency guards;
- the pre-bill duplicate path returns the passed session's `PreAuthorizedAmt`, which is zero for a fresh duplicate request and does not copy the prior `QuotaSessionID`;
- `PostBill` duplicate idempotency returns the passed session's `SettledAmt`, which is zero for a fresh duplicate request;
- quota-backed PreConsume/Settle/Refund exists through `QuotaManager`, but hook-level idempotency must preserve billing session context if it suppresses a duplicate quota call.

Current `quota.Service` behavior:
- `PreConsume` is organization-scoped and idempotent by `(organization_id, idempotency_key)`;
- migrations add a unique partial index on `(organization_id, idempotency_key)` where idempotency key is non-empty;
- `Settle` delegates to store settlement, which refunds the preauthorized difference;
- `Refund` delegates to store refund, which restores the preauthorized amount;
- duplicate Settle/Refund currently return errors through the store if a session is already settled/refunded.

Supported Phase 14 route policy facts:
- production-supported billed endpoints require trusted internal auth, trusted user ID, trusted organization ID, rate-limit policy, and route-decision audit.
- production-disabled endpoints already fail closed before handler/provider dispatch.
</current_state>

<decisions>
## Phase 15 Decisions

### Settlement policy
Supported synchronous endpoints should use a single policy vocabulary:
- `preauthorize_then_settle_usage`: pre-authorize estimated usage, settle provider-reported usage on 2xx success, refund if provider fails or required usage is absent.
- `preauthorize_then_settle_estimate`: pre-authorize estimated usage, settle estimated usage on 2xx success for endpoints where the provider response does not return structured usage yet.
- `production_disabled`: no commercial charge path because production rejects before handler/provider execution.

Initial supported endpoint policy:
- Chat, Responses non-streaming, Embeddings, and Legacy Completions require usage settlement. Missing usage after an apparently successful provider response is a billing integrity failure and must refund.
- Images, Audio, and Moderations can settle the request estimate for Phase 15 because current handlers do not parse provider usage. They must still refund provider error responses and nil responses.
- Responses streaming and Chat streaming are not commercially proven by the current handler code. Phase 15 should either reject the streaming path in production with documented reason or keep the route production-enabled only when the handler has tested settlement. The narrow safe choice for Phase 15 is production-disable unsupported streaming/realtime behavior until a tested stream accumulator exists.
- Realtime, Batch, Files, Fine-tuning, Assistants, Threads, and Runs remain production-disabled with documented reasons unless Phase 15 adds a full tested model. This plan keeps them production-disabled.

### Error and refund policy
- Provider responses with `StatusCode >= 400` count as failed calls for quota settlement and must refund the preauthorization.
- `nil` provider response after dispatch counts as a failed call and must refund.
- Missing required usage counts as a failed settlement and must refund.
- Refund failures must be returned to the caller as billing errors, not swallowed.
- Settlement failures must be returned to the caller as billing errors, not swallowed.

### Channel and request identity
- Billing sessions must capture the actual selected channel ID from `RouteWithBilling` after channel selection.
- Billing sessions must capture trusted request ID from Relay context when available.
- Timeout/polling tasks must keep carrying channel, quota session, user, organization, and idempotency context.

### Idempotency
- Exactly-once means no double quota charge or double quota settlement per organization-scoped idempotency key.
- If hook-level in-memory idempotency suppresses a duplicate call, it must copy the prior preauthorization/session context into the new billing session.
- If quota-backed idempotency returns an existing session, `BillingHook` must copy the returned quota session ID, preauthorized amount, status, and settled amount into the Relay billing session.
</decisions>

<canonical_refs>
## Canonical References

Downstream agents MUST read these before implementing.

### Commercial Objective
- `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`
- `.planning/PROJECT.md`
- `.planning/REQUIREMENTS.md`
- `.planning/ROADMAP.md`
- `.planning/STATE.md`

### Phase 13-14 Authority
- `.planning/phases/13-relay-endpoint-authority-and-fail-closed/13-CONTEXT.md`
- `.planning/phases/14-relay-provider-bypass-and-cost-abuse-guardrails/14-CONTEXT.md`
- `docs/release/relay-route-table.md`
- `docs/release/commercial-gates.md`
- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/router.go`

### Billing Code
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/router_test.go`
- `src/server/internal/relay/billing.go`
- `src/server/internal/relay/billing_test.go`
- `src/server/internal/relay/billing_worker.go`
- `src/server/internal/quota/service.go`
- `src/server/internal/quota/service_test.go`
- `src/server/internal/relay/types/types.go`

### Supported Handler Code
- `src/server/internal/relay/handler/chat.go`
- `src/server/internal/relay/handler/responses.go`
- `src/server/internal/relay/handler/embeddings.go`
- `src/server/internal/relay/handler/images.go`
- `src/server/internal/relay/handler/audio.go`
- `src/server/internal/relay/handler/moderations.go`
- `src/server/internal/relay/handler/completions.go`
</canonical_refs>

<verification>
## Expected Verification

- RED tests must first prove current `RouteWithBilling` does not refund provider error responses, nil responses, or missing required usage, and does not propagate settlement/refund errors.
- RED tests must prove duplicate idempotency with fresh sessions preserves billing/quota session context.
- GREEN tests must prove selected channel ID and trusted request ID are recorded in the billing lifecycle.
- Policy tests and docs checks must prove every route has an explicit Phase 15 billing/settlement policy.
- Phase 15 summary must close only `BILL-01` and `BILL-02`, keep `DOC-04` open for Phase 16, and keep v06-v08 visible.
</verification>

---
*Phase: 15-relay-billing-settlement-and-refund-semantics*
*Context gathered: 2026-05-28*

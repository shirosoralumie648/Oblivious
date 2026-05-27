---
phase: 15-relay-billing-settlement-and-refund-semantics
plan: 01
status: complete
completed_at: 2026-05-28T09:10:00+08:00
requirements: [BILL-01, BILL-02]
---

# Phase 15 Plan 01 Summary

## Result

Implemented the third v05 Relay Authority boundary.

Delivered:
- `RouteWithBilling` now selects the actual channel before preauthorization, records the selected channel on the billing session, and carries trusted request ID into billing context.
- Supported calls preauthorize before provider dispatch, settle successful calls, and refund provider errors, nil provider responses, missing required usage, and upstream errors.
- Settlement and refund errors now return explicit `RouterError` values instead of being swallowed.
- `BillingHook` now keeps organization-scoped idempotency snapshots so fresh duplicate sessions copy the original preauthorization, quota session, settlement, and refund context.
- Provider response usage is parsed from upstream JSON bodies for usage-settled endpoints.
- `OpenAIAdapter.EstimateUsage` now returns non-nil estimates for supported commercial surfaces, and default pricing covers supported billing dimensions.
- Route policy now declares explicit billing policies:
  - `preauthorize_then_settle_usage`
  - `preauthorize_then_settle_estimate`
  - `production_disabled`
- Chat and Responses streaming now return `streaming_settlement_not_supported` instead of bypassing billing.
- `docs/release/relay-route-table.md`, `docs/release/commercial-gates.md`, and docs quality gates now include Phase 15 billing policy evidence.

## Verification

Passed:
- RED before implementation: `cd src/server && go test ./internal/relay -run 'RouteWithBilling_Refunds|RouteWithBilling_Returns|RouteWithBilling_Records|BillingHook_Duplicate.*FreshSession' -count=1`
  - Failed as expected because provider error/nil responses were not refunded, settlement/refund errors were swallowed, selected channel was not recorded, and duplicate idempotency sessions lost quota context.
- RED before implementation: `cd src/server && go test ./internal/relay/handler -run 'BillingSettlementPolicy|RoutePoliciesDeclareBilling' -count=1`
  - Failed as expected because route policies had no `BillingPolicy`.
- RED before implementation: `cd src/server && go test ./internal/relay/channel -run 'EstimateUsage' -count=1`
  - Failed as expected because `OpenAIAdapter.EstimateUsage` returned nil.
- RED before implementation: `cd src/server && go test ./internal/relay/handler -run 'ProviderResponseFromHTTP' -count=1`
  - Failed as expected because provider usage parsing did not exist.
- GREEN: `cd src/server && go test ./internal/relay/channel -run 'EstimateUsage' -count=1`
- GREEN: `cd src/server && go test ./internal/relay/handler -run 'ProviderResponseFromHTTP|ChatStreaming|ResponsesStreaming|BillingSettlementPolicy|RoutePoliciesDeclareBilling' -count=1`
- GREEN: `cd src/server && go test ./internal/relay -run 'DefaultPricingCovers|RouteWithBilling|BillingHook_Duplicate.*FreshSession' -count=1`
- Final focused gate: `cd src/server && go test ./internal/relay/... ./internal/quota -run 'Billing|Settlement|Refund|Idempotency|Streaming|RoutePolicies|ProviderResponse|EstimateUsage|DefaultPricing' -count=1`
- `cd src/server && go test ./internal/relay/... -count=1`
- `cd src/server && go test ./internal/quota -count=1`
- `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `bash scripts/check.sh relay-security`
- `bash scripts/check.sh server`
- `git diff --check`

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| BILL-01 | `router.go`, `billing.go`, billing/router tests, provider usage parsing tests, adapter estimate tests, and quota tests prove preauthorization, exactly-once idempotency, settlement, and refund behavior |
| BILL-02 | `policy.go`, policy tests, `responses.go`, route table docs, and docs quality gates prove streaming/realtime/file/batch/async routes have explicit settlement policies or production-disabled evidence |

## Residual v05 Work

- DOC-04 remains required: Phase 16 must close v05 with reproducible Relay route table, endpoint policy, and verification evidence.

## Remaining Commercial Program Work

- v06 remains required for Stripe checkout/webhooks, subscriptions, top-ups, refunds, invoices, Marketplace settlement, platform fees, payouts, and moderation workflows.
- v07 remains required for production orchestration, backup/restore smoke, observability, alerts, dashboards, and runbooks.
- v08 remains required for product completeness, public docs, onboarding, pricing, and final commercial journeys.

## Next Phase Readiness

Phase 16 should close v05 with evidence only after the final verification sweep passes. It must not claim final commercial readiness.

---
*Summary written: 2026-05-28*

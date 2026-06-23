# Oblivious Pricing Model

`PROD-05` pricing surface.

This document explains the implemented pricing and billing model. It does not set production prices, currency strategy, tax treatment, payout schedule, or provider-specific commercial terms. Operators configure those values through Admin plans and payment-provider settings outside committed secrets.

## Billing Units

| Unit | Implemented behavior |
| --- | --- |
| Subscription | Subscription lifecycle records model active, failed-payment, plan-change, and cancellation states for organization billing. |
| Top-up | Top-up orders can add quota after payment-backed fulfillment. |
| Quota | Relay usage preauthorizes quota, settles exactly once per idempotency key, and refunds failed or partial calls according to route billing policy. |
| Invoice | Invoice state is recorded for admin inspection and release evidence. |
| Refund | Refund state can reverse quota and preserve audit evidence. |
| Marketplace settlement | Paid Marketplace installs create order and settlement records, including publisher revenue, platform fee, payout state, and refund impact. |

## Relay Usage

All provider-facing AI usage is priced through Relay. Chat, Agent, Knowledge embeddings, and supported `/v1/*` Relay endpoints use the same authority boundary for identity, rate limiting, audit, quota preauthorization, settlement, and refund.

Relay pricing settings include model and trusted user-group multipliers. Model multipliers adjust configured model prices, while user-group multipliers apply to the trusted Relay user group carried by internal identity headers or Relay API token identity. The group multiplier affects Relay quota preauthorization, API-token quota preauthorization and settlement, BillingHook pre/post billing, and usage cost records; channel cost multipliers are applied after the group adjustment.

Routes that are not commercially supported fail closed in production instead of bypassing billing. Supported commercial routes are documented in `docs/release/relay-route-table.md`.

## Plan Configuration

Admins configure available plans through Admin Plans. A plan should define:

- Customer-visible plan name.
- Quota allowance or entitlement.
- Renewal or one-time top-up behavior.
- Operational limits such as rate-limit policy and supported model/channel access.
- Refund and downgrade policy owned by the deployment operator.

Committed documentation must not contain live Stripe keys, provider keys, payout credentials, or hard-coded production prices.

## Marketplace Pricing

Marketplace agents can be free or paid according to the implemented review and settlement model:

- Free installs can be installed directly after marketplace approval.
- Paid installs remain checkout-backed and settlement-aware.
- Publisher revenue, platform fee, payout state, refund impact, governance events, and abuse workflows are visible to Admin and publisher surfaces.

## Current Completion Boundary

Phase 29 closes only `PROD-05` pricing documentation alignment. Phase 30 still must prove subscription, top-up, bill usage, admin billing inspection, Marketplace paid install, settlement, refund impact, deploy, backup, and restore journeys with current evidence.

`no-final-readiness`: this pricing model is not a final commercial readiness claim until Phase 30 and `AUDIT-01` pass.

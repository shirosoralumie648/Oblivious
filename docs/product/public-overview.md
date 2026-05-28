# Oblivious Public Product Overview

`PROD-05` documentation surface.

Oblivious is a multi-tenant AI SaaS platform that combines LobeHub-style C-end Chat and Agent experience with New-API-style B-end channel, billing, and operations controls. The product is designed for organizations that need governed AI workspaces, commercial usage accounting, and operator-visible evidence before deployment.

## Product Surfaces

| Surface | Implemented behavior |
| --- | --- |
| Chat | Authenticated workspace conversations, message history, model configuration, Knowledge binding, SOLO handoff, quota-aware errors, and Relay-backed model access. |
| Agent and SOLO | Durable `agent_runs` and `agent_tool_runs`, memory evidence, approval-required tool pauses, reject/retry APIs, commercial run readiness, budget context, and visible failure recovery. |
| Knowledge | Relay-produced embeddings, pgvector chunk retrieval, `embedding_rag` metadata, and source citations for commercial RAG behavior. |
| MCP tools | `calculator` and `datetime` are real default commercial built-ins. `web_search` and `http_request` are disabled by default until a real provider or tenant-safe outbound policy is configured. |
| Relay | The only provider-facing AI entrypoint. Chat, Agent, Knowledge embeddings, and external `/v1/*` surfaces use Relay for billing, rate limiting, auditing, and monitoring. |
| Admin | Channels, routes, plans, billing inspection, users, audit logs, and Marketplace review queues are routed commercial operations surfaces. |
| Marketplace | Browse, publish, review, install, owner stats, governance events, paid-install order handling, settlement, platform fee, payout state, and refund impact evidence. |
| Operations | Compose validation, Kubernetes manifest validation path, backup/restore smoke, observability artifacts, alert rules, dashboards, SLOs, release/rollback, incident, and disaster recovery runbooks. |

## Tenant And Relay Boundary

Organizations are first-class tenants. Tenant-scoped data and admin operations must carry organization identity, and cross-tenant reads or writes are denied by service and HTTP tests.

All AI calls must go through Relay. Production application services must not call upstream provider SDKs or provider URLs directly. Relay classifies `/v1/*` routes, fails closed for disabled production endpoints, enforces identity and rate-limit policy, records audit decisions, and settles quota through billing policy.

## Commercial Billing Boundary

The implemented commercial model supports subscription lifecycle records, top-up orders, quota preauthorization/settlement/refund, invoices, payment intent metadata, Stripe webhook ledger events, admin billing inspection, and Marketplace settlement/payout state.

This overview does not define production price amounts. Pricing is configured through Admin plans and deployment-specific payment provider settings; see `docs/product/pricing.md`.

## Current Completion Boundary

Phase 29 closes only `PROD-05`: public docs, onboarding, pricing, and operator guide alignment. Phase 30 still must prove end-to-end commercial journeys and `AUDIT-01`.

`no-final-readiness`: this repository must not claim final commercial readiness until Phase 30 verification maps every commercial gate to current evidence.

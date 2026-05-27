# Phase 10 Discussion Log

**Date:** 2026-05-28
**Mode:** Auto-locked defaults from commercial complete spec, Phase 9 output, and current codebase evidence.

## Locked Decisions

1. Phase 10 proves TENANT-02, TENANT-03, SEC-01, SEC-02, and SEC-03 only.
2. Organization roles are `owner`, `admin`, and `member`; these are separate from the existing global `users.role`.
3. Every active organization must have exactly one active owner after Phase 10 membership mutations.
4. Membership, invitation, removal, role change, ownership transfer, and acceptance flows must write audit events.
5. Cookie-authenticated mutating routes require CSRF protection using the existing signed session-cookie boundary.
6. Auth and sensitive admin/organization operations need enforced server-side rate limits, not UI-only throttling.
7. Password policy and session rotation/revocation are part of the security foundation; user-facing polish is deferred.
8. Tenant-scoping Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace data remains Phase 11.

## Reasoning

The final target is a directly deployable commercial SaaS product. Phase 9 created first-class organizations, but organizations are still not commercially meaningful until users can hold auditable memberships and sensitive auth routes fail closed. Phase 10 therefore adds enforceable identity and auth controls without migrating domain data prematurely.

## Deferred

- Cross-domain tenant data filters and denial tests are Phase 11.
- Relay endpoint billing completeness is v05.
- Stripe, invoices, top-ups, refunds, and Marketplace settlement are v06.
- Kubernetes/equivalent production runtime proof is v07.
- Product polish, durable Agent workflows, and RAG upgrade are v08.

---

*Phase: 10-membership-roles-and-auth-security*

# Oblivious Onboarding Guide

`PROD-05` onboarding surface.

This guide describes how implemented Oblivious behavior is introduced to customers, admins, publishers, and operators. It is aligned with the current multi-tenant commercial platform and keeps Phase 30 journey proof open.

## Customer Onboarding

1. Register or log in through `/register` or `/login`.
2. Complete workspace onboarding and enter the authenticated workspace.
3. Use Chat from `/chat`, choose available models, bind Knowledge when needed, and expect all AI calls to go through Relay.
4. Create or use Agents from the Agent/SOLO surfaces. Durable run state records execution, tool calls, approval boundaries, memory evidence, budget context, and retry/failure state.
5. Add Knowledge bases and documents from `/knowledge`. Retrieval uses Relay embeddings, pgvector RAG, and source citations.
6. Install Marketplace agents from `/marketplace` after reviewing free or paid install boundaries.
7. Monitor quota and billing signals surfaced by workspace and Admin views.

## Organization And Admin Onboarding

1. Create or select the organization tenant.
2. Invite members and assign roles through the implemented membership and admin surfaces.
3. Configure Relay channels in Admin Channels.
4. Configure model routes in Admin Routes.
5. Configure quota and subscription plans in Admin Plans.
6. Inspect billing sessions, payment intents, webhook events, subscriptions, top-ups, invoices, refunds, Marketplace settlements, and payouts in Admin Billing.
7. Review users, audit logs, and Marketplace review queue entries before approving commercial Marketplace exposure.

## Publisher Onboarding

1. Open `/marketplace/publish`.
2. Submit the agent package with visibility and pricing type.
3. Treat paid submissions as review-gated until approval and settlement evidence exists.
4. Use `/marketplace/my-agents` and publisher stats to inspect owned agents, install state, review status, revenue, platform fee, payout state, and refund impact.
5. Respond to governance events, takedowns, appeals, and abuse reports through the implemented Marketplace governance flow.

## Operator Onboarding

1. Copy environment variables from `config/.env.example` and replace placeholders outside git.
2. Keep `RELAY_ENABLED=true` for commercial operation.
3. Apply migrations with `go run ./cmd/migrate` from `src/server`.
4. Run local validation with `bash scripts/check.sh docs`, `bash scripts/check.sh`, and `bash scripts/test.sh` as appropriate for the environment.
5. Validate deployment with `bash scripts/deploy-validate.sh`.
6. Validate backup and restore with `bash scripts/backup-restore-smoke.sh` or the explicit backup/restore commands in `docs/release/backup-restore-runbook.md`.
7. Attach observability, alert, release, rollback, incident, and disaster recovery evidence from the v07 runbooks before production use.

## Current Completion Boundary

Phase 29 closes only `PROD-05` onboarding documentation alignment. Phase 30 must still prove signup, organization setup, provider/channel configuration, subscription, top-up, Chat, Agent, Knowledge, Marketplace, billing, deploy, backup, and restore as one or more end-to-end commercial journeys.

`no-final-readiness`: onboarding docs are not final commercial readiness evidence without Phase 30 journey verification and `AUDIT-01`.

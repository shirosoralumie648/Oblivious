# Incident Response Runbook

This runbook closes the v07 `OPS-06` incident response procedure for Phase 23 alerts and SLOs. Incident mitigation must preserve Relay-only provider access, quota/billing idempotency, webhook signature verification, and tenant isolation.

## Common Incident Rules

- Capture alert name, timestamp, owner, severity, dashboard panel, metric expression, command output, and affected environment class.
- Do not paste secrets, prompt text, response text, raw Stripe payloads, database URLs, customer data, kubeconfig material, or backup dump contents into incident records.
- If external Prometheus, Grafana, OpenTelemetry, or error-tracking vendor integrations are unavailable, record that explicitly. Repository-local artifacts alone are not live vendor proof.
- Roll back through `docs/release/release-rollback-runbook.md` when mitigation cannot restore safe operation quickly.
- Trigger disaster recovery through `docs/release/disaster-recovery-runbook.md` when data restoration or fresh infrastructure is required.

## Alert Triage Matrix

| Alert | Owner | Severity | Source metric | Dashboard/SLO reference |
| --- | --- | --- | --- | --- |
| `RelayOutage` | platform-ops | critical | `relay_route_decisions_total` | `docs/release/observability-slos.md`, Relay route decisions panel |
| `QuotaSettlementFailure` | billing-ops | critical | `quota_settlement_failures_total` | Quota settlement failures panel |
| `StripeWebhookFailure` | billing-ops | critical | `stripe_webhook_failures_total`, `stripe_webhook_events_total` | Stripe webhook failures panel |
| `MigrationFailure` | platform-ops | critical | `migration_runs_total` | Migration runs panel |
| `HighProviderErrorRate` | relay-ops | warning | `provider_failures_total`, `provider_request_duration_seconds_count` | Provider failures and latency panels |
| `TenantIsolationIncident` | security-ops | critical | `http_requests_total{status_class="4xx"}` plus tenant-isolation evidence | Tenant isolation signal panel |

## RelayOutage

First 15 minutes:

1. Confirm `/healthz` and `/metrics` with `BASE_URL=<target> bash scripts/deploy-smoke.sh`.
2. Inspect `relay_route_decisions_total` and recent `relay.route_decision` events.
3. Confirm `/v1/chat/completions` reaches local Relay auth/policy handling and does not return `404`.
4. Check provider/channel health and route policy config.

Mitigation:

- Keep all provider traffic through Relay; do not add direct provider SDK calls.
- Disable failing channels through approved channel config and rerun smoke.
- Roll back if the outage started with a release and route-policy behavior regressed.

Evidence:

- Alert output, deploy smoke output, route decision samples, changed config, and rollback decision.

## QuotaSettlementFailure

First 15 minutes:

1. Inspect `quota_settlement_failures_total` by stage.
2. Identify affected `billing_session_id` values from structured events without exposing customer content.
3. Verify idempotency keys before retrying preauthorization, settlement, or refund steps.

Mitigation:

- Do not manually credit or debit quota without append-only lifecycle evidence.
- Pause affected paid flows if settlement or refund idempotency is uncertain.
- Roll back the release if failures began after quota/billing changes.

Evidence:

- Metric sample, sanitized billing session IDs, idempotency checks, retry decision, and affected tenant count.

## StripeWebhookFailure

First 15 minutes:

1. Inspect `stripe_webhook_failures_total` by reason.
2. Confirm raw-body signature verification configuration and `stripe_webhook_events` ledger writes.
3. Check lifecycle application errors without logging raw webhook payloads.

Mitigation:

- Do not bypass Stripe signature verification.
- Retry idempotent ledger processing only after confirming event IDs and transition keys.
- Roll back if webhook ingestion or lifecycle behavior regressed with the release.

Evidence:

- Alert output, event IDs, sanitized error reason, retry result, and rollback decision.

## MigrationFailure

First 15 minutes:

1. Stop the release.
2. Preserve migration command output from `scripts/deploy-validate.sh` or `src/server/cmd/migrate`.
3. Compare `schema_migrations` filenames and checksums against checked-in migrations.
4. Confirm a pre-release backup manifest exists.

Mitigation:

- Do not continue app rollout after migration failure.
- Restore into a fresh database first with `BACKUP_FILE=<dump> RESTORE_DATABASE_URL=<fresh-url> bash scripts/restore-postgres.sh`.
- Roll back app images/config if migration did not modify production data. Trigger disaster recovery if data restoration is needed.

Evidence:

- Migration output, backup manifest path, restore verification, smoke output, and final go/no-go decision.

## HighProviderErrorRate

First 15 minutes:

1. Inspect `provider_failures_total` and `provider_request_duration_seconds`.
2. Identify provider/channel IDs and affected API type.
3. Confirm Relay-only provider access with `scripts/verify-relay-security.sh`.

Mitigation:

- Route traffic away from failing provider/channel using approved Relay channel controls.
- Keep quota settlement behavior intact for failed or partial calls.
- Roll back only if provider errors are caused by a release regression.

Evidence:

- Provider/channel IDs, failure reasons, route decision samples, relay-security output, and mitigation result.

## TenantIsolationIncident

First 15 minutes:

1. Inspect structured `http.request` events with `organization_id` and `user_id` when known.
2. Identify whether 4xx denial volume is expected abuse, auth drift, or possible cross-tenant access.
3. Run representative tenant-isolation tests or the relevant DB-backed integration suite before declaring containment.

Mitigation:

- Freeze suspicious organization access if needed.
- Do not weaken tenant filters, admin boundaries, or session checks to restore traffic.
- Trigger rollback if a release changed tenant scoping behavior.
- Escalate to disaster recovery only if data corruption or unauthorized writes are proven.

Evidence:

- Sanitized request IDs, tenant/user IDs, test output, access freeze record, communication log, and rollback/DR decision.

## Communication

- Owner posts initial status within 15 minutes for critical alerts and within 30 minutes for warning alerts.
- Include impact, affected surface, mitigation status, rollback/DR decision, and next update time.
- Preserve all evidence in the Phase 24 or release incident record with secret values redacted.

## Boundary

This runbook supports v07 Operations Gate closeout only after Phase 24 verification links it to current alert artifacts and command evidence. Final commercial readiness remains blocked by v08 Product Completeness and the final commercial audit.

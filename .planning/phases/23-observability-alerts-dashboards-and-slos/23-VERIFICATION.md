# Phase 23 Verification: Observability Alerts Dashboards and SLOs

## Scope

Phase 23 closes `OPS-04` and `OPS-05` only. It does not close `OPS-02`, `OPS-06`, `DOC-06`, v07 Operations Gate closeout, v08 Product Completeness, or final commercial readiness.

## Evidence Summary

| Requirement | Result | Evidence |
| --- | --- | --- |
| `OPS-04` | Passed | Shared observability primitives, structured HTTP/Relay/provider logs, Prometheus metrics, OpenTelemetry span hooks, and no-op-safe reporter primitives cover HTTP, Relay, billing lifecycle, quota settlement, Stripe webhooks, Marketplace settlement, task/job transitions, provider failures, and migration runs. |
| `OPS-05` | Passed | `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, and `docs/release/observability-slos.md` define alerts, dashboard panels, SLO thresholds, owners, severity, evidence commands, and Phase 24 runbook targets. |

## Commands Run

```bash
cd src/server && go test ./internal/observability -count=1
```

Result: passed.

```bash
cd src/server && go test ./internal/http ./internal/metrics -run 'Observability|Logging|Metrics|Request' -count=1
```

Result: passed.

```bash
cd src/server && go test ./internal/relay ./internal/relay/handler -run 'Observability|RouteDecision|ProviderFailure' -count=1
```

Result: passed.

```bash
cd src/server && go test ./internal/stripe ./internal/quota ./internal/marketplace ./internal/task ./cmd/migrate -run 'Observability|Metrics|Failure|Webhook|Settlement|Migration' -count=1
```

Result: passed.

```bash
cd src/server && go test ./internal/observability ./internal/http ./internal/metrics ./internal/relay ./internal/relay/handler ./internal/stripe ./internal/quota ./internal/marketplace ./internal/task ./cmd/migrate -count=1
```

Result: passed.

```bash
rg -n "RelayOutage|QuotaSettlementFailure|StripeWebhookFailure|MigrationFailure|HighProviderErrorRate|TenantIsolationIncident|OPS-04|OPS-05" deploy/observability docs/release/observability-slos.md docs/release/commercial-gates.md docs/release/rc-checklist.md
```

Result: passed. Output included all required alert names and OPS-04/OPS-05 references.

```bash
bash scripts/check.sh docs
```

Result: passed.

```bash
git diff --check
```

Result: passed.

## Code Evidence

- `src/server/internal/observability/observability.go`: structured JSON logging, sensitive field sanitization, OpenTelemetry `StartSpan`, and no-op/in-memory error reporter primitives.
- `src/server/internal/http/middleware.go`: `http.request` spans, structured JSON request event, request/tenant/user scope, `http_requests_total`, and `http_request_duration_seconds`.
- `src/server/internal/relay/handler/policy.go`: `relay.route_policy` spans, `relay.route_decision` structured events, and `relay_route_decisions_total`.
- `src/server/internal/relay/router.go`: `relay.route` and `relay.provider_call` spans, provider latency histogram, provider failure counter, and sanitized provider failure events.
- `src/server/internal/stripe/webhook.go`: `stripe.webhook` span, webhook success/failure metrics, invalid-signature failure metric.
- `src/server/internal/stripe/lifecycle.go`: `billing.lifecycle` span and `billing_lifecycle_events_total`.
- `src/server/internal/quota/service.go`: quota preauthorization, settlement, and refund spans plus `quota_settlement_failures_total` and billing lifecycle events.
- `src/server/internal/marketplace/settlement.go`: Marketplace paid-install, refund, payout spans and `marketplace_settlement_events_total`.
- `src/server/internal/task/service.go`: task/job lifecycle spans and `job_events_total`.
- `src/server/cmd/migrate/main.go`: `migration.apply` span and `migration_runs_total`.
- `src/server/internal/metrics/prometheus.go`: production metric definitions used by alert/dashboard/SLO artifacts.

## Alert, Dashboard, and SLO Evidence

- `deploy/observability/prometheus-alerts.yaml` defines:
  - `RelayOutage`
  - `QuotaSettlementFailure`
  - `StripeWebhookFailure`
  - `MigrationFailure`
  - `HighProviderErrorRate`
  - `TenantIsolationIncident`
- `deploy/observability/grafana-dashboard.json` defines the `Oblivious Production Operations` dashboard and panels for HTTP, Relay decisions, provider failures/latency, billing lifecycle, webhook failures, quota settlement failures, Marketplace settlement, migrations, task/job events, and tenant isolation signal.
- `docs/release/observability-slos.md` maps each signal to metrics, thresholds, owners, severity, evidence commands, and Phase 24 runbook targets.
- `scripts/verify-quality-gates.sh` now guards the observability artifacts and required metric/alert strings.

## Skipped Or Unavailable Checks

- No external Prometheus server evaluation was run.
- No Grafana import was run.
- No OpenTelemetry collector/exporter was configured.
- No external error-tracking vendor account was configured.

These are acceptable Phase 23 boundaries because the requirement is repository-local hooks/artifacts. Phase 24 must verify incident/runbook flow, and deployment-specific vendor provisioning remains outside this phase.

## Residual Work

- `OPS-02`: final normal/restricted release-path evidence remains Phase 24.
- `OPS-06`: release, rollback, incident response, and DR runbooks remain Phase 24.
- `DOC-06`: v07 closeout evidence and no-final-readiness boundary remain Phase 24.
- v08 Product Completeness remains required.
- Final commercial readiness remains unclaimed.

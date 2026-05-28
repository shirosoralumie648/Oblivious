# Phase 23 Context: Observability Alerts Dashboards and SLOs

## Milestone

v07 Production Operations.

## Why This Phase Exists

Phase 21 proved an equivalent production runtime can build, migrate, start, and smoke health, metrics, app, and Relay routes. Phase 22 proved PostgreSQL tenant-commercial data can be backed up and restored with migration-ledger integrity. The platform is still not operable by a production team unless failures are visible, measurable, traceable, alertable, and tied to dashboards plus SLOs.

This phase closes `OPS-04` and `OPS-05` only. It does not close release/rollback/incident/DR runbooks, v07 closeout, v08 product completeness, or final commercial readiness.

## Requirements

- **OPS-04:** Structured logs, Prometheus metrics, OpenTelemetry tracing hooks, and error-tracking integration points cover HTTP, Relay, billing, jobs, and provider failures.
- **OPS-05:** Alert rules, dashboards, and SLO definitions exist for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, and tenant isolation incidents.

## Current Evidence And Gaps

Existing assets:

- `src/server/internal/http/router.go` exposes `GET /metrics` through `promhttp.Handler()`.
- `src/server/internal/metrics/prometheus.go` defines Relay-oriented Prometheus helpers for request, duration, tokens, billing amount, channel health, channel latency, and rate-limit exceeded events.
- `src/server/internal/http/middleware.go` creates a request ID and logs method, path, status, duration, and request ID through `log.Printf`.
- `src/server/internal/relay/handler/policy.go` already models route class, route decision, tenant identity, request ID, and failure reason through `RouteAuditEvent`.
- `src/server/internal/relay/router.go` records channel success/failure in circuit breakers and owns selected channel/provider failure boundaries.
- `src/server/internal/stripe/webhook.go`, `src/server/internal/stripe/lifecycle.go`, `src/server/internal/quota/service.go`, and `src/server/internal/marketplace/*` hold billing and settlement transitions that need production visibility.
- `src/server/cmd/migrate/main.go` is the migration failure boundary used by deployment validation.

Current gaps:

- HTTP access logs are not structured JSON and do not consistently carry tenant, user, normalized route, latency milliseconds, error code, or component fields.
- Request/user/tenant context is not available as a reusable observability scope for downstream handlers and services.
- Existing metrics are Relay-specific and do not cover HTTP status/latency, route-policy decisions, billing lifecycle transitions, webhook failures, settlement failures, migration failures, task/job outcomes, or provider error rates.
- There are no OpenTelemetry tracing hooks or trace propagation helpers in `go.mod`.
- There is no error-reporting integration point that can be configured without requiring a live vendor account in tests.
- No alert-rule artifact exists for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, or tenant isolation incidents.
- No dashboard artifact maps HTTP, Relay, billing, Marketplace, migration, job, and provider signals into operator-facing panels.
- No SLO document defines objectives, burn alerts, owners, evidence commands, or runbook links.
- `scripts/verify-quality-gates.sh` does not yet guard observability artifacts.

## Implementation Decisions

### Observability Package

- Add a small `src/server/internal/observability` package rather than spreading direct Prometheus, logger, tracing, and error-reporter calls across unrelated services.
- Use the existing Prometheus dependency for metrics and keep `/metrics` on the default registry.
- Use the Go standard library `log/slog` for structured JSON logs so the server does not need a new logging dependency.
- Add OpenTelemetry API/SDK dependencies only where required for real tracing hooks; keep exporter configuration optional and no-op by default so automated tests and local smoke do not require a vendor endpoint.
- Add an error reporter interface with a no-op default and environment-configured webhook/DSN placeholders only. Tests must use an in-memory reporter.

### Field Contract

Observability events should use stable, low-cardinality fields:

- `component`: `http`, `relay`, `billing`, `marketplace`, `job`, `migration`, or `provider`.
- `event`: stable event name such as `http.request`, `relay.route_decision`, `billing.settlement_failed`, `stripe.webhook_failed`, `migration.failed`.
- `request_id`, `trace_id`, `span_id`.
- `organization_id` and `user_id` when known.
- `method`, `route`, `status`, `latency_ms`.
- `relay_route_class`, `relay_api_type`, `billing_policy`, `billing_session_id`, `channel_id`, `provider`, and `failure_reason` where applicable.
- Avoid full URL, email, API key, raw webhook payload, prompt content, response content, or other sensitive customer data.

### Alert And SLO Artifacts

- Add Prometheus alert rules under `deploy/observability/prometheus-alerts.yaml`.
- Add a Grafana-compatible dashboard JSON under `deploy/observability/grafana-dashboard.json`.
- Add `docs/release/observability-slos.md` mapping each alert to metric, threshold, owner, severity, dashboard panel, and runbook placeholder.
- Alert rules must rely on metrics emitted by the code or deployment scripts, not wishful future metric names.

### Phase Boundary

Phase 23 should create observability primitives and artifacts that Phase 24 runbooks can reference. Phase 24 owns release, rollback, incident response, DR runbooks, and v07 closeout. v08 still owns customer-facing product completeness.

## Verification Targets

Focused RED/GREEN:

- HTTP middleware tests should fail before structured access logs include request ID, normalized route, status, latency, and session tenant/user fields.
- Metrics tests should fail before HTTP, Relay decision, billing lifecycle, webhook, migration, job, and provider-failure metrics are registered and incrementable.
- Error reporter tests should fail before reporter implementations can capture component/event/failure reason without leaking secrets.
- Alert-rule tests or script checks should fail before required alert names and referenced metrics exist.

Broader phase verification:

- `cd src/server && go test ./internal/observability ./internal/http ./internal/metrics ./internal/relay ./internal/stripe ./internal/quota ./internal/marketplace -count=1`
- `cd src/server && go test ./cmd/migrate -count=1`
- `bash scripts/check.sh docs`
- `bash scripts/deploy-smoke.sh` against a running server should continue to prove `/metrics` is mounted.
- `git diff --check`

## Residual Risk After This Phase

Phase 23 should close only logs, metrics, tracing hooks, error-reporting integration points, alert rules, dashboards, and SLO definitions. The product remains non-commercial-complete until Phase 24 verifies release/rollback/incident/DR evidence and v08 completes customer-facing product behavior plus final commercial journeys.

---

*Phase: 23-observability-alerts-dashboards-and-slos*
*Context gathered: 2026-05-28 from current repository evidence*

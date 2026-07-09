# Relay Route Audit Sink Wiring Evidence

Date: 2026-07-02

Agent: Codex

Commit: pending

## Runtime Claim

HTTP server Relay construction now injects a `RouteAuditSink` backed by the configured request-log sink, so production route policy decisions can be persisted to `request_logs` instead of causing Files upload to fail with `relay_audit_sink_required` at the server wiring layer.

## Oblivious Files Changed

```text
src/server/internal/http/relay_route_audit_sink.go
src/server/internal/http/relay_route_audit_sink_test.go
src/server/internal/http/server.go
```

## Contract Changes

- `buildRelayConfig` passes `RouteAuditSink: newRelayRouteAuditRequestLogSink(currentRequestLogSink())`.
- The adapter maps `handler.RouteAuditEvent` to an `observability.Event` and writes it through `observability.WriteRequestLog`.
- Production still fails closed if `APP_ENV=production`, Relay is enabled, and `OBSERVABILITY_REQUEST_LOG_BACKEND=none`; this keeps the injected sink tied to the existing ClickHouse request-log requirement.

## Verification Commands

```text
command: go test ./internal/http -run "TestBuildRelayConfigInjectsRouteAuditSinkFromRequestLogSink|TestNewServerConfiguresClickHouseRequestLogSink|TestRouteSurface.*Admin" -count=1
result: passed

command: go test ./internal/relay ./internal/relay/handler -run "Test(ProductionFilesUpload|NewRelayFiles|RelayStore.*FileMapping|Audio|Moderations|Pricing)" -count=1
result: passed
```

## Runtime Evidence IDs

```text
event: relay.route_decision
request_log_metadata: relay_api_type
request_log_metadata: relay_route_class
request_log_metadata: relay_route_result
failure_code_closed_before_fix: relay_audit_sink_required
```

## Remaining Boundary

- This is repository-local wiring proof. It does not provide target ClickHouse row evidence.
- Standalone relay command still needs its own production-grade route audit sink if it is used as the commercial deployment surface outside the HTTP server.

# Observability Request Log Fail-Closed Refresh

Date: 2026-07-02

## Summary

The production request-log sink blocker is closed at configuration/startup level:

- `config.Load()` rejects `APP_ENV=production` with `RELAY_ENABLED=true` and `OBSERVABILITY_REQUEST_LOG_BACKEND=none` (`src/server/internal/config/config.go:514-527`).
- `OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse` requires `CLICKHOUSE_DSN` (`src/server/internal/config/config.go:522-524`).
- `configureRequestLogSink` opens and pings ClickHouse, then panics on failure in production (`src/server/internal/http/server.go:259-288`).
- `NewServer` installs the SQL request-log sink for HTTP request logging (`src/server/internal/http/server_test.go:326-363`).

This means production Relay no longer silently runs on the noop request-log sink.

## Verification

```text
go test ./internal/config ./internal/http ./internal/observability -run "TestLoad(ObservabilityClickHouseRequestLogConfig|RejectsClickHouseRequestLogWithoutDSN|RejectsProductionRelayWithoutRequestLogSink|RejectsProductionWithoutRelay)|Test(NewServerConfiguresClickHouseRequestLogSink|ConfigureRequestLogSinkPanicsInProductionWhenClickHouseUnavailable)|Test(WriteRequestLog|SQLRequestLogSink|ClickHouseRequestLogsMigration)" -count=1 -v
```

Result: PASS

## Remaining Gap

Commercial completion still requires target-runtime evidence from a real ClickHouse deployment showing:

- `request_logs` migration applied.
- A live request writes a request-log row.
- The row joins to Relay route decision, usage, quota/billing, and request ID evidence.
- Alert/SLO delivery consumes the same request-log/cost/error signal.

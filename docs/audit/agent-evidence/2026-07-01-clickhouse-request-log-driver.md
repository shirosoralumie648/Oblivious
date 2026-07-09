# ClickHouse Request Log Driver and Production Fail-Closed Startup

Date: 2026-07-01

## Scope

Production Relay deployments require `OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse`, but the server was opening the configured SQL driver without importing a ClickHouse `database/sql` driver. If the driver was missing or ClickHouse was unreachable, the server logged a warning and continued with no durable request-log sink.

## Changes

- Added `github.com/ClickHouse/clickhouse-go/v2 v2.47.0` to `src/server/go.mod`.
- Registered the ClickHouse SQL driver through a side-effect import in `src/server/internal/http/server.go`.
- Changed ClickHouse request-log sink initialization to:
  - open the configured driver,
  - `PingContext` the sink with a 5-second timeout,
  - panic in `APP_ENV=production` if open or ping fails,
  - keep warning-only behavior outside production.
- Added `TestConfigureRequestLogSinkPanicsInProductionWhenClickHouseUnavailable` to lock the production fail-closed behavior.

## Source Check

- `pkg.go.dev` reports `github.com/ClickHouse/clickhouse-go/v2` version `v2.47.0`, published 2026-06-26, and its README documents `database/sql` support over both TCP and HTTP transports.

## Verification

- `git diff --check` passed.
- 2026-07-01: `go test ./src/server/internal/http ./src/server/internal/config` could not run because `go` was not installed or not on `PATH`.
- 2026-07-02: `go test ./internal/config ./internal/http ./internal/observability -run "TestLoad(ObservabilityClickHouseRequestLogConfig|RejectsClickHouseRequestLogWithoutDSN|RejectsProductionRelayWithoutRequestLogSink|RejectsProductionWithoutRelay)|Test(NewServerConfiguresClickHouseRequestLogSink|ConfigureRequestLogSinkPanicsInProductionWhenClickHouseUnavailable)|Test(WriteRequestLog|SQLRequestLogSink|ClickHouseRequestLogsMigration)" -count=1 -v` passed.
- 2026-07-01: `curl` to `proxy.golang.org` and `sum.golang.org` failed in this Windows environment with SChannel credential errors, and the Node runtime was unavailable. Because of that, `go.sum` was not regenerated in that pass.

## Residual Risk

- A Go-enabled environment must run `go mod tidy` or the targeted Go tests to populate and verify the `go.sum` entries for `github.com/ClickHouse/clickhouse-go/v2 v2.47.0`.
- End-to-end ClickHouse evidence still requires a real ClickHouse instance with `request_logs` migration applied and a server smoke request proving persisted rows.

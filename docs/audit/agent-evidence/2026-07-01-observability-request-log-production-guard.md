# Agent Evidence: production Relay startup guards

Date: 2026-07-01

Agent: main

Commit: pending

## Runtime Claim

`config.Load()` now rejects `APP_ENV=production` when `RELAY_ENABLED=false`.

`config.Load()` also rejects `APP_ENV=production` with `RELAY_ENABLED=true` when `OBSERVABILITY_REQUEST_LOG_BACKEND=none`.

Together these guards prevent production deployments from using local demo AI fallbacks or silently dropping request-log evidence at configuration load time.

Historical note: this evidence originally treated standalone `src/server/cmd/relay` as non-production. A later deployment hardening pass wired `cmd/relay` for production with DB-backed channel loading, token auth, quota, pricing, rate limits, migrations, and durable usage recording. The current release contract is enforced by `scripts/verify_deployment_operations_contract.py`.

2026-07-02 update: ClickHouse connection health is now checked during `NewServer` startup. `configureRequestLogSink` opens the configured driver, pings ClickHouse, and panics in production when configuration or connectivity fails.

## Reference Inputs

```text
docs/audit/product-roadmap-v2-from-reference.md - P0 requires request log and usage evidence for billable calls.
src/server/internal/observability/request_log.go - current request log sink and Noop sink behavior.
src/server/internal/config/config.go - current request log backend config parsing.
src/server/internal/http/server.go - current ClickHouse sink wiring opens, pings, and fails closed in production.
src/server/cmd/relay/main.go - standalone relay command production wiring was later completed and is now covered by the deployment operations contract.
```

## Oblivious Files Changed

```text
config/.env.example
docs/architecture/current-system-contracts.md
src/server/internal/config/config.go
src/server/internal/config/config_test.go
src/server/cmd/relay/main.go
docs/audit/agent-evidence-template.md
docs/audit/parallel-agent-execution-plan.md
docs/audit/agent-evidence/2026-07-01-observability-request-log-production-guard.md
```

## Contract Changes

Production deployments must keep Relay enabled.

Production Relay deployments must configure a non-`none` request-log backend.

`OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse` still requires `CLICKHOUSE_DSN`.

The standalone `cmd/relay` entrypoint must remain commercially wired in production; `scripts/verify_deployment_operations_contract.py` now checks for DB, migration, quota, API-token, usage, and rate-limit wiring.

## Verification Commands

```text
command: git diff --check -- src/server/internal/config/config.go src/server/internal/config/config_test.go src/server/cmd/relay/main.go config/.env.example docs/architecture/current-system-contracts.md docs/audit/agent-evidence-template.md docs/audit/parallel-agent-execution-plan.md docs/audit/agent-evidence/2026-07-01-observability-request-log-production-guard.md
result: passed; Git reported LF-to-CRLF warnings only.

command: scripts/verify-openapi-contract.sh
result: blocked; Git Bash is available, but ruby is not on PATH. Error: scripts/verify-openapi-contract.sh: line 18: ruby: command not found.

command: go test ./internal/config -run 'TestLoad(ObservabilityClickHouseRequestLogConfig|RejectsClickHouseRequestLogWithoutDSN|RejectsProductionRelayWithoutRequestLogSink|RejectsProductionWithoutRelay)$' -count=1 -v
result: originally blocked in the 2026-07-01 environment. Re-run on 2026-07-02 with Go in PATH passed.

command: go test ./internal/http -run 'Test(NewServerConfiguresClickHouseRequestLogSink|ConfigureRequestLogSinkPanicsInProductionWhenClickHouseUnavailable)' -count=1 -v
result: passed on 2026-07-02.
```

## Runtime Evidence IDs

Not applicable; this is a startup configuration guard.

## Failure Evidence

The new unit test `TestLoadRejectsProductionWithoutRelay` exercises the local-demo fallback prevention path:

```text
APP_ENV=production
RELAY_ENABLED=false
```

Expected result: configuration load fails before the router can wire Chat/Agent to a local demo generator.

The new unit test `TestLoadRejectsProductionRelayWithoutRequestLogSink` exercises the request-log negative path:

```text
APP_ENV=production
RELAY_ENABLED=true
OBSERVABILITY_REQUEST_LOG_BACKEND=none
```

Expected result: configuration load fails before the server can start without request-log persistence.

## Unsupported / Deferred Surfaces

Target runtime evidence is still required to prove a real ClickHouse instance has the `request_logs` migration applied and receives rows joined to usage/billing records.

## Known Residual Risk

Commercial completion still requires target ClickHouse persistence proof, request-id joins across Relay/usage/billing/request logs, and the alert/SLO delivery loop.

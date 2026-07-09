# Observability Request Log Sink Failure Alert

Date: 2026-07-04

## Scope

This slice closes a request-log visibility gap: HTTP responses still survive a ClickHouse/request-log write failure, but the failure is no longer silent. It is routed into the existing alert and recovery evidence chain.

## Runtime Changes

- Added `observability.ComponentObservability` as a first-class component.
- Added `routeRequestLogSinkAlert` in HTTP middleware.
- Changed request logging middleware to route `WriteRequestLog` errors into alert delivery and recovery handling.
- Added the default server recovery policy `record-request-log-sink-failure`.

## RED Evidence

Command:

```bash
cd src/server && go test ./internal/http -run TestWithLoggingRoutesRequestLogSinkFailureToAlertAndRecovery -count=1 -v
```

Observed failure before implementation:

```text
internal/http/middleware_test.go:349:33: undefined: observability.ComponentObservability
internal/http/middleware_test.go:375:154: undefined: observability.ComponentObservability
FAIL	oblivious/server/internal/http [build failed]
```

This proved the observability failure component and request-log sink failure routing did not exist yet.

## GREEN Evidence

Focused command:

```bash
cd src/server && go test ./internal/http -run 'TestWithLoggingRoutesRequestLogSinkFailureToAlertAndRecovery|TestWithLoggingRequestLogSinkFailureDoesNotBreakResponse' -count=1 -v
```

Result:

```text
PASS
ok  	oblivious/server/internal/http	0.016s
```

Package command:

```bash
cd src/server && go test ./internal/http ./internal/observability -count=1 -timeout 300s
```

Result:

```text
ok  	oblivious/server/internal/http	1.608s
ok  	oblivious/server/internal/observability	0.017s
```

## Remaining Boundary

This slice makes request-log sink failure visible through repository-local alert and recovery stores. Target runtime proof still needs real ClickHouse outage/recovery evidence plus configured external alert delivery in the deployment environment.

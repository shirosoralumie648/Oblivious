# Observability HTTP Latency SLO Alert

Date: 2026-07-04

## Scope

This slice closes part of the observability SLO loop: HTTP request latency can now produce a first-class alert and recovery evidence record instead of only being emitted as request-log/metrics data.

## Runtime Changes

- Added a default HTTP latency SLO threshold in the HTTP middleware.
- Added `routeHTTPLatencySLOAlert` to route slow requests into the existing alert sink and recovery controller.
- Added `httpLatencySLOAlertEvent` with `slo`, `latency_ms`, `threshold_ms`, `request_id`, route, method, status, and source evidence fields.
- Added the default server recovery policy `record-http-latency-slo`, ordered before the generic HTTP warning policy so latency SLO alerts record `scale_out` recovery instead of generic restart recovery.
- Added `OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS` as a validated runtime setting for monolith and split-service config loaders.
- Added the latency SLO threshold to `.env.example` and the Kubernetes config map.
- Added target-release evidence requirements for `latencySLOTrigger`, `latencySLOAlertDelivery`, and `latencySLORecoveryAction` so final readiness cannot pass with only ClickHouse request-log proof.
- Tightened `assemble-target-release-evidence.sh` / `assemble_target_release_evidence.py` so these SLO fields require explicit target-run environment inputs instead of being auto-filled as `pass`.
- Kept `/healthz` and `/metrics` out of latency SLO alerting to avoid noisy health-check alerts.

## RED Evidence

Command:

```bash
cd src/server && go test ./internal/http -run TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery -count=1 -v
```

Observed failure before implementation:

```text
internal/http/middleware_test.go:470:22: undefined: setHTTPAlertLatencySLOThresholdForTest
FAIL	oblivious/server/internal/http [build failed]
```

This proved the middleware had no latency SLO threshold hook or alert routing path.

Server wiring RED command:

```bash
cd src/server && go test ./internal/http -run TestConfigureHTTPAlertingRoutesLatencySLOToSignedWebhookAndRecovery -count=1 -v
```

Observed failure before adding the default recovery policy:

```text
server_test.go:538: expected recorded latency SLO scale-out recovery action, got [{... PolicyName:record-http-5xx ... Type:restart ...}]
FAIL	oblivious/server/internal/http
```

This proved the runtime SLO event was being misclassified by the generic HTTP warning recovery policy.

Config RED command:

```bash
cd src/server && go test ./internal/config ./pkg/config ./internal/http -run 'TestLoadObservabilityHTTPAlertConfig|TestLoadRejectsInvalidObservabilityHTTPLatencySLOThreshold|TestLoadCommonObservabilityHTTPAlertConfig|TestLoadCommonRejectsInvalidObservabilityHTTPLatencySLOThreshold|TestConfigureHTTPAlertingRoutesLatencySLOToSignedWebhookAndRecovery' -count=1 -v
```

Observed failure before adding the config field and runtime wiring:

```text
cfg.ObservabilityHTTPLatencySLOThresholdMS undefined
unknown field ObservabilityHTTPLatencySLOThresholdMS in struct literal of type config.Config
FAIL
```

This proved the SLO threshold was still hardcoded and not commercially configurable.

Target evidence RED command:

```bash
bash scripts/verify-target-release-evidence-fixtures.sh
```

Observed failure before extending the verifier:

```text
[target-release-evidence-fixtures] missing-latency-slo-trigger-proof unexpectedly passed
```

This proved the final target-release evidence gate still allowed missing SLO trigger/delivery/recovery proof.

Target evidence assembler RED command:

```bash
bash scripts/assemble-target-release-evidence-fixtures.sh
```

Observed failure before requiring explicit SLO target inputs:

```text
[assemble-target-release-evidence-fixtures] missing required latency SLO proof unexpectedly passed
```

This proved the manifest assembler could still auto-fill SLO proof as `pass` without concrete target-run input.

## GREEN Evidence

Focused command:

```bash
cd src/server && go test ./internal/http -run TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery -count=1 -v
```

Result:

```text
--- PASS: TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery (0.00s)
PASS
ok  	oblivious/server/internal/http	0.012s
```

Server wiring command:

```bash
cd src/server && go test ./internal/http -run 'TestConfigureHTTPAlertingRoutesLatencySLOToSignedWebhookAndRecovery|TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery' -count=1 -v
```

Result:

```text
--- PASS: TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery (0.00s)
--- PASS: TestConfigureHTTPAlertingRoutesLatencySLOToSignedWebhookAndRecovery (0.00s)
PASS
ok  	oblivious/server/internal/http	0.018s
```

Config command:

```bash
cd src/server && go test ./internal/config ./pkg/config ./internal/http -run 'TestLoadObservabilityHTTPAlertConfig|TestLoadRejectsInvalidObservabilityHTTPLatencySLOThreshold|TestLoadCommonObservabilityHTTPAlertConfig|TestLoadCommonRejectsInvalidObservabilityHTTPLatencySLOThreshold|TestConfigureHTTPAlertingRoutesLatencySLOToSignedWebhookAndRecovery|TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery' -count=1 -v
```

Result:

```text
ok  	oblivious/server/internal/config	0.004s
ok  	oblivious/server/pkg/config	0.003s
ok  	oblivious/server/internal/http	0.037s
```

Target evidence fixture command:

```bash
bash scripts/verify-target-release-evidence-fixtures.sh
```

Result:

```text
[target-release-evidence-fixtures] rejected missing-latency-slo-trigger-proof
[target-release-evidence-fixtures] rejected failed-latency-slo-alert-delivery-proof
[target-release-evidence-fixtures] rejected failed-latency-slo-recovery-action-proof
[target-release-evidence-fixtures] target release evidence verifier behavior is guarded.
```

Target evidence assembler command:

```bash
bash scripts/assemble-target-release-evidence-fixtures.sh
```

Result:

```text
[assemble-target-release-evidence-fixtures] assembled and validated target evidence manifest
[assemble-target-release-evidence-fixtures] rejected missing required latency SLO proof
[assemble-target-release-evidence-fixtures] rejected invalid environment class
```

Package command:

```bash
cd src/server && go test ./internal/http ./internal/observability -count=1 -timeout 300s
```

Result:

```text
ok  	oblivious/server/internal/http	1.578s
ok  	oblivious/server/internal/observability	0.015s
```

## Remaining Boundary

This is repository-local SLO alert routing evidence. Commercial readiness still needs target runtime SLO policy configuration, real traffic/ClickHouse request-log proof, external alert delivery configuration, and recovery runbook evidence.

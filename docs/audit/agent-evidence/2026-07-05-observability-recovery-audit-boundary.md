# Observability Recovery Audit Boundary

Date: 2026-07-05

## Scope

This slice closes the local wording/runtime gap where "HTTP recovery" could be read as automatic infrastructure remediation. The runtime now exposes it as recovery audit: it records planned remediation actions for operators and explicitly states that no infrastructure mutation was executed.

## Runtime Changes

- Added preferred monolith env aliases `OBSERVABILITY_HTTP_RECOVERY_AUDIT_ENABLED` and `OBSERVABILITY_HTTP_RECOVERY_AUDIT_COOLDOWN_MS`.
- Added the same preferred env aliases to split-service common config.
- Kept legacy `OBSERVABILITY_HTTP_RECOVERY_ENABLED` and `OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS` as backward-compatible aliases.
- Annotated every `RecoveryAction.Reason` created by `RecoveryController` with `audit-only remediation recorded; no infrastructure mutation executed`.
- Updated `.env.example` and audit docs to describe recovery audit as an operator evidence record, not automatic remediation.

## RED Evidence

Monolith config RED command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/config -run TestLoadObservabilityHTTPRecoveryAuditConfig -count=1 -v
```

Observed failure before monolith config alias support:

```text
=== RUN   TestLoadObservabilityHTTPRecoveryAuditConfig
    config_test.go:661: expected HTTP recovery audit to be enabled
--- FAIL: TestLoadObservabilityHTTPRecoveryAuditConfig (0.00s)
FAIL
FAIL	oblivious/server/internal/config	0.002s
FAIL
```

Recovery controller RED command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/observability -run TestRecoveryControllerRecordsAuditOnlyRemediationReason -count=1 -v
```

Observed failure before runtime reason annotation:

```text
=== RUN   TestRecoveryControllerRecordsAuditOnlyRemediationReason
    recovery_test.go:92: expected audit-only remediation reason, got "HTTP 5xx threshold breached"
--- FAIL: TestRecoveryControllerRecordsAuditOnlyRemediationReason (0.00s)
FAIL
FAIL	oblivious/server/internal/observability	0.006s
FAIL
```

Split-service common config RED command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./pkg/config -run TestLoadCommonObservabilityHTTPRecoveryAuditConfig -count=1 -v
```

Observed failure before common config alias support:

```text
=== RUN   TestLoadCommonObservabilityHTTPRecoveryAuditConfig
    common_test.go:96: expected HTTP recovery audit to be enabled
--- FAIL: TestLoadCommonObservabilityHTTPRecoveryAuditConfig (0.00s)
FAIL
FAIL	oblivious/server/pkg/config	0.001s
FAIL
```

## GREEN Evidence

Monolith config GREEN command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/config -run 'TestLoadObservabilityHTTPRecoveryAuditConfig|TestLoadObservabilityHTTPAlertConfig|TestLoadRejectsInvalidObservabilityHTTPRecoveryCooldown' -count=1 -v
```

Result:

```text
--- PASS: TestLoadObservabilityHTTPAlertConfig (0.00s)
--- PASS: TestLoadObservabilityHTTPRecoveryAuditConfig (0.00s)
--- PASS: TestLoadRejectsInvalidObservabilityHTTPRecoveryCooldown (0.00s)
PASS
ok  	oblivious/server/internal/config	0.002s
```

Recovery controller GREEN command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/observability -run 'TestRecoveryControllerRecordsAuditOnlyRemediationReason|TestRecoveryControllerRecordsCriticalPolicyActionWithCooldown|TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts|TestRecoveryControllerMatchesPanicAndOOMRecoverySignals' -count=1 -v
```

Result:

```text
--- PASS: TestRecoveryControllerRecordsCriticalPolicyActionWithCooldown (0.00s)
--- PASS: TestRecoveryControllerRecordsAuditOnlyRemediationReason (0.00s)
--- PASS: TestRecoveryControllerSchedulesRestartBackoffAndExhaustsAfterFiveAttempts (0.00s)
--- PASS: TestRecoveryControllerMatchesPanicAndOOMRecoverySignals (0.00s)
PASS
ok  	oblivious/server/internal/observability	0.008s
```

Split-service common config GREEN command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./pkg/config -run 'TestLoadCommonObservabilityHTTPRecoveryAuditConfig|TestLoadCommonObservabilityHTTPAlertConfig' -count=1 -v
```

Result:

```text
--- PASS: TestLoadCommonObservabilityHTTPAlertConfig (0.00s)
--- PASS: TestLoadCommonObservabilityHTTPRecoveryAuditConfig (0.00s)
PASS
ok  	oblivious/server/pkg/config	0.001s
```

## Commercial Boundary

This closes the misleading local recovery naming/recording gap. It does not prove target-runtime observability completeness. Commercial release still needs target evidence that request logs, usage/billing joins, alert delivery, and recovery-audit records persist correctly in the deployed ClickHouse/Postgres environment.

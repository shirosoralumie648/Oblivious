# Secret Storage Audit Scanner

Date: 2026-07-04

## Runtime Claim

The migration package now has `AuditSecretStorage`, a reusable scanner for secret-bearing storage surfaces. It detects legacy plaintext and malformed protected secrets across Relay channels, publishing channel configs, Workflow definitions, Workflow execution snapshots, Workflow node execution payloads, and Observability alert provider configs without returning or logging secret values.

## Reference Inputs

```text
docs/audit/implementation-roadmap.md - secret plaintext migration/production deny policy remains a P0 release gate.
docs/audit/stub-hardcoded-todo-report.md - legacy secret rows still need target rotation/detection evidence.
src/server/internal/relay/store.go - Relay channels store api_key_encrypted using secretbox.
src/server/internal/channel/store.go - Publishing channel configs store secret-bearing JSON maps using secretbox.
src/server/internal/workflow/store.go - Workflow definitions store nested secret-bearing JSON maps using secretbox.
src/server/internal/workflow/store.go - Workflow execution snapshots and node input/context payloads store protected workflow secrets.
src/server/internal/observability/alert_provider_config_sql_store.go - Alert provider configs store secret-bearing maps using secretbox.
```

## Oblivious Files Changed

```text
src/server/internal/migration/secret_storage_audit.go
src/server/internal/migration/secret_storage_audit_test.go
docs/audit/agent-evidence/2026-07-04-secret-storage-audit-scanner.md
```

## Contract Changes

None for HTTP/API/database. Internal Go API adds `AuditSecretStorage(ctx, db)` and `SecretStorageFinding`.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/migration -run 'TestAuditSecretStorageFindsPlaintextAndInvalidStoredSecrets' -count=1 -v
result: RED before implementation; build failed because AuditSecretStorage and SecretStorageFinding did not exist.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/migration -run 'TestAuditSecretStorageFindsPlaintextAndInvalidStoredSecrets' -count=1 -v
result: pass after implementation.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/migration -run 'TestAuditSecretStorageFindsPlaintextAndInvalidStoredSecrets' -count=1 -v
result: RED after expanding expected coverage; scanner returned 3 findings instead of 7 because publishing channel, workflow snapshot, and workflow node payload surfaces were not included yet.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/migration -run 'TestAuditSecretStorageFindsPlaintextAndInvalidStoredSecrets' -count=1 -v
result: pass after adding channel_configs, workflow_executions.workflow_snapshot, and workflow_node_executions input/context scans.
```

## Runtime Evidence IDs

```text
tables: channels, channel_configs, workflows, workflow_executions, workflow_node_executions, observability_alert_provider_configs
paths: api_key_encrypted, config.secret, definition.webhook_secret, workflow_snapshot.webhook_secret, input.secret, context.secret, config.api_key
statuses: plaintext, invalid_protected
```

## Failure Evidence

The test fixture injects a plaintext Relay channel API key, an invalid protected Relay channel API key, a plaintext Workflow webhook secret, a plaintext publishing channel secret, a plaintext Workflow execution snapshot secret, and plaintext Workflow node input/context secrets. The scanner reports only rotation-required findings and does not include raw secret values in finding messages.

## Unsupported / Deferred Surfaces

- The scanner covers the main confirmed secretbox-backed storage surfaces but is not yet exposed through an operator CLI/admin endpoint.
- Target database sweep evidence remains follow-up hardening.

## Known Residual Risk

This is a reusable migration scanner with mock DB coverage. Commercial release still needs a target database run, rotation workflow, and operator-visible report/audit trail before marking the secret migration gate complete.

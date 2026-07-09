# Secretbox Legacy Secret Inspection

Date: 2026-07-04

## Runtime Claim

`secretbox` now exposes non-decrypting `InspectStored` and `InspectStoredMap` classification helpers for migration and operator audits. They identify empty, protected, legacy plaintext, and invalid protected stored secret values, including nested JSON/map configuration secrets, without returning or logging secret material.

## Reference Inputs

```text
docs/audit/implementation-roadmap.md - secret plaintext migration/production deny policy remains a P0 release gate.
docs/audit/stub-hardcoded-todo-report.md - legacy rows still need rotation/detection evidence.
src/server/internal/secretbox/secretbox.go - existing production plaintext rejection behavior inspected before implementation.
src/server/internal/workflow/store.go - workflow definitions store nested secret-bearing JSON maps.
src/server/internal/channel/store.go - publishing channel configs store secret-bearing JSON maps.
src/server/internal/observability/alert_provider_config_sql_store.go - alert provider configs store secret-bearing maps.
```

## Oblivious Files Changed

```text
src/server/internal/secretbox/secretbox.go
src/server/internal/secretbox/secretbox_test.go
docs/audit/agent-evidence/2026-07-04-secretbox-legacy-secret-inspection.md
```

## Contract Changes

None for HTTP/API/database. Internal Go API adds `SecretStorageStatus`, `SecretStorageInspection`, `InspectStored(domain, stored)`, and `InspectStoredMap(domain, payload, isSecretKey)`.

## Verification Commands

```text
command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -run 'TestInspectStoredClassifiesLegacyPlaintextWithoutReturningSecret' -count=1 -v
result: RED before implementation; build failed because InspectStored and status types did not exist.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -run 'TestInspectStoredClassifiesLegacyPlaintextWithoutReturningSecret' -count=1 -v
result: pass after implementation.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -run 'TestInspectStoredMapFindsNestedLegacySecretsWithoutLeakingValues' -count=1 -v
result: RED before implementation; build failed because InspectStoredMap did not exist.

command: cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/secretbox -run 'TestInspectStored(MapFindsNestedLegacySecretsWithoutLeakingValues|ClassifiesLegacyPlaintextWithoutReturningSecret)' -count=1 -v
result: pass after implementation.
```

## Runtime Evidence IDs

```text
domain: relay-channel-api-key
status_values: empty, protected, plaintext, invalid_protected
paths: webhook_secret, nodes[0].input.secret, nodes[1].input.secret
```

## Failure Evidence

The negative paths classify legacy plaintext and malformed protected payloads as needing rotation. Nested-map testing covers workflow-style paths such as `webhook_secret` and `nodes[*].input.secret`. The tests also assert inspection messages do not contain either the plaintext legacy secret or the decrypted protected secret.

## Unsupported / Deferred Surfaces

- This adds safe classification primitives for values and nested maps; it does not yet run a live SQL sweep by itself.
- Operator-facing migration status endpoints, rotation job wiring, and target database evidence remain release-gate work.

## Known Residual Risk

`InspectStored` validates protected payload shape but intentionally does not decrypt. A separate authenticated rotation workflow should still verify decryptability when rotating rows.

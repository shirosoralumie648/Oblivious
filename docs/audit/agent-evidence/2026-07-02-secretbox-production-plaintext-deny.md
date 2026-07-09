# Secretbox Production Plaintext Deny Evidence - 2026-07-02

## Claim

Repository-owned secret storage no longer accepts legacy unprotected plaintext secret values in production runtime paths.

## Runtime Behavior

- `secretbox.Protect` continues to encrypt supported stored secrets with AES-GCM and domain-bound associated data.
- `secretbox.Open` continues to read protected values in all environments.
- `secretbox.Open` continues to read legacy plaintext values outside production so operators can migrate old rows.
- `secretbox.Open` now rejects legacy plaintext values when `APP_ENV=production`, returning `ErrPlaintextSecretRejected` without returning the secret value.
- Production config now requires `OBLIVIOUS_SECRET_ENCRYPTION_KEY` to be explicitly set, at least 32 characters, non-default, and distinct from `SESSION_SECRET`.
- Production server startup now panics if Relay channel pool loading fails, so a legacy plaintext channel secret cannot silently degrade to an empty production Relay pool.
- Standalone `cmd/relay` now exits fatally if Relay channel pool loading fails.

## Covered Secret Domains

The fail-closed behavior applies to all current callers of `secretbox.Open`, including:

- Relay channel API keys.
- Publishing channel secret config.
- Observability alert-provider secret config.
- Workflow definition and execution secret payloads.

## Changed Files

- `src/server/internal/secretbox/secretbox.go`
- `src/server/internal/secretbox/secretbox_test.go`
- `src/server/internal/config/config.go`
- `src/server/internal/config/config_test.go`
- `src/server/pkg/config/common.go`
- `src/server/pkg/config/common_test.go`
- `src/server/internal/http/server.go`
- `src/server/internal/http/server_test.go`
- `src/server/cmd/relay/main.go`
- `scripts/verify_deployment_operations_contract.py`
- `config/.env.example`
- `docs/architecture/current-system-contracts.md`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/oblivious-gap-matrix.md`
- `docs/audit/stub-hardcoded-todo-report.md`
- `docs/audit/vertical-slice-gap-report.md`

## Verification

```bash
cd src/server
export PATH="/c/Program Files/Go/bin:$PATH"
go test ./internal/secretbox -count=1 -v
go test ./internal/config -run "TestLoadRejectsProduction|TestLoadAcceptsProduction" -count=1 -v
go test ./pkg/config -run "TestLoadCommon|TestServiceConfigsUseServiceDatabaseURLsInMicroservicesMode|TestLoadWorkflow" -count=1 -v
go test ./internal/http -run "TestRelayPoolConfigurationErrorPanicsInProduction|TestConfigureRequestLogSinkPanicsInProductionWhenClickHouseUnavailable" -count=1 -v
go test ./internal/relay -run "TestRelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey" -count=1 -v
go test ./internal/http -run "Test(ObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted|PublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers|WorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers)$" -count=1 -v
```

Result:

- `go test ./internal/secretbox -count=1 -v` passed.
- Production config, microservice common config, and server fail-fast tests passed.
- Relay and HTTP secret-response tests compiled and followed the existing local skip rule because `TEST_DATABASE_URL` is not set in this environment.

## Negative Path

`TestOpenRejectsLegacyPlaintextInProduction` sets `APP_ENV=production` and verifies an unprotected stored secret returns `ErrPlaintextSecretRejected` and an empty plaintext result.

`TestLoadRejectsProductionWeakSecretEncryptionKey` and `TestLoadCommonRejectsProductionWeakSecretEncryptionKey` verify production config rejects a missing, default, short, or session-key-reused at-rest encryption key.

`TestRelayPoolConfigurationErrorPanicsInProduction` verifies a production Relay pool load failure is not downgraded to a warning.

## Residual Risk

This closes the production runtime deny policy and startup fail-fast path. Commercial release still needs target-environment evidence that existing production-like databases contain only protected secret values, plus an operator runbook for rotating any legacy rows before enabling `APP_ENV=production`; the existing target release manifest requires a `secretAudit` artifact for that final proof.

# Observability Request Log Request-ID Join

Date: 2026-07-04

## Scope

Strengthened the Observability request-log slice so ClickHouse request rows carry an explicit `request_id` column that can be joined against Relay `usage_records.request_id` and billing evidence. Also tightened the target release manifest so commercial readiness cannot pass unless `requestLogObservability.requestUsageJoin` is `pass`.

## Changed Files

- `src/server/internal/observability/request_log.go`
- `src/server/internal/observability/request_log_test.go`
- `src/server/migrations/clickhouse/0001_request_logs.sql`
- `scripts/verify_target_release_evidence.py`
- `scripts/assemble_target_release_evidence.py`
- `scripts/target_release_fixture_mutations.py`
- `scripts/verify-target-release-evidence-fixtures.sh`
- `docs/release/rc-checklist.md`
- `docs/release/fusion-spec-evidence-pack.md`
- `docs/audit/current-implementation-depth.md`
- `docs/audit/oblivious-gap-matrix.md`
- `docs/audit/implementation-roadmap.md`

## Verification

- RED: `bash scripts/verify-target-release-evidence-fixtures.sh` failed because `failed-request-log-usage-join-proof` unexpectedly passed before verifier support.
- RED: `cd src/server && go test ./internal/observability -run 'TestRequestLogRowFromEventMapsRelayCompletion|TestSQLRequestLogSinkInsertsClickHouseRequestLogs|TestClickHouseRequestLogsMigrationMatchesSpec' -count=1` failed because `RequestLogRow.RequestID` and the ClickHouse `request_id` column did not exist.
- GREEN: `bash scripts/verify-target-release-evidence-fixtures.sh && bash scripts/assemble-target-release-evidence-fixtures.sh && python -m py_compile scripts/verify_target_release_evidence.py scripts/assemble_target_release_evidence.py scripts/target_release_fixture_mutations.py`
- GREEN: `cd src/server && go test ./internal/observability -run 'TestRequestLogRowFromEventMapsRelayCompletion|TestSQLRequestLogSinkInsertsClickHouseRequestLogs|TestClickHouseRequestLogsMigrationMatchesSpec' -count=1`

## Remaining Boundary

This is repository-local runtime and verifier hardening. Final commercial readiness still requires a target ClickHouse artifact proving ingest/query smoke and the `request_id` join from `request_logs` to usage/billing records.

## Admin API/UI Exposure Follow-Up

Added repository-local Admin visibility for the same join evidence:

- `AdminUsageLogEntry` now documents `requestLogEvidence` in `docs/api/openapi.yaml`.
- `AdminRequestLogEvidence` is covered by `scripts/verify_openapi_contract.py`, including request/log IDs, ClickHouse timing/cost/token fields, trace ID, and metadata.
- `src/web/src/types/admin.ts` now types `RequestLogEvidence`.
- `src/web/src/routes/admin/AdminUsageLogsPage.tsx` renders the joined ClickHouse request-log row beside each usage row when evidence is available.

Verification:

- RED: `cd src/web && npm test -- --run src/routes/admin/AdminUsageLogsPage.test.tsx` failed because the Usage Logs page did not render `requestLogEvidence`.
- RED: `bash scripts/verify-openapi-contract.sh` failed because `AdminUsageLogEntry.requestLogEvidence` and `AdminRequestLogEvidence` were undocumented.
- GREEN: `bash scripts/verify-openapi-contract.sh`
- GREEN: `cd src/web && npm test -- --run src/routes/admin/AdminUsageLogsPage.test.tsx`
- GREEN: `cd src/web && npm test -- --run src/features/admin/api.test.ts`
- GREEN: `cd src/web && npx tsc --noEmit`
- GREEN: `cd src/server && go test ./internal/admin ./internal/http ./internal/observability -run 'TestServiceListUsageLogsAttachesRequestLogEvidenceByRequestID|TestClickHouseUsageAnalyticsStore|TestAdminHandlerListsRelayProviderCatalog|TestAdminHandlerListsUsageLogs' -count=1 -v`

This makes the repository-owned usage-log API and Admin UI able to show ClickHouse join evidence, but it still does not replace the target-runtime ClickHouse ingest/query artifact required for commercial release.

## Coverage Summary Endpoint Follow-Up

Added an operator-facing coverage summary for the same request-log/usage join:

- Added `GET /api/v1/admin/billing/reconciliation/usage-request-logs`.
- The endpoint scans filtered `usage_records`, counts rows with and without `request_id`, joins non-empty request IDs to ClickHouse `request_logs`, and samples `missing_request_id` / `missing_request_log` issues.
- `NewServer` now wires the ClickHouse-backed request-log evidence store into Admin service when `OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse`; the same ClickHouse connection backs request-log writes and evidence lookups.
- OpenAPI and `docs/api/route-surface-manifest.json` now document `getAdminUsageRequestLogCoverage`.
- `src/web/src/features/admin/api.ts` exposes `getUsageRequestLogCoverage`, with typed frontend response models.

RED / GREEN evidence:

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin -run TestServiceGetUsageRequestLogCoverageSummarizesMissingRequestLogs -count=1 -v` failed with `service.GetUsageRequestLogCoverage undefined`.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin -run 'TestService(GetUsageRequestLogCoverageSummarizesMissingRequestLogs|ListUsageLogsAttachesRequestLogEvidenceByRequestID)' -count=1 -v` passed.
- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run TestAdminHandlerGetsUsageRequestLogCoverageWithFilters -count=1 -v` failed with `handler.getUsageRequestLogCoverage undefined`.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'Test(AdminHandlerGetsUsageRequestLogCoverageWithFilters|ConfigureRequestLogSinkReturnsClickHouseEvidenceStore|NewServerConfiguresClickHouseRequestLogSink|RouteSurfaceAdminRoutesRequireAdmin|RouteSurfaceManifestRoutesAreRegisteredWithoutDatabase|RouteSurfaceManifestAdminRoutesDispatchWithAdminSessionWithoutDatabase)' -count=1 -v` passed; `TestRouteSurfaceAdminRoutesRequireAdmin` skipped locally because `TEST_DATABASE_URL` was not set, while DB-free manifest and admin dispatch coverage passed.
- RED: `cd src/web && npm test -- --run src/features/admin/api.test.ts -t "serializes usage request-log coverage filters"` failed with `api.getUsageRequestLogCoverage is not a function`.
- GREEN: `cd src/web && npm test -- --run src/features/admin/api.test.ts -t "(serializes usage request-log coverage filters|serializes relay usage price reconciliation filters)"` passed.
- GREEN: `bash scripts/verify-openapi-contract.sh` passed.
- GREEN: `git diff --check -- src/server/internal/admin/usage_log_service.go src/server/internal/admin/types.go src/server/internal/admin/usage_log_service_test.go src/server/internal/http/admin_handler.go src/server/internal/http/router.go src/server/internal/http/admin_marketplace_handler_test.go src/server/internal/http/server.go src/server/internal/http/server_test.go src/server/internal/http/route_surface_test.go src/web/src/types/admin.ts src/web/src/features/admin/api.ts src/web/src/features/admin/api.test.ts docs/api/openapi.yaml docs/api/route-surface-manifest.json` passed.

Remaining boundary: this is still repository-local wiring and contract evidence. It proves the production code path can expose request-log/usage join coverage when ClickHouse is configured, but final commercial readiness still needs target-environment ClickHouse ingest/query artifacts and a real usage-to-request-log join sample from deployment data.

## Live Relay Route Join Follow-Up

Added repository-local runtime proof that the live Relay Chat path now carries the same metered request identity into the HTTP request log row and the Relay usage ledger:

- `Relay RequestLogScope` now carries request ID, organization/user identity, channel/provider, billing session, token counts, cost, price source, and immutable price snapshot from `Router.RouteWithBilling` back to HTTP middleware.
- HTTP middleware enriches the final Relay request-log event with Router settlement metadata before writing the request-log row.
- Request-log sanitization now keeps operational metering fields such as `request_tokens`, `response_tokens`, `total_tokens`, and preauthorization amounts while continuing to drop credentials/body/prompt/response fields.
- Added a route-level test that sends a real `/v1/chat/completions` request through `withRequestID -> withLogging -> Relay.Engine()`, uses a test upstream, records one usage ledger row, and verifies the request-log row joins to that usage record by `request_id`.

Changed files:

- `src/server/internal/relay/types/types.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/router_test.go`
- `src/server/internal/http/middleware.go`
- `src/server/internal/http/middleware_test.go`
- `src/server/internal/http/relay_alias_routes_test.go`
- `src/server/internal/observability/observability.go`

RED / GREEN evidence:

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/http -run TestWithLoggingEnrichesRelayRequestLogFromBillingScope -count=1 -v && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/relay -run TestRouterRouteWithBillingRecordsRequestLogBillingMetadata -count=1 -v` failed because `relay/types` had no request-log scope.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/http -run TestWithLoggingEnrichesRelayRequestLogFromBillingScope -count=1 -v && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/relay -run TestRouterRouteWithBillingRecordsRequestLogBillingMetadata -count=1 -v` passed.
- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/http -run TestRelayChatRequestLogJoinsUsageLedgerByRequestID -count=1 -v` first exposed `no_available_channel`, then missing prompt/completion prices, then the real join issue where HTTP request logs used the outer middleware request ID while Relay usage rows used `req_join_live`.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/http -run TestRelayChatRequestLogJoinsUsageLedgerByRequestID -count=1 -v` passed.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache OBLIVIOUS_REQUIRE_TEST_DATABASE=false go test ./internal/observability ./internal/relay ./internal/relay/handler ./internal/http -count=1 -timeout 300s` passed.
- GREEN: `git -C ../.. diff --check -- src/server/internal/http/middleware.go src/server/internal/http/middleware_test.go src/server/internal/http/relay_alias_routes_test.go src/server/internal/observability/observability.go src/server/internal/relay/router.go src/server/internal/relay/router_test.go src/server/internal/relay/types/types.go` passed.

Remaining boundary: this proves the live repository route path now creates joinable request-log and usage-ledger evidence for Relay Chat. Commercial release still requires target-environment ClickHouse ingest/query artifacts and deployment data proving the same join in the configured production observability backend.

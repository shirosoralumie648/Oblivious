# Relay Pricing Maintenance Run Ledger Evidence

Date: 2026-07-02

Agent: Codex

Commit: pending

## Runtime Claim

Relay pricing now has a repository-local maintenance run ledger for provider-source freshness sync and ledger-vs-price-snapshot reconciliation. The maintenance worker can fetch a configured LiteLLM HTTPS source on an interval, record failed or unchanged runs, create a pending import only when the provider source materially changes the active catalog, and record reconciliation runs with issue counts. Price changes still require explicit catalog import approval.

## Oblivious Files Changed

```text
src/server/migrations/0090_relay_pricing_sync_runs.sql
src/server/internal/admin/pricing_catalog_litellm.go
src/server/internal/admin/pricing_maintenance_worker.go
src/server/internal/admin/pricing_catalog_store.go
src/server/internal/admin/pricing_catalog_service.go
src/server/internal/admin/types.go
src/server/internal/config/config.go
src/server/internal/http/admin_handler.go
src/server/internal/http/router.go
src/server/internal/http/server.go
src/web/src/types/admin.ts
src/web/src/features/admin/api.ts
docs/api/openapi.yaml
docs/api/route-surface-manifest.json
```

## Contract Changes

- Database: adds `relay_pricing_sync_runs` with `freshness` and `reconciliation` jobs, `pending_import` / `unchanged` / `succeeded` / `issues_found` / `failed` states, source hash, import linkage, issue counts, and metadata.
- Worker: `RELAY_PRICING_MAINTENANCE_ENABLED=true` starts a server-managed maintenance worker when Relay is enabled; configuration requires a provider and HTTPS LiteLLM source URL.
- API: adds `GET /api/v1/admin/pricing/relay-catalog/sync-runs` for Admin inspection.
- Approval boundary: scheduled freshness creates pending imports only for material catalog diffs and never approves them automatically.

## Verification Commands

```text
command: go test ./internal/admin -run "Test(Service.*RelayPricingCatalog|ParseLiteLLM|RunRelayPricingCatalog|RelayPricingMaintenance|RecordRelayUsagePriceReconciliation)" -count=1
result: passed

command: go test ./internal/http -run "TestAdminHandler.*RelayPricing|TestRouteSurface.*Admin|TestRouteSurfaceDeclared|TestRouteSurfaceManifest|TestRouteSurfaceRuntime" -count=1
result: passed

command: go test ./internal/config -run "TestLoad(RelayPricingMaintenance|Defaults)|TestLoadRelayPricingMaintenance" -count=1
result: passed

command: go test ./cmd/migrate -count=1
result: passed

command: npm test -- --run src/features/admin/api.test.ts
result: passed, 1 test file and 26 tests
```

## Runtime Evidence IDs

```text
table: relay_pricing_sync_runs
api_route: GET /api/v1/admin/pricing/relay-catalog/sync-runs
worker_env: RELAY_PRICING_MAINTENANCE_ENABLED
audit_action: pricing.relay_catalog.sync.freshness_pending_import
audit_action: pricing.relay_catalog.sync.freshness_unchanged
audit_action: pricing.relay_catalog.reconciliation.run
```

## Unsupported / Deferred Surfaces

- Target-runtime provider export proof is still not collected.
- Target price freshness SLO proof is still not collected.
- Reconciliation remains ledger-vs-price-snapshot focused; checkout, webhook, quota, refund, payout, provider usage export, and request-log cross-system reconciliation breadth is still incomplete.
- No final no-skip commercial readiness claim is renewed by this repository-local worker and ledger evidence.

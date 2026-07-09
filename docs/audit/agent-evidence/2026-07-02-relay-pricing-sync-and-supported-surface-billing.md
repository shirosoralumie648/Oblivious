# Relay Pricing Sync And Supported Surface Billing Evidence

Date: 2026-07-02

Agent: Codex

Commit: pending

## Runtime Claim

Admin can parse a LiteLLM-style provider price JSON source into a pending Relay pricing catalog import with a SHA-256 `sourceHash`; approval remains the only path that mutates active `relay_pricing_entries`. Supported audio transcription, audio translation, and moderation routes now pass estimated usage into `RouteWithBilling` instead of settling with nil usage.

## Oblivious Files Changed

```text
src/server/internal/admin/pricing_catalog_litellm.go
src/server/internal/admin/pricing_maintenance_worker.go
src/server/internal/admin/pricing_catalog_service_test.go
src/server/internal/admin/service.go
src/server/internal/admin/types.go
src/server/internal/http/admin_handler.go
src/server/internal/http/router.go
src/server/internal/http/admin_marketplace_handler_test.go
src/server/internal/http/route_surface_test.go
src/server/internal/relay/handler/audio.go
src/server/internal/relay/handler/audio_test.go
src/server/internal/relay/handler/moderations.go
src/server/internal/relay/handler/moderations_test.go
src/server/internal/relay/pricing.go
src/server/internal/relay/pricing_test.go
src/server/migrations/0085_relay_pricing_entries.sql
src/server/migrations/0090_relay_pricing_sync_runs.sql
src/web/src/types/admin.ts
src/web/src/features/admin/api.ts
src/web/src/features/admin/api.test.ts
docs/api/openapi.yaml
docs/api/route-surface-manifest.json
```

## Contract Changes

- API: adds `POST /api/v1/admin/pricing/relay-catalog/sync`.
- Admin sync: accepts HTTPS `sourceUrl` or inline `sourceJson`, rejects invalid/non-HTTPS remote sources, computes `sha256:*`, parses recognized LiteLLM provider rows, checks required model coverage, records a sync run, and creates a pending import through the existing approval flow.
- Maintenance sync: `relay_pricing_sync_runs` records scheduled freshness and reconciliation runs; scheduled freshness creates pending imports only for material diffs and never auto-approves.
- Billing: audio STT, audio translation, and moderation handlers pass estimated usage into the billing router; default and seed SQL catalogs include audio seconds and moderation token prices for supported route startup coverage.

## Verification Commands

```text
command: go test ./internal/admin -count=1
result: passed

command: go test ./internal/http -run "Test(AdminHandler.*RelayPricing|RouteSurface.*Admin|RouteSurfaceDeclared|RouteSurfaceManifest|RouteSurfaceRuntime)" -count=1
result: passed

command: go test ./internal/relay ./internal/relay/handler -run "Test(Pricing|Audio|Moderations)" -count=1
result: passed

command: npm test -- --run src/features/admin/api.test.ts
result: passed, 1 test file and 26 tests

command: python scripts/verify_openapi_contract.py
result: passed

command: go test ./cmd/migrate -count=1
result: passed

command: go test ./internal/config -run "TestLoad(RelayPricingMaintenance|Defaults)|TestLoadRelayPricingMaintenance" -count=1
result: passed
```

## Runtime Evidence IDs

```text
api_route: POST /api/v1/admin/pricing/relay-catalog/sync
api_route: GET /api/v1/admin/pricing/relay-catalog/sync-runs
table: relay_pricing_sync_runs
audit_action: pricing.relay_catalog.sync.create
source_hash: sha256:*
pending_import_status: pending
catalog_dimensions: audio_seconds, prompt_tokens, total_tokens
```

## Unsupported / Deferred Surfaces

- Repository-local scheduled provider price maintenance wiring and run ledger are implemented.
- Target-environment price freshness proof is not collected.
- Cross-system reconciliation across provider usage exports, request logs, checkout, webhook, quota, refund, and payout systems is still incomplete beyond the ledger-vs-price-snapshot maintenance run.
- Batch, realtime, fine-tuning, assistants, threads, and runs remain disabled until their commercial lifecycles are implemented.

## Known Residual Risk

This closes the repository-local admin-triggered LiteLLM price-source import path, scheduled maintenance run ledger, and the nil-usage billing gap on supported audio/moderation routes. It does not prove live provider price freshness, target provider exports, broad cross-system reconciliation, or final no-skip commercial release readiness.

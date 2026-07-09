# Relay Pricing Catalog Import Approval Evidence

Date: 2026-07-02

Agent: Codex

Commit: pending

## Runtime Claim

Admins can create a pending Relay pricing catalog import from provider-source price rows, inspect the computed add/update/unchanged/deactivate diff, reject a bad pending import, approve a valid import to mutate `relay_pricing_entries` inside an audited SQL transaction, and generate a pending rollback import from an approved import when the active catalog has not been changed by a later import.

## Reference Inputs

```text
reference/sub2api/backend/resources/model-pricing/README.md:3 - local mirrored model pricing fallback pattern used as provider-source import input context
reference/sub2api/backend/resources/model-pricing/README.md:19 - remote-first and local-fallback pricing update flow used as future sync context
reference/sub2api/backend/internal/service/pricing_service_test.go:12 - model price JSON parsing with priority/service-tier fields used as normalization context
reference/sub2api/backend/internal/service/pricing_service_test.go:242 - provider model listing from parsed price data used as import filtering context
reference/litellm/ARCHITECTURE.md:233 - cost attribution flow used to keep runtime usage snapshots separate from catalog import approval
reference/litellm/ci_cd/check_files_match.py:11 - checked-in model price file parity pattern rejected as insufficient without admin approval/audit
```

## Oblivious Files Changed

```text
src/server/migrations/0089_relay_pricing_catalog_imports.sql
src/server/migrations/microservices/table-ownership.json
src/server/internal/admin/types.go
src/server/internal/admin/pricing_catalog_litellm.go
src/server/internal/admin/pricing_catalog_service.go
src/server/internal/admin/pricing_catalog_store.go
src/server/internal/admin/pricing_catalog_service_test.go
src/server/internal/http/admin_handler.go
src/server/internal/http/router.go
src/server/internal/http/admin_marketplace_handler_test.go
src/server/internal/http/route_surface_test.go
src/web/src/types/admin.ts
src/web/src/features/admin/api.ts
src/web/src/features/admin/api.test.ts
docs/api/openapi.yaml
docs/api/route-surface-manifest.json
docs/audit/agent-evidence/2026-07-02-relay-pricing-catalog-import-approval.md
```

## Contract Changes

- Database: adds `relay_pricing_catalog_imports` and `relay_pricing_catalog_events`.
- API: adds `GET /api/v1/admin/pricing/relay-catalog/imports`, `POST /api/v1/admin/pricing/relay-catalog/imports`, `POST /api/v1/admin/pricing/relay-catalog/sync`, `POST /api/v1/admin/pricing/relay-catalog/imports/{importId}/approve`, `POST /api/v1/admin/pricing/relay-catalog/imports/{importId}/reject`, and `POST /api/v1/admin/pricing/relay-catalog/imports/{importId}/rollback`.
- Billing: catalog approval updates active `relay_pricing_entries`; import, reject, rollback-draft creation, and per-entry mutation events retain actor and before/after payloads.
- Ownership: the new tables are assigned to the `relay` microservice boundary.

## Verification Commands

```text
command: go test ./internal/admin -count=1
result: passed

command: go test ./internal/http -count=1
result: passed

command: npm test -- --run src/features/admin/api.test.ts
result: passed, 1 test file and 26 tests

command: python scripts/verify_openapi_contract.py
result: passed
```

## Runtime Evidence IDs

```text
relay_pricing_catalog_import_id: rpci_*
relay_pricing_entry_id: rpe_*
audit_action: pricing.relay_catalog.import.create
audit_action: pricing.relay_catalog.sync.create
audit_action: pricing.relay_catalog.import.approve
audit_action: pricing.relay_catalog.import.reject
audit_action: pricing.relay_catalog.import.rollback.create
catalog_event_action: import_created
catalog_event_action: rejected
catalog_event_action: entry_added
catalog_event_action: entry_updated
catalog_event_action: entry_deactivated
```

## Failure Evidence

The service rejects malformed imports before persistence, including unsupported API types, unsupported pricing dimensions, missing model names for model-priced APIs, non-positive unit costs, duplicate catalog keys, missing provider, missing source, or empty entries. Approval and rejection fail closed when the import is missing or no longer pending. Rollback-draft creation fails closed when the source import is not approved or the active catalog key has changed after that source import.

## Unsupported / Deferred Surfaces

- Scheduled remote provider price fetch/sync is not implemented.
- Scheduled reconciliation across provider usage exports, request logs, checkout, webhook, quota, refund, and payout systems is not implemented.
- Target-environment live provider price import and approval evidence is not collected.

## Known Residual Risk

This closes the repository-local manual import, admin-triggered LiteLLM sync-to-pending-import, diff, reject, approval, rollback-draft, and event-audit loop. It does not prove provider price freshness, scheduled sync safety, scheduled exception queues, target-environment price freshness, or final no-skip commercial release readiness.

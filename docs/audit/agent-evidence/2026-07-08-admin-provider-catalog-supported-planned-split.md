# Admin Provider Catalog Supported/Planned Split Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: admin provider catalog must distinguish runtime-ready configurable providers from planned catalog entries so operators cannot configure providers that do not have a supported Relay adapter.

## Runtime Contract

- `src/server/internal/admin/channel_service.go` validates channel creation through the Relay provider catalog and rejects providers whose status is not `supported`.
- `src/server/internal/admin/channel_service.go` sets `configurable`, `installable`, and `runtimeReady` from catalog runtime readiness.
- `src/server/internal/http/admin_handler.go` keeps the existing flat `providers` response and now adds explicit `supportedProviders` and `plannedProviders` groups.
- `src/web/src/routes/admin/AdminChannelsPage.tsx` filters provider options to `status === "supported"` with installable and runtime-ready flags.
- `src/web/src/routes/admin/AdminChannelsPage.test.tsx` already covers excluding planned/catalog-only/runtime-beta providers from filters and channel forms.

## Verification

Server command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin ./internal/http -run 'Test(ValidateChannelProviderUsesRelayProviderCatalog|ListChannelProvidersMarksPlannedProvidersNotConfigurable|AdminHandlerListsRelayProviderCatalog)' -count=1 -v
```

Result: passed.

Frontend command:

```bash
npm --prefix src/web test -- src/routes/admin/AdminChannelsPage.test.tsx
```

Result: passed, 1 test file and 11 tests.

## Remaining Commercial Blockers

This closes the local API/UI split contract. Deployed-console completeness still requires target admin route smoke evidence against the release environment.

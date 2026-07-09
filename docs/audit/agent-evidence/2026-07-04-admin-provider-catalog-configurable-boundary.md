# Admin Provider Catalog Configurable Boundary

Date: 2026-07-04

## Scope

This slice prevents planned Relay providers from being mistaken for installable or runtime-ready Admin channel providers. The Admin provider catalog still exposes planned providers for roadmap visibility, but it now marks whether each provider is configurable.

## Runtime Changes

- Added `configurable` to `admin.ChannelProviderInfo`.
- `ListChannelProviders` now marks only `supported` providers as configurable.
- `validateChannelProvider` now rejects planned providers with `channel provider <id> is not configurable`.
- Admin Channels UI filters provider options by `status === "supported"` and `configurable !== false`, so planned providers do not appear in channel filters or create/edit forms.

## RED Evidence

Command:

```bash
cd src/server && go test ./internal/admin ./internal/http -run 'TestValidateChannelProviderUsesRelayProviderCatalog|TestListChannelProvidersMarksPlannedProvidersNotConfigurable|TestAdminHandlerListsRelayProviderCatalog' -count=1 -v
```

Observed failure before implementation:

```text
internal/admin/service_test.go:205:22: byID[provider].Configurable undefined (type ChannelProviderInfo has no field or method Configurable)
internal/admin/service_test.go:210:21: byID[provider].Configurable undefined (type ChannelProviderInfo has no field or method Configurable)
FAIL	oblivious/server/internal/admin [build failed]
```

This proved the catalog did not yet expose an explicit configurable boundary.

## GREEN Evidence

Focused backend command:

```bash
cd src/server && go test ./internal/admin ./internal/http -run 'TestValidateChannelProviderUsesRelayProviderCatalog|TestListChannelProvidersMarksPlannedProvidersNotConfigurable|TestAdminHandlerListsRelayProviderCatalog' -count=1 -v
```

Result:

```text
PASS
ok  	oblivious/server/internal/admin	0.009s
PASS
ok  	oblivious/server/internal/http	0.021s
```

Focused frontend command:

```bash
cd src/web && npm test -- --run src/routes/admin/AdminChannelsPage.test.tsx
```

Result:

```text
Test Files  1 passed (1)
Tests  11 passed (11)
```

## Remaining Boundary

This closes the local provider-catalog configurability boundary. It does not make planned providers runtime-ready; each planned provider still needs a supported adapter, auth model, pricing coverage, request logging, usage settlement, and target-runtime evidence before it can be marked configurable.

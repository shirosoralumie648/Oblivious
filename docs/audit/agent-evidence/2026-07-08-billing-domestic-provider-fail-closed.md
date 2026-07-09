# Billing Domestic Provider Fail-Closed Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: Alipay and WeChatPay must not be exposed as usable checkout providers unless they have a valid hosted checkout configuration, and production startup must reject insecure domestic payment configuration.

## Runtime Contract

- `src/server/internal/payment/provider.go` keeps domestic providers registered but unconfigured by default; unconfigured providers fail `Resolve` and are omitted from `AvailableProviders`.
- `src/server/internal/config/config.go` requires each production domestic provider to configure checkout URL and webhook secret together.
- `src/server/internal/config/config.go` now rejects non-HTTPS production domestic checkout URLs through `validateHTTPSURL`.
- `src/server/internal/http/payment_provider_config.go` now requires hosted domestic checkout base URLs to use HTTPS, include a host, omit embedded credentials, and omit secret-like query or fragment values.
- `src/server/internal/http/payment_provider_config.go` now validates a domestic checkout base URL before registering that provider as configured or adding its checkout creator.
- Console billing and marketplace detail responses expose only providers that are both registry-available and backed by checkout creators.
- Marketplace paid install rejects a configured provider without a checkout creator before settlement, so it cannot create pending paid orders against a provider that cannot start checkout.

## Verification

Focused command:

```bash
cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'Test(BuildPaymentCheckoutProvidersHidesInvalidDomesticHostedProviders|HostedCheckoutCreatorRejectsSecretBearingBaseURLs|ConsoleBillingPaymentProvidersExposeConfiguredCheckoutProviders|MarketplaceAgentDetailExposesOnlyConfiguredPaymentProviders|MarketplaceAgentDetailExposesConfiguredDomesticPaymentProviders|MarketplacePaidInstallCheckoutRejectsConfiguredProviderWithoutCheckoutCreatorBeforeSettlement|MarketplacePaidInstallCheckoutUsesSelectedProviderAndReturnsCheckoutSession)' -count=1 -v
```

Result: passed.

## Remaining Commercial Blockers

This closes the local hide/fail-closed contract, not the full payment rail. Commercial completion still requires target-runtime evidence for real Alipay/WeChatPay checkout creation, signed webhook ingestion, refund lifecycle, reconciliation, request-log linkage, and settlement/payout joins.

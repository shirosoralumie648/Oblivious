# Production Payment Provider Configuration

Date: 2026-07-01

## Scope

Commercial production startup must fail closed when billing checkout providers are missing or only partially configured. A deployment must not advertise or attempt checkout unless at least one provider has the runtime secrets, redirect URLs, and webhook verification material needed for lifecycle reconciliation.

## Changes

- Added production-only payment provider validation in `src/server/internal/config/config.go`.
- `APP_ENV=production` now requires at least one fully configured payment provider:
  - Stripe: `STRIPE_SECRET_KEY`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`, and `STRIPE_WEBHOOK_SECRET`.
  - Alipay: `ALIPAY_CHECKOUT_BASE_URL` and `ALIPAY_WEBHOOK_SECRET`.
  - WeChat Pay: `WECHATPAY_CHECKOUT_BASE_URL` and `WECHATPAY_WEBHOOK_SECRET`.
- Partial provider configuration fails startup with a provider-specific error before the server is constructed.
- Added config tests for:
  - no production payment provider,
  - partial Stripe configuration,
  - partial domestic payment configuration,
  - valid production Stripe configuration,
  - valid production domestic payment configuration.
- Updated production deployment examples:
  - `deploy/kubernetes/configmap.yaml` includes Stripe success/cancel redirect URLs.
  - `deploy/kubernetes/secret.example.yaml` includes Stripe secret and webhook secret placeholders.
  - `config/.env.example` documents the payment provider environment variables and production requirement.

## Verification

- `git diff --check` passed.
- `go test ./src/server/internal/config` could not run in this environment because `go` is not installed or not on `PATH`.

## Residual Risk

- This closes startup/configuration safety for production server launches through `config.Load()`.
- End-to-end payment provider verification still requires live or sandbox Stripe/domestic-provider credentials and webhook delivery in a deployment environment.

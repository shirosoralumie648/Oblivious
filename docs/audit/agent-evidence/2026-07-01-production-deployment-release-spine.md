# Production Deployment Release Spine

## Scope

This evidence note records release-readiness fixes for deployment/runtime gaps found during the commercial launch pass.

## ClickHouse Request Logs

- Added `deploy/kubernetes/clickhouse.yaml`.
- Provides:
  - `oblivious-clickhouse` Service on ports `8123` and `9000`.
  - Persistent storage for ClickHouse data.
  - ClickHouse Deployment using `clickhouse/clickhouse-server:24.8`.
  - `oblivious-clickhouse-migrations` ConfigMap with `request_logs` DDL.
  - `oblivious-clickhouse-migrate` Job that waits for ClickHouse, creates database `oblivious`, and applies `0001_request_logs.sql`.
- This closes the mismatch where production config selected `OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse` and `CLICKHOUSE_DSN=tcp://oblivious-clickhouse...`, but Kubernetes had no matching service or schema initialization path.

## Payment Provider Deployment Config

- Added Alipay and WeChat Pay checkout base URLs to `deploy/kubernetes/configmap.yaml`.
- Added `ALIPAY_WEBHOOK_SECRET` and `WECHATPAY_WEBHOOK_SECRET` placeholders to Kubernetes secret templates.
- Kept marketplace payout webhook provider config explicit:
  - `MARKETPLACE_PAYOUT_PROVIDER=webhook`
  - `MARKETPLACE_PAYOUT_WEBHOOK_URL`
  - `MARKETPLACE_PAYOUT_WEBHOOK_SECRET`

## Microservice Database Selection

- Added `DB_URL_RAG` and `DB_URL_BILLING` to `.env` and Kubernetes secret templates.
- Updated `src/server/pkg/config` so `LoadBillingConfig` and `LoadRAGConfig` use service-specific database URLs when `OBLIVIOUS_DB_MODE` is not `monolith`.
- Added tests covering billing/RAG service database selection.

## API Contract Coverage

- Added `Document.indexStatus`, `Document.indexError`, and `Document.indexedAt` to `docs/api/openapi.yaml`.
- Added marketplace payout inbound webhook path:
  - `POST /api/v1/billing/marketplace-payout/webhook`
- Added outbound marketplace payout webhook contract through `x-oblivious-outbound-webhooks.marketplacePayoutDispatch`.

## Verification

- `git diff --check` passes.
- Go tests and `gofmt` could not run because `go` and `gofmt` are not available in this environment.

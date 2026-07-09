# Quota Transactional Settlement And Refund

Date: 2026-07-01

## Commercial release blocker

Quota preauthorization had already been made atomic, but settlement and refund still updated `billing_sessions` and quota balances separately. If quota refunding failed after the session state update, the ledger could show a settled or refunded billing session while the quota balance remained reserved.

## Changes

- `SQLStore.SettleBillingSession` now runs in one transaction with `FOR UPDATE` on the billing session row.
- `SQLStore.RefundBillingSession` now runs in one transaction with `FOR UPDATE` on the billing session row.
- Both paths check `RowsAffected` for session state transitions.
- Partial settlement and full refund now update quota `balance` and reduce quota `used` in the same transaction.
- Settlement now rejects negative amounts and amounts greater than the preauthorized amount.
- Fake-store and SQL-store tests were extended to cover `used` ledger correction and over-preauthorization settlement rejection.

## Verification

- `git diff --check` is the available repository-level static check in this environment.
- `go test ./src/server/internal/quota` could not run because `go` is not installed on PATH.
- `gofmt` could not run because it is not installed on PATH.

## Residual risk

- Runtime verification still requires a local Go toolchain and a test PostgreSQL database for the SQL integration tests.

# Quota Atomic Preauthorization Evidence - 2026-07-01

## Scope

- Hardened Relay quota preauthorization against concurrent overspend.
- Kept the existing `quota.Store` contract stable by adding an optional SQL-only preauthorization path.
- Added regression coverage for invalid amounts, SQL idempotency, and concurrent reservation.

## Implementation

- `src/server/internal/quota/service.go`
  - `Service.PreConsume` now rejects non-positive preauthorization amounts before touching quota state.
  - SQL stores are detected through the optional `PreauthorizeBillingSession` capability.
  - `SQLStore.PreauthorizeBillingSession` inserts the billing session and reserves quota in one transaction.
  - Reservation updates `quotas` with `balance >= amount` in the `UPDATE` predicate, preventing two concurrent requests from both spending the same balance.
  - Duplicate `(organization_id, idempotency_key)` inserts are treated as idempotent retries and return the existing billing session.

- `src/server/internal/quota/service_test.go`
  - Added `TestPreConsumeRejectsNonPositiveAmount`.
  - Added `TestSQLStorePreConsumeIdempotencyDoesNotDoubleReserve`.
  - Added `TestSQLStorePreConsumeAtomicallyReservesQuota`.

## Verification

- Passed:
  - `git diff --check -- src/server/internal/quota/service.go src/server/internal/quota/service_test.go`

- Blocked by local toolchain:
  - `gofmt -w src\server\internal\quota\service.go src\server\internal\quota\service_test.go`
    - Failed because `gofmt` is not on PATH.
  - `go test ./internal/quota -run TestPreConsumeRejectsNonPositiveAmount -count=1 -v`
    - Failed because `go` is not on PATH.
  - `where go`, `where gofmt`, and `dir /s /b go.exe` did not find a local Go binary under the workspace.

## Residual Risk

- Settlement and refund still update the billing session and quota in separate statements. They should be made transactional before final commercial release.
- The full test suite still needs to run in an environment with Go installed and `TEST_DATABASE_URL` configured for SQL-backed quota tests.

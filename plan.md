1. **Fix `requestClientIP` in `src/server/internal/http/admin_handler.go`**
   - The current implementation extracts the left-most IP from the `X-Forwarded-For` header (`parts[0]`).
   - According to the system guidelines, this introduces an IP spoofing vulnerability. The right-most IP should be extracted (`parts[len(parts)-1]`), as it is appended by the trusted edge proxy.
   - Update `requestClientIP` to return `strings.TrimSpace(parts[len(parts)-1])`.

2. **Fix `clientIP` in `src/server/internal/http/tenant_handler.go`**
   - The current implementation extracts the left-most IP from the `X-Forwarded-For` header (`strings.Split(forwarded, ",")[0]`).
   - Update `clientIP` to return `strings.TrimSpace(parts[len(parts)-1])`.

3. **Update tests in `src/server/internal/http/admin_marketplace_handler_test.go`**
   - The tests currently assert that the `X-Forwarded-For` IP extracted is the left-most IP (e.g. `203.0.113.10` or `203.0.113.25`).
   - Update the assertions in `TestAdminHandlerUpdateUserQuotaValidatesAndAudits` and `TestAdminUserQuotaRoutePersistsWithPostgres` to expect the right-most IP (`198.51.100.2`).

4. **Update tests in `src/server/internal/http/admin_pricing_catalog_handler_test.go`**
   - The test `TestAdminPricingCatalogHandlerCreatesImport` asserts that the `X-Forwarded-For` IP extracted is the left-most IP (`203.0.113.10`).
   - Update the assertion to expect the right-most IP (`198.51.100.2`).

5. **Verify changes**
   - Run tests using `cd src/server && go test ./...`.

6. **Complete pre-commit steps**
   - Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.

7. **Create Sentinel Security Journal Entry**
   - Create or append to `.jules/sentinel.md` noting the critical finding related to `X-Forwarded-For` IP spoofing vulnerability when picking the left-most rather than the right-most IP.

8. **Submit changes**
   - Stage files, request code review, and commit with a PR message `🛡️ Sentinel: [CRITICAL] Fix IP spoofing vulnerability in X-Forwarded-For parsing`.

## 2024-10-10 - SQL Injection in Dynamic Identifier Interpolation
**Vulnerability:** String interpolation (`fmt.Sprintf`) was used directly to construct table and column names in SQL queries in migration scripts (`validator.go`, `secret_storage_audit.go`).
**Learning:** Even internal queries operating on structural metadata must quote table and column names to prevent SQL injection or query syntax errors from unexpected naming conventions (e.g., spaces or keywords). Do not use `pq.QuoteIdentifier()` as it introduces global driver side-effects.
**Prevention:** Implement and use a manual ANSI-standard double-quoting utility function for dynamic database identifiers across all migration packages.

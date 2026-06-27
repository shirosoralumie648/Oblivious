## 2025-02-14 - Fix SQL Injection in Migration Validator
**Vulnerability:** Direct string formatting (fmt.Sprintf) used for SQL table and column names instead of parameterized identifiers or proper escaping.
**Learning:** The 'database/sql' library in Go does not support parameterization for SQL object identifiers (e.g., table or column names). We must safely escape them in dynamic SQL queries to avoid SQL injection.
**Prevention:** Use pq.QuoteIdentifier() from 'github.com/lib/pq' to securely escape database identifiers before interpolating them with string formatting.

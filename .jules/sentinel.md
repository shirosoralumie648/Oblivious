## 2025-02-14 - Fix SQL injection in migration validator
**Vulnerability:** SQL injection vulnerability in `src/server/internal/migration/validator.go`. Table and column names were interpolated directly into the query strings.
**Learning:** Standard SQL parameterization does not apply to identifiers (e.g. table names, column names). These must be escaped or quoted correctly.
**Prevention:** Use `pq.QuoteIdentifier()` from `github.com/lib/pq` to safely escape variable table and column identifiers in PostgreSQL queries.

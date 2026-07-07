## 2025-02-27 - Fix SQL Injection in Migration Validator
**Vulnerability:** SQL Injection in dynamic query generation (`src/server/internal/migration/validator.go`) where table and column names were directly concatenated using `fmt.Sprintf` without escaping.
**Learning:** Even internal tooling or validators must treat identifiers as potentially unsafe if dynamically generated. Go does not have built-in identifier escaping in standard `database/sql`, so manual string replacing is necessary.
**Prevention:** Use a `quoteIdentifier` function to escape table and column names using standard ANSI SQL double-quoting (`"` and replacing interior quotes with `""`). Do not use `pq.QuoteIdentifier` as it registers the postgres driver globally.

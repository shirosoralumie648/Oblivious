## 2024-05-24 - SQL Injection risk via unquoted dynamic SQL identifiers in migration validators
**Vulnerability:** Dynamic SQL identifiers like table and column names were directly concatenated into queries using `fmt.Sprintf` without escaping in backend migration validators.
**Learning:** Do not use `pq.QuoteIdentifier()` from `github.com/lib/pq` to escape dynamic SQL identifiers, as importing it registers the Postgres driver globally as a side effect and breaks database agnosticism.
**Prevention:** Instead, use standard string manipulation (e.g., ANSI-standard double-quoting and `strings.ReplaceAll(identifier, "\"", "\"\"")`) to safely escape dynamic SQL identifiers like table or column names.

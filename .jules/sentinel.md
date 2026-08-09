
## 2024-08-09 - Fix SQL injection in migration tools
**Vulnerability:** SQL Injection risk due to dynamically constructing SQL queries using `fmt.Sprintf` with table/column strings.
**Learning:** `pq.QuoteIdentifier` from `github.com/lib/pq` should not be used in the Go backend. Instead, standard string manipulation (e.g., ANSI-standard double-quoting and `strings.ReplaceAll(identifier, "\"", "\"\"")`) must be used to safely escape dynamic SQL identifiers.
**Prevention:** Always use a custom quoting function like `quoteIdentifier` for dynamic table and column names in SQL queries.

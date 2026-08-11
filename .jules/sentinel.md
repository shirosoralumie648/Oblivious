
## 2024-08-11 - [SQL Injection in Dynamic Identifiers]
**Vulnerability:** SQL injection possible through unescaped dynamic table/column names in `fmt.Sprintf` queries inside `secret_storage_audit.go` and `validator.go`.
**Learning:** Go database drivers `Query` args only parameterize values, not identifiers. `github.com/lib/pq`'s `pq.QuoteIdentifier()` has the side effect of registering the driver globally.
**Prevention:** Use standard string manipulation (e.g., ANSI-standard double-quoting and `strings.ReplaceAll(identifier, "\"", "\"\"")`) to safely escape dynamic SQL identifiers to avoid SQLi without importing driver side effects.

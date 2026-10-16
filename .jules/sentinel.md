## 2026-10-16 - SQL Injection in schema auditing
**Vulnerability:** Found fmt.Sprintf directly interpolating table/column names in dynamic queries in schema migrations validation (validator.go, secret_storage_audit.go).
**Learning:** SQL injection can happen even in administrative routines where standard param placeholders ($1) cannot be used for identifiers.
**Prevention:** Quote identifiers when interpolating tables/columns dynamically: func quoteIdentifier(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"" }.

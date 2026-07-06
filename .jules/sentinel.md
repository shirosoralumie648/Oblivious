## 2025-07-06 - SQL Injection in Database Migrator
**Vulnerability:** SQL injection vulnerability in migration validator where dynamic table names and primary key columns were being directly interpolated into raw SQL query strings (`fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)`).
**Learning:** Avoid using `pq.QuoteIdentifier()` to safely quote dynamic SQL identifiers as importing it causes `github.com/lib/pq` to register the Postgres driver globally, introducing side effects and breaking database agnosticism.
**Prevention:** Securely escape dynamic SQL identifiers manually (e.g., using ANSI-standard double quoting: `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`) to prevent injection without importing specific drivers.

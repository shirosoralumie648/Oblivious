## 2026-08-10 - SQL Injection via unescaped identifiers in fmt.Sprintf
**Vulnerability:** SQL Injection in dynamic queries using `fmt.Sprintf` for table and column names (`fmt.Sprintf("SELECT %s FROM %s", column, table)`).
**Learning:** Using `fmt.Sprintf` directly for SQL identifiers (table names, column names) is unsafe because standard parameterized queries (e.g., `$1`, `?`) do not support identifiers. We cannot use `pq.QuoteIdentifier` because it has unintended side-effects (registering the Postgres driver globally).
**Prevention:** Always escape dynamic SQL identifiers using a custom ANSI-standard double-quoting helper function: `` `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"` ``.

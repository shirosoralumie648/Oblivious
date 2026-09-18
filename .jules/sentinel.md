## 2025-02-27 - SQL Injection Prevention in Migrations
**Vulnerability:** SQL injection vulnerability in migration validation queries using `fmt.Sprintf` directly with unquoted dynamic inputs (`tableName`, `pkColumn`, `spec.ID`, etc).
**Learning:** Even internal tooling and migration scripts must use proper identifier quoting. Standard database/sql parameterization (e.g. `$1`) cannot be used for table or column names, meaning manual quoting is necessary. The memory explicitly prevents using `pq.QuoteIdentifier` as it breaks db-agnosticism.
**Prevention:** Always manually quote identifiers using ANSI-standard double-quoting and replacing internal quotes (`""" + strings.ReplaceAll(s, "\"", "\"\"") + """`) for dynamic table or column names.

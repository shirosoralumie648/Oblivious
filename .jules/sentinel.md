## 2023-10-27 - [High] Fix SQL Injection via dynamically provided table and column identifiers
**Vulnerability:** SQL Injection in `validator.go` and `secret_storage_audit.go` due to unsafe string formatting of table and column names (`query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)`).
**Learning:** Concatenating dynamically provided identifiers directly into queries (using `%s`) is an injection vulnerability that `database/sql` arguments don't prevent. Go `database/sql` parameterization (`$?`) only escapes values, not table or column identifiers.
**Prevention:** If an identifier needs to be dynamically provided, always wrap it with ANSI-standard double-quoting and escape internal quotes to prevent injection using `` `"` + strings.ReplaceAll(s, `"`, `""`) + `"` ``.

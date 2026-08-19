## 2026-08-19 - Fix SQL Injection in Migration Validators
**Vulnerability:** SQL injection vulnerability in migration validation queries using `fmt.Sprintf` directly with table/column names.
**Learning:** Migration schemas often concatenate table names. If names contain special characters or spaces without quoting, it can lead to SQL syntax errors or injections.
**Prevention:** Always quote identifiers (tables, columns) using ANSI-standard double-quoting and replacing internal quotes when dynamically building SQL queries.

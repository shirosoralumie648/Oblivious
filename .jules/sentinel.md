## 2024-10-06 - Fix SQL injection in migration validators
**Vulnerability:** SQL injection vulnerability in migration validation queries caused by dynamic unquoted string interpolation of identifiers.
**Learning:** Using `fmt.Sprintf` to build raw SQL queries with dynamically provided table or column names without properly escaping them introduces SQL injection vulnerabilities.
**Prevention:** Use a standard identifier quoting mechanism (e.g., ANSI-standard double quoting) for dynamic database table and column names before inserting them into raw queries.

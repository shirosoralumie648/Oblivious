
## 2024-09-10 - Fix SQL Injection in Migration Queries
**Vulnerability:** Dynamic table and column names were directly interpolated into SQL queries via `fmt.Sprintf` without quoting or validation, leading to SQL injection risk.
**Learning:** Raw string interpolation for table or column names breaks database security. Go's database/sql does not support parameterizing identifiers.
**Prevention:** Manually quote identifiers using ANSI-standard double-quoting and replacing internal quotes, e.g. using `strings.ReplaceAll(s, "\"", "\"\"")`.

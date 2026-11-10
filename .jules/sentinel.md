
## 2026-11-10 - Quote dynamically injected SQL identifiers
**Vulnerability:** SQL injection risks in internal database migration tools when taking table and column names dynamically in raw query string construction (`fmt.Sprintf("SELECT %s FROM %s", ...)`).
**Learning:** Even internal queries that pull from structs rather than user request payloads should be explicitly defended using proper SQL identifier quoting.
**Prevention:** Avoid dynamic table names, but if required, wrap table and column variables using double quotes and escape internal quotes to ensure they are parsed as literal SQL identifiers (`"` + strings.ReplaceAll(name, `"`, `""`) + `"`).

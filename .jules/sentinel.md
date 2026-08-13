## 2024-08-13 - Fix SQL Injection in dynamic query identifiers
**Vulnerability:** SQL Injection in dynamic identifiers (table names, column names) used in `fmt.Sprintf` database queries.
**Learning:** Using `fmt.Sprintf` directly for identifiers exposes the codebase to SQL injection because identifiers cannot be parameterized. Standard escaping practices using double-quotes and string replacement must be used. Also, don't use `pq.QuoteIdentifier` since it imports the `pq` Postgres driver globally and breaks database agnosticism.
**Prevention:** Always escape dynamic table or column names by wrapping them in double quotes and escaping inner quotes (`strings.ReplaceAll(identifier, "\"", "\"\"")`).

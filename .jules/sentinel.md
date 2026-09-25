## 2025-02-24 - SQL Injection in Dynamic Queries
**Vulnerability:** SQL Injection in migration validator and secret audit where user-provided table/column names are directly concatenated into `fmt.Sprintf("SELECT ... FROM %s")`.
**Learning:** Found dynamic SQL construction lacking standard ANSI double-quoting and inner quote escaping, allowing injection via specially crafted table/column names. Also learned that `github.com/lib/pq`'s `pq.QuoteIdentifier()` shouldn't be used to prevent driver lock-in. Instead, identifiers must be manually double-quoted with inner quotes replaced by two double quotes (`""`).
**Prevention:** Always quote dynamic identifiers (tables, columns) manually in `fmt.Sprintf` calls, specifically when constructing dynamic schemas.

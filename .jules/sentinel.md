## 2025-05-18 - [Fix SQL injection in dynamic identifier formatting]
**Vulnerability:** SQL injection risks were identified where table names and column names were directly injected into SQL queries via `fmt.Sprintf` in `billing_store.go` and `migration/validator.go`.
**Learning:** SQL injection can occur not only via unsanitized values in `WHERE` clauses but also when dynamically building table or column names, which cannot be parameterized using standard `$1` placeholders in PostgreSQL.
**Prevention:** Always use `pq.QuoteIdentifier()` from `github.com/lib/pq` to safely escape and quote any dynamic SQL identifiers (such as table names, column names) in Go programs interacting with PostgreSQL.

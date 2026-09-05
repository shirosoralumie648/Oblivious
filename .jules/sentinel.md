## 2024-05-15 - ANSI SQL Quoting Without External Driver Dependency
**Vulnerability:** SQL injection risks in internal migration utilities using `fmt.Sprintf` directly on table/column names.
**Learning:** Relying on `github.com/lib/pq`s `pq.QuoteIdentifier()` breaks database agnosticism and has global driver side effects. Go applications need a reliable internal utility.
**Prevention:** Create an internal `quoteIdentifier` using ANSI-standard double-quoting and replacing internal quotes: `\"\"\" + strings.ReplaceAll(s, \"\\"\", \"\\"\\"\") + \"\"\"`.

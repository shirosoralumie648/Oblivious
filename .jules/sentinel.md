## 2025-02-24 - [SQL Injection] Fix dynamic table name injection in migration validator
**Vulnerability:** Found SQL injection vulnerabilities in src/server/internal/migration/validator.go where tableName and pkColumn variables are directly interpolated into string format expressions without proper escaping.
**Learning:** Even internal tooling or migration validators are susceptible to SQL injection. Using fmt.Sprintf directly with dynamic input without escaping exposes a vulnerability.
**Prevention:** Always escape dynamic identifiers (tables, columns) using ANSI-standard double quoting if parameterization isn't available for structural parts of a SQL query.

## 2025-02-24 - [Go vet] Fix deferred time.Since evaluation in metrics
**Vulnerability:** Found an issue in src/server/internal/knowledge/service.go where time.Since(startedAt).Seconds() was called directly in a defer statement (defer metrics.ObserveRAGRetrievalLatency(options.Mode, time.Since(startedAt).Seconds())), causing it to be evaluated immediately when the defer statement was registered, not when the function returned.
**Learning:** In Go, arguments to deferred functions are evaluated at the time the defer statement is executed, not when the deferred function is called.
**Prevention:** Wrap metrics calls that calculate duration in a closure: defer func() { metrics.ObserveRAGRetrievalLatency(options.Mode, time.Since(startedAt).Seconds()) }().

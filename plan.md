1. **Analyze `src/server/internal/migration/secret_storage_audit.go` and `src/server/internal/migration/validator.go`**:
   - Both files have a SQL injection vulnerability where `fmt.Sprintf` is used to directly concatenate table or column names into queries. For example, `fmt.Sprintf("SELECT %s, %s FROM %s", spec.ID, spec.Column, spec.Table)` and `fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)`.

2. **Fix `src/server/internal/migration/validator.go`**:
   - Create a function `escapeSQLIdentifier(identifier string) string` in this file.
   - It will return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"` to safely escape identifiers.
   - Use this to escape `tableName` and `pkColumn`.

3. **Fix `src/server/internal/migration/secret_storage_audit.go`**:
   - Also create `escapeSQLIdentifier(identifier string) string` in this file, or move it to a shared place in `migration` package. Since they're in the same package, we can define it once in `validator.go`.
   - Update `auditScalarSecretRows` and `auditJSONSecretRows` to escape `spec.Table`, `spec.ID`, and `spec.Column`.

4. **Update the mock test `secretAuditDB` in `src/server/internal/migration/secret_storage_audit_test.go`**:
   - Since the SQL queries will now have double quotes, the test's query matcher `strings.Contains(query, "FROM "+table)` will fail because `table` might be `"channels"` instead of `channels`.
   - Add a check for `strings.Contains(query, "FROM \""+table+"\"")`.

5. **Run tests**:
   - `cd src/server && go test ./...`
   - Pre-commit instructions.

package migration

import "strings"

func quoteIdentifier(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
}

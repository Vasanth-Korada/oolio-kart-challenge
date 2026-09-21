// Package migrations embeds the SQL migration files so cmd/server can
// apply them at startup without needing a separate migration tool or
// filesystem access to the repo layout at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

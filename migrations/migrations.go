// Package migrations embeds the SQL migration files so cmd/server can
// apply them at startup without a separate migration tool.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// Package migrations embeds the SQL schema migrations, so the server binary
// carries them.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

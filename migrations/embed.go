// Package migrations embeds the SQL migration files into the binary, so the
// migrate command works without the .sql files present on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

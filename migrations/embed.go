// Package migrations embeds the SQL schema migrations that internal/store
// applies at Open. Files are named NNNN_name.sql; NNNN is the target
// PRAGMA user_version after the file has been applied.
package migrations

import "embed"

// FS holds every *.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS

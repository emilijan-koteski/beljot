// Package migrations embeds the SQL migration files so the api binary can apply
// them at start (internal/dbmigrate) and no separate migrate image is needed.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

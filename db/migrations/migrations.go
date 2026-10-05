// Package migrations embeds the SQL migrations, so Go code can apply them with golang-migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
